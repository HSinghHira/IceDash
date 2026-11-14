package mods

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, body)
	}
	zw.Close()
	return buf.Bytes()
}

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

const paperTheme = `{"scheme":"light","colors":{"bg":"#eef2f7","surface":"#ffffff","text":"#0d1726","accent":"#2f6fec","accent2":"#1d4ed8","accentDark":"#2459c7","ink":"#ffffff","warn":"#b45309"}}`

func paperZip(t *testing.T) []byte {
	return zipOf(t, map[string]string{
		"mod.json":   `{"id":"paper","name":"Paper","kind":"theme","version":"1.0.0","author":"IceDash","description":"Light."}`,
		"theme.json": paperTheme,
	})
}

func weatherZip(t *testing.T, version string) []byte {
	return zipOf(t, map[string]string{
		"mod.json":          `{"id":"weather","name":"Weather","kind":"widget","version":"` + version + `","author":"IceDash","description":"Weather.","entry":"index.html","size":"small","permissions":{"internet":["api.open-meteo.com"],"location":true}}`,
		"index.html":        `<!doctype html><html><head><title>w</title></head><body><script src="icedash-widget.js"></script></body></html>`,
		"icedash-widget.js": `window.IceDash={}`,
	})
}

// fakeGitHub serves the catalog, its zips, and one third-party repo's release.
type fakeGitHub struct {
	srv     *httptest.Server
	files   map[string][]byte
	catalog Catalog
}

func newGitHub(t *testing.T) *fakeGitHub {
	g := &fakeGitHub{files: map[string][]byte{}}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/index.json":
			json.NewEncoder(w).Encode(g.catalog)
		case r.URL.Path == "/repos/someone/clock-widget/releases/latest":
			fmt.Fprintf(w, `{"assets":[{"name":"mod.zip","browser_download_url":"%s/dl/clock.zip"},{"name":"mod.zip.sha256","browser_download_url":"%s/dl/clock.zip.sha256"}]}`, g.srv.URL, g.srv.URL)
		case r.URL.Path == "/repos/someone/no-release/releases/latest":
			w.WriteHeader(http.StatusNotFound)
		default:
			b, ok := g.files[strings.TrimPrefix(r.URL.Path, "/dl/")]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write(b)
		}
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGitHub) publish(file string, data []byte, m Manifest, checksum string) {
	g.files[file] = data
	if checksum == "" {
		checksum = sum(data)
	}
	g.catalog.Mods = append(g.catalog.Mods, CatalogMod{Manifest: m, SHA256: checksum, File: file, URL: g.srv.URL + "/dl/" + file})
}

func service(t *testing.T, g *fakeGitHub) *Service {
	dir := t.TempDir()
	return &Service{
		Dir: filepath.Join(dir, "mods"), State: filepath.Join(dir, "mods.json"), Cache: filepath.Join(dir, "cat.json"),
		CatalogURL: g.srv.URL + "/index.json", GitHubAPI: g.srv.URL, HTTP: g.srv.Client(),
	}
}

func TestManifestRules(t *testing.T) {
	ok := Manifest{ID: "weather", Name: "W", Kind: "widget", Version: "1.0.0", Author: "a", Description: "d", Entry: "index.html", Size: "small",
		Permissions: &Permissions{Internet: []string{"api.open-meteo.com"}}}
	if err := ok.Check(); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"192.168.68.17", "[::1]", "localhost", "zimaos.local", "nas.lan", "x", "printer.home"} {
		m := ok
		m.Permissions = &Permissions{Internet: []string{h}}
		if err := m.Check(); !errors.Is(err, ErrInvalid) {
			t.Errorf("host %q allowed", h)
		}
	}
	for _, bad := range []func(*Manifest){
		func(m *Manifest) { m.ID = "Bad ID" },
		func(m *Manifest) { m.Version = "1.0" },
		func(m *Manifest) { m.Kind = "plugin" },
		func(m *Manifest) { m.Entry = "../x.html" },
		func(m *Manifest) { m.Size = "huge" },
	} {
		m := ok
		bad(&m)
		if m.Check() == nil {
			t.Errorf("accepted %+v", m)
		}
	}
	th := Manifest{ID: "x1", Name: "X", Kind: "theme", Version: "1.0.0", Author: "a", Description: "d", Permissions: &Permissions{System: true}}
	if th.Check() == nil {
		t.Error("a theme asked for permissions")
	}
}

