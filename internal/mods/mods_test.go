package mods

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// fakeGitHub serves repos the way GitHub does: commits/HEAD, a recursive
// tree with git hashes, and raw files at a commit.
type fakeGitHub struct {
	srv    *httptest.Server
	mu     sync.Mutex
	repos  map[string]map[string][]byte // owner/repo → path → bytes
	rev    map[string]int               // bumps the commit
	tamper map[string][]byte            // raw path → bytes served instead
	calls  int
}

func newGitHub(t *testing.T) *fakeGitHub {
	g := &fakeGitHub{repos: map[string]map[string][]byte{}, rev: map[string]int{}, tamper: map[string][]byte{}}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGitHub) commit(repo string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s@%d", repo, g.rev[repo])))
	return hex.EncodeToString(h[:])
}

func treeHash(files map[string][]byte, folder string) string {
	var keys []string
	for k := range files {
		if strings.HasPrefix(k, folder+"/") {
			keys = append(keys, k+gitHash(files[k]))
		}
	}
	sort.Strings(keys)
	h := sha1.Sum([]byte(strings.Join(keys, ",")))
	return hex.EncodeToString(h[:])
}

func (g *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	p := r.URL.Path
	if rest, ok := strings.CutPrefix(p, "/repos/"); ok {
		parts := strings.SplitN(rest, "/", 3)
		repo := parts[0] + "/" + parts[1]
		files, ok := g.repos[repo]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch {
		case parts[2] == "commits/HEAD":
			fmt.Fprintf(w, `{"sha":%q}`, g.commit(repo))
		case strings.HasPrefix(parts[2], "git/trees/"):
			var tree []map[string]any
			folders := map[string]bool{}
			for path, b := range files {
				tree = append(tree, map[string]any{"path": path, "type": "blob", "sha": gitHash(b), "size": len(b)})
				if top, _, ok := strings.Cut(path, "/"); ok {
					folders[top] = true
				}
			}
			for f := range folders {
				tree = append(tree, map[string]any{"path": f, "type": "tree", "sha": treeHash(files, f)})
			}
			json.NewEncoder(w).Encode(map[string]any{"tree": tree})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
		return
	}
	// raw: /owner/repo/<commit>/<path>
	parts := strings.SplitN(strings.TrimPrefix(p, "/"), "/", 4)
	if len(parts) < 4 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	repo := parts[0] + "/" + parts[1]
	if parts[2] != g.commit(repo) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if b, ok := g.tamper[parts[3]]; ok {
		w.Write(b)
		return
	}
	b, ok := g.repos[repo][parts[3]]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Write(b)
}

var webp = append([]byte("RIFF\x10\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)

const auroraJSON = `{"name":"Aurora","author":"IceDash","description":"Violet.","scheme":"dark","colors":{"bg":"#05040d","surface":"#0e0c1c","text":"#ffffff","accent":"#a78bfa","accent2":"#5eead4","accentDark":"#8b5cf6","ink":"#140a2e","warn":"#f5b84c"}}`

func officialRepos(g *fakeGitHub) {
	g.repos["HSinghHira/IceColors"] = map[string][]byte{
		"README.md":             []byte("# IceColors"),
		"aurora/theme.json":     []byte(auroraJSON),
		"aurora/wallpaper.webp": webp,
		"broken/theme.json":     []byte(`{"name":"Broken","author":"x","description":"d","scheme":"dark","colors":{"bg":"red"}}`),
		"Not_A_Mod/theme.json":  []byte(auroraJSON),
		"notes/readme.txt":      []byte("not a mod"),
	}
	g.repos["HSinghHira/IceCards"] = map[string][]byte{
		"weather/widget.json": []byte(`{"name":"Weather","author":"IceDash","description":"Weather.","entry":"index.html","size":"small","permissions":{"internet":["api.open-meteo.com"],"location":true}}`),
		"weather/index.html":  []byte(`<!doctype html><html><head><title>w</title></head><body>weather</body></html>`),
		"spy/widget.json":     []byte(`{"name":"Spy","author":"x","description":"d","size":"small","permissions":{"internet":["192.168.68.17"]}}`),
		"spy/index.html":      []byte(`x`),
	}
}

func service(t *testing.T, g *fakeGitHub) *Service {
	dir := t.TempDir()
	return &Service{Dir: filepath.Join(dir, "mods"), State: filepath.Join(dir, "mods.json"), Cache: filepath.Join(dir, "cols.json"),
		GitHubAPI: g.srv.URL, RawBase: g.srv.URL, HTTP: g.srv.Client()}
}

func ids(c Collection) []string {
	var out []string
	for _, it := range c.Items {
		out = append(out, it.Folder)
	}
	return out
}

func TestOfficialCollections(t *testing.T) {
	g := newGitHub(t)
	officialRepos(g)
	s := service(t, g)
	cols := s.Collections(context.Background(), false)
	if len(cols) != 2 || cols[0].Source != OfficialThemes || !cols[0].Official {
		t.Fatalf("%+v", cols)
	}
	// broken colours, a capitalised folder, a folder that isn't a mod, and a
	// widget asking for an address are all left out.
	if got := strings.Join(ids(cols[0]), ","); got != "aurora" {
		t.Fatalf("themes: %s", got)
	}
	if got := strings.Join(ids(cols[1]), ","); got != "weather" {
		t.Fatalf("widgets: %s", got)
	}
	a := cols[0].Items[0]
	if a.ID != "hsinghhira--icecolors--aurora" || a.Colors["accent"] != "#a78bfa" || !strings.Contains(a.Wallpaper, "/aurora/wallpaper.webp") {
		t.Fatalf("aurora: %+v", a)
	}
	// A second look within 6 hours asks GitHub nothing.
	before := g.calls
	s.Collections(context.Background(), false)
	if g.calls != before {
		t.Fatal("the cache wasn't used")
	}
}

func TestInstallThemeAndWidget(t *testing.T) {
	g := newGitHub(t)
	officialRepos(g)
	s := service(t, g)
	ctx := context.Background()
	in, err := s.Install(ctx, "hsinghhira--icecolors--aurora")
	if err != nil || !in.Official || in.Commit == "" {
		t.Fatalf("%+v %v", in, err)
	}
	if _, err := s.Install(ctx, "hsinghhira--icecards--weather"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTheme("Hira", in.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPanel("Hira", []string{"hsinghhira--icecards--weather"}); err != nil {
		t.Fatal(err)
	}
	v := s.List("Hira")
	if v.Prefs.Theme != in.ID || len(v.Mods) != 2 {
		t.Fatalf("%+v", v)
	}
	for _, m := range v.Mods {
		if m.Kind == "theme" && m.Wallpaper != FilesPrefix+in.ID+"/wallpaper" {
			t.Fatalf("wallpaper should be served from the box: %s", m.Wallpaper)
		}
	}
	rec := httptest.NewRecorder()
	s.ServeWallpaper(rec, httptest.NewRequest("GET", "/", nil), in.ID)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/webp" {
		t.Fatalf("wallpaper: %d %v", rec.Code, rec.Header())
	}
	if s.List("testmember").Prefs.Theme != "" {
		t.Fatal("the theme leaked to another user")
	}
	if err := s.Remove(in.ID); err != nil {
		t.Fatal(err)
	}
	if s.List("Hira").Prefs.Theme != "" {
		t.Fatal("removing the theme should put its user back on Ice")
	}
}

func TestTamperedFileIsRefused(t *testing.T) {
	g := newGitHub(t)
	officialRepos(g)
	s := service(t, g)
	s.Collections(context.Background(), false)
	g.tamper["aurora/wallpaper.webp"] = append(webp, 'x')
	if _, err := s.Install(context.Background(), "hsinghhira--icecolors--aurora"); !errors.Is(err, ErrTampered) {
		t.Fatalf("%v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "hsinghhira--icecolors--aurora")); err == nil {
		t.Fatal("files were left behind")
	}
}

func TestWallpaperMustBeAnImage(t *testing.T) {
	g := newGitHub(t)
	officialRepos(g)
	g.repos["HSinghHira/IceColors"]["aurora/wallpaper.webp"] = []byte("<script>alert(1)</script>")
	s := service(t, g)
	if _, err := s.Install(context.Background(), "hsinghhira--icecolors--aurora"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("%v", err)
	}
}

func TestWidgetPageGetsPolicyAndBridge(t *testing.T) {
	g := newGitHub(t)
	officialRepos(g)
	s := service(t, g)
	id := "hsinghhira--icecards--weather"
	if _, err := s.Install(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.ServeFile(rec, httptest.NewRequest("GET", "http://192.168.68.17"+FilesPrefix+id+"/files/"+p, nil), id, p)
		return rec
	}
	rec := get("index.html")
	body := rec.Body.String()
	if !strings.HasPrefix(body, `<!doctype html><meta http-equiv="Content-Security-Policy"`) || !strings.Contains(body, `<script src="icedash-widget.js"></script><html>`) {
		t.Fatalf("page: %s", body)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "connect-src https://api.open-meteo.com") || !strings.Contains(csp, "sandbox allow-scripts") || strings.Contains(csp, "'self'") {
		t.Fatalf("policy: %s", csp)
	}
	if rec := get("icedash-widget.js"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "window.IceDash") {
		t.Fatalf("bridge: %d", rec.Code)
	}
	for _, p := range []string{"../../mods.json", "widget.json/../../x", ".git"} {
		if rec := get(p); rec.Code != 404 {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	s.SetEnabled(id, false)
	if rec := get("index.html"); rec.Code != 404 {
		t.Fatal("a switched-off widget is still served")
	}
}

func TestThirdPartyCollection(t *testing.T) {
	g := newGitHub(t)
	officialRepos(g)
	g.repos["someone/my-cards"] = map[string][]byte{
		"clock/widget.json": []byte(`{"name":"Clock","author":"someone","description":"A clock.","size":"small","permissions":{"system":true}}`),
		"clock/index.html":  []byte(`<p>tick</p>`),
	}
	g.repos["someone/empty"] = map[string][]byte{"README.md": []byte("hi")}
	s := service(t, g)
	ctx := context.Background()
	c, err := s.AddCollection(ctx, "https://github.com/someone/my-cards")
	if err != nil || c.Official || len(c.Items) != 1 || c.Items[0].Official {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := s.AddCollection(ctx, "someone/empty"); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := s.AddCollection(ctx, "someone/nothing-here"); !errors.Is(err, ErrNoRepo) {
		t.Fatalf("missing: %v", err)
	}
	if len(s.Collections(ctx, false)) != 3 {
		t.Fatal("the added collection isn't listed")
	}
	if _, err := s.Install(ctx, "someone--my-cards--clock"); err != nil {
		t.Fatal(err)
	}
	// An update upstream shows as Update.
	g.repos["someone/my-cards"]["clock/index.html"] = []byte(`<p>tock</p>`)
	g.rev["someone/my-cards"]++
	s.Collections(ctx, true)
	for _, m := range s.List("Hira").Mods {
		if m.ID == "someone--my-cards--clock" && !m.Update {
			t.Fatal("no update shown")
		}
	}
	if err := s.RemoveCollection(OfficialWidgets); !errors.Is(err, ErrOfficial) {
		t.Fatalf("official: %v", err)
	}
	if err := s.RemoveCollection("github.com/someone/my-cards"); err != nil {
		t.Fatal(err)
	}
	if len(s.Collections(ctx, false)) != 2 {
		t.Fatal("still listed")
	}
}

func TestAddressesAndIDs(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/someone/my-cards":          "github.com/someone/my-cards",
		"https://github.com/someone/my-cards/": "github.com/someone/my-cards",
		"someone/my-cards":                     "github.com/someone/my-cards",
	} {
		if got, err := ParseCollection(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for _, bad := range []string{"https://evil.example/a/b", "github.com/a/..", "a/b/c/d", "weather"} {
		if _, err := ParseCollection(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if id := ModID("github.com/Some_One/My.Cards", "clock"); id != "some-one--my-cards--clock" || !ValidID(id) {
		t.Fatalf("id %q", id)
	}
	for _, h := range []string{"192.168.68.17", "[::1]", "localhost", "box.local", "nas.lan"} {
		w := WidgetFile{Name: "n", Author: "a", Description: "d", Size: "small", Permissions: Permissions{Internet: []string{h}}}
		if w.Check() == nil {
			t.Errorf("host %q allowed", h)
		}
	}
	if ImageType([]byte("\x89PNG\r\n\x1a\nxxxx")) != "image/png" || ImageType([]byte{0xFF, 0xD8, 0xFF, 0}) != "image/jpeg" || ImageType([]byte("<svg")) != "" {
		t.Fatal("image types")
	}
}

func TestModsFromV070AreDropped(t *testing.T) {
	dir := t.TempDir()
	old := `{"mods":{"paper":{"id":"paper","kind":"theme"},"weather":{"id":"weather","kind":"widget"}},"users":{"casaos":{"theme":"paper","panel":["weather"]}}}`
	os.MkdirAll(filepath.Join(dir, "mods", "paper", "1.0.0"), 0o700)
	os.WriteFile(filepath.Join(dir, "mods.json"), []byte(old), 0o600)
	s := &Service{Dir: filepath.Join(dir, "mods"), State: filepath.Join(dir, "mods.json"), Cache: filepath.Join(dir, "c.json")}
	v := s.List("casaos")
	if len(v.Mods) != 0 || v.Prefs.Theme != "" || len(v.Prefs.Panel) != 0 {
		t.Fatalf("%+v", v)
	}
	if _, err := os.Stat(filepath.Join(dir, "mods", "paper")); err == nil {
		t.Fatal("old files left")
	}
	if err := s.SetPanel("casaos", nil); err != nil {
		t.Fatal(err)
	}
}

func TestAllowedURL(t *testing.T) {
	for _, ok := range []string{"https://api.github.com/repos/a/b/commits/HEAD", "https://raw.githubusercontent.com/a/b/c/d"} {
		if AllowedURL(ok) != nil {
			t.Errorf("%s refused", ok)
		}
	}
	for _, no := range []string{"http://api.github.com/x", "https://evil.example/x", "https://192.168.68.17/v1/users", "https://github.com.evil.example/x"} {
		if AllowedURL(no) == nil {
			t.Errorf("%s allowed", no)
		}
	}
}

func TestBrokenModsAreListedWithTheirReason(t *testing.T) {
	g := newGitHub(t)
	officialRepos(g)
	s := service(t, g)
	why := map[string]string{}
	for _, c := range s.Collections(context.Background(), false) {
		for _, r := range c.Refused {
			why[r.Folder] = r.Reason
		}
	}
	if !strings.Contains(why["broken"], "colour") && !strings.Contains(why["broken"], "color") {
		t.Errorf("broken: %q", why["broken"])
	}
	if !strings.Contains(why["spy"], "is an address") {
		t.Errorf("spy: %q", why["spy"])
	}
	if _, ok := why["notes"]; ok {
		t.Error("a folder that isn't a mod is reported")
	}
}
