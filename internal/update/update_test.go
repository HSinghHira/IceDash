package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HSinghHira/IceDash/internal/notify"
)

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"0.1.0", "v0.2.0", -1},
		{"v0.2.0", "0.2.0", 0},
		{"0.10.0", "0.9.9", 1},
		{"0.3.0-dev7", "0.3.0", -1},
		{"0.3.0-dev10", "0.3.0-dev7", 1},
		{"0.0.0-dev+abc123", "v0.1.0", -1},
		{"1.0.0+build1", "1.0.0+build2", 0},
		{"0.0.1-test", "0.0.1", -1},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestAllowedURL(t *testing.T) {
	for u, ok := range map[string]bool{
		"https://github.com/HSinghHira/IceDash/releases/download/v0.1.0/icedash-amd64.raw": true,
		"https://release-assets.githubusercontent.com/github-production-release-asset/1/2": true,
		"http://github.com/x":                 false,
		"https://github.com.evil.example/x":   false,
		"https://evilgithubusercontent.com/x": false,
	} {
		if err := AllowedURL(u); (err == nil) != ok {
			t.Errorf("AllowedURL(%s) = %v", u, err)
		}
	}
}

// fake is a GitHub API plus download host, and a record of what the box was told to do.
type fake struct {
	t       *testing.T
	srv     *httptest.Server
	tag     string
	image   []byte
	sum     string // what the .sha256 says
	noRel   bool
	limited bool

	mu   sync.Mutex
	runs []string
	list map[string]string // image path -> unsquashfs -lc output
}

func newFake(t *testing.T, tag string) *fake {
	f := &fake{t: t, tag: tag, image: []byte("new image bytes"), list: map[string]string{}}
	f.sum = sha(f.image)
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/HSinghHira/IceDash/releases/latest":
			if f.limited {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if f.noRel {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(Release{Tag: f.tag, Body: "Shorter checks.", Assets: []Asset{
				{Name: "icedash-amd64.raw", URL: f.srv.URL + "/dl/icedash-amd64.raw"},
				{Name: "icedash-amd64.raw.sha256", URL: f.srv.URL + "/dl/icedash-amd64.raw.sha256"},
				{Name: "icedash.raw", URL: f.srv.URL + "/dl/icedash.raw"},
			}})
		case r.URL.Path == "/dl/icedash-amd64.raw":
			w.Write(f.image)
		case r.URL.Path == "/dl/icedash-amd64.raw.sha256":
			fmt.Fprintf(w, "%s  icedash-amd64.raw\n", f.sum)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func (f *fake) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs = append(f.runs, name+" "+strings.Join(args, " "))
	if name == "unsquashfs" {
		if out, ok := f.list[args[len(args)-1]]; ok {
			return []byte(out), nil
		}
		return []byte(f.list["*"]), nil // any other image
	}
	return nil, nil
}

type notes struct {
	mu  sync.Mutex
	got []notify.Notification
}

func (n *notes) Add(x notify.Notification) (notify.Notification, bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.got = append(n.got, x)
	return x, true, nil
}

func (n *notes) kinds() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	var k []string
	for _, x := range n.got {
		k = append(k, x.Kind)
	}
	return k
}

type card struct {
	mu            sync.Mutex
	shown, gone   int
	shownVersions []string
}

func (c *card) Show(_ context.Context, v string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shown++
	c.shownVersions = append(c.shownVersions, v)
	return nil
}

func (c *card) Withdraw(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gone++
	return nil
}

const imageList = `squashfs-root
squashfs-root/usr
squashfs-root/usr/bin
squashfs-root/usr/bin/icedash
squashfs-root/usr/lib/extension-release.d/extension-release.icedash
squashfs-root/usr/share/casaos/modules/icedash.json
`

func manager(t *testing.T, f *fake) (*Manager, *notes, *card) {
	t.Helper()
	base := t.TempDir()
	ext := filepath.Join(base, "extensions")
	root := filepath.Join(base, "host")
	os.MkdirAll(ext, 0o755)
	os.WriteFile(filepath.Join(ext, "icedash.raw"), []byte("installed image"), 0o644)
	// the host already has IceDash's own files, as after an install
	for _, p := range []string{"usr/bin/icedash", "usr/share/casaos/modules/icedash.json"} {
		os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte("x"), 0o644)
	}
	f.list["*"] = imageList
	n, c := &notes{}, &card{}
	m := &Manager{
		Version:  "0.1.0",
		Arch:     "amd64",
		Dir:      filepath.Join(base, "self-update"),
		ExtDir:   ext,
		Root:     root,
		GitHub:   GitHub{API: f.srv.URL, Repo: Repo, HTTP: f.srv.Client()},
		HTTP:     f.srv.Client(),
		Run:      f.run,
		Notify:   n,
		Card:     c,
		AllowURL: func(string) error { return nil },
	}
	return m, n, c
}

