// Package casa talks to CasaOS's app management (PLAN.md, Phase 3a, "Apps on
// CasaOS"). It gives the apps manager the same calls package zima does on
// ZimaOS, so the manager doesn't know which system it runs on.
//
// Measured on CasaOS 0.4.15 (docs/HOST-NOTES.md, "Apps on CasaOS"):
//   - the app list, logs and app grid have ZimaOS's paths and shapes, so those
//     calls are zima.Client's own;
//   - installs are POST /compose with the YAML, and always check ports; an
//     installed app is changed with PUT /compose/{name}, which leaves its own
//     ports out of the check;
//   - neither returns a task: the message bus says how it went. A failed
//     install sends app:install-error, then app:install-end, so "end" alone
//     isn't success;
//   - start, stop and restart are PUT /compose/{name}/status with "start",
//     "stop" or "restart"; uninstall is DELETE /compose/{name}.
package casa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/HSinghHira/IceDash/internal/zima"
)

// DefaultBusURLFile holds the message bus address (world-readable, measured).
const DefaultBusURLFile = "/var/run/casaos/message-bus.url"

// Client is CasaOS's app management. Call Listen once in the background.
type Client struct {
	*zima.Client        // List, Logs, AppGrid and the address
	BusURLFile   string // default DefaultBusURLFile
	BusURL       string // tests: used instead of the file
	// Poll is how long a task without bus news waits before asking app
	// management directly (default 5 s).
	Poll time.Duration

	mu      sync.Mutex
	tasks   map[string]*task
	current map[string]*task // the running task per app name
	live    bool             // the bus connection is up
	seq     int
}

// New returns a client for the CasaOS box this runs on.
func New() *Client { return &Client{Client: &zima.Client{System: "CasaOS"}} }

type task struct {
	id, name, kind string // kind: install, apply
	started        time.Time
	ended          bool
	errMsg         string
	state          string // running, succeeded, failed
}

// ---- requests ----

