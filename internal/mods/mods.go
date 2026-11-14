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
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/HSinghHira/IceDash/internal/state"
)

// DefaultCatalog is the catalog's index (IceDash-Mods, latest release).
const DefaultCatalog = "https://github.com/HSinghHira/IceDash-Mods/releases/latest/download/index.json"

// Errors the API words for the UI.
var (
	ErrNotFound    = errors.New("there's no such mod")
	ErrChecksum    = errors.New("the download doesn't match its checksum, so it wasn't installed")
	ErrNoRelease   = errors.New("that repository has no release with mod.zip and mod.zip.sha256")
	ErrBadAddress  = errors.New("write the mod's address as github.com/owner/repo")
	ErrNotWidget   = errors.New("that isn't an installed widget")
	ErrUnreachable = errors.New("GitHub can't be reached right now")
)

// Service is safe for concurrent use.
type Service struct {
	Dir        string // /DATA/.icedash/mods
	State      string // /DATA/.icedash/mods.json
	Cache      string // /DATA/.icedash/mods-catalog.json
	CatalogURL string
	GitHubAPI  string // https://api.github.com
	HTTP       *http.Client
	AllowURL   func(string) error // downloads: GitHub only

	mu      sync.Mutex
	st      *file
	cat     *Catalog
	catAt   time.Time
	pending map[string]download
}

// Installed is a mod on the box.
type Installed struct {
	Manifest
	Source    string    `json:"source"`  // catalog, or github.com/owner/repo
	Checked   bool      `json:"checked"` // from the catalog, checked by IceDash's CI
	SHA256    string    `json:"sha256"`
	Enabled   bool      `json:"enabled"`
	Installed time.Time `json:"installed"`
	Theme     *Theme    `json:"theme,omitempty"` // themes: colours and CSS, read from disk
}

// Prefs are one user's choices.
type Prefs struct {
	Theme string   `json:"theme"` // "" is Ice, the built-in
	Panel []string `json:"panel"` // widget ids, top to bottom
}

type file struct {
	Mods  map[string]*Installed `json:"mods"`
	Users map[string]*Prefs     `json:"users"`
}

// Catalog is index.json.
type Catalog struct {
	Version int          `json:"version"`
	Mods    []CatalogMod `json:"mods"`
}

// CatalogMod is a mod the catalog offers.
type CatalogMod struct {
	Manifest
	SHA256 string `json:"sha256"`
	File   string `json:"file"`
	URL    string `json:"url"`
}

type download struct {
	data    []byte
	sum     string
	checked bool
	at      time.Time
}

func (s *Service) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (s *Service) get(ctx context.Context, u string, max int64) ([]byte, error) {
	if s.AllowURL != nil {
		if err := s.AllowURL(u); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "IceDash-mods")
	c := *s.client()
	allow := s.AllowURL
	c.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if allow != nil {
			return allow(r.URL.String())
		}
		return nil
	}
	res, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w (%s)", ErrUnreachable, res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, bad("it's larger than 5 MB")
	}
	return b, nil
}

// ---- state ----

func (s *Service) load() *file {
	if s.st != nil {
		return s.st
	}
	s.st = &file{Mods: map[string]*Installed{}, Users: map[string]*Prefs{}}
	if b, err := os.ReadFile(s.State); err == nil {
		var f file
		if json.Unmarshal(b, &f) == nil {
			if f.Mods == nil {
				f.Mods = map[string]*Installed{}
			}
			if f.Users == nil {
				f.Users = map[string]*Prefs{}
			}
			s.st = &f
		}
	}
	return s.st
}

func (s *Service) save() error {
	b, err := json.MarshalIndent(s.st, "", "  ")
	if err != nil {
		return err
	}
	return state.WriteFile(s.State, b)
}

// ---- the catalog ----

const catalogAge = 6 * time.Hour

