package apps

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Frame says whether an app may be shown in a window inside IceDash. Apps
// refuse with X-Frame-Options or a CSP frame-ancestors list; those open in a
// new tab instead. IceDash and the app are different origins (different
// ports), so "sameorigin" and 'self' refuse too.
type Frame struct {
	Name      string `json:"name"`
	Frameable bool   `json:"frameable"`
	Reason    string `json:"reason,omitempty"`
}

type frameEntry struct {
	f  Frame
	at time.Time
}

// Frameable checks the app's own answer on loopback, following up to five
// redirects on the same port, and remembers the result for 10 minutes.
func (m *Manager) Frameable(ctx context.Context, name string) (Frame, error) {
	m.mu.Lock()
	if e, ok := m.frames[name]; ok && time.Since(e.at) < 10*time.Minute {
		m.mu.Unlock()
		return e.f, nil
	}
	m.mu.Unlock()

	grid, err := m.Zima.AppGrid(ctx)
	if err != nil {
		return Frame{}, err
	}
	var port, index string
	found := false
	for _, g := range grid {
		if g.Name == name {
			port, index, found = string(g.Port), g.Index, true
		}
	}
	if !found {
		return Frame{}, ErrNotInstalled
	}
	f := Frame{Name: name, Frameable: true}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		f.Frameable, f.Reason = false, "the app has no web address"
	} else {
		if !strings.HasPrefix(index, "/") {
			index = "/" + index
		}
		f = checkFrame(ctx, name, "http://127.0.0.1:"+port+index)
	}
	m.mu.Lock()
	if m.frames == nil {
		m.frames = map[string]frameEntry{}
	}
	m.frames[name] = frameEntry{f, time.Now()}
	m.mu.Unlock()
	return f, nil
}

func checkFrame(ctx context.Context, name, start string) Frame {
	hc := &http.Client{
		Timeout:       8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	u, _ := url.Parse(start)
	for hop := 0; hop < 6; hop++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			break
		}
		res, err := hc.Do(req)
		if err != nil {
			// Not answering yet (still starting): let the window try.
			return Frame{Name: name, Frameable: true}
		}
		res.Body.Close()
		if why := refusal(res.Header); why != "" {
			return Frame{Name: name, Frameable: false, Reason: why}
		}
		if res.StatusCode < 300 || res.StatusCode >= 400 {
			break
		}
		next, err := u.Parse(res.Header.Get("Location"))
		if err != nil || portOf(next) != u.Port() {
			break // another port is another app: nothing more to check here
		}
		next.Scheme, next.Host = "http", u.Host // same app: stay on loopback
		u = next
	}
	return Frame{Name: name, Frameable: true}
}

// refusal reads X-Frame-Options and CSP frame-ancestors.
func refusal(h http.Header) string {
	if x := strings.ToLower(strings.TrimSpace(h.Get("X-Frame-Options"))); x != "" {
		return "the app sends X-Frame-Options: " + x
	}
	for _, csp := range h.Values("Content-Security-Policy") {
		for _, d := range strings.Split(csp, ";") {
			f := strings.Fields(strings.ToLower(strings.TrimSpace(d)))
			if len(f) == 0 || f[0] != "frame-ancestors" {
				continue
			}
			for _, src := range f[1:] {
				if src == "*" || src == "http:" || src == "https:" {
					return ""
				}
			}
			return "the app limits frame-ancestors"
		}
	}
	return ""
}

// portOf is the URL's port, with the scheme's default when none is written.
func portOf(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}