func waitState(t *testing.T, m *Manager, want string) Status {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := m.Status()
		if st.State == want || st.State == StateFailed && want != StateFailed {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("state %s, want %s: %+v", st.State, want, st)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCheckFindsNewerRelease(t *testing.T) {
	f := newFake(t, "v0.2.0")
	m, n, c := manager(t, f)
	st := m.Check(context.Background())
	if st.State != StateAvailable || st.Latest != "v0.2.0" || st.Notes != "Shorter checks." || st.CheckedAt == nil {
		t.Fatalf("status %+v", st)
	}
	if k := n.kinds(); len(k) != 1 || k[0] != notify.KindUpdateAvailable || n.got[0].Action == nil || n.got[0].Key != "update:v0.2.0" {
		t.Fatalf("notifications %+v", n.got)
	}
	if c.shown != 1 || c.shownVersions[0] != "v0.2.0" {
		t.Fatalf("card shown %d %v", c.shown, c.shownVersions)
	}
}

func TestCheckSameOrOlderIsQuiet(t *testing.T) {
	for _, tag := range []string{"v0.1.0", "v0.0.9"} {
		f := newFake(t, tag)
		m, n, c := manager(t, f)
		if st := m.Check(context.Background()); st.State != StateIdle || st.Latest != tag {
			t.Fatalf("%s: %+v", tag, st)
		}
		if len(n.got) != 0 || c.shown != 0 {
			t.Fatalf("%s raised something", tag)
		}
	}
}

func TestCheckWithoutReleaseOrWithLimit(t *testing.T) {
	f := newFake(t, "v0.2.0")
	f.noRel = true
	m, _, _ := manager(t, f)
	if st := m.Check(context.Background()); st.State != StateIdle || st.Error != "" {
		t.Fatalf("no release: %+v", st)
	}
	f.noRel, f.limited = false, true
	if st := m.Check(context.Background()); st.State != StateIdle || !strings.Contains(st.Error, "limit") {
		t.Fatalf("rate limited: %+v", st)
	}
}

func TestApplyRunsTheStepsAndHandsOver(t *testing.T) {
	f := newFake(t, "v0.2.0")
	m, _, _ := manager(t, f)
	m.Check(context.Background())
	if err := m.Apply(); err != nil {
		t.Fatal(err)
	}
	// installing stays installing after the handover: the restart ends this process
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := m.Status()
		if n := len(st.Steps); n > 0 && st.Steps[n-1].ID == "handover" {
			break
		}
		if st.State == StateFailed || time.Now().After(deadline) {
			t.Fatalf("status %+v", st)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := m.Apply(); !errors.Is(err, ErrBusy) {
		t.Fatalf("second apply: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(m.Dir, "new", "icedash.raw"))
	if string(got) != "new image bytes" {
		t.Fatalf("downloaded %q", got)
	}
	prev, _ := os.ReadFile(filepath.Join(m.Dir, "previous", "icedash.raw"))
	if string(prev) != "installed image" {
		t.Fatalf("previous %q", prev)
	}
	script, err := os.ReadFile(filepath.Join(m.Dir, "run.sh"))
	if err != nil || !strings.Contains(string(script), `WANT="0.2.0"`) || !strings.Contains(string(script), `OLD="0.1.0"`) {
		t.Fatalf("script: %v\n%s", err, script)
	}
	last := f.runs[len(f.runs)-1]
	if !strings.HasPrefix(last, "systemd-run --unit=icedash-self-update --collect --no-block /bin/sh ") {
		t.Fatalf("handover: %s", last)
	}
}

func TestApplyStopsOnChecksumMismatch(t *testing.T) {
	f := newFake(t, "v0.2.0")
	f.sum = sha([]byte("something else"))
	m, _, _ := manager(t, f)
	m.Check(context.Background())
	m.Apply()
	st := waitState(t, m, StateFailed)
	if st.State != StateFailed || !strings.Contains(st.Error, "checksum") {
		t.Fatalf("status %+v", st)
	}
	if st.Steps[len(st.Steps)-1].State != StepFailed {
		t.Fatalf("last step %+v", st.Steps)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(r, "systemd-run") {
			t.Fatal("handed over despite a bad checksum")
		}
	}
	// a failed install can be tried again
	f.sum = sha(f.image)
	if err := m.Apply(); err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitHandover(t, m)
}

// waitHandover waits until an install reached the handover step.
func waitHandover(t *testing.T, m *Manager) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := m.Status()
		if n := len(st.Steps); n > 0 && st.Steps[n-1].ID == "handover" {
			return
		}
		if st.State == StateFailed || time.Now().After(deadline) {
			t.Fatalf("no handover: %+v", st)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestApplyRefusesAFileOfZimaOS(t *testing.T) {
	f := newFake(t, "v0.2.0")
	m, _, _ := manager(t, f)
	// the new image also ships /usr/bin/cmp, which the host has
	os.WriteFile(filepath.Join(m.Root, "usr/bin/cmp"), []byte("busybox"), 0o755)
	f.list[filepath.Join(m.Dir, "new", "icedash.raw")] = imageList + "squashfs-root/usr/bin/cmp\n"
	f.list[filepath.Join(m.ExtDir, "icedash.raw")] = imageList
	m.Check(context.Background())
	m.Apply()
	st := waitState(t, m, StateFailed)
	if !strings.Contains(st.Error, "/usr/bin/cmp") || strings.Contains(st.Error, "/usr/bin/icedash") {
		t.Fatalf("status %+v", st)
	}
}

func TestApplyWithoutUpdate(t *testing.T) {
	f := newFake(t, "v0.1.0")
	m, _, _ := manager(t, f)
	m.Check(context.Background())
	if err := m.Apply(); !errors.Is(err, ErrNothingToDo) {
		t.Fatalf("apply: %v", err)
	}
}

func TestResumeAnnouncesEachResultOnce(t *testing.T) {
	for _, c := range []struct {
		result, kind string
	}{
		{"result ok", notify.KindUpdated},
		{"result rolled-back", notify.KindUpdateFailed},
		{"result rollback-failed: run 'sudo zpkg install --force x'", notify.KindRollbackFailed},
	} {
		f := newFake(t, "v0.2.0")
		m, n, _ := manager(t, f)
		m.Version = "0.2.0"
		os.MkdirAll(m.Dir, 0o700)
		os.WriteFile(filepath.Join(m.Dir, "log"), []byte("swap 2026-11-11T10:00:00Z from 0.1.0 to 0.2.0\n"+c.result+"\n"), 0o600)
		m.Resume(context.Background())
		m.Resume(context.Background()) // a later restart says nothing again
		if k := n.kinds(); len(k) != 1 || k[0] != c.kind {
			t.Fatalf("%s: %v", c.result, k)
		}
		if b, _ := os.ReadFile(filepath.Join(m.Dir, "started")); string(b) != "0.2.0" {
			t.Fatalf("started marker %q", b)
		}
		if c.kind == notify.KindRollbackFailed && !strings.Contains(n.got[0].Body, "sudo zpkg install --force x") {
			t.Fatalf("fix missing: %s", n.got[0].Body)
		}
	}
}

func TestResumeWithoutUpdateIsQuiet(t *testing.T) {
	f := newFake(t, "v0.2.0")
	m, n, _ := manager(t, f)
	m.Resume(context.Background())
	if len(n.got) != 0 {
		t.Fatalf("raised %v", n.kinds())
	}
}

func TestCardFollowsTheState(t *testing.T) {
	f := newFake(t, "v0.2.0")
	m, _, c := manager(t, f)
	m.Check(context.Background())
	m.syncCard(context.Background())
	if c.shown != 2 {
		t.Fatalf("shown %d, want 2 (check + re-send)", c.shown)
	}
	f.tag = "v0.1.0" // e.g. after updating, nothing newer waits
	m.Check(context.Background())
	m.syncCard(context.Background())
	m.syncCard(context.Background())
	if c.gone != 1 {
		t.Fatalf("withdrawn %d, want 1", c.gone)
	}
}

// The swap script itself, against stand-ins for zpkg and systemctl: an image
// file holding "good <version>" makes its service write $D/started; "broken"
// never starts, like the [broken-image] test release.
func TestSwapScript(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("needs a Unix sh")
	}
	old := startWait
	startWait = 3
	t.Cleanup(func() { startWait = old })

	for _, c := range []struct {
		name, newImg, prevImg, result string
	}{
		{"good update", "good 0.2.0", "good 0.1.0", "result ok"},
		{"broken image rolls back", "broken", "good 0.1.0", "result rolled-back"},
		{"rollback fails too", "broken", "broken", "result rollback-failed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := t.TempDir()
			for _, sub := range []string{"new", "previous"} {
				os.MkdirAll(filepath.Join(dir, sub), 0o700)
			}
			os.WriteFile(filepath.Join(dir, "new", "icedash.raw"), []byte(c.newImg), 0o600)
			os.WriteFile(filepath.Join(dir, "previous", "icedash.raw"), []byte(c.prevImg), 0o600)
			// zpkg remembers which image is installed; restarting "starts" it
			stub := func(name, body string) {
				os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755)
			}
			stub("zpkg", `cp "$3" "`+dir+`/installed"`)
			stub("systemctl", `[ "$1" = restart ] || exit 0
read -r kind ver < "`+dir+`/installed"
[ "$kind" = good ] && printf %s "$ver" > "`+dir+`/started"
exit 0`)
			stub("sleep", "exit 0") // no waiting in tests
			script := filepath.Join(dir, "run.sh")
			os.WriteFile(script, []byte(SwapScript(dir, "0.2.0", "0.1.0")), 0o700)

			cmd := exec.Command(sh, script)
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
			_ = cmd.Run()
			_, result, ok := readLog(filepath.Join(dir, "log"))
			log, _ := os.ReadFile(filepath.Join(dir, "log"))
			if !ok || !strings.HasPrefix("result "+result, c.result) {
				t.Fatalf("result %q, want %q\n%s", result, c.result, log)
			}
		})
	}
}
