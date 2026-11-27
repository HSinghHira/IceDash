package apps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HSinghHira/IceDash/internal/compose"
	"github.com/HSinghHira/IceDash/internal/notify"
	"github.com/HSinghHira/IceDash/internal/zima"
)

type fakeDocker struct {
	mu      sync.Mutex
	cs      map[string][]Container
	digests map[string][]string
	have    map[string]bool
	pulls   []string
	pullErr error
	csErr   error // container reads fail
}

func (d *fakeDocker) Containers(_ context.Context, p string) ([]Container, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.csErr != nil {
		return nil, d.csErr
	}
	return append([]Container{}, d.cs[p]...), nil
}

func (d *fakeDocker) ImageDigests(_ context.Context, id string) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.digests[id], nil
}

func (d *fakeDocker) HasImage(_ context.Context, ref string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.have[ref], nil
}

func (d *fakeDocker) Pull(_ context.Context, ref string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pulls = append(d.pulls, ref)
	return d.pullErr
}

func (d *fakeDocker) set(p string, c ...Container) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cs[p] = c
}

const digest = "mostafawahied/autoxpose@sha256:1111111111111111111111111111111111111111111111111111111111111111"

// autoSetup has AutoXpose 0.6.0 installed and running, with IceStore at 0.6.1.
// Installs change the installed copy: a pinned file is 0.6.0, any other 0.6.1.
func autoSetup(t *testing.T) (*Manager, *fakeZima, *notes, *fakeDocker) {
	t.Helper()
	m, z, n := newManager(t)
	z.apply = func(b []byte) string {
		if strings.Contains(string(b), "@sha256:") {
			return "0.6.0"
		}
		return "0.6.1"
	}
	d := &fakeDocker{cs: map[string][]Container{}, digests: map[string][]string{"sha256:old": {digest}}, have: map[string]bool{}}
	d.set("autoxpose", Container{ID: "c1", Service: "autoxpose", ImageID: "sha256:old", State: "running"})
	m.Docker, m.Steady = d, 20*time.Millisecond
	old, _ := os.ReadFile("../compose/testdata/autoxpose.yml")
	doc, _ := compose.Parse(old)
	_ = doc.Apply(compose.Choices{Values: map[string]string{"autoxpose/SERVER_IP": "198.51.100.7", "autoxpose/LAN_IP": "192.168.68.17"}})
	b, _ := doc.Bytes()
	z.installed["autoxpose"] = zima.Installed{Name: "autoxpose", StoreID: "com.icestore.autoxpose", Version: "0.6.0", Title: "AutoXpose", Compose: jsonOf(t, b)}
	z.grid = []zima.Grid{{Name: "autoxpose", Status: "running"}}
	return m, z, n, d
}

func info(t *testing.T, m *Manager) Info {
	t.Helper()
	l, err := m.List(context.Background(), "")
	if err != nil || len(l) != 1 {
		t.Fatalf("list %v %v", l, err)
	}
	return l[0]
}

func lastNote(n *notes) notify.Notification {
	l := n.list()
	if len(l) == 0 {
		return notify.Notification{}
	}
	return l[len(l)-1]
}