func TestThemeCSSRules(t *testing.T) {
	for _, css := range []string{
		"@import 'x.css';",
		"body{background:url(https://evil.example/x)}",
		"body{background:url( 'https://evil.example/x' )}",
		"input[value^=a]{color:red}",
		`a{background:\75rl(https://x)}`,
		"a{background:image-set('x.png' 1x)}",
		"a{x:expression(alert(1))}",
		"</style><script>",
	} {
		if err := CheckCSS(css); err == nil {
			t.Errorf("allowed %q", css)
		}
	}
	for _, css := range []string{"a{color:red}", "b{background:url(data:image/png;base64,AA)}", `b{background:url( "data:image/svg+xml,x" )}`} {
		if err := CheckCSS(css); err != nil {
			t.Errorf("refused %q: %v", css, err)
		}
	}
}

func TestReadZipRefusesTricks(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"escape":      {"mod.json": `{}`, "../evil": "x"},
		"no manifest": {"index.html": "x"},
		"no entry":    {"mod.json": `{"id":"w1","name":"W","kind":"widget","version":"1.0.0","author":"a","description":"d","entry":"index.html","size":"small"}`},
	} {
		if _, _, err := readZip(zipOf(t, files)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, _, err := readZip([]byte("not a zip")); err == nil {
		t.Error("not a zip: accepted")
	}
}

func TestParseSource(t *testing.T) {
	for in, want := range map[string]string{
		"weather":                                 "weather",
		"github.com/someone/clock-widget":         "github.com/someone/clock-widget",
		"https://github.com/someone/clock-widget": "github.com/someone/clock-widget",
		"someone/clock-widget":                    "github.com/someone/clock-widget",
		"https://github.com/someone/clock.git":    "github.com/someone/clock",
	} {
		if got, err := ParseSource(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for _, bad := range []string{"https://evil.example/a/b", "github.com/a/..", "a/b/c/d", ""} {
		if _, err := ParseSource(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestInstallFromTheCatalog(t *testing.T) {
	g := newGitHub(t)
	p := paperZip(t)
	g.publish("paper-1.0.0.zip", p, Manifest{ID: "paper", Name: "Paper", Kind: "theme", Version: "1.0.0", Author: "IceDash", Description: "Light."}, "")
	w := weatherZip(t, "1.0.0")
	g.publish("weather-1.0.0.zip", w, Manifest{ID: "weather", Name: "Weather", Kind: "widget", Version: "1.0.0", Author: "IceDash", Description: "Weather.", Entry: "index.html", Size: "small"}, "")
	s := service(t, g)
	ctx := context.Background()

	c, err := s.Catalog(ctx, false)
	if err != nil || len(c.Mods) != 2 {
		t.Fatalf("%+v %v", c, err)
	}
	o, err := s.Inspect(ctx, "weather")
	if err != nil || !o.Checked || o.Permissions == nil || !o.Permissions.Location || o.Permissions.Internet[0] != "api.open-meteo.com" {
		t.Fatalf("inspect: %+v %v", o, err)
	}
	if _, err := s.Install(ctx, "weather"); err != nil {
		t.Fatal(err)
	}
	in, err := s.Install(ctx, "paper")
	if err != nil || !in.Checked || in.Source != "paper" {
		t.Fatalf("%+v %v", in, err)
	}
	if err := s.SetTheme("Hira", "paper"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPanel("Hira", []string{"weather", "weather"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPanel("Hira", []string{"paper"}); !errors.Is(err, ErrNotWidget) {
		t.Fatalf("a theme on the panel: %v", err)
	}
	v := s.List("Hira")
	if v.Prefs.Theme != "paper" || len(v.Prefs.Panel) != 1 || len(v.Mods) != 2 {
		t.Fatalf("view: %+v", v)
	}
	for _, m := range v.Mods {
		if m.ID == "paper" && (m.Theme == nil || m.Theme.Colors["bg"] != "#eef2f7" || m.Theme.Scheme != "light") {
			t.Fatalf("theme not read back: %+v", m.Theme)
		}
	}
	// Another user keeps Ice.
	if other := s.List("testmember"); other.Prefs.Theme != "" {
		t.Fatalf("theme leaked to another user: %+v", other.Prefs)
	}
	// Removing the theme puts its user back on Ice.
	if err := s.Remove("paper"); err != nil {
		t.Fatal(err)
	}
	if v := s.List("Hira"); v.Prefs.Theme != "" {
		t.Fatalf("after removal: %+v", v.Prefs)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "paper")); err == nil {
		t.Fatal("files left after removal")
	}
}

func TestTamperedDownloadIsRefused(t *testing.T) {
	g := newGitHub(t)
	g.publish("paper-1.0.0.zip", paperZip(t), Manifest{ID: "paper", Name: "Paper", Kind: "theme", Version: "1.0.0", Author: "IceDash", Description: "Light."}, strings.Repeat("ab", 32))
	s := service(t, g)
	if _, err := s.Install(context.Background(), "paper"); !errors.Is(err, ErrChecksum) {
		t.Fatalf("%v", err)
	}
	if v := s.List("Hira"); len(v.Mods) != 0 {
		t.Fatal("installed anyway")
	}
}

func TestThirdPartyFromGitHub(t *testing.T) {
	g := newGitHub(t)
	clock := zipOf(t, map[string]string{
		"mod.json":   `{"id":"clock","name":"Clock","kind":"widget","version":"0.1.0","author":"someone","description":"A clock.","entry":"index.html","size":"small","permissions":{"system":true}}`,
		"index.html": `<p>tick</p>`,
	})
	g.files["clock.zip"] = clock
	g.files["clock.zip.sha256"] = []byte(sum(clock) + "  mod.zip\n")
	s := service(t, g)
	ctx := context.Background()
	o, err := s.Inspect(ctx, "https://github.com/someone/clock-widget")
	if err != nil || o.Checked || o.Source != "github.com/someone/clock-widget" || !o.Permissions.System {
		t.Fatalf("%+v %v", o, err)
	}
	in, err := s.Install(ctx, "github.com/someone/clock-widget")
	if err != nil || in.Checked || in.ID != "clock" {
		t.Fatalf("%+v %v", in, err)
	}
	if _, err := s.Inspect(ctx, "someone/no-release"); !errors.Is(err, ErrNoRelease) {
		t.Fatalf("no release: %v", err)
	}
	g.files["clock.zip.sha256"] = []byte(strings.Repeat("0", 64))
	s.pending = nil
	if _, err := s.Install(ctx, "someone/clock-widget"); !errors.Is(err, ErrChecksum) {
		t.Fatalf("bad sum: %v", err)
	}
}

func TestServeFileWritesThePolicyFirst(t *testing.T) {
	g := newGitHub(t)
	w := weatherZip(t, "1.0.0")
	g.publish("weather-1.0.0.zip", w, Manifest{ID: "weather", Name: "Weather", Kind: "widget", Version: "1.0.0", Author: "IceDash", Description: "Weather.", Entry: "index.html", Size: "small"}, "")
	s := service(t, g)
	if _, err := s.Install(context.Background(), "weather"); err != nil {
		t.Fatal(err)
	}
	get := func(p string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://192.168.68.17"+FilesPrefix+"weather/files/"+p, nil)
		rec := httptest.NewRecorder()
		s.ServeFile(rec, r, "weather", p)
		return rec
	}
	rec := get("index.html")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.HasPrefix(body, `<!doctype html><meta http-equiv="Content-Security-Policy"`) {
		t.Fatalf("%d %s", rec.Code, body)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"connect-src https://api.open-meteo.com", "script-src http://192.168.68.17/v2/icedash/mods/weather/files/", "sandbox allow-scripts", "default-src 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("policy lacks %q: %s", want, csp)
		}
	}
	if strings.Contains(csp, "'self'") {
		t.Error("'self' would let a widget reach IceDash's and the system's APIs")
	}
	if rec := get("icedash-widget.js"); rec.Code != 200 || rec.Header().Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("sdk: %d %v", rec.Code, rec.Header())
	}
	for _, p := range []string{"../../mods.json", "mod.json/../../x", ".hidden"} {
		if rec := get(p); rec.Code != 404 {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	s.SetEnabled("weather", false)
	if rec := get("index.html"); rec.Code != 404 {
		t.Fatalf("switched off: %d", rec.Code)
	}
}

func TestPolicyWithoutInternet(t *testing.T) {
	p := Policy("http://box", "clock", &Permissions{System: true})
	if !strings.Contains(p, "connect-src 'none'") {
		t.Fatal(p)
	}
}
