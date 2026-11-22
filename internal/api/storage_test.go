package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/HSinghHira/IceDash/internal/files"
	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/HSinghHira/IceDash/internal/storage"
)

func TestStorage(t *testing.T) {
	data := filepath.ToSlash(t.TempDir())
	fs := &files.Service{Root: data, MountInfo: filepath.Join(t.TempDir(), "none")}
	s := &Server{Verify: session.Disabled{}, Files: fs}
	h := s.Handler()
	read := func(auth string) StorageView {
		t.Helper()
		w := call(h, "GET", "/v2/icedash/storage", auth, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		var v StorageView
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	// No ZimaOS list (CasaOS): Files' own disks, without health.
	if v := read(admin); v.Health || len(v.Pools) != 1 || v.Pools[0].Name != "DATA" || v.Pools[0].Files != "/" {
		t.Fatalf("fallback %+v", v)
	}
	// ZimaOS's list: Files' paths for admins only, no host folders.
	zima := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/local_storage/storages" {
			w.Write([]byte(`[{"extensions":{"health":true,"size":10,"used":4},"name":"ZimaOS-HD","path":"` + data + `","type":"SYSTEM"}]`))
			return
		}
		http.NotFound(w, r)
	}))
	defer zima.Close()
	s.Storage = &storage.Client{Gateway: zima.URL}
	h = s.Handler()
	v := read(admin)
	if !v.Health || len(v.Pools) != 1 || v.Pools[0].Health != "ok" || v.Pools[0].Files != "/" || v.Pools[0].Dir != "" {
		t.Fatalf("admin %+v", v)
	}
	if v := read(member); v.Pools[0].Files != "" {
		t.Fatalf("member sees Files path %+v", v)
	}
	if w := call(h, "GET", "/v2/icedash/storage", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", w.Code)
	}
}
