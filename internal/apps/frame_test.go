package apps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/HSinghHira/IceDash/internal/zima"
)

func TestRefusal(t *testing.T) {
	for _, c := range []struct {
		h    map[string]string
		want bool // refused
	}{
		{map[string]string{}, false},
		{map[string]string{"X-Frame-Options": "DENY"}, true},
		{map[string]string{"X-Frame-Options": "SAMEORIGIN"}, true},
		{map[string]string{"Content-Security-Policy": "default-src 'self'; frame-ancestors 'self'"}, true},
		{map[string]string{"Content-Security-Policy": "frame-ancestors 'none'"}, true},
		{map[string]string{"Content-Security-Policy": "frame-ancestors *"}, false},
		{map[string]string{"Content-Security-Policy": "default-src 'self'"}, false},
	} {
		h := http.Header{}
		for k, v := range c.h {
			h.Set(k, v)
		}
		if got := refusal(h) != ""; got != c.want {
			t.Errorf("%v: refused=%v", c.h, got)
		}
	}
}

func TestCheckFrameFollowsTheSameApp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			// Absolute, with the LAN address: still this app (same port).
			u, _ := url.Parse("http://" + r.Host)
			http.Redirect(w, r, "http://192.168.68.17:"+u.Port()+"/signin", http.StatusFound)
		case "/signin":
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		case "/other":
			http.Redirect(w, r, "http://192.168.68.17/elsewhere", http.StatusFound)
		}
	}))
	defer srv.Close()
	if f := checkFrame(context.Background(), "x", srv.URL+"/"); f.Frameable {
		t.Errorf("want the sign-in page's refusal, got %+v", f)
	}
	if f := checkFrame(context.Background(), "x", srv.URL+"/other"); !f.Frameable {
		t.Errorf("a redirect to another port isn't this app: %+v", f)
	}
}

func TestFrameableNeedsAnAddress(t *testing.T) {
	m, z, _ := newManager(t)
	z.grid = []zima.Grid{{Name: "noport", Status: "running"}}
	f, err := m.Frameable(context.Background(), "noport")
	if err != nil || f.Frameable {
		t.Errorf("%+v %v", f, err)
	}
	if _, err := m.Frameable(context.Background(), "missing"); err != ErrNotInstalled {
		t.Errorf("missing: %v", err)
	}
}
