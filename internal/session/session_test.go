package session

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// token builds a JWT-shaped string with this payload (the signature is fake;
// the verifiers in these tests don't check it).
func token(payload string) string {
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(`{"alg":"ES256"}`)) + "." + enc.EncodeToString([]byte(payload)) + ".sig"
}

// The payloads ZimaOS 1.8.0 issued for an admin and a member (HOST-NOTES item 5).
var (
	adminToken  = token(`{"username":"Hira","role":"admin","id":1,"iss":"zimaos","exp":1794392503}`)
	memberToken = token(`{"username":"testmember","role":"user","id":10000,"iss":"zimaos","exp":1794398489}`)
)

type verifier struct{ err error }

func (v verifier) Verify(string) error { return v.err }

func serve(h http.Handler, auth string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/v2/icedash/x", nil)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestMiddleware(t *testing.T) {
	var got Claims
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = FromContext(r.Context())
	})
	h := Middleware(verifier{})(ok)

	if w := serve(h, ""); w.Code != 401 || !strings.Contains(w.Body.String(), "auth_required") {
		t.Fatalf("no token: %d %s", w.Code, w.Body)
	}
	if w := serve(h, "Bearer "+adminToken); w.Code != 200 || got.Username != "Hira" || !got.Admin() {
		t.Fatalf("bearer admin: %d %+v", w.Code, got)
	}
	// ZimaOS's own UI sends the bare form
	if w := serve(h, memberToken); w.Code != 200 || got.Username != "testmember" || got.Admin() {
		t.Fatalf("bare member: %d %+v", w.Code, got)
	}

	bad := Middleware(verifier{err: errors.New("token expired")})(ok)
	if w := serve(bad, "Bearer "+adminToken); w.Code != 401 || !strings.Contains(w.Body.String(), "auth_invalid") {
		t.Fatalf("rejected token: %d %s", w.Code, w.Body)
	}
	if w := serve(h, "Bearer not-a-jwt"); w.Code != 401 {
		t.Fatalf("garbage token passed: %d", w.Code)
	}
}

func TestRequireAdmin(t *testing.T) {
	h := Middleware(verifier{})(RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	if w := serve(h, "Bearer "+adminToken); w.Code != 200 {
		t.Fatalf("admin: %d", w.Code)
	}
	w := serve(h, "Bearer "+memberToken)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "admin_only") {
		t.Fatalf("member: %d %s", w.Code, w.Body)
	}
}
