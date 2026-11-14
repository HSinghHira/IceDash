package api

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HSinghHira/IceDash/internal/mods"
	"github.com/HSinghHira/IceDash/internal/session"
)

func TestModsMembersAndAdmins(t *testing.T) {
	dir := t.TempDir()
	s := &Server{Verify: session.Disabled{}, Mods: &mods.Service{
		Dir: filepath.Join(dir, "mods"), State: filepath.Join(dir, "mods.json"), CatalogURL: "http://127.0.0.1:1/index.json",
	}}
	h := s.Handler()
	// Everyone signed in reads and chooses for themselves.
	if w := call(h, "GET", "/v2/icedash/mods", member, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"panel":[]`) {
		t.Fatalf("member list: %d %s", w.Code, w.Body)
	}
	if w := call(h, "POST", "/v2/icedash/mods/theme", member, `{"id":""}`); w.Code != http.StatusOK {
		t.Fatalf("member back to Ice: %d %s", w.Code, w.Body)
	}
	if w := call(h, "POST", "/v2/icedash/mods/theme", member, `{"id":"nope"}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown theme: %d", w.Code)
	}
	// Changing what's installed is for admins.
	for _, c := range []struct{ path, body string }{
		{"/v2/icedash/mods/inspect", `{"source":"weather"}`},
		{"/v2/icedash/mods/install", `{"source":"weather"}`},
		{"/v2/icedash/mods/weather/remove", ``},
		{"/v2/icedash/mods/weather/enabled", `{"enabled":false}`},
		{"/v2/icedash/mods/catalog/refresh", ``},
	} {
		if w := call(h, "POST", c.path, member, c.body); w.Code != http.StatusForbidden {
			t.Errorf("member %s: %d", c.path, w.Code)
		}
	}
	if w := call(h, "POST", "/v2/icedash/mods/install", admin, `{"source":"https://evil.example/x/y"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad address: %d %s", w.Code, w.Body)
	}
	// A widget's files need no sign-in, but only installed, switched-on widgets have any.
	if w := call(h, "GET", "/v2/icedash/mods/weather/files/index.html", "", ""); w.Code != http.StatusNotFound {
		t.Fatalf("missing widget: %d", w.Code)
	}
}
