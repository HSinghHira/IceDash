package home

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func www(t *testing.T) string {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "js"), 0o755)
	os.MkdirAll(filepath.Join(d, "modules", "other"), 0o755)
	os.WriteFile(filepath.Join(d, "index.html"), []byte("<html>stock</html>"), 0o644)
	os.WriteFile(filepath.Join(d, "js", "app.js"), []byte("console.log(1)"), 0o644)
	os.WriteFile(filepath.Join(filepath.Dir(d), "secret"), []byte("no"), 0o644)
	return d
}

func get(h http.Handler, p string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
	return w
}

func TestFront(t *testing.T) {
	on := true
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("icedash " + r.URL.Path)) })
	f := &Front{WWW: www(t), Next: next, On: func() bool { return on }}
	if w := get(f, "/"); w.Code != http.StatusFound || w.Header().Get("Location") != "/modules/icedash/" || w.Header().Get(Marker) == "" {
		t.Fatalf("/ with IceDash on: %d %v", w.Code, w.Header())
	}
	if w := get(f, "/modules/icedash/index.html"); w.Body.String() != "icedash /modules/icedash/index.html" {
		t.Fatalf("own files: %s", w.Body)
	}
	if w := get(f, "/zimaos/"); !strings.Contains(w.Body.String(), "stock") || w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("stock page: %d %s", w.Code, w.Body)
	}
	if w := get(f, "/zimaos"); w.Header().Get("Location") != "/zimaos/" {
		t.Fatal("no slash")
	}
	// The stock page's files at their own addresses, and under /zimaos/ too.
	for _, p := range []string{"/js/app.js", "/zimaos/js/app.js"} {
		if w := get(f, p); w.Body.String() != "console.log(1)" || !strings.Contains(w.Header().Get("Content-Type"), "javascript") {
			t.Fatalf("%s: %d %q", p, w.Code, w.Header().Get("Content-Type"))
		}
	}
	for _, p := range []string{"/js/", "/modules/other/", "/../secret", "/zimaos/../../secret", "/nope.js"} {
		if w := get(f, p); w.Code != http.StatusNotFound {
			t.Fatalf("%s: %d %s", p, w.Code, w.Body)
		}
	}
	// Turned off: "/" is the stock page again, as before.
	on = false
	if w := get(f, "/"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "stock") {
		t.Fatalf("/ off: %d", w.Code)
	}
	w := httptest.NewRecorder()
	f.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/js/app.js", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("POST: %d", w.Code)
	}
	if Ready(f.WWW) != nil || Ready(t.TempDir()) == nil {
		t.Fatal("ready")
	}
}

func TestCheck(t *testing.T) {
	f := &Front{WWW: www(t), Next: http.NotFoundHandler(), On: func() bool { return true }}
	ours := httptest.NewServer(f)
	defer ours.Close()
	if err := Check(context.Background(), ours.URL, nil); err != nil {
		t.Fatal(err)
	}
	theirs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("caddy")) }))
	defer theirs.Close()
	if err := Check(context.Background(), theirs.URL, nil); err != ErrNotTaken {
		t.Fatalf("not ours: %v", err)
	}
}
