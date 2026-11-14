package api

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HSinghHira/IceDash/internal/media"
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

func TestPicturesArePerUser(t *testing.T) {
	e := newEnv(t)
	e.srv.Prefs = prefs.Open(filepath.Join(t.TempDir(), "prefs.json"))
	e.srv.Media = &media.Store{Dir: filepath.Join(t.TempDir(), "media")}
	e.srv.Users = func(ctx context.Context, token string) ([]Account, error) {
		return []Account{{Name: "Hira", Admin: true}, {Name: "testmember"}}, nil
	}
	h := e.srv.Handler()
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("x", 64)
	if r := call(h, "POST", Route+"/me/avatar", member, "<svg onload=alert(1)>"); r.Code != http.StatusBadRequest {
		t.Fatalf("svg: %d", r.Code)
	}
	r := call(h, "POST", Route+"/me/avatar", member, png)
	if r.Code != http.StatusOK || strings.Contains(r.Body.String(), `"avatar":0`) {
		t.Fatalf("avatar: %d %s", r.Code, r.Body)
	}
	r = call(h, "POST", Route+"/me/wallpaper", member, png)
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"kind":"image"`) {
		t.Fatalf("wallpaper: %d %s", r.Code, r.Body)
	}
	if r := call(h, "GET", Route+"/me/wallpaper", member, ""); r.Code != http.StatusOK || r.Header().Get("Content-Type") != "image/png" || r.Body.String() != png {
		t.Fatalf("get: %d %q", r.Code, r.Header().Get("Content-Type"))
	}
	// Someone else's wallpaper isn't theirs to see; avatars are.
	if r := call(h, "GET", Route+"/me/wallpaper", admin, ""); r.Code != http.StatusNotFound {
		t.Fatalf("other wallpaper: %d", r.Code)
	}
	if r := call(h, "GET", Route+"/users/testmember/avatar", admin, ""); r.Code != http.StatusOK {
		t.Fatalf("avatar by admin: %d", r.Code)
	}
	if r := call(h, "GET", Route+"/users/testmember/avatar", "", ""); r.Code != http.StatusUnauthorized {
		t.Fatalf("avatar signed out: %d", r.Code)
	}
	if r := call(h, "GET", Route+"/users", admin, ""); !strings.Contains(r.Body.String(), `{"name":"testmember","admin":false,"avatar":true}`) {
		t.Fatalf("users: %s", r.Body)
	}
	r = call(h, "POST", Route+"/me/wallpaper/remove", member, "")
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"kind":""`) {
		t.Fatalf("remove: %d %s", r.Code, r.Body)
	}
	if r := call(h, "GET", Route+"/me/wallpaper", member, ""); r.Code != http.StatusNotFound {
		t.Fatalf("after remove: %d", r.Code)
	}
	if r := call(h, "POST", Route+"/prefs", member, `{"wallpaper":{"kind":"preset","preset":"aurora","dim":20,"blur":4}}`); r.Code != http.StatusOK {
		t.Fatalf("preset: %d %s", r.Code, r.Body)
	}
	if r := call(h, "POST", Route+"/prefs", member, `{"avatar_style":"lorelei","avatar_seed":"tm 2"}`); r.Code != http.StatusOK {
		t.Fatalf("drawn avatar: %d %s", r.Code, r.Body)
	}
	if r := call(h, "GET", Route+"/users", admin, ""); !strings.Contains(r.Body.String(), `"name":"testmember","admin":false,"avatar":true,"avatar_style":"lorelei","avatar_seed":"tm 2"`) {
		t.Fatalf("users with styles: %s", r.Body)
	}
	for _, bad := range []string{`{"avatar_style":"avataaars"}`, `{"avatar_seed":"<x>"}`, `{"wallpaper":{"kind":"preset","preset":"nope"}}`, `{"wallpaper":{"kind":"color","color":"red"}}`, `{"wallpaper":{"dim":99}}`, `{"wallpaper":{"fit":"stretch"}}`} {
		if r := call(h, "POST", Route+"/prefs", member, bad); r.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, r.Code)
		}
	}
}
