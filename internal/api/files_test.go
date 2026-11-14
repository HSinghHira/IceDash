package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HSinghHira/IceDash/internal/files"
	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/HSinghHira/IceDash/internal/term"
)

func filesEnv(t *testing.T) (http.Handler, string) {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Documents"), 0o755)
	os.WriteFile(filepath.Join(root, "Documents", "page.html"), []byte("<script>alert(1)</script>"), 0o644)
	os.WriteFile(filepath.Join(root, "Documents", "clip.mp4"), []byte("0123456789"), 0o644)
	s := &Server{
		Verify: session.Disabled{},
		Files:  &files.Service{Root: root},
		Term:   &term.Service{Check: func(_ context.Context, u, p string) (bool, error) { return p == "right", nil }},
	}
	return s.Handler(), root
}

func TestFilesAreForAdminsOnly(t *testing.T) {
	h, _ := filesEnv(t)
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/v2/icedash/files/list?path=/", ""},
		{"POST", "/v2/icedash/files/ticket", `{"paths":["/Documents/clip.mp4"]}`},
		{"POST", "/v2/icedash/files/folder", `{"path":"/","name":"x"}`},
		{"POST", "/v2/icedash/files/delete", `{"paths":["/Documents"]}`},
		{"GET", "/v2/icedash/files/trash", ""},
		{"POST", "/v2/icedash/terminal/ticket", `{"password":"right"}`},
	} {
		if w := call(h, c.method, c.path, member, c.body); w.Code != http.StatusForbidden {
			t.Errorf("member %s %s: %d", c.method, c.path, w.Code)
		}
		if w := call(h, c.method, c.path, "", c.body); w.Code != http.StatusUnauthorized {
			t.Errorf("signed out %s %s: %d", c.method, c.path, w.Code)
		}
	}
	if w := call(h, "GET", "/v2/icedash/files/list?path=/Documents", admin, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "page.html") {
		t.Fatalf("admin list: %d %s", w.Code, w.Body)
	}
}

func ticketFor(t *testing.T, h http.Handler, p string) string {
	t.Helper()
	w := call(h, "POST", "/v2/icedash/files/ticket", admin, `{"paths":["`+p+`"]}`)
	var v struct{ Ticket string }
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &v) != nil || v.Ticket == "" {
		t.Fatalf("ticket: %d %s", w.Code, w.Body)
	}
	return v.Ticket
}

func TestRawNeverRunsAPage(t *testing.T) {
	h, _ := filesEnv(t)
	id := ticketFor(t, h, "/Documents/page.html")
	w := call(h, "GET", "/v2/icedash/files/raw?ticket="+id, "", "")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/plain; charset=utf-8" ||
		!strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	w = call(h, "GET", "/v2/icedash/files/raw?ticket="+id+"&download=1", "", "")
	if !strings.HasPrefix(w.Header().Get("Content-Disposition"), `attachment; filename="page.html"`) {
		t.Fatalf("download: %v", w.Header())
	}
	if w := call(h, "GET", "/v2/icedash/files/raw?ticket=forged", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("forged ticket: %d", w.Code)
	}
}

func TestRawRanges(t *testing.T) {
	h, _ := filesEnv(t)
	id := ticketFor(t, h, "/Documents/clip.mp4")
	r := httptest.NewRequest("GET", "/v2/icedash/files/raw?ticket="+id, nil)
	r.Header.Set("Range", "bytes=2-5")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusPartialContent || w.Body.String() != "2345" || w.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("%d %q %v", w.Code, w.Body, w.Header())
	}
}

func TestUploadAndClashOverHTTP(t *testing.T) {
	h, root := filesEnv(t)
	up := func(clash string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/v2/icedash/files/upload?path=/Documents&name=a.txt&clash="+clash, strings.NewReader("hello"))
		r.Header.Set("Authorization", admin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := up(""); w.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "Documents", "a.txt")); string(b) != "hello" {
		t.Fatalf("on disk: %q", b)
	}
	if w := up(""); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"code":"exists"`) {
		t.Fatalf("clash: %d %s", w.Code, w.Body)
	}
	w := call(h, "POST", "/v2/icedash/files/copy", admin, `{"paths":["/Documents/a.txt"],"to":"/Documents"}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"names":["a.txt"]`) {
		t.Fatalf("copy clash: %d %s", w.Code, w.Body)
	}
}

func TestTerminalTicketOverHTTP(t *testing.T) {
	h, _ := filesEnv(t)
	if w := call(h, "POST", "/v2/icedash/terminal/ticket", admin, `{"password":"wrong"}`); w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "wrong_password") {
		t.Fatalf("wrong: %d %s", w.Code, w.Body)
	}
	if w := call(h, "POST", "/v2/icedash/terminal/ticket", admin, `{"password":"right"}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "ticket") {
		t.Fatalf("right: %d %s", w.Code, w.Body)
	}
}