func (c *Client) send(ctx context.Context, method, path, ctype string, body []byte) ([]byte, int, error) {
	base, err := c.BaseURL()
	if err != nil {
		return nil, 0, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, rd)
	if err != nil {
		return nil, 0, err
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if t := zima.TokenFrom(ctx); t != "" {
		req.Header.Set("Authorization", t)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 2 * time.Minute}
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("CasaOS app management: %w", err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	return b, res.StatusCode, err
}

// answer is CasaOS's usual reply: a message, sometimes data.
type answer struct {
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func failure(status int, raw []byte) error {
	var a answer
	_ = json.Unmarshal(raw, &a)
	return &zima.Error{Status: status, Message: a.Message}
}

func (c *Client) exists(ctx context.Context, name string) (bool, string, error) {
	raw, status, err := c.send(ctx, http.MethodGet, "/compose/"+url.PathEscape(name), "", nil)
	if err != nil {
		return false, "", err
	}
	switch {
	case status == http.StatusNotFound:
		return false, "", nil
	case status >= 300:
		return false, "", failure(status, raw)
	}
	var v struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &v)
	return true, v.Data.Status, nil
}

// ---- install ----

// projectName reads the compose file's top-level name, the app's name.
func projectName(b []byte) (string, error) {
	var v struct {
		Name string `yaml:"name"`
	}
	if err := yaml.Unmarshal(b, &v); err != nil {
		return "", err
	}
	if v.Name == "" {
		return "", errors.New("the compose file has no name")
	}
	return v.Name, nil
}

// Install installs a new app (POST /compose) or changes an installed one
// (PUT /compose/{name}), which is how IceStore apps update. dryRun only
// checks the file and its ports. CasaOS takes the icon from the file.
func (c *Client) Install(ctx context.Context, b []byte, _ string, dryRun bool) (zima.InstallResult, error) {
	name, err := projectName(b)
	if err != nil {
		return zima.InstallResult{}, err
	}
	installed, _, err := c.exists(ctx, name)
	if err != nil {
		return zima.InstallResult{}, err
	}
	q := url.Values{"check_port_conflict": {"true"}}
	if dryRun {
		q.Set("dry_run", "true")
	}
	method, path, kind := http.MethodPost, "/compose?"+q.Encode(), "install"
	if installed {
		method, path, kind = http.MethodPut, "/compose/"+url.PathEscape(name)+"?"+q.Encode(), "apply"
	}

	// The task exists before the call: a quick failure can be on the bus
	// before the answer is back.
	var t *task
	if !dryRun {
		t = c.begin(name, kind)
	}
	raw, status, err := c.send(ctx, method, path, "application/yaml", b)
	if err != nil {
		c.drop(t)
		return zima.InstallResult{}, err
	}
	var a answer
	_ = json.Unmarshal(raw, &a)
	var res zima.InstallResult
	_ = json.Unmarshal(a.Data, &res)
	// {"data":{"ports_in_use":{"TCP":["22"],"UDP":[]}},"message":"there are ports in use"}
	if len(res.PortsInUse["TCP"])+len(res.PortsInUse["UDP"]) > 0 {
		c.drop(t)
		return res, zima.ErrPortsInUse
	}
	if status >= 300 {
		c.drop(t)
		return res, failure(status, raw)
	}
	res.Accepted = true
	res.AppID = name
	if t != nil {
		res.TaskID = t.id
	}
	return res, nil
}

func (c *Client) begin(name, kind string) *task {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tasks == nil {
		c.tasks, c.current = map[string]*task{}, map[string]*task{}
	}
	c.seq++
	t := &task{id: "casaos-" + kind + "-" + name + "-" + strconv.Itoa(c.seq), name: name, kind: kind, started: time.Now(), state: "running"}
	c.tasks[t.id], c.current[name] = t, t
	return t
}

func (c *Client) drop(t *task) {
	if t == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tasks, t.id)
	if c.current[t.name] == t {
		delete(c.current, t.name)
	}
}

// Task reports an install or change. The bus decides; without it (or while
// it is down), the app's own state does: an install is done once the app
// runs.
func (c *Client) Task(ctx context.Context, id string) (zima.Task, error) {
	c.mu.Lock()
	t, ok := c.tasks[id]
	if !ok {
		c.mu.Unlock()
		return zima.Task{}, &zima.Error{Status: http.StatusNotFound, Message: "no such task"}
	}
	cp, live := *t, c.live
	c.mu.Unlock()

	out := zima.Task{ID: cp.id, AppID: cp.name, Type: cp.kind, State: cp.state, Phase: "installing"}
	if cp.state != "running" {
		out.Error = cp.errMsg
		return out, nil
	}
	// With the bus up, wait for its end event; after 2 minutes without one
	// (an event lost in a reconnect), look at the app anyway.
	if !cp.ended && live && time.Since(cp.started) < 2*time.Minute {
		return out, nil
	}
	poll := c.Poll
	if poll == 0 {
		poll = 5 * time.Second
	}
	if !cp.ended && time.Since(cp.started) < poll {
		return out, nil
	}
	there, status, err := c.exists(ctx, cp.name)
	if err != nil {
		return out, err
	}
	running := strings.HasPrefix(status, "running")
	// A change keeps the old container running for a while, so without the
	// bus it counts as done only after a minute.
	settled := cp.kind == "install" || time.Since(cp.started) > time.Minute
	switch {
	case cp.ended && cp.errMsg != "":
		out.State, out.Error = "failed", cp.errMsg
	case cp.ended && !there:
		out.State, out.Error = "failed", "CasaOS didn't install it"
	case cp.ended, there && running && settled:
		out.State = "succeeded"
	default:
		return out, nil
	}
	c.finish(cp.id, out.State, out.Error)
	return out, nil
}

func (c *Client) finish(id, state, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.tasks[id]; ok {
		t.state, t.errMsg = state, msg
		if c.current[t.name] == t {
			delete(c.current, t.name)
		}
	}
}

// ---- the message bus ----

// busEvents are the events Listen asks for.
var busEvents = []string{
	"app:install-begin", "app:install-end", "app:install-error",
	"app:apply-changes-begin", "app:apply-changes-end", "app:apply-changes-error",
}

// Event is one message bus event.
type Event struct {
	Name       string            `json:"name"`
	Properties map[string]string `json:"properties"`
}

func (c *Client) busURL() (string, error) {
	if c.BusURL != "" {
		return strings.TrimRight(c.BusURL, "/"), nil
	}
	f := c.BusURLFile
	if f == "" {
		f = DefaultBusURLFile
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return "", err
	}
	u := strings.TrimSpace(string(b))
	if !strings.HasPrefix(u, "http://127.0.0.1:") && !strings.HasPrefix(u, "http://localhost:") {
		return "", fmt.Errorf("unexpected message bus address %q", u)
	}
	return u, nil
}

// Listen follows app events on the message bus until ctx ends, reconnecting
// when the bus restarts (its address changes then; it is read each time).
func (c *Client) Listen(ctx context.Context) {
	wait := time.Second
	for ctx.Err() == nil {
		err := c.listenOnce(ctx)
		c.setLive(false)
		if ctx.Err() != nil {
			return
		}
		log.Printf("[icedash] CasaOS message bus: %v; retrying", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if wait < 30*time.Second {
			wait *= 2
		}
	}
}

func (c *Client) listenOnce(ctx context.Context) error {
	base, err := c.busURL()
	if err != nil {
		return err
	}
	q := url.Values{"names": busEvents} // repeated names=; a comma list is refused (measured)
	ws, err := dialWS(ctx, base+"/v2/message_bus/event/app-management?"+q.Encode())
	if err != nil {
		return err
	}
	defer ws.Close()
	stop := context.AfterFunc(ctx, func() { ws.Close() })
	defer stop()
	c.setLive(true)
	for {
		msg, err := ws.Read()
		if err != nil {
			return err
		}
		var e Event
		if json.Unmarshal(msg, &e) == nil {
			c.Handle(e)
		}
	}
}

func (c *Client) setLive(v bool) {
	c.mu.Lock()
	c.live = v
	c.mu.Unlock()
}

// Handle applies one bus event to the app's running task.
func (c *Client) Handle(e Event) {
	name := e.Properties["app:name"]
	kind, what, ok := strings.Cut(strings.TrimPrefix(e.Name, "app:"), "-")
	if name == "" || !ok {
		return
	}
	if kind == "apply" { // apply-changes-end
		kind, what = "apply", strings.TrimPrefix(what, "changes-")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.current[name]
	if t == nil || t.kind != kind || t.ended {
		return
	}
	switch what {
	case "error":
		if t.errMsg == "" {
			t.errMsg = strings.TrimSpace(e.Properties["message"])
			if t.errMsg == "" {
				t.errMsg = "CasaOS reported an error"
			}
		}
	case "end":
		t.ended = true
	}
}

// ---- actions ----

// Start, Stop and Restart: PUT /compose/{name}/status.
func (c *Client) Start(ctx context.Context, name string) error { return c.status(ctx, name, "start") }
func (c *Client) Stop(ctx context.Context, name string) error  { return c.status(ctx, name, "stop") }
func (c *Client) Restart(ctx context.Context, name string) error {
	return c.status(ctx, name, "restart")
}

func (c *Client) status(ctx context.Context, name, to string) error {
	raw, status, err := c.send(ctx, http.MethodPut, "/compose/"+url.PathEscape(name)+"/status", "application/json", []byte(strconv.Quote(to)))
	if err != nil {
		return err
	}
	if status >= 300 {
		return failure(status, raw)
	}
	return nil
}

// Uninstall removes an app; its data folder stays unless deleteData.
func (c *Client) Uninstall(ctx context.Context, name string, deleteData bool) error {
	raw, status, err := c.send(ctx, http.MethodDelete, "/compose/"+url.PathEscape(name)+"?delete_config_folder="+strconv.FormatBool(deleteData), "", nil)
	if err != nil {
		return err
	}
	if status >= 300 {
		return failure(status, raw)
	}
	return nil
}
