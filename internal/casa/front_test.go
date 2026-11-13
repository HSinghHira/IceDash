package casa

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeGateway is the gateway's management API: it keeps routes.json as CasaOS does.
type fakeGateway struct {
	mu     sync.Mutex
	dir    string
	routes map[string]string
	posts  int
}

func (g *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/gateway/routes" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var v struct{ Path, Target string }
	_ = json.NewDecoder(r.Body).Decode(&v)
	g.mu.Lock()
	g.routes[v.Path] = v.Target
	g.posts++
	b, _ := json.Marshal(g.routes)
	g.mu.Unlock()
	_ = os.WriteFile(filepath.Join(g.dir, "routes.json"), b, 0o600)
	w.WriteHeader(http.StatusCreated)
}

func setup(t *testing.T) (*fakeGateway, string, *httptest.Server) {
	dir := t.TempDir()
	stock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "stock "+r.URL.Path)
	}))
	t.Cleanup(stock.Close)
	g := &fakeGateway{dir: dir, routes: map[string]string{"/": stock.URL, "/v1/users": "http://127.0.0.1:1"}}
	gw := httptest.NewServer(g)
	t.Cleanup(gw.Close)
	for name, v := range map[string]string{"management.url": gw.URL, "static.url": stock.URL} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(v+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := json.Marshal(g.routes)
	_ = os.WriteFile(filepath.Join(dir, "routes.json"), b, 0o600)
	return g, dir, stock
}

func TestKeeperTakesOverAndGivesBack(t *testing.T) {
	g, dir, stock := setup(t)
	k := &Keeper{RuntimePath: dir, Target: "http://127.0.0.1:47000"}
	ctx := context.Background()

	changed, err := k.Ensure(ctx)
	if err != nil || !changed {
		t.Fatalf("%v %v", changed, err)
	}
	for _, p := range Routes {
		if g.routes[p] != k.Target {
			t.Fatalf("%s -> %s", p, g.routes[p])
		}
	}
	if g.routes["/v1/users"] != "http://127.0.0.1:1" {
		t.Fatal("CasaOS's own routes must stay")
	}
	posts := g.posts
	if changed, _ := k.Ensure(ctx); changed || g.posts != posts {
		t.Fatal("nothing to do the second time")
	}

	// A gateway restart: "/" goes back to the stock UI, and only "/" is posted again.
	g.routes["/"] = stock.URL
	b, _ := json.Marshal(g.routes)
	_ = os.WriteFile(filepath.Join(dir, "routes.json"), b, 0o600)
	if changed, err := k.Ensure(ctx); err != nil || !changed || g.posts != posts+1 || g.routes["/"] != k.Target {
		t.Fatalf("after reset: %v %v posts %d", changed, err, g.posts-posts)
	}

	if err := k.Restore(ctx); err != nil || g.routes["/"] != stock.URL {
		t.Fatalf("restore: %v %s", err, g.routes["/"])
	}
}

func TestFront(t *testing.T) {
	_, dir, _ := setup(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "icedash "+r.URL.Path) })
	f := httptest.NewServer(&Front{RuntimePath: dir, Next: next})
	defer f.Close()
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	for path, want := range map[string]string{
		"/":                       "icedash /",
		"/modules/icedash/app.js": "icedash /modules/icedash/app.js",
		"/v2/icedash/health":      "icedash /v2/icedash/health",
		"/casaos/":                "stock /",
		"/app.15331e6e.js":        "stock /app.15331e6e.js",
		"/js/custom.js":           "stock /js/custom.js",
		"/favicon.ico":            "stock /favicon.ico",
	} {
		res, err := noFollow.Get(f.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if string(b) != want {
			t.Errorf("%s: %q, want %q", path, b, want)
		}
	}
	res, err := noFollow.Get(f.URL + "/casaos")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound || !strings.HasSuffix(res.Header.Get("Location"), "/casaos/") {
		t.Fatalf("/casaos: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
}
