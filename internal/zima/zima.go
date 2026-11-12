// Package zima talks to ZimaOS's app management, the same calls the stock App
// Store makes (read from its web bundle and tried on ZimaOS 1.8.0; see
// docs/HOST-NOTES.md, "Phase 2: verify first").
//
// The service listens on loopback at the address in
// /var/run/casaos/app-management.url and trusts local callers, so IceDash
// checks the user's role itself before any change (api.RequireAdmin). The
// user's own token is passed along anyway.
package zima

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultURLFile holds app management's address (world-readable, measured).
const DefaultURLFile = "/var/run/casaos/app-management.url"

// Client calls app management.
type Client struct {
	URLFile string // default DefaultURLFile
	Base    string // tests: used instead of the file
	HTTP    *http.Client
}

// ZimaAppID is how ZimaOS 1.8 names a v2 app.
func ZimaAppID(name string) string { return "zimaapp://v2app/" + name }

type tokenKey struct{}

// WithToken passes the signed-in user's token along with calls made with ctx.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey{}, token)
}

func (c *Client) base() (string, error) {
	if c.Base != "" {
		return strings.TrimRight(c.Base, "/") + "/v2/app_management", nil
	}
	f := c.URLFile
	if f == "" {
		f = DefaultURLFile
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return "", fmt.Errorf("ZimaOS app management isn't running: %w", err)
	}
	u := strings.TrimSpace(string(b))
	if !strings.HasPrefix(u, "http://127.0.0.1:") && !strings.HasPrefix(u, "http://localhost:") {
		return "", fmt.Errorf("unexpected app management address %q", u)
	}
	return u + "/v2/app_management", nil
}

// Error is an answer app management refused.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("ZimaOS answered %d", e.Status)
	}
	return e.Message
}

func (c *Client) do(ctx context.Context, method, path string, body any, ctype string, out any) error {
	base, err := c.base()
	if err != nil {
		return err
	}
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		rd = bytes.NewReader(b)
	default:
		j, err := json.Marshal(b)
		if err != nil {
			return err
		}
		rd, ctype = bytes.NewReader(j), "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, rd)
	if err != nil {
		return err
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if t, _ := ctx.Value(tokenKey{}).(string); t != "" {
		req.Header.Set("Authorization", t)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 2 * time.Minute}
	}
	res, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("ZimaOS app management: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
			Data    struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.Data.Message
		if msg == "" {
			msg = e.Message
		}
		return &Error{Status: res.StatusCode, Message: msg}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("ZimaOS app management: %w", err)
		}
	}
	return nil
}

// Installed is one installed app as app management reports it.
type Installed struct {
	Name    string `json:"name"`
	StoreID string `json:"store_id"` // x-casaos.id, com.icestore.* for IceStore apps
	Version string `json:"version"`
	Title   string `json:"title"`
	// Compose is the installed file as app management keeps it (JSON form).
	Compose json.RawMessage `json:"-"`
}

