package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/HSinghHira/IceDash/internal/system"
)

func token(payload string) string {
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(`{"alg":"ES256"}`)) + "." + enc.EncodeToString([]byte(payload)) + ".sig"
}

func server(t *testing.T) http.Handler {
	t.Helper()
	s := &Server{
		Version: "0.1.0",
		Arch:    "amd64",
		Started: time.Date(2026, 11, 11, 8, 0, 0, 0, time.UTC),
		Verify:  session.Disabled{},
		System:  &system.Sampler{Root: t.TempDir()},
	}
	return s.Handler()
}

func get(h http.Handler, path, auth string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHealthNeedsNoToken(t *testing.T) {
	w := get(server(t), Route+"/health", "")
	if w.Code != 200 {
		t.Fatalf("health: %d %s", w.Code, w.Body)
	}
	var h Health
	if err := json.Unmarshal(w.Body.Bytes(), &h); err != nil {
		t.Fatal(err)
	}
	if h.Version != "0.1.0" || h.Arch != "amd64" || h.Started.IsZero() {
		t.Fatalf("health %+v", h)
	}
}

// Acceptance check 10: every endpoint except /health answers 401 without a token.
func TestEverythingElseNeedsAToken(t *testing.T) {
	h := server(t)
	for _, p := range []string{Route + "/system", Route + "/notifications", Route + "/nope", Route + "/"} {
		if w := get(h, p, ""); w.Code != 401 {
			t.Errorf("%s without a token: %d", p, w.Code)
		}
	}
	tok := "Bearer " + token(`{"username":"Hira","role":"admin","id":1}`)
	if w := get(h, Route+"/system", tok); w.Code != 200 {
		t.Errorf("system with a token: %d %s", w.Code, w.Body)
	}
}

func TestCrossOriginPostRefused(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, Route+"/update/check", nil)
	r.Host = "192.168.68.17"
	r.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	server(t).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross-origin POST: %d", w.Code)
	}
}
