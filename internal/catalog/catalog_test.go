package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const indexJSON = `{"version":2,"updated_at":"2026-11-12T08:53:03Z","app_count":3,"base_url":"https://icestore.hsinghhira.me","apps":[
 {"id":"com.icestore.documenso","title":"Documenso","category":"Productivity","icon":"/apps/com.icestore.documenso/assets/icon.png","compose_url":"/apps/com.icestore.documenso/docker-compose.yml","meta_url":"/apps/com.icestore.documenso/meta.json","content_hash":"aa","version":"v2.21.1"},
 {"id":"org.other.thing","title":"Not ours"},
 {"id":"com.icestore.../etc","title":"Bad name"}
]}`

func server(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		switch r.URL.Path {
		case "/index.json":
			_, _ = w.Write([]byte(indexJSON))
		case "/apps/com.icestore.documenso/meta.json":
			_, _ = w.Write([]byte(`{"id":"com.icestore.documenso","title":"Documenso","description":"Sign things","screenshot_link":["/apps/com.icestore.documenso/assets/s1.webp"],"tips":{"before_install":"Read me"}}`))
		case "/raw/Apps/documenso/docker-compose.yml":
			_, _ = w.Write([]byte("name: documenso\nservices:\n  documenso:\n    image: x\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newCatalog(srv *httptest.Server, dir string) *Catalog {
	return &Catalog{Site: srv.URL, Raw: srv.URL + "/raw", HTTP: srv.Client(), Cache: filepath.Join(dir, "index.json")}
}

func TestIndexKeepsIceStoreAppsWithAbsoluteLinks(t *testing.T) {
	var hits int32
	srv := server(t, &hits)
	c := newCatalog(srv, t.TempDir())
	idx, err := c.Index(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Apps) != 1 {
		t.Fatalf("apps %+v", idx.Apps)
	}
	a := idx.Apps[0]
	if a.Name != "documenso" || a.Icon != srv.URL+"/apps/com.icestore.documenso/assets/icon.png" || a.MetaURL != srv.URL+"/apps/com.icestore.documenso/meta.json" {
		t.Errorf("app %+v", a)
	}
	// Cached: no second fetch within MaxAge.
	if _, err := c.Index(context.Background(), false); err != nil || atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits %d err %v", hits, err)
	}
	// Forced: fetched again.
	if _, err := c.Index(context.Background(), true); err != nil || atomic.LoadInt32(&hits) != 2 {
		t.Errorf("hits %d err %v", hits, err)
	}
}

func TestIndexFallsBackToTheCacheOffline(t *testing.T) {
	var hits int32
	srv := server(t, &hits)
	dir := t.TempDir()
	if _, err := newCatalog(srv, dir).Index(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	srv.Close()
	c := newCatalog(srv, dir)
	c.Now = func() time.Time { return time.Now().Add(7 * time.Hour) } // stale
	idx, err := c.Index(context.Background(), false)
	if err == nil || len(idx.Apps) != 1 {
		t.Fatalf("want cached apps with an error, got %d apps, %v", len(idx.Apps), err)
	}
}

func TestMetaAndCompose(t *testing.T) {
	var hits int32
	srv := server(t, &hits)
	c := newCatalog(srv, t.TempDir())
	m, err := c.Meta(context.Background(), "documenso")
	if err != nil {
		t.Fatal(err)
	}
	if m.Description != "Sign things" || m.Tips.BeforeInstall != "Read me" || m.Screenshots[0] != srv.URL+"/apps/com.icestore.documenso/assets/s1.webp" {
		t.Errorf("meta %+v", m)
	}
	before := atomic.LoadInt32(&hits)
	if _, err := c.Meta(context.Background(), "com.icestore.documenso"); err != nil || atomic.LoadInt32(&hits) != before {
		t.Errorf("meta not cached: %v", err)
	}
	b, err := c.Compose(context.Background(), "documenso")
	if err != nil || !strings.HasPrefix(string(b), "name: documenso") {
		t.Errorf("compose %q %v", b, err)
	}
	if _, err := c.Meta(context.Background(), "nothing"); !errors.Is(err, ErrUnknownApp) {
		t.Errorf("unknown app: %v", err)
	}
}

func TestOnlyCatalogHosts(t *testing.T) {
	c := &Catalog{}
	for u, ok := range map[string]bool{
		"https://icestore.hsinghhira.me/index.json":                    true,
		"https://raw.githubusercontent.com/HSinghHira/IceStore/main/x": true,
		"http://icestore.hsinghhira.me/index.json":                     false,
		"https://evil.example/index.json":                              false,
	} {
		if c.allowed(u) != ok {
			t.Errorf("%s: want %v", u, ok)
		}
	}
	for s, ok := range map[string]bool{"documenso": true, "pocket-id": true, "../x": false, "a/b": false, "": false, "Up": false} {
		if ValidName(s) != ok {
			t.Errorf("ValidName(%q) want %v", s, ok)
		}
	}
}