// Catalog returns the catalog, from memory, the cache file or GitHub.
func (s *Service) Catalog(ctx context.Context, force bool) (Catalog, error) {
	s.mu.Lock()
	if !force && s.cat != nil && time.Since(s.catAt) < catalogAge {
		c := *s.cat
		s.mu.Unlock()
		return c, nil
	}
	s.mu.Unlock()
	u := s.CatalogURL
	if u == "" {
		u = DefaultCatalog
	}
	b, err := s.get(ctx, u, 4<<20)
	if err != nil {
		// Offline: the last copy will do.
		if cb, cerr := os.ReadFile(s.Cache); cerr == nil {
			var c Catalog
			if json.Unmarshal(cb, &c) == nil {
				return c, nil
			}
		}
		return Catalog{Mods: []CatalogMod{}}, err
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return Catalog{Mods: []CatalogMod{}}, fmt.Errorf("the catalog can't be read: %w", err)
	}
	// Leave out anything that wouldn't install anyway.
	c.Mods = slices.DeleteFunc(c.Mods, func(m CatalogMod) bool {
		if m.Preview != nil && m.Preview.Check() != nil {
			return true
		}
		return m.Check() != nil || len(m.SHA256) != 64
	})
	s.mu.Lock()
	s.cat, s.catAt = &c, time.Now()
	s.mu.Unlock()
	if s.Cache != "" {
		_ = state.WriteFile(s.Cache, b)
	}
	return c, nil
}

// ---- inspecting and installing ----

// Offer is what installing would bring, for the UI to ask about first.
type Offer struct {
	Manifest
	Source    string `json:"source"`
	Checked   bool   `json:"checked"`
	Installed string `json:"installed,omitempty"` // the version on the box, if any
}

var ghRx = regexp.MustCompile(`^(?:https?://)?(?:www\.)?(?:github\.com/)?([A-Za-z0-9-]{1,39})/([A-Za-z0-9._-]{1,100}?)(?:\.git)?/?$`)

// ParseSource turns what was typed into a source: a catalog id, or
// github.com/owner/repo.
func ParseSource(src string) (string, error) {
	src = strings.TrimSpace(src)
	if ValidID(src) {
		return src, nil
	}
	m := ghRx.FindStringSubmatch(src)
	if m == nil || m[2] == "." || m[2] == ".." {
		return "", ErrBadAddress
	}
	return "github.com/" + m[1] + "/" + m[2], nil
}

// Inspect fetches a mod and reads it without installing anything.
func (s *Service) Inspect(ctx context.Context, src string) (Offer, error) {
	src, err := ParseSource(src)
	if err != nil {
		return Offer{}, err
	}
	d, err := s.fetch(ctx, src)
	if err != nil {
		return Offer{}, err
	}
	m, _, err := readZip(d.data)
	if err != nil {
		return Offer{}, err
	}
	o := Offer{Manifest: m, Source: src, Checked: d.checked}
	s.mu.Lock()
	if in, ok := s.load().Mods[m.ID]; ok {
		o.Installed = in.Version
	}
	s.mu.Unlock()
	return o, nil
}

// fetch downloads a mod's zip and checks its sha256, keeping it 10 minutes
// so installing after inspecting doesn't download it again.
func (s *Service) fetch(ctx context.Context, src string) (download, error) {
	s.mu.Lock()
	if d, ok := s.pending[src]; ok && time.Since(d.at) < 10*time.Minute {
		s.mu.Unlock()
		return d, nil
	}
	s.mu.Unlock()

	var data []byte
	var want string
	checked := false
	if !strings.HasPrefix(src, "github.com/") {
		c, err := s.Catalog(ctx, false)
		if err != nil && len(c.Mods) == 0 {
			return download{}, err
		}
		i := slices.IndexFunc(c.Mods, func(m CatalogMod) bool { return m.ID == src })
		if i < 0 {
			return download{}, ErrNotFound
		}
		cm := c.Mods[i]
		if data, err = s.get(ctx, cm.URL, MaxZip); err != nil {
			return download{}, err
		}
		want, checked = cm.SHA256, true
	} else {
		api := s.GitHubAPI
		if api == "" {
			api = "https://api.github.com"
		}
		var rel struct {
			Assets []struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			} `json:"assets"`
		}
		b, err := s.getAPI(ctx, strings.TrimRight(api, "/")+"/repos/"+strings.TrimPrefix(src, "github.com/")+"/releases/latest")
		if err != nil {
			return download{}, err
		}
		if err := json.Unmarshal(b, &rel); err != nil {
			return download{}, ErrNoRelease
		}
		var zipURL, sumURL string
		for _, a := range rel.Assets {
			switch a.Name {
			case "mod.zip":
				zipURL = a.URL
			case "mod.zip.sha256":
				sumURL = a.URL
			}
		}
		if zipURL == "" || sumURL == "" {
			return download{}, ErrNoRelease
		}
		sb, err := s.get(ctx, sumURL, 4096)
		if err != nil {
			return download{}, err
		}
		f := strings.Fields(string(sb))
		if len(f) == 0 || len(f[0]) != 64 {
			return download{}, ErrNoRelease
		}
		want = strings.ToLower(f[0])
		if data, err = s.get(ctx, zipURL, MaxZip); err != nil {
			return download{}, err
		}
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return download{}, ErrChecksum
	}
	d := download{data: data, sum: got, checked: checked, at: time.Now()}
	s.mu.Lock()
	if s.pending == nil {
		s.pending = map[string]download{}
	}
	s.pending[src] = d
	s.mu.Unlock()
	return d, nil
}

