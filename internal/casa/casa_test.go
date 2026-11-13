package casa

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HSinghHira/IceDash/internal/zima"
)

const gitsave = `name: gitsave
services:
  gitsave:
    image: ghcr.io/hsinghhira/gitsave:latest
    ports:
      - target: 3000
        published: "6181"
        protocol: tcp
x-casaos:
  id: com.icestore.gitsave
  main: gitsave
`

// fakeCasa answers like CasaOS 0.4.15's app management (HOST-NOTES, "Apps on CasaOS").
type fakeCasa struct {
	mu        sync.Mutex
	installed map[string]string // name → status
	busy      map[string]bool   // published ports in use
	calls     []string
}

func (f *fakeCasa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	f.calls = append(f.calls, r.Method+" "+r.URL.RequestURI())
	p := strings.TrimPrefix(r.URL.Path, "/v2/app_management")
	clash := func() bool {
		for port := range f.busy {
			if strings.Contains(string(body), `published: "`+port+`"`) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"data":{"ports_in_use":{"TCP":["`+port+`"],"UDP":[]}},"message":"there are ports in use"}`)
				return true
			}
		}
		return false
	}
	switch {
	case r.Method == http.MethodPost && p == "/compose":
		if r.Header.Get("Content-Type") != "application/yaml" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if clash() {
			return
		}
		if r.URL.Query().Get("dry_run") == "true" {
			_, _ = io.WriteString(w, `{"message":"only validation has been done because `+"`dry_run`"+` is specified - skipping compose app installation"}`)
			return
		}
		name, _ := projectName(body)
		f.installed[name] = "running(1)"
		_, _ = io.WriteString(w, `{"message":"compose app is being installed asynchronously"}`)
	case strings.HasPrefix(p, "/compose/") && strings.HasSuffix(p, "/status"):
		name := strings.TrimSuffix(strings.TrimPrefix(p, "/compose/"), "/status")
		if _, ok := f.installed[name]; !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"not found"}`)
			return
		}
		var to string
		_ = json.Unmarshal(body, &to)
		if to != "start" && to != "stop" && to != "restart" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"message":"request body has an error"}`)
			return
		}
		f.installed[name] = map[string]string{"start": "running(1)", "stop": "exited(1)", "restart": "running(1)"}[to]
		_, _ = io.WriteString(w, `{"message":"compose app status is being changed asynchronously"}`)
	case strings.HasPrefix(p, "/compose/"):
		name := strings.TrimPrefix(p, "/compose/")
		st, ok := f.installed[name]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"compose app `+"`"+name+"`"+` not found"}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = io.WriteString(w, `{"data":{"status":"`+st+`","compose":{}}}`)
		case http.MethodPut:
			if clash() {
				return
			}
			_, _ = io.WriteString(w, `{"message":"compose app is being applied with changes asynchroniously"}`)
		case http.MethodDelete:
			delete(f.installed, name)
			_, _ = io.WriteString(w, `{"message":"compose app is being uninstalled asynchronously"}`)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newClient(t *testing.T) (*Client, *fakeCasa) {
	f := &fakeCasa{installed: map[string]string{}, busy: map[string]bool{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	c := &Client{Client: &zima.Client{Base: srv.URL, System: "CasaOS"}, Poll: time.Millisecond}
	return c, f
}

func TestInstallNewAppPostsYAML(t *testing.T) {
	c, f := newClient(t)
	ctx := context.Background()
	if _, err := c.Install(ctx, []byte(gitsave), "", true); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	res, err := c.Install(ctx, []byte(gitsave), "", false)
	if err != nil || res.TaskID == "" || res.AppID != "gitsave" {
		t.Fatalf("install: %+v %v", res, err)
	}
	want := "POST /v2/app_management/compose?check_port_conflict=true"
	if !strings.Contains(strings.Join(f.calls, "\n"), want) {
		t.Fatalf("calls: %v", f.calls)
	}
	// Installed now, so a second install is a change: PUT, own ports excluded.
	if _, err := c.Install(ctx, []byte(gitsave), "", true); err != nil {
		t.Fatalf("dry run on the installed app: %v", err)
	}
	if last := f.calls[len(f.calls)-1]; !strings.HasPrefix(last, "PUT /v2/app_management/compose/gitsave?") || !strings.Contains(last, "dry_run=true") {
		t.Fatalf("last call %q", last)
	}
}

func TestPortsInUse(t *testing.T) {
	c, f := newClient(t)
	f.busy["6181"] = true
	res, err := c.Install(context.Background(), []byte(gitsave), "", true)
	if !errors.Is(err, zima.ErrPortsInUse) || len(res.PortsInUse["TCP"]) != 1 || res.PortsInUse["TCP"][0] != "6181" {
		t.Fatalf("%+v %v", res, err)
	}
	// A failed real install leaves no task behind.
	if _, err := c.Install(context.Background(), []byte(gitsave), "", false); !errors.Is(err, zima.ErrPortsInUse) {
		t.Fatal(err)
	}
	if len(c.current) != 0 {
		t.Fatalf("tasks left: %v", c.current)
	}
}

// The events below are the ones CasaOS 0.4.15 sent (HOST-NOTES, "Notifications,
// message bus and uninstall on CasaOS").
func ev(name, app, msg string) Event {
	p := map[string]string{"app:name": app}
	if msg != "" {
		p["message"] = msg
	}
	return Event{Name: name, Properties: p}
}

func TestTaskFollowsTheBus(t *testing.T) {
	c, _ := newClient(t)
	ctx := context.Background()
	c.setLive(true)
	res, err := c.Install(ctx, []byte(gitsave), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if tk, _ := c.Task(ctx, res.TaskID); tk.State != "running" || tk.Done() {
		t.Fatalf("before any event: %+v", tk)
	}
	c.Handle(ev("app:install-begin", "gitsave", ""))
	c.Handle(ev("app:install-end", "gitsave", ""))
	// An error after the end belongs to something else (measured: a change started mid-install).
	c.Handle(ev("app:install-error", "gitsave", "No such container"))
	if tk, _ := c.Task(ctx, res.TaskID); tk.State != "succeeded" {
		t.Fatalf("after end: %+v", tk)
	}
}

func TestFailedInstallErrorThenEnd(t *testing.T) {
	c, f := newClient(t)
	ctx := context.Background()
	c.setLive(true)
	res, err := c.Install(ctx, []byte(gitsave), "", false)
	if err != nil {
		t.Fatal(err)
	}
	delete(f.installed, "gitsave") // CasaOS removes it after the pull fails
	c.Handle(ev("app:install-error", "gitsave", "Error response from daemon: error from registry: denied\ndenied"))
	if tk, _ := c.Task(ctx, res.TaskID); tk.Done() {
		t.Fatalf("an error before the end isn't the end: %+v", tk)
	}
	c.Handle(ev("app:install-end", "gitsave", ""))
	tk, _ := c.Task(ctx, res.TaskID)
	if tk.State != "failed" || !strings.Contains(tk.Error, "registry: denied") {
		t.Fatalf("%+v", tk)
	}
}

func TestTaskWithoutTheBus(t *testing.T) {
	c, _ := newClient(t)
	ctx := context.Background()
	res, err := c.Install(ctx, []byte(gitsave), "", false)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if tk, _ := c.Task(ctx, res.TaskID); tk.State != "succeeded" {
		t.Fatalf("the app runs, so the install is done: %+v", tk)
	}
}

func TestActions(t *testing.T) {
	c, f := newClient(t)
	ctx := context.Background()
	f.installed["gitsave"] = "running(1)"
	for _, step := range []struct {
		do   func(context.Context, string) error
		want string
	}{{c.Stop, "exited(1)"}, {c.Start, "running(1)"}, {c.Restart, "running(1)"}} {
		if err := step.do(ctx, "gitsave"); err != nil {
			t.Fatal(err)
		}
		if f.installed["gitsave"] != step.want {
			t.Fatalf("status %q, want %q", f.installed["gitsave"], step.want)
		}
	}
	if err := c.Uninstall(ctx, "gitsave", false); err != nil {
		t.Fatal(err)
	}
	if last := f.calls[len(f.calls)-1]; last != "DELETE /v2/app_management/compose/gitsave?delete_config_folder=false" {
		t.Fatalf("%q", last)
	}
	if err := c.Start(ctx, "gone"); err == nil {
		t.Fatal("starting a missing app should fail")
	}
}

// busServer is a WebSocket server that sends the given events, then waits.
func busServer(t *testing.T, events []string, gotQuery chan<- string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery <- r.URL.RawQuery
		key := r.Header.Get("Sec-WebSocket-Key")
		sum := sha1.Sum([]byte(key + wsGUID))
		hj, _ := w.(http.Hijacker)
		conn, rw, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " +
			base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n")
		_ = rw.Flush()
		for _, e := range events {
			frame := []byte{0x81, byte(len(e))} // events here are under 126 bytes
			_, _ = conn.Write(append(frame, e...))
		}
		_, _ = conn.Write([]byte{0x89, 0}) // a ping, which the client must answer
		_, _ = bufio.NewReader(conn).ReadByte()
		time.Sleep(200 * time.Millisecond)
	}))
}

func TestListenReadsEvents(t *testing.T) {
	c, _ := newClient(t)
	q := make(chan string, 4)
	srv := busServer(t, []string{
		`{"name":"app:install-end","properties":{"app:name":"gitsave"}}`,
	}, q)
	defer srv.Close()
	c.BusURL = srv.URL
	c.begin("gitsave", "install")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go c.Listen(ctx)
	if got := <-q; !strings.Contains(got, "names=app%3Ainstall-end") || strings.Count(got, "names=") != len(busEvents) {
		t.Fatalf("query %q", got)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		ended := c.current["gitsave"].ended
		c.mu.Unlock()
		if ended {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the end event never arrived")
}
