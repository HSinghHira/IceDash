package apps

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HSinghHira/IceDash/internal/catalog"
	"github.com/HSinghHira/IceDash/internal/compose"
	"github.com/HSinghHira/IceDash/internal/creds"
	"github.com/HSinghHira/IceDash/internal/notify"
	"github.com/HSinghHira/IceDash/internal/zima"
	"gopkg.in/yaml.v3"
)

const autoxposeIndex = `{"version":2,"apps":[{"id":"com.icestore.autoxpose","title":"AutoXpose","icon":"/apps/com.icestore.autoxpose/assets/icon.png","compose_url":"/apps/com.icestore.autoxpose/docker-compose.yml","meta_url":"/apps/com.icestore.autoxpose/meta.json","content_hash":"h1","version":"0.6.1"}]}`

const autoxposeMeta = `{"id":"com.icestore.autoxpose","title":"AutoXpose","tagline":"Label a container","description":"Intro\n\n**How to use?**\n\n1. Open AutoXpose from the Zima OS dashboard.\n\n2. Go to Settings and add a DNS provider\n   and a proxy provider.\n\n**More**\n\n3. not a step","tips":{"before_install":"Read first"}}`

func catalogServer(t *testing.T) *catalog.Catalog {
	t.Helper()
	compose, err := os.ReadFile("../compose/testdata/autoxpose.yml")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = w.Write([]byte(autoxposeIndex))
		case "/apps/com.icestore.autoxpose/meta.json":
			_, _ = w.Write([]byte(autoxposeMeta))
		case "/raw/Apps/autoxpose/docker-compose.yml":
			_, _ = w.Write(compose)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return &catalog.Catalog{Site: srv.URL, Raw: srv.URL + "/raw", HTTP: srv.Client()}
}

type fakeZima struct {
	mu        sync.Mutex
	installed map[string]zima.Installed
	busy      []string // published ports in use
	installs  [][]byte
	task      zima.Task
	grid      []zima.Grid
	calls     []string
}

func (f *fakeZima) List(context.Context) (map[string]zima.Installed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]zima.Installed{}
	for k, v := range f.installed {
		out[k] = v
	}
	return out, nil
}

func (f *fakeZima) Install(_ context.Context, b []byte, _ string, dry bool) (zima.InstallResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, err := compose.Parse(b)
	if err != nil {
		return zima.InstallResult{}, err
	}
	var clash []string
	for _, p := range d.Ports() {
		for _, b := range f.busy {
			if p.Published == b {
				clash = append(clash, b)
			}
		}
	}
	if len(clash) > 0 {
		return zima.InstallResult{PortsInUse: map[string][]string{"TCP": clash}}, zima.ErrPortsInUse
	}
	if dry {
		return zima.InstallResult{}, nil
	}
	f.installs = append(f.installs, b)
	return zima.InstallResult{Accepted: true, TaskID: "t1"}, nil
}

func (f *fakeZima) Task(context.Context, string) (zima.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.task, nil
}

func (f *fakeZima) rec(s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, s)
	return nil
}

func (f *fakeZima) Start(_ context.Context, n string) error   { return f.rec("start " + n) }
func (f *fakeZima) Stop(_ context.Context, n string) error    { return f.rec("stop " + n) }
func (f *fakeZima) Restart(_ context.Context, n string) error { return f.rec("restart " + n) }
func (f *fakeZima) Uninstall(_ context.Context, n string, del bool) error {
	if del {
		return f.rec("uninstall+data " + n)
	}
	return f.rec("uninstall " + n)
}
func (f *fakeZima) Logs(context.Context, string, int) (string, error) { return "line 1\nline 2\n", nil }
func (f *fakeZima) AppGrid(context.Context) ([]zima.Grid, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]zima.Grid{}, f.grid...), nil
}

type notes struct {
	mu    sync.Mutex
	items []notify.Notification
}

func (n *notes) Add(x notify.Notification) (notify.Notification, bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, y := range n.items {
		if x.Key != "" && y.Key == x.Key {
			return y, false, nil
		}
	}
	n.items = append(n.items, x)
	return x, true, nil
}

func (n *notes) list() []notify.Notification {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]notify.Notification{}, n.items...)
}

type fakeCreds struct{}

func (fakeCreds) Read(_ context.Context, c string) (creds.Creds, error) {
	if c != "autoxpose" {
		return creds.Creds{}, creds.ErrNone
	}
	return creds.Creds{Fields: []creds.Field{{Label: "User", Value: "admin"}}}, nil
}