func (s *Service) getAPI(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "IceDash-mods")
	res, err := s.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return nil, ErrNoRelease
	case res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("%w (GitHub's limit for this hour is used up)", ErrUnreachable)
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w (%s)", ErrUnreachable, res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 1<<20))
}

// readZip checks a mod's zip: its manifest and, for themes, theme.json and
// theme.css. It returns the manifest and the theme (nil for widgets).
func readZip(data []byte) (Manifest, *Theme, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Manifest{}, nil, bad("it isn't a zip")
	}
	files := map[string]*zip.File{}
	var total uint64
	for _, f := range zr.File {
		name, ok := cleanZipName(f.Name)
		if !ok {
			return Manifest{}, nil, bad("it has a file outside its folder (%q)", f.Name)
		}
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			return Manifest{}, nil, bad("it has a link (%q)", f.Name)
		}
		total += f.UncompressedSize64
		if total > 4*MaxZip || len(files) > 500 {
			return Manifest{}, nil, bad("it unpacks to more than 20 MB or 500 files")
		}
		if !f.FileInfo().IsDir() {
			files[name] = f
		}
	}
	read := func(name string) ([]byte, bool) {
		f, ok := files[name]
		if !ok {
			return nil, false
		}
		rc, err := f.Open()
		if err != nil {
			return nil, false
		}
		defer rc.Close()
		b, err := io.ReadAll(io.LimitReader(rc, MaxZip))
		return b, err == nil
	}
	mb, ok := read("mod.json")
	if !ok {
		return Manifest{}, nil, bad("it has no mod.json at its top")
	}
	var m Manifest
	if err := json.Unmarshal(mb, &m); err != nil {
		return Manifest{}, nil, bad("mod.json can't be read: %v", err)
	}
	m.Preview = nil
	if err := m.Check(); err != nil {
		return Manifest{}, nil, err
	}
	switch m.Kind {
	case "theme":
		tb, ok := read("theme.json")
		if !ok {
			return Manifest{}, nil, bad("a theme needs theme.json")
		}
		var t Theme
		if err := json.Unmarshal(tb, &t); err != nil {
			return Manifest{}, nil, bad("theme.json can't be read: %v", err)
		}
		t.CSS = ""
		if css, ok := read("theme.css"); ok {
			t.CSS = string(css)
		}
		if err := t.Check(); err != nil {
			return Manifest{}, nil, err
		}
		return m, &t, nil
	case "widget":
		if _, ok := files[m.Entry]; !ok {
			return Manifest{}, nil, bad("its entry %s isn't in the zip", m.Entry)
		}
	}
	return m, nil, nil
}