func TestUpdateKeepsVersionAndGoesBack(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	if i := info(t, m); i.Back != "" || !i.Update {
		t.Fatalf("before %+v", i)
	}
	if _, err := m.Rollback(ctx, "autoxpose"); !errors.Is(err, ErrNoPrevious) {
		t.Errorf("rollback without a kept version: %v", err)
	}
	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, m, "autoxpose"); j.State != "succeeded" {
		t.Fatalf("update %+v", j)
	}
	// The new image was pulled first; the old version is kept, pinned, 0600.
	if len(d.pulls) != 1 || d.pulls[0] != "mostafawahied/autoxpose:0.6.0" {
		t.Errorf("pulls %v", d.pulls)
	}
	kept := filepath.Join(filepath.Dir(m.State), "app-versions", "autoxpose.yml")
	fi, err := os.Stat(kept)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("kept file %v %v", fi, err)
	}
	if b, _ := os.ReadFile(kept); !strings.Contains(string(b), digest) || !strings.Contains(string(b), "SERVER_IP=198.51.100.7") {
		t.Errorf("kept file:\n%s", b)
	}
	if i := info(t, m); i.Back != "0.6.0" || i.Version != "0.6.1" {
		t.Fatalf("after update %+v", i)
	}

	// The user changes a setting on 0.6.1, then goes back: the change stays.
	in := z.installed["autoxpose"]
	cur, _ := compose.Parse(in.Compose)
	cur.SetEnv("autoxpose", "LAN_IP", "10.0.0.9")
	cb, _ := cur.Bytes()
	in.Compose = jsonOf(t, cb)
	z.installed["autoxpose"] = in
	d.have[digest] = true
	if _, err := m.Rollback(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, m, "autoxpose"); j.State != "succeeded" || j.Kind != "rollback" {
		t.Fatalf("rollback %+v", j)
	}
	got := string(z.installs[len(z.installs)-1])
	if !strings.Contains(got, digest) || !strings.Contains(got, "LAN_IP=10.0.0.9") || !strings.Contains(got, "SERVER_IP=198.51.100.7") {
		t.Errorf("rollback file:\n%s", got)
	}
	i := info(t, m)
	if i.Version != "0.6.0" || i.Back != "" || i.Held != "0.6.1" || i.HeldBy != "user" {
		t.Errorf("after rollback %+v", i)
	}
	if l := lastNote(n); l.Kind != notify.KindAppRolledBack || l.Title != "AutoXpose is back on 0.6.0" {
		t.Errorf("note %+v", l)
	}
	if _, err := os.Stat(kept); !os.IsNotExist(err) {
		t.Error("kept file not removed after going back")
	}
	// Updating by hand to the held version clears the hold.
	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, "autoxpose")
	if i := info(t, m); i.Held != "" || i.Back != "0.6.0" {
		t.Errorf("after updating again %+v", i)
	}
	// Uninstall forgets it all.
	_ = m.SetAuto(ctx, "autoxpose", true)
	_ = m.Uninstall(ctx, "autoxpose", false)
	f := m.load()
	if len(f.Auto) != 0 || len(f.Prev) != 0 {
		t.Errorf("after uninstall %+v", f)
	}
}

func TestGoBackFailsSafelyWhenImageGone(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, "autoxpose")
	installs := len(z.installs)
	d.pullErr = errors.New("manifest unknown")
	if _, err := m.Rollback(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	j := waitJob(t, m, "autoxpose")
	if j.State != "failed" || !strings.Contains(j.Error, "can't be downloaded any more") || len(z.installs) != installs {
		t.Errorf("job %+v, installs %d → %d", j, installs, len(z.installs))
	}
	if l := lastNote(n); l.Kind != notify.KindAppRollbackFailed {
		t.Errorf("note %+v", l)
	}
	if i := info(t, m); i.Back != "0.6.0" {
		t.Errorf("a failed go back lost the kept version: %+v", i)
	}
}

func TestFailedPullChangesNothing(t *testing.T) {
	m, z, n, d := autoSetup(t)
	d.pullErr = errors.New("couldn't download")
	if _, err := m.Update(context.Background(), "autoxpose"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, m, "autoxpose"); j.State != "failed" || len(z.installs) != 0 {
		t.Errorf("job %+v installs %d", j, len(z.installs))
	}
	if l := lastNote(n); l.Kind != notify.KindAppUpdateFailed {
		t.Errorf("note %+v", l)
	}
	if i := info(t, m); i.Back != "" {
		t.Errorf("%+v", i)
	}
}

// 03:00 on 2026-11-27 in Auckland.
var night = time.Date(2026, 11, 26, 14, 0, 0, 0, time.UTC)

const zone = "Pacific/Auckland"