func newManager(t *testing.T) (*Manager, *fakeZima, *notes) {
	z := &fakeZima{installed: map[string]zima.Installed{}, task: zima.Task{State: "succeeded"}}
	n := &notes{}
	m := &Manager{Catalog: catalogServer(t), Zima: z, Creds: fakeCreds{}, Notify: n, State: filepath.Join(t.TempDir(), "apps.json"), Poll: time.Millisecond}
	return m, z, n
}

var choices = compose.Choices{Values: map[string]string{"autoxpose/SERVER_IP": "203.0.113.1", "autoxpose/LAN_IP": "192.168.68.17"}}

func waitJob(t *testing.T, m *Manager, name string) Job {
	t.Helper()
	for i := 0; i < 500; i++ {
		for _, j := range m.Jobs() {
			if j.Name == name && j.State != "running" {
				return j
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("job didn't finish")
	return Job{}
}

func TestSetupCheckInstall(t *testing.T) {
	m, z, n := newManager(t)
	ctx := context.Background()

	v, err := m.Setup(ctx, "autoxpose", compose.Context{Host: "192.168.68.17"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Installed || v.BeforeInstall != "Read first" || v.Name != "autoxpose" || !v.Access.DockerSocket {
		t.Errorf("setup %+v", v)
	}

	// Port 6103 is taken: Check says so and suggests 6104.
	z.busy = []string{"6103"}
	res, err := m.Check(ctx, "autoxpose", choices)
	if err != nil || res.OK || res.PortsInUse[0] != "6103" || res.Suggest["autoxpose/3000/tcp"] != "6104" {
		t.Fatalf("check %+v %v", res, err)
	}
	// Missing input is the user's to fix.
	var ie *InputError
	if _, err := m.Check(ctx, "autoxpose", compose.Choices{}); !errors.As(err, &ie) {
		t.Errorf("missing SERVER_IP: %v", err)
	}

	ch := compose.Choices{Values: choices.Values, Ports: map[string]string{"autoxpose/3000/tcp": "6104"}}
	j, err := m.Install(ctx, "autoxpose", ch)
	if err != nil || j.Kind != "install" || j.TaskID != "t1" {
		t.Fatalf("install %+v %v", j, err)
	}
	j = waitJob(t, m, "autoxpose")
	if j.State != "succeeded" {
		t.Fatalf("job %+v", j)
	}
	if !strings.Contains(string(z.installs[0]), "SERVER_IP=203.0.113.1") || !strings.Contains(string(z.installs[0]), `published: "6104"`) {
		t.Errorf("installed file:\n%s", z.installs[0])
	}
	if l := n.list(); len(l) != 1 || l[0].Kind != notify.KindAppInstalled || l[0].App != "autoxpose" {
		t.Errorf("notes %+v", l)
	}

	// Installed now: it shows its first-launch popup until dismissed.
	z.installed["autoxpose"] = zima.Installed{Name: "autoxpose", StoreID: "com.icestore.autoxpose", Version: "0.6.0", Title: "AutoXpose", Compose: jsonOf(t, z.installs[0])}
	list, _ := m.List(ctx, "hira")
	if len(list) != 1 || !list[0].FirstLaunch || !list[0].Update || list[0].Latest != "0.6.1" {
		t.Errorf("list %+v", list)
	}
	fl, err := m.FirstLaunchFor(ctx, "autoxpose", true)
	if err != nil || len(fl.Steps) != 2 || fl.Steps[1] != "Go to Settings and add a DNS provider and a proxy provider." || fl.Creds == nil {
		t.Errorf("first launch %+v %v", fl, err)
	}
	if fl, _ := m.FirstLaunchFor(ctx, "autoxpose", false); fl.Creds != nil {
		t.Error("a member got credentials")
	}
	m.Dismiss("hira", "autoxpose")
	if list, _ := m.List(ctx, "hira"); list[0].FirstLaunch {
		t.Error("still shown after dismiss")
	}
	if list, _ := m.List(ctx, "other"); !list[0].FirstLaunch {
		t.Error("dismissed for everyone")
	}
	// The state survives a restart.
	m2 := &Manager{Catalog: m.Catalog, Zima: z, State: m.State}
	if list, _ := m2.List(ctx, "hira"); list[0].FirstLaunch {
		t.Error("dismiss not saved")
	}
}

func jsonOf(t *testing.T, y []byte) []byte {
	var v any
	if err := yaml.Unmarshal(y, &v); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUpdateKeepsSettingsAndNotifiesOnce(t *testing.T) {
	m, z, n := newManager(t)
	ctx := context.Background()
	old, _ := os.ReadFile("../compose/testdata/autoxpose.yml")
	d, _ := compose.Parse(old)
	_ = d.Apply(compose.Choices{Values: map[string]string{"autoxpose/SERVER_IP": "198.51.100.7", "autoxpose/LAN_IP": "192.168.68.17"}, Ports: map[string]string{"autoxpose/3000/tcp": "6120"}})
	b, _ := d.Bytes()
	z.installed["autoxpose"] = zima.Installed{Name: "autoxpose", StoreID: "com.icestore.autoxpose", Version: "0.6.0", Title: "AutoXpose", Compose: jsonOf(t, b)}

	m.CheckUpdates(ctx)
	m.CheckUpdates(ctx)
	if l := n.list(); len(l) != 1 || l[0].Kind != notify.KindAppUpdateAvailable || l[0].Version != "0.6.1" {
		t.Fatalf("notes %+v", l)
	}

	if _, err := m.Update(ctx, "autoxpose"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, m, "autoxpose"); j.Kind != "update" || j.State != "succeeded" {
		t.Errorf("job %+v", j)
	}
	got := string(z.installs[0])
	if !strings.Contains(got, "SERVER_IP=198.51.100.7") || !strings.Contains(got, `published: "6120"`) {
		t.Errorf("update lost settings:\n%s", got)
	}
	if l := n.list(); l[len(l)-1].Kind != notify.KindAppUpdated {
		t.Errorf("notes %+v", l)
	}
	// Not from IceStore: refused.
	z.installed["homeassistant"] = zima.Installed{Name: "homeassistant", StoreID: "org.icewhale.homeassistant"}
	var ie *InputError
	if _, err := m.Update(ctx, "homeassistant"); !errors.As(err, &ie) {
		t.Errorf("non-IceStore update: %v", err)
	}
}

func TestFailedInstallNotifies(t *testing.T) {
	m, z, n := newManager(t)
	z.task = zima.Task{State: "failed", Error: "image pull failed"}
	if _, err := m.Install(context.Background(), "autoxpose", choices); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, m, "autoxpose"); j.State != "failed" || j.Error != "image pull failed" {
		t.Errorf("job %+v", j)
	}
	if l := n.list(); len(l) != 1 || l[0].Kind != notify.KindAppInstallFailed || l[0].Body != "image pull failed" {
		t.Errorf("notes %+v", l)
	}
}

func TestActionsAndWatcher(t *testing.T) {
	m, z, n := newManager(t)
	ctx := context.Background()
	z.installed["gitsave"] = zima.Installed{Name: "gitsave", Title: "GitSave"}
	z.installed["pocket-id"] = zima.Installed{Name: "pocket-id", Title: "Pocket ID"}

	if err := m.Stop(ctx, "nope"); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("stop unknown: %v", err)
	}
	z.grid = []zima.Grid{{Name: "gitsave", Status: "running"}, {Name: "pocket-id", Status: "running"}}
	m.watchStatus(ctx) // first look: nothing to report
	_ = m.Stop(ctx, "gitsave")
	z.grid = []zima.Grid{{Name: "gitsave", Status: "exited"}, {Name: "pocket-id", Status: "exited"}}
	m.watchStatus(ctx)
	l := n.list()
	if len(l) != 1 || l[0].App != "pocket-id" || l[0].Kind != notify.KindAppStopped || !strings.HasPrefix(l[0].Title, "Pocket ID") {
		t.Errorf("only pocket-id stopped unexpectedly: %+v", l)
	}
	_ = m.Restart(ctx, "gitsave")
	_ = m.Start(ctx, "gitsave")
	_ = m.Uninstall(ctx, "gitsave", false)
	if strings.Join(z.calls, ",") != "stop gitsave,restart gitsave,start gitsave,uninstall gitsave" {
		t.Errorf("calls %v", z.calls)
	}
	if s, err := m.Logs(ctx, "pocket-id", 50); err != nil || !strings.Contains(s, "line 2") {
		t.Errorf("logs %q %v", s, err)
	}
}

func TestSteps(t *testing.T) {
	if s := Steps("no heading\n1. x"); len(s) != 0 {
		t.Errorf("%v", s)
	}
	s := Steps("**How to use?**\n\n1. One.\n\n2. Two\n   wraps.\n")
	if len(s) != 2 || s[1] != "Two wraps." {
		t.Errorf("%q", s)
	}
}

func TestTarGz(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "db"), 0o700)
	_ = os.WriteFile(filepath.Join(dir, "db", "data.sqlite"), []byte("hello"), 0o600)
	var buf bytes.Buffer
	if err := TarGz(context.Background(), dir, "gitsave", &buf); err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
	if strings.Join(names, ",") != "gitsave/,gitsave/db/,gitsave/db/data.sqlite" {
		t.Errorf("entries %v", names)
	}
	if err := TarGz(context.Background(), filepath.Join(dir, "missing"), "x", io.Discard); !errors.Is(err, ErrNoData) {
		t.Errorf("missing dir: %v", err)
	}
}
