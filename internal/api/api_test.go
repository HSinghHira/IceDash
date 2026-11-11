package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HSinghHira/IceDash/internal/notify"
	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/HSinghHira/IceDash/internal/settings"
	"github.com/HSinghHira/IceDash/internal/system"
	"github.com/HSinghHira/IceDash/internal/update"
)

func token(payload string) string {
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(`{"alg":"ES256"}`)) + "." + enc.EncodeToString([]byte(payload)) + ".sig"
}

var (
	admin  = "Bearer " + token(`{"username":"Hira","role":"admin","id":1}`)
	member = "Bearer " + token(`{"username":"testmember","role":"user","id":10000}`)
)

type fakeUpdater struct {
	st      update.Status
	applied int
	err     error
	checked int
}

func (f *fakeUpdater) Status() update.Status { return f.st }
func (f *fakeUpdater) Check(context.Context) update.Status {
	f.checked++
	return f.st
}
func (f *fakeUpdater) Apply() error {
	if f.err != nil {
		return f.err
	}
	f.applied++
	return nil
}

type env struct {
	h     http.Handler
	up    *fakeUpdater
	notes *notify.Store
}

func newEnv(t *testing.T) env {
	t.Helper()
	dir := t.TempDir()
	notes, err := notify.Open(filepath.Join(dir, "notifications.json"))
	if err != nil {
		t.Fatal(err)
	}
	up := &fakeUpdater{st: update.Status{Current: "0.1.0", State: update.StateAvailable, Latest: "v0.2.0", Steps: []update.Step{}}}
	s := &Server{
		Version:  "0.1.0",
		Arch:     "amd64",
		Started:  time.Date(2026, 11, 11, 8, 0, 0, 0, time.UTC),
		Verify:   session.Disabled{},
		System:   &system.Sampler{Root: dir},
		Update:   up,
		Notes:    notes,
		Settings: settings.Open(filepath.Join(dir, "settings.json")),
	}
	return env{h: s.Handler(), up: up, notes: notes}
}

func call(h http.Handler, method, path, auth, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Host = "192.168.68.17"
	if method == http.MethodPost {
		r.Header.Set("Origin", "http://192.168.68.17") // what a browser on the box sends
	}
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHealthNeedsNoToken(t *testing.T) {
	w := call(newEnv(t).h, "GET", Route+"/health", "", "")
	var h Health
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &h) != nil || h.Version != "0.1.0" || h.Arch != "amd64" || h.Started.IsZero() {
		t.Fatalf("health: %d %s", w.Code, w.Body)
	}
}

// Acceptance check 10: every endpoint except /health answers 401 without a token.
func TestEverythingElseNeedsAToken(t *testing.T) {
	h := newEnv(t).h
	for _, c := range []struct{ method, path string }{
		{"GET", "/system"}, {"GET", "/notifications"}, {"POST", "/notifications/read"},
		{"POST", "/notifications/clear"}, {"GET", "/update"}, {"POST", "/update/check"},
		{"POST", "/update/apply"}, {"GET", "/settings"}, {"POST", "/settings"},
		{"GET", "/nope"}, {"GET", "/"},
	} {
		if w := call(h, c.method, Route+c.path, "", ""); w.Code != 401 {
			t.Errorf("%s %s without a token: %d", c.method, c.path, w.Code)
		}
	}
}