func TestNightUpdateHealthy(t *testing.T) {
	m, z, n, _ := autoSetup(t)
	ctx := context.Background()
	m.NightStep(ctx, zone, night)
	if len(z.installs) != 0 {
		t.Fatal("updated without auto on")
	}
	// Auto on: the "is in IceStore" notice stays away, the night installs it.
	if err := m.SetAuto(ctx, "autoxpose", true); err != nil {
		t.Fatal(err)
	}
	m.CheckUpdates(ctx)
	if len(n.list()) != 0 {
		t.Errorf("notes %+v", n.list())
	}
	m.NightStep(ctx, zone, night.Add(-3*time.Hour)) // 00:00: not yet
	if len(z.installs) != 0 {
		t.Fatal("updated outside the window")
	}
	m.NightStep(ctx, zone, night)
	if j := waitJob(t, m, "autoxpose"); j.State != "succeeded" {
		t.Fatalf("job %+v", j)
	}
	if l := lastNote(n); l.Kind != notify.KindAppUpdated || l.Title != "AutoXpose was updated overnight" {
		t.Errorf("note %+v", l)
	}
	if len(m.load().Pending) != 0 {
		t.Error("pending left")
	}
	// Once a night only.
	in := z.installed["autoxpose"]
	in.Version = "0.6.0"
	z.installed["autoxpose"] = in
	m.NightStep(ctx, zone, night.Add(time.Minute))
	if len(z.installs) != 1 {
		t.Errorf("installs %d", len(z.installs))
	}
}

func TestNightUpdateBrokenGoesBackAndHolds(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	_ = m.SetAuto(ctx, "autoxpose", true)
	d.have[digest] = true
	// After the update the new container keeps failing its healthcheck.
	z.after = func() {
		d.set("autoxpose", Container{ID: "c2", Service: "autoxpose", ImageID: "sha256:new", State: "running", Health: "unhealthy"})
	}
	m.NightStep(ctx, zone, night)
	j := waitJob(t, m, "autoxpose")
	// The old version is applied, but the fake container stays unhealthy.
	if j.Kind != "rollback" || j.State != "failed" {
		t.Errorf("job %+v", j)
	}
	if len(z.installs) != 2 || !strings.Contains(string(z.installs[1]), digest) {
		t.Fatalf("installs %d", len(z.installs))
	}
	i := info(t, m)
	if i.Held != "0.6.1" || i.HeldBy != "night" || i.Version != "0.6.0" {
		t.Errorf("after %+v", i)
	}
	if l := lastNote(n); l.Kind != notify.KindAppRollbackFailed || !strings.Contains(l.Title, "isn't running") {
		t.Errorf("note %+v", l)
	}
	// Next night the held version isn't tried again.
	m.NightStep(ctx, zone, night.Add(24*time.Hour))
	if len(z.installs) != 2 {
		t.Errorf("held version installed again")
	}
}

func TestNightRollbackComesUp(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	_ = m.SetAuto(ctx, "autoxpose", true)
	d.have[digest] = true
	installs := 0
	z.after = func() {
		installs++
		if installs == 1 {
			d.set("autoxpose", Container{ID: "c2", Service: "autoxpose", ImageID: "sha256:new", State: "restarting", Restarts: 5})
		} else {
			d.set("autoxpose", Container{ID: "c3", Service: "autoxpose", ImageID: "sha256:old", State: "running"})
		}
	}
	m.NightStep(ctx, zone, night)
	if j := waitJob(t, m, "autoxpose"); j.Kind != "rollback" || j.State != "succeeded" {
		t.Fatalf("job %+v", j)
	}
	if l := lastNote(n); l.Kind != notify.KindAppRolledBack || l.Title != "AutoXpose went back to 0.6.0" {
		t.Errorf("note %+v", l)
	}
	if i := info(t, m); i.Held != "0.6.1" || i.HeldBy != "night" {
		t.Errorf("%+v", i)
	}
}