func cleanZipName(n string) (string, bool) {
	n = strings.ReplaceAll(n, `\`, "/")
	if strings.HasPrefix(n, "/") || strings.ContainsRune(n, 0) {
		return "", false
	}
	c := path.Clean(n)
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", false
	}
	return strings.TrimPrefix(c, "./"), true
}

// Install fetches, checks and unpacks a mod: /DATA/.icedash/mods/<id>/<version>.
func (s *Service) Install(ctx context.Context, src string) (Installed, error) {
	src, err := ParseSource(src)
	if err != nil {
		return Installed{}, err
	}
	d, err := s.fetch(ctx, src)
	if err != nil {
		return Installed{}, err
	}
	m, _, err := readZip(d.data)
	if err != nil {
		return Installed{}, err
	}
	if !strings.HasPrefix(src, "github.com/") && m.ID != src {
		return Installed{}, bad("the catalog's %s holds a mod named %s", src, m.ID)
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return Installed{}, err
	}
	modDir := filepath.Join(s.Dir, m.ID)
	tmp, err := os.MkdirTemp(s.Dir, ".install-")
	if err != nil {
		return Installed{}, err
	}
	defer os.RemoveAll(tmp)
	if err := unpack(d.data, tmp); err != nil {
		return Installed{}, err
	}
	if err := os.MkdirAll(modDir, 0o700); err != nil {
		return Installed{}, err
	}
	dest := filepath.Join(modDir, m.Version)
	_ = os.RemoveAll(dest)
	if err := os.Rename(tmp, dest); err != nil {
		return Installed{}, err
	}
	// Only this version stays.
	if ents, err := os.ReadDir(modDir); err == nil {
		for _, e := range ents {
			if e.Name() != m.Version {
				_ = os.RemoveAll(filepath.Join(modDir, e.Name()))
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.load()
	in := &Installed{Manifest: m, Source: src, Checked: d.checked, SHA256: d.sum, Enabled: true, Installed: time.Now().UTC()}
	if old, ok := f.Mods[m.ID]; ok {
		in.Enabled = old.Enabled
	}
	f.Mods[m.ID] = in
	delete(s.pending, src)
	if err := s.save(); err != nil {
		return Installed{}, err
	}
	return *in, nil
}

func unpack(data []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		name, ok := cleanZipName(f.Name)
		if !ok || name == "." {
			continue
		}
		dst := filepath.Join(dir, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, io.LimitReader(rc, int64(4*MaxZip)))
		rc.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Remove deletes a mod. Users of a removed theme are back on Ice; a removed
// widget leaves every panel.
func (s *Service) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.load()
	if _, ok := f.Mods[id]; !ok || !ValidID(id) {
		return ErrNotFound
	}
	delete(f.Mods, id)
	for _, p := range f.Users {
		if p.Theme == id {
			p.Theme = ""
		}
		p.Panel = slices.DeleteFunc(p.Panel, func(x string) bool { return x == id })
	}
	if err := s.save(); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.Dir, id))
}

// SetEnabled switches a widget on or off for everyone.
func (s *Service) SetEnabled(id string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.load().Mods[id]
	if !ok {
		return ErrNotFound
	}
	m.Enabled = on
	return s.save()
}

func (s *Service) prefs(user string) *Prefs {
	f := s.load()
	p, ok := f.Users[user]
	if !ok {
		p = &Prefs{Panel: []string{}}
		f.Users[user] = p
	}
	return p
}

// SetTheme picks a user's theme; "" is Ice.
func (s *Service) SetTheme(user, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != "" {
		m, ok := s.load().Mods[id]
		if !ok || m.Kind != "theme" {
			return ErrNotFound
		}
	}
	s.prefs(user).Theme = id
	return s.save()
}

// SetPanel sets the widgets on a user's panel, top to bottom.
func (s *Service) SetPanel(user string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.load()
	out := []string{}
	for _, id := range ids {
		m, ok := f.Mods[id]
		if !ok || m.Kind != "widget" {
			return ErrNotWidget
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	s.prefs(user).Panel = out
	return s.save()
}

// View is what the UI needs for a user.
type View struct {
	Mods  []Installed `json:"mods"`
	Prefs Prefs       `json:"prefs"`
}

// List returns the installed mods (themes with their colours and CSS) and
// the user's choices.
func (s *Service) List(user string) View {
	s.mu.Lock()
	f := s.load()
	mods := make([]Installed, 0, len(f.Mods))
	for _, m := range f.Mods {
		mods = append(mods, *m)
	}
	p := Prefs{Panel: []string{}}
	if up, ok := f.Users[user]; ok {
		p = Prefs{Theme: up.Theme, Panel: slices.Clone(up.Panel)}
	}
	s.mu.Unlock()
	for i := range mods {
		if mods[i].Kind == "theme" {
			mods[i].Theme = s.readTheme(mods[i].ID, mods[i].Version)
		}
	}
	slices.SortFunc(mods, func(a, b Installed) int { return strings.Compare(a.Name, b.Name) })
	return View{Mods: mods, Prefs: p}
}

func (s *Service) readTheme(id, version string) *Theme {
	dir := filepath.Join(s.Dir, id, version)
	b, err := os.ReadFile(filepath.Join(dir, "theme.json"))
	if err != nil {
		return nil
	}
	var t Theme
	if json.Unmarshal(b, &t) != nil {
		return nil
	}
	t.CSS = ""
	if css, err := os.ReadFile(filepath.Join(dir, "theme.css")); err == nil {
		t.CSS = string(css)
	}
	if t.Check() != nil {
		return nil
	}
	return &t
}

// Widget returns an installed, switched-on widget's folder and manifest.
func (s *Service) Widget(id string) (string, Installed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.load().Mods[id]
	if !ok || m.Kind != "widget" || !m.Enabled {
		return "", Installed{}, ErrNotWidget
	}
	return filepath.Join(s.Dir, id, m.Version), *m, nil
}