// Acceptance check 10: a member sees the update but can't install it.
func TestMemberSeesButCantUpdate(t *testing.T) {
	e := newEnv(t)
	if w := call(e.h, "GET", Route+"/update", member, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"latest":"v0.2.0"`) {
		t.Fatalf("member status: %d %s", w.Code, w.Body)
	}
	for _, p := range []string{"/update/apply", "/update/check"} {
		w := call(e.h, "POST", Route+p, member, "")
		if w.Code != 403 || !strings.Contains(w.Body.String(), "admin_only") {
			t.Fatalf("member %s: %d %s", p, w.Code, w.Body)
		}
	}
	if e.up.applied != 0 || e.up.checked != 0 {
		t.Fatal("a member's request reached the updater")
	}
	if w := call(e.h, "POST", Route+"/settings", member, `{"auto_check":false}`); w.Code != 403 {
		t.Fatalf("member settings: %d", w.Code)
	}
}

func TestAdminUpdates(t *testing.T) {
	e := newEnv(t)
	if w := call(e.h, "POST", Route+"/update/check", admin, ""); w.Code != 200 || e.up.checked != 1 {
		t.Fatalf("check: %d", w.Code)
	}
	if w := call(e.h, "POST", Route+"/update/apply", admin, ""); w.Code != 202 || e.up.applied != 1 {
		t.Fatalf("apply: %d %s", w.Code, w.Body)
	}
	e.up.err = update.ErrBusy
	if w := call(e.h, "POST", Route+"/update/apply", admin, ""); w.Code != 409 || !strings.Contains(w.Body.String(), `"busy"`) {
		t.Fatalf("busy: %d %s", w.Code, w.Body)
	}
	e.up.err = update.ErrNothingToDo
	if w := call(e.h, "POST", Route+"/update/apply", admin, ""); w.Code != 409 || !strings.Contains(w.Body.String(), "nothing_to_install") {
		t.Fatalf("nothing: %d %s", w.Code, w.Body)
	}
}

func TestNotificationsPerUser(t *testing.T) {
	e := newEnv(t)
	a, _, _ := e.notes.Add(notify.Notification{Kind: notify.KindUpdateAvailable, Title: "Update available"})
	e.notes.Add(notify.Notification{Kind: notify.KindUpdated, Title: "Updated"})

	var list NotificationList
	w := call(e.h, "GET", Route+"/notifications", admin, "")
	json.Unmarshal(w.Body.Bytes(), &list)
	if w.Code != 200 || len(list.Items) != 2 || list.Unread != 2 {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	w = call(e.h, "GET", Route+"/notifications?since="+itoa(a.ID), admin, "")
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Fatalf("since: %s", w.Body)
	}
	if w := call(e.h, "GET", Route+"/notifications?since=x", admin, ""); w.Code != 400 {
		t.Fatalf("bad since: %d", w.Code)
	}

	w = call(e.h, "POST", Route+"/notifications/read", admin, `{"id":`+itoa(a.ID)+`}`)
	json.Unmarshal(w.Body.Bytes(), &list)
	if w.Code != 200 || list.Unread != 1 {
		t.Fatalf("read one: %d %s", w.Code, w.Body)
	}
	call(e.h, "POST", Route+"/notifications/read", admin, "") // all
	call(e.h, "POST", Route+"/notifications/clear", admin, "")
	w = call(e.h, "GET", Route+"/notifications", admin, "")
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 0 {
		t.Fatalf("after clear: %s", w.Body)
	}
	// the member's list is untouched
	w = call(e.h, "GET", Route+"/notifications", member, "")
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 2 || list.Unread != 2 {
		t.Fatalf("member: %s", w.Body)
	}
}

func TestSettings(t *testing.T) {
	e := newEnv(t)
	if w := call(e.h, "GET", Route+"/settings", member, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"auto_check":true`) {
		t.Fatalf("default: %d %s", w.Code, w.Body)
	}
	if w := call(e.h, "POST", Route+"/settings", admin, `{"auto_check":false}`); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	if w := call(e.h, "GET", Route+"/settings", member, ""); !strings.Contains(w.Body.String(), `"auto_check":false`) {
		t.Fatalf("after save: %s", w.Body)
	}
}

func TestCrossOriginPostRefused(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, Route+"/update/apply", nil)
	r.Host = "192.168.68.17"
	r.Header.Set("Origin", "http://evil.example")
	r.Header.Set("Authorization", admin)
	w := httptest.NewRecorder()
	e := newEnv(t)
	e.h.ServeHTTP(w, r)
	if w.Code != 403 || e.up.applied != 0 {
		t.Fatalf("cross-origin POST: %d", w.Code)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