func TestNightSkips(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	_ = m.SetAuto(ctx, "autoxpose", true)
	z.grid = []zima.Grid{{Name: "autoxpose", Status: "exited"}}
	m.NightStep(ctx, zone, night)
	if i := info(t, m); i.NightSkip != SkipStopped || len(z.installs) != 0 {
		t.Errorf("stopped: %+v", i)
	}
	// No digest for its image: it can't keep a way back, so it doesn't update.
	z.grid = []zima.Grid{{Name: "autoxpose", Status: "running"}}
	d.digests = map[string][]string{}
	m.NightStep(ctx, zone, night.Add(24*time.Hour))
	if i := info(t, m); i.NightSkip != SkipNoCopy || len(z.installs) != 0 {
		t.Errorf("no copy: %+v", i)
	}
	if l := lastNote(n); l.Title != "AutoXpose can't update by itself" {
		t.Errorf("note %+v", l)
	}
	// Lasting reason: the "is in IceStore" notice shows again.
	m.CheckUpdates(ctx)
	if l := lastNote(n); l.Kind != notify.KindAppUpdateAvailable {
		t.Errorf("note %+v", l)
	}
	// Non-IceStore apps can't be switched on.
	z.installed["ha"] = zima.Installed{Name: "ha", StoreID: "org.icewhale.ha"}
	var ie *InputError
	if err := m.SetAuto(ctx, "ha", true); !errors.As(err, &ie) {
		t.Errorf("non-IceStore: %v", err)
	}
}

// A restart during the check picks it up again from apps.json.
func TestNightResumesAfterRestart(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	_ = m.SetAuto(ctx, "autoxpose", true)
	m.NightStep(ctx, zone, night)
	waitJob(t, m, "autoxpose")
	// As if IceDash stopped during the check of the next update.
	m.mu.Lock()
	m.load().Pending["autoxpose"] = &Pending{Name: "autoxpose", Title: "AutoXpose", Kind: "update", Night: true, Phase: "checking", From: "0.6.0", To: "0.6.1"}
	m.save()
	m.mu.Unlock()
	m2 := &Manager{Catalog: m.Catalog, Zima: z, Notify: n, State: m.State, Docker: d, Steady: m.Steady, Poll: time.Millisecond}
	m2.Resume(ctx)
	if j := waitJob(t, m2, "autoxpose"); j.State != "succeeded" {
		t.Fatalf("resumed job %+v", j)
	}
	if len(m2.load().Pending) != 0 || info(t, m2).Back != "0.6.0" {
		t.Errorf("after resume %+v", m2.load())
	}
}

// Review round 1.

// readerZima reads the installed file as zima and casa do; err makes it fail.
type readerZima struct {
	*fakeZima
	err error
}

func (r *readerZima) ComposeFile(_ context.Context, name string) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.installed[name].Compose, nil
}

type countLock struct {
	sync.Mutex
	n int
}

func (l *countLock) Lock() { l.Mutex.Lock(); l.n++ }

// 1. Without a Docker socket a manual update still installs; a night one doesn't.
func TestNoDockerSocket(t *testing.T) {
	m, z, _, d := autoSetup(t)
	d.pullErr = fmt.Errorf("%w: dial unix /var/run/docker.sock: connect: no such file", ErrNoDocker)
	if _, err := m.Update(context.Background(), "autoxpose"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, m, "autoxpose"); j.State != "succeeded" || len(z.installs) != 1 {
		t.Errorf("manual: job %+v, installs %d", j, len(z.installs))
	}

	m, z, _, d = autoSetup(t)
	var lock countLock
	m.NightLock = &lock
	d.pullErr = fmt.Errorf("%w: dial unix", ErrNoDocker)
	_ = m.SetAuto(context.Background(), "autoxpose", true)
	m.NightStep(context.Background(), zone, night)
	if j := waitJob(t, m, "autoxpose"); j.State != "failed" || len(z.installs) != 0 {
		t.Errorf("night: job %+v, installs %d", j, len(z.installs))
	}
	if lock.n != 1 {
		t.Errorf("night lock taken %d times", lock.n)
	}
	// A real download error still stops a manual update.
	m, z, _, d = autoSetup(t)
	d.pullErr = errors.New("couldn't download x: manifest unknown")
	_, _ = m.Update(context.Background(), "autoxpose")
	if j := waitJob(t, m, "autoxpose"); j.State != "failed" || len(z.installs) != 0 {
		t.Errorf("download error: job %+v, installs %d", j, len(z.installs))
	}
}

