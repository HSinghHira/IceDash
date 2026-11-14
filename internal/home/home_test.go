package home

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeCaddy answers the few admin calls IceDash makes, like Caddy 2 does.
type fakeCaddy struct {
	mu     sync.Mutex
	routes []map[string]any // srv0's routes
	puts   int
}

func (f *fakeCaddy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	find := func() int {
		for i, rt := range f.routes {
			if rt["@id"] == ID {
				return i
			}
		}
		return -1
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/config/apps/http/servers":
		w.Write([]byte(`{"srv0":{"listen":[":80"],"routes":[]},"srv1":{"listen":[":9527"]}}`))
	case r.URL.Path == "/id/"+ID && r.Method == http.MethodGet:
		if find() < 0 {
			http.Error(w, `{"error":"unknown object ID 'icedash-home'"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(f.routes[find()])
	case r.URL.Path == "/id/"+ID && r.Method == http.MethodDelete:
		i := find()
		if i < 0 {
			http.Error(w, `{"error":"unknown object ID 'icedash-home'"}`, http.StatusNotFound)
			return
		}
		f.routes = append(f.routes[:i], f.routes[i+1:]...)
	case r.Method == http.MethodPut && r.URL.Path == "/config/apps/http/servers/srv0/routes/0":
		var rt map[string]any
		b, _ := io.ReadAll(r.Body)
		if json.Unmarshal(b, &rt) != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		f.puts++
		f.routes = append([]map[string]any{rt}, f.routes...)
	default:
		http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	}
}

func TestApplyAndRemove(t *testing.T) {
	f := &fakeCaddy{routes: []map[string]any{{"handle": "the gateway's"}}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := &Caddy{Base: srv.URL, HTTP: srv.Client()}
	ctx := context.Background()
	if ok, err := c.Present(ctx); ok || err != nil {
		t.Fatalf("present before: %v %v", ok, err)
	}
	if err := c.Apply(ctx, "80"); err != nil {
		t.Fatal(err)
	}
	// Once only, and first, before the gateway's own route.
	if err := c.Apply(ctx, "80"); err != nil || f.puts != 1 || f.routes[0]["@id"] != ID || len(f.routes) != 2 {
		t.Fatalf("apply twice: %v puts=%d routes=%v", err, f.puts, f.routes)
	}
	rt := f.routes[0]
	match := rt["match"].([]any)[0].(map[string]any)
	if m := match["path"].([]any); len(m) != 1 || m[0] != "/" {
		t.Fatalf("match %v", m)
	}
	// Only while IceDash's page is on disk, so a removed IceDash can't leave
	// "/" redirecting to a 404.
	file, _ := match["file"].(map[string]any)
	if file["root"] != "/usr/share/casaos/www" || len(file["try_files"].([]any)) != 1 || file["try_files"].([]any)[0] != "/modules/icedash/index.html" {
		t.Fatalf("file matcher %v", file)
	}
	if ok, _ := c.Present(ctx); !ok {
		t.Fatal("not present")
	}
	if err := c.Remove(ctx); err != nil || len(f.routes) != 1 {
		t.Fatalf("remove: %v %v", err, f.routes)
	}
	if err := c.Remove(ctx); err != nil {
		t.Fatalf("remove again: %v", err)
	}
	if err := c.Apply(ctx, "8080"); err == nil || !strings.Contains(err.Error(), ":8080") {
		t.Fatalf("no server on the port: %v", err)
	}
}

func TestCheck(t *testing.T) {
	ours := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(Marker, "zimaos")
		http.Redirect(w, r, Target, http.StatusFound)
	}))
	defer ours.Close()
	if err := Check(context.Background(), ours.URL, nil); err != nil {
		t.Fatal(err)
	}
	theirs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>zimaos</html>")) }))
	defer theirs.Close()
	if err := Check(context.Background(), theirs.URL, nil); err != ErrNotTaken {
		t.Fatalf("not ours: %v", err)
	}
}
