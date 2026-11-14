// Package home opens IceDash at ZimaOS's main address when an admin asks
// (PLAN.md 5b.1).
//
// ZimaOS's gateway is Caddy, and the gateway writes every route it is given
// as `reverse_proxy <path>/* …` (measured on the box, 2026-11-15), so a route
// for "/" would become "//*" and match nothing. Instead IceDash adds one route
// of its own through Caddy's admin socket (/run/caddy-admin.sock, root only):
// the exact path "/" answers a redirect to /modules/icedash/. Every other path
// stays Caddy's, so the stock dashboard keeps working at /index.html and no
// ZimaOS file changes.
//
// Caddy runs with persist_config off and the gateway reloads its Caddyfile
// whenever routes change, which drops the route: IceDash puts it back
// within 10 s while the switch is on. Turning it off deletes the route at
// once.
package home

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Socket is Caddy's admin socket on ZimaOS (HOST-NOTES item 3).
const Socket = "/run/caddy-admin.sock"

// StockPath is where the stock dashboard stays while IceDash has "/".
const StockPath = "/index.html"

// Target is where "/" sends people.
const Target = "/modules/icedash/"

// Marker is the header IceDash's redirect carries, so a check through the
// gateway can tell it is IceDash's.
const Marker = "X-IceDash-Front"

// ID is the route's @id in Caddy's config.
const ID = "icedash-home"

// Caddy talks to Caddy's admin API.
type Caddy struct {
	// Base is the API's address; "" is the unix socket (Socket).
	Base string
	HTTP *http.Client
}

func (c *Caddy) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", Socket)
		}},
	}
}

func (c *Caddy) base() string {
	if c.Base != "" {
		return strings.TrimRight(c.Base, "/")
	}
	return "http://localhost"
}

func (c *Caddy) do(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+path, rd)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return res.StatusCode, b, nil
}

// WebRoot is the gateway's file_server root, and Page is IceDash's page under
// it. The route matches only while Page is on disk, so if IceDash is removed
// without giving "/" back (zpkg remove alone, a crash, an older version's
// uninstall), "/" falls through to ZimaOS instead of redirecting to a 404.
const (
	WebRoot = "/usr/share/casaos/www"
	Page    = Target + "index.html"
)

// Route is IceDash's route: exactly "/" (with or without a query), a
// redirect, while IceDash's page exists. Caddy's `file` matcher is in the
// same module as the gateway's file_server.
func Route() map[string]any {
	return map[string]any{
		"@id": ID,
		"match": []any{map[string]any{
			"path": []string{"/"},
			"file": map[string]any{"root": WebRoot, "try_files": []string{Page}},
		}},
		"handle": []any{map[string]any{
			"handler":     "static_response",
			"status_code": 302,
			"headers": map[string][]string{
				"Location":      {Target},
				Marker:          {"zimaos"},
				"Cache-Control": {"no-store"},
			},
		}},
		"terminal": true,
	}
}

// Present reports whether Caddy has the route now.
func (c *Caddy) Present(ctx context.Context) (bool, error) {
	code, _, err := c.do(ctx, http.MethodGet, "/id/"+ID, nil)
	if err != nil {
		return false, err
	}
	return code == http.StatusOK, nil
}

// server finds the server that listens on the gateway's port.
func (c *Caddy) server(ctx context.Context, port string) (string, error) {
	code, b, err := c.do(ctx, http.MethodGet, "/config/apps/http/servers", nil)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("caddy answered %d for its servers", code)
	}
	var servers map[string]struct {
		Listen []string `json:"listen"`
	}
	if err := json.Unmarshal(b, &servers); err != nil {
		return "", fmt.Errorf("caddy's servers: %w", err)
	}
	for name, s := range servers {
		for _, l := range s.Listen {
			if strings.HasSuffix(l, ":"+port) {
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("no caddy server listens on :%s", port)
}

// Apply puts the route first on the gateway's server, once.
func (c *Caddy) Apply(ctx context.Context, port string) error {
	if ok, err := c.Present(ctx); err != nil || ok {
		return err
	}
	name, err := c.server(ctx, port)
	if err != nil {
		return err
	}
	// PUT on an array index inserts there: first, before the gateway's own.
	code, b, err := c.do(ctx, http.MethodPut, "/config/apps/http/servers/"+url.PathEscape(name)+"/routes/0", Route())
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("caddy answered %d: %s", code, strings.TrimSpace(string(b)))
	}
	return nil
}

// Remove takes the route away; none there is fine.
func (c *Caddy) Remove(ctx context.Context) error {
	code, b, err := c.do(ctx, http.MethodDelete, "/id/"+ID, nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusNotFound && !strings.Contains(string(b), "unknown object ID") {
		return fmt.Errorf("caddy answered %d: %s", code, strings.TrimSpace(string(b)))
	}
	return nil
}

// ErrNotTaken is when "/" through the gateway isn't IceDash's redirect.
var ErrNotTaken = errors.New("ZimaOS's gateway didn't send its main address to IceDash")

// Check asks the gateway for "/" (base is http://127.0.0.1[:port]) and
// reports whether IceDash's redirect answered.
func Check(ctx context.Context, base string, c *http.Client) error {
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Second}
	}
	cc := *c
	cc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err != nil {
		return err
	}
	res, err := cc.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.Header.Get(Marker) == "" || res.Header.Get("Location") != Target {
		return ErrNotTaken
	}
	return nil
}