// 2. A night update whose task fails after changing the app goes back and is held.
func TestNightFailedChangeIsHeld(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	_ = m.SetAuto(ctx, "autoxpose", true)
	d.have[digest] = true
	z.task = zima.Task{State: "failed", Error: "container exited"}
	installs := 0
	z.after = func() {
		installs++
		if installs == 1 {
			d.set("autoxpose", Container{ID: "c2", Service: "autoxpose", ImageID: "sha256:new", State: "exited", ExitCode: 1})
			return
		}
		z.task = zima.Task{State: "succeeded"}
		d.set("autoxpose", Container{ID: "c3", Service: "autoxpose", ImageID: "sha256:old", State: "running"})
	}
	m.NightStep(ctx, zone, night)
	if j := waitJob(t, m, "autoxpose"); j.Kind != "rollback" || j.State != "succeeded" {
		t.Fatalf("job %+v", j)
	}
	if i := info(t, m); i.Held != "0.6.1" || i.HeldBy != "night" {
		t.Errorf("not held: %+v", i)
	}
	if l := lastNote(n); !strings.Contains(l.Body, "didn't finish") || !strings.Contains(l.Body, "0.6.1 won't install by itself again") {
		t.Errorf("note %+v", l)
	}
	for k := 1; k <= 3; k++ {
		m.NightStep(ctx, zone, night.Add(time.Duration(k)*24*time.Hour))
	}
	if len(z.installs) != 2 {
		t.Errorf("tried again on later nights: %d installs", len(z.installs))
	}
}

// 3. Stopping IceDash during a resumed check is no verdict: nothing changes.
func TestResumeStopsCleanlyOnShutdown(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, "autoxpose")
	notesBefore := len(n.list())
	d.set("autoxpose", Container{ID: "c2", Service: "autoxpose", ImageID: "sha256:new", State: "running", Health: "unhealthy"})
	m.mu.Lock()
	m.load().Pending["autoxpose"] = &Pending{Name: "autoxpose", Title: "AutoXpose", Kind: "update", Night: true, Phase: "checking", From: "0.6.0", To: "0.6.1"}
	m.save()
	m.mu.Unlock()
	m2 := &Manager{Catalog: m.Catalog, Zima: z, Notify: n, State: m.State, Docker: d, Steady: time.Second, Poll: time.Millisecond}
	sctx, stop := context.WithCancel(ctx)
	m2.Resume(sctx)
	time.Sleep(30 * time.Millisecond)
	stop()
	time.Sleep(50 * time.Millisecond)
	m3 := &Manager{Catalog: m.Catalog, Zima: z, State: m.State}
	f := m3.load()
	if f.Pending["autoxpose"] == nil || len(f.Held) != 0 || f.Prev["autoxpose"].From != "0.6.0" {
		t.Errorf("state changed on shutdown: %+v", f)
	}
	if _, _, err := m3.readFiles("autoxpose"); err != nil {
		t.Errorf("kept files gone: %v", err)
	}
	if len(n.list()) != notesBefore || len(z.installs) != 1 {
		t.Errorf("notes %d → %d, installs %d", notesBefore, len(n.list()), len(z.installs))
	}
}

// 5 and 6. With a reader, an unreadable installed file means no copy before
// an update, and an error (nothing applied) on Go back.
func TestUnreadableInstalledFile(t *testing.T) {
	m, z, _, _ := autoSetup(t)
	r := &readerZima{fakeZima: z}
	m.Zima = r
	ctx := context.Background()
	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, "autoxpose")
	if i := info(t, m); i.Back != "0.6.0" {
		t.Fatalf("with a working reader: %+v", i)
	}
	r.err = errors.New("app management answered 500")
	installs := len(z.installs)
	if _, err := m.Rollback(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, m, "autoxpose"); j.State != "failed" || !strings.Contains(j.Error, "can't be read") || len(z.installs) != installs {
		t.Errorf("go back: job %+v, installs %d → %d", j, installs, len(z.installs))
	}
	if i := info(t, m); i.Back != "0.6.0" {
		t.Errorf("a failed go back lost the copy: %+v", i)
	}

	// Before an update: no copy, so no way back for it.
	m, z, _, _ = autoSetup(t)
	m.Zima = &readerZima{fakeZima: z, err: errors.New("down")}
	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, "autoxpose")
	if i := info(t, m); i.Back != "" {
		t.Errorf("kept the list's copy: %+v", i)
	}
}

