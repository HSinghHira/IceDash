package api

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HSinghHira/IceDash/internal/prefs"
)

func TestPrefsArePerUser(t *testing.T) {
	e := newEnv(t)
	e.srv.Prefs = prefs.Open(filepath.Join(t.TempDir(), "prefs.json"))
	h := e.srv.Handler()
	if r := call(h, "POST", Route+"/prefs", member, `{"clock":"24","dock":{"pinned":["gitsave"]}}`); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"clock":"24"`) {
		t.Fatalf("member save: %d %s", r.Code, r.Body)
	}
	// Part of the choices: the rest keeps its saved value.
	if r := call(h, "POST", Route+"/prefs", member, `{"hide_panel":true}`); !strings.Contains(r.Body.String(), `"clock":"24"`) || !strings.Contains(r.Body.String(), `"gitsave"`) {
		t.Fatalf("partial save lost values: %s", r.Body)
	}
	if r := call(h, "GET", Route+"/prefs", admin, ""); !strings.Contains(r.Body.String(), `"clock":""`) {
		t.Fatalf("another user's choices leaked: %s", r.Body)
	}
	if r := call(h, "POST", Route+"/prefs", member, `{"clock":"25"}`); r.Code != http.StatusBadRequest {
		t.Fatalf("bad value: %d", r.Code)
	}
	if r := call(h, "GET", Route+"/prefs", "", ""); r.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", r.Code)
	}
}

func TestNightSettingNeedsAZoneAndAnAdmin(t *testing.T) {
	e := newEnv(t)
	h := e.srv.Handler()
	if r := call(h, "POST", Route+"/settings", member, `{"auto_install":true,"zone":"Pacific/Auckland"}`); r.Code != http.StatusForbidden {
		t.Fatalf("member: %d", r.Code)
	}
	if r := call(h, "POST", Route+"/settings", admin, `{"auto_install":true}`); r.Code != http.StatusBadRequest {
		t.Fatalf("no zone: %d", r.Code)
	}
	if r := call(h, "POST", Route+"/settings", admin, `{"auto_install":true,"zone":"Mars/Base"}`); r.Code != http.StatusBadRequest {
		t.Fatalf("bad zone: %d", r.Code)
	}
	r := call(h, "POST", Route+"/settings", admin, `{"auto_install":true,"zone":"Pacific/Auckland"}`)
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"auto_check":true`) || !strings.Contains(r.Body.String(), `"zone":"Pacific/Auckland"`) {
		t.Fatalf("admin: %d %s", r.Code, r.Body)
	}
}

func TestUsersList(t *testing.T) {
	e := newEnv(t)
	e.srv.Users = func(ctx context.Context, token string) ([]Account, error) {
		if token == "" {
			t.Error("no token passed on")
		}
		return []Account{{Name: "Hira", Admin: true}, {Name: "testmember"}}, nil
	}
	r := call(e.srv.Handler(), "GET", Route+"/users", member, "")
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `{"name":"testmember","admin":false}`) {
		t.Fatalf("%d %s", r.Code, r.Body)
	}
}