// List returns every installed compose app (GET /compose).
func (c *Client) List(ctx context.Context) (map[string]Installed, error) {
	var res struct {
		Data map[string]struct {
			Compose json.RawMessage `json:"compose"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/compose", nil, "", &res); err != nil {
		return nil, err
	}
	out := make(map[string]Installed, len(res.Data))
	for name, v := range res.Data {
		var x struct {
			X struct {
				ID      string            `json:"id"`
				Version string            `json:"version"`
				Title   map[string]string `json:"title"`
			} `json:"x-casaos"`
		}
		_ = json.Unmarshal(v.Compose, &x)
		title := x.X.Title["custom"]
		if title == "" {
			title = x.X.Title["en_US"]
		}
		out[name] = Installed{Name: name, StoreID: x.X.ID, Version: x.X.Version, Title: title, Compose: v.Compose}
	}
	return out, nil
}

// InstallResult is the answer to an install or its dry run.
type InstallResult struct {
	Accepted    bool                `json:"accepted"`
	TaskID      string              `json:"task_id"`
	ZimaAppID   string              `json:"zimaapp_id"`
	AppID       string              `json:"app_id"`
	Rebuilt     bool                `json:"rebuilt"`
	PortChanges []map[string]any    `json:"port_changes"`
	PortsInUse  map[string][]string `json:"ports_in_use"`
}

// ErrPortsInUse comes with InstallResult.PortsInUse filled in.
var ErrPortsInUse = errors.New("some ports are in use")

// Install hands a compose file to ZimaOS (POST /zimaapp/install, inline
// source). dryRun only checks the file and its ports. An existing app of the
// same name is rebuilt with the new file, which is how IceStore apps update.
func (c *Client) Install(ctx context.Context, yaml []byte, icon string, dryRun bool) (InstallResult, error) {
	src := map[string]any{"kind": "inline", "baseYAML": string(yaml)}
	if icon != "" {
		src["icon"] = icon
	}
	body := map[string]any{
		"type": "v2app",
		"v2app": map[string]any{
			"source": src,
			"options": map[string]any{
				"dryRun": dryRun, "checkPortConflict": true, "failureClean": false, "sync": dryRun,
			},
		},
	}
	raw, status, err := c.raw(ctx, http.MethodPost, "/zimaapp/install", body)
	if err != nil {
		return InstallResult{}, err
	}
	var res struct {
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	_ = json.Unmarshal(raw, &res)
	var out InstallResult
	_ = json.Unmarshal(res.Data, &out)
	// A port clash: {"data":{"ports_in_use":{"TCP":["6101"],"UDP":[]}}} (measured).
	if n := len(out.PortsInUse["TCP"]) + len(out.PortsInUse["UDP"]); n > 0 {
		return out, ErrPortsInUse
	}
	if status >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(res.Data, &e)
		if e.Message == "" {
			e.Message = res.Message
		}
		return out, &Error{Status: status, Message: e.Message}
	}
	return out, nil
}

func (c *Client) raw(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	base, err := c.base()
	if err != nil {
		return nil, 0, err
	}
	j, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(j))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if t, _ := ctx.Value(tokenKey{}).(string); t != "" {
		req.Header.Set("Authorization", t)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 2 * time.Minute}
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("ZimaOS app management: %w", err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return b, res.StatusCode, err
}

// Task is a ZimaOS app task (install, start, stop, update…).
type Task struct {
	ID         string     `json:"id"`
	AppID      string     `json:"app_id"`
	Type       string     `json:"type"`
	State      string     `json:"state"` // running, succeeded, failed, canceled, superseded, interrupted
	Phase      string     `json:"phase"`
	Error      string     `json:"error"`
	FinishedAt *time.Time `json:"finished_at"`
}

// Done reports whether the task has finished, one way or another.
func (t Task) Done() bool {
	switch t.State {
	case "succeeded", "failed", "canceled", "superseded", "interrupted":
		return true
	}
	return false
}

// Task reads one task (GET /zimaapp/v2app/task/query?id=).
func (c *Client) Task(ctx context.Context, id string) (Task, error) {
	var res struct {
		Data Task `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/zimaapp/v2app/task/query?id="+url.QueryEscape(id), nil, "", &res)
	return res.Data, err
}

// Start and Stop take app names (POST /zimaapp/v2app/start|stop).
func (c *Client) Start(ctx context.Context, name string) error {
	return c.batch(ctx, "/zimaapp/v2app/start", name)
}

func (c *Client) Stop(ctx context.Context, name string) error {
	return c.batch(ctx, "/zimaapp/v2app/stop", name)
}

func (c *Client) batch(ctx context.Context, path, name string) error {
	var res struct {
		Data struct {
			Items []struct {
				Error any `json:"error"`
			} `json:"items"`
			Summary struct {
				Failed int `json:"failed"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, path, map[string]any{"zimaapp_ids": []string{ZimaAppID(name)}, "sync": false}, "", &res); err != nil {
		return err
	}
	if res.Data.Summary.Failed > 0 {
		for _, it := range res.Data.Items {
			if it.Error != nil {
				return &Error{Status: http.StatusConflict, Message: fmt.Sprint(it.Error)}
			}
		}
		return &Error{Status: http.StatusConflict, Message: "ZimaOS couldn't do that"}
	}
	return nil
}

// Restart restarts an app (POST /zimaapp/v2app/restart?zimaapp_id=&sync=false).
func (c *Client) Restart(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodPost, "/zimaapp/v2app/restart?zimaapp_id="+url.QueryEscape(ZimaAppID(name))+"&sync=false", nil, "", nil)
}

// Uninstall removes an app (POST /zimaapp/uninstall). The data folder stays
// unless deleteData is set.
func (c *Client) Uninstall(ctx context.Context, name string, deleteData bool) error {
	return c.do(ctx, http.MethodPost, "/zimaapp/uninstall", map[string]any{
		"zimaapp_id": ZimaAppID(name),
		"options":    map[string]any{"delete_app_data": deleteData, "sync": false},
	}, "", nil)
}

// Logs returns the last lines of an app's log (GET /compose/{id}/logs?lines=).
func (c *Client) Logs(ctx context.Context, name string, lines int) (string, error) {
	var res struct {
		Data string `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/compose/"+url.PathEscape(name)+"/logs?lines="+strconv.Itoa(lines), nil, "", &res)
	return res.Data, err
}

// Grid is one app grid entry: what the desktop shows, with its state.
type Grid struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	AppType string `json:"app_type"`
}

// AppGrid returns the app grid (GET /web/appgrid), for the status watcher.
func (c *Client) AppGrid(ctx context.Context) ([]Grid, error) {
	var res struct {
		Data []Grid `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/web/appgrid", nil, "", &res)
	return res.Data, err
}