// 7. A manual update that fails after changing the app keeps the way back.
func TestManualFailedChangeKeepsCopy(t *testing.T) {
	m, z, _, d := autoSetup(t)
	z.task = zima.Task{State: "failed", Error: "container exited"}
	z.after = func() {
		d.set("autoxpose", Container{ID: "c2", Service: "autoxpose", ImageID: "sha256:new", State: "exited", ExitCode: 1})
	}
	if _, err := m.Update(context.Background(), "autoxpose"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, m, "autoxpose"); j.State != "failed" || j.Kind != "update" {
		t.Errorf("job %+v", j)
	}
	if i := info(t, m); i.Back != "0.6.0" || i.Held != "" {
		t.Errorf("after %+v", i)
	}
	if len(z.installs) != 1 {
		t.Errorf("went back by itself: %d installs", len(z.installs))
	}
}

// 9. The night run only updates an app it can see running.
func TestNightReady(t *testing.T) {
	m, z, _, d := autoSetup(t)
	ctx := context.Background()
	for _, c := range []struct {
		grid []zima.Grid
		c    Container
		want string
	}{
		{nil, Container{State: "running"}, SkipStopped},
		{[]zima.Grid{{Name: "other", Status: "running"}}, Container{State: "running"}, SkipStopped},
		{[]zima.Grid{{Name: "autoxpose", Status: "running"}}, Container{State: "paused"}, SkipStopped},
		{[]zima.Grid{{Name: "autoxpose", Status: "running"}}, Container{State: "created"}, SkipStopped},
		{[]zima.Grid{{Name: "autoxpose", Status: "running"}}, Container{State: "exited", ExitCode: 1}, SkipUnhealthy},
		{[]zima.Grid{{Name: "autoxpose", Status: "running"}}, Container{State: "exited", ExitCode: 0}, ""},
		{[]zima.Grid{{Name: "autoxpose", Status: "running"}}, Container{State: "running"}, ""},
	} {
		z.grid = c.grid
		c.c.ID, c.c.Service = "c1", "autoxpose"
		d.set("autoxpose", c.c)
		if got := m.nightReady(ctx, "autoxpose"); got != c.want {
			t.Errorf("grid %v, container %+v: %q, want %q", c.grid, c.c, got, c.want)
		}
	}
}

// Review round 2.

// A failed update that touched nothing leaves the kept copy alone, also when
// the app was stopped before.
func TestFailedUpdateOfStoppedAppKeepsCopy(t *testing.T) {
	m, z, _, d := autoSetup(t)
	ctx := context.Background()
	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, "autoxpose")
	if i := info(t, m); i.Back != "0.6.0" {
		t.Fatalf("first update %+v", i)
	}
	// Stopped (exited 1) before the next update, which fails without a change.
	d.set("autoxpose", Container{ID: "c1", Service: "autoxpose", ImageID: "sha256:old", State: "exited", ExitCode: 1})
	z.task = zima.Task{State: "failed", Error: "pull failed"}
	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, m, "autoxpose"); j.State != "failed" {
		t.Fatalf("job %+v", j)
	}
	if i := info(t, m); i.Back != "0.6.0" {
		t.Errorf("the way back to 0.6.0 is gone: %+v", i)
	}
}

// A night update that fails without touching an app with a finished one-shot
// is no change: nothing applied again, nothing held.
func TestNightFailedUntouchedOneShot(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	_ = m.SetAuto(ctx, "autoxpose", true)
	d.set("autoxpose",
		Container{ID: "c1", Service: "autoxpose", ImageID: "sha256:old", State: "running"},
		Container{ID: "c0", Service: "init", ImageID: "sha256:init", State: "exited"})
	z.task = zima.Task{State: "failed", Error: "pull failed"}
	m.NightStep(ctx, zone, night)
	if j := waitJob(t, m, "autoxpose"); j.Kind != "update" || j.State != "failed" {
		t.Fatalf("job %+v", j)
	}
	if len(z.installs) != 1 {
		t.Errorf("applied again: %d installs", len(z.installs))
	}
	if i := info(t, m); i.Held != "" {
		t.Errorf("held %+v", i)
	}
	if l := lastNote(n); l.Title != "AutoXpose didn't update overnight" {
		t.Errorf("note %+v", l)
	}
}

// Without Docker to look at, only a new installed version counts as a change.
func TestFailedUpdateWithoutDockerUsesVersion(t *testing.T) {
	for _, c := range []struct {
		version string
		back    string
	}{{"0.6.0", ""}, {"0.6.1", "0.6.0"}} {
		m, z, _, d := autoSetup(t)
		z.task = zima.Task{State: "failed", Error: "x"}
		z.apply = func([]byte) string { return c.version }
		z.after = func() { d.mu.Lock(); d.csErr = errors.New("docker isn't reachable"); d.mu.Unlock() }
		if _, err := m.Update(context.Background(), "autoxpose"); err != nil {
			t.Fatal(err)
		}
		waitJob(t, m, "autoxpose")
		if i := info(t, m); i.Back != c.back {
			t.Errorf("installed %s after the failure: back %q, want %q", c.version, i.Back, c.back)
		}
	}
}

// A restart during "applying" with the installed file unreadable keeps the
// copy, so a broken night update still goes back.
func TestResumeWithoutAfterStillGoesBack(t *testing.T) {
	m, z, n, d := autoSetup(t)
	ctx := context.Background()
	_ = m.SetAuto(ctx, "autoxpose", true)
	old, before, err := m.snapshot(ctx, "autoxpose", z.installed["autoxpose"])
	if err != nil || m.keepNext("autoxpose", old) != nil {
		t.Fatal(err)
	}
	// As left by the restart: the new version is in, and broken.
	in := z.installed["autoxpose"]
	in.Version = "0.6.1"
	z.installed["autoxpose"] = in
	d.have[digest] = true
	d.set("autoxpose", Container{ID: "c2", Service: "autoxpose", ImageID: "sha256:new", State: "running", Health: "unhealthy"})
	z.after = func() {
		z.taskErr = nil // the go back's own task is known
		d.set("autoxpose", Container{ID: "c3", Service: "autoxpose", ImageID: "sha256:old", State: "running"})
	}
	z.taskErr = errors.New("task not found")
	m.mu.Lock()
	m.load().Pending["autoxpose"] = &Pending{Name: "autoxpose", Title: "AutoXpose", Kind: "update", Night: true, Phase: "applying", TaskID: "t9", From: "0.6.0", To: "0.6.1", Before: before}
	m.save()
	m.mu.Unlock()
	m2 := &Manager{Catalog: m.Catalog, Zima: &readerZima{fakeZima: z, err: errors.New("down")}, Notify: n, State: m.State, Docker: d, Steady: m.Steady, Poll: time.Millisecond}
	m2.Resume(ctx)
	if j := waitJob(t, m2, "autoxpose"); j.Kind != "rollback" || j.State != "succeeded" {
		t.Fatalf("job %+v", j)
	}
	if len(z.installs) != 1 || !strings.Contains(string(z.installs[0]), digest) {
		t.Errorf("installs %d", len(z.installs))
	}
	if i := info(t, m2); i.Held != "0.6.1" {
		t.Errorf("%+v", i)
	}
}

// A copy kept without "after" isn't applied by hand: the settings changed
// since couldn't be told apart, so going back could drop them.
func TestManualGoBackNeedsAfter(t *testing.T) {
	m, _, _, _ := autoSetup(t)
	ctx := context.Background()
	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, "autoxpose")
	_ = os.Remove(filepath.Join(m.versionsDir(), "autoxpose.after.yml"))
	var ie *InputError
	if _, err := m.Rollback(ctx, "autoxpose"); !errors.As(err, &ie) || !errors.Is(err, errNoAfter) {
		t.Errorf("go back without after: %v", err)
	}
}
