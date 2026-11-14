package mods

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/HSinghHira/IceDash/internal/state"
)

// Service is safe for concurrent use.
type Service struct {
	Dir       string // /DATA/.icedash/mods
	State     string // /DATA/.icedash/mods.json
	Cache     string // /DATA/.icedash/mods-collections.json
	GitHubAPI string // https://api.github.com
	RawBase   string // https://raw.githubusercontent.com
	HTTP      *http.Client
	AllowURL  func(string) error // downloads: GitHub only

	mu   sync.Mutex
	st   *file
	cols map[string]*Collection
}

// Installed is a mod on the box.
type Installed struct {
	Item
	Commit    string    `json:"commit"`
	Enabled   bool      `json:"enabled"`
	Installed time.Time `json:"installed"`
	Update    bool      `json:"update,omitempty"` // a newer version is in its collection
}

// Prefs are one user's choices.
type Prefs struct {
	Theme string   `json:"theme"` // "" is Ice, the built-in
	Panel []string `json:"panel"` // widget ids, top to bottom
}

type file struct {
	Mods        map[string]*Installed `json:"mods"`
	Users       map[string]*Prefs     `json:"users"`
	Collections []string              `json:"collections"` // added by admins, besides the official ones
}

// ---- state ----

func (s *Service) load() *file {
	if s.st != nil {
		return s.st
	}
	s.st = &file{Mods: map[string]*Installed{}, Users: map[string]*Prefs{}, Collections: []string{}}
	if b, err := os.ReadFile(s.State); err == nil {
		var f file
		if json.Unmarshal(b, &f) == nil {
			if f.Mods == nil {
				f.Mods = map[string]*Installed{}
			}
			if f.Users == nil {
				f.Users = map[string]*Prefs{}
			}
			if f.Collections == nil {
				f.Collections = []string{}
			}
			// Mods from before collections (v0.7.0) can't be updated or
			// served any more: they go, and so do their files.
			for id := range f.Mods {
				if !ValidID(id) {
					delete(f.Mods, id)
					_ = os.RemoveAll(filepath.Join(s.Dir, id))
				}
			}
			// …and nobody's theme or panel points at what's gone.
			for _, p := range f.Users {
				if _, ok := f.Mods[p.Theme]; !ok {
					p.Theme = ""
				}
				p.Panel = slices.DeleteFunc(p.Panel, func(id string) bool { _, ok := f.Mods[id]; return !ok })
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

// ---- collections ----

func (s *Service) sources() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append(slices.Clone(Official), s.load().Collections...)
}

// Collections lists every collection's mods, from memory if under 6 hours
// old (refresh asks GitHub again), else from GitHub, else from the last copy.
func (s *Service) Collections(ctx context.Context, refresh bool) []Collection {
	s.loadCache()
	out := []Collection{}
	for _, src := range s.sources() {
		s.mu.Lock()
		c := s.cols[src]
		s.mu.Unlock()
		if c == nil || refresh || time.Since(c.At) > collectionAge {
			fresh, err := s.read(ctx, src)
			if err != nil {
				if c == nil {
					c = &Collection{Source: src, Official: IsOfficial(src), Items: []Item{}}
				}
				cp := *c
				cp.Error = err.Error()
				c = &cp
			} else {
				c = fresh
				s.mu.Lock()
				s.cols[src] = c
				s.mu.Unlock()
				s.saveCache()
			}
		}
		out = append(out, *c)
	}
	return out
}

// The cache keeps each collection's listing (with its files' hashes, which
// install needs) across restarts.
type cached struct {
	Collection
	Files map[string][]treeFile `json:"files"`
}

func (s *Service) loadCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cols != nil {
		return
	}
	s.cols = map[string]*Collection{}
	b, err := os.ReadFile(s.Cache)
	if err != nil {
		return
	}
	var all []cached
	if json.Unmarshal(b, &all) != nil {
		return
	}
	for _, c := range all {
		col := c.Collection
		for i := range col.Items {
			col.Items[i].commit, col.Items[i].files = col.Commit, c.Files[col.Items[i].Folder]
		}
		s.cols[col.Source] = &col
	}
}

func (s *Service) saveCache() {
	if s.Cache == "" {
		return
	}
	s.mu.Lock()
	all := []cached{}
	for _, c := range s.cols {
		files := map[string][]treeFile{}
		for _, it := range c.Items {
			files[it.Folder] = it.files
		}
		all = append(all, cached{Collection: *c, Files: files})
	}
	s.mu.Unlock()
	if b, err := json.Marshal(all); err == nil {
		_ = state.WriteFile(s.Cache, b)
	}
}

// AddCollection adds a repo by its address, once it lists at least one mod.
func (s *Service) AddCollection(ctx context.Context, address string) (Collection, error) {
	src, err := ParseCollection(address)
	if err != nil {
		return Collection{}, err
	}
	if IsOfficial(src) {
		return Collection{}, errors.New("that collection is already there: it's one of the official ones")
	}
	c, err := s.read(ctx, src)
	if err != nil {
		return Collection{}, err
	}
	if len(c.Items) == 0 && len(c.Refused) == 0 {
		return Collection{}, ErrEmpty
	}
	s.loadCache()
	s.mu.Lock()
	f := s.load()
	if !slices.ContainsFunc(f.Collections, func(x string) bool { return strings.EqualFold(x, src) }) {
		f.Collections = append(f.Collections, src)
	}
	s.cols[src] = c
	err = s.save()
	s.mu.Unlock()
	s.saveCache()
	return *c, err
}

// RemoveCollection forgets an added collection; its installed mods stay.
func (s *Service) RemoveCollection(address string) error {
	src, err := ParseCollection(address)
	if err != nil {
		return err
	}
	if IsOfficial(src) {
		return ErrOfficial
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.load()
	i := slices.IndexFunc(f.Collections, func(x string) bool { return strings.EqualFold(x, src) })
	if i < 0 {
		return ErrNotFound
	}
	f.Collections = slices.Delete(f.Collections, i, i+1)
	delete(s.cols, src)
	return s.save()
}

func (s *Service) find(ctx context.Context, id string) (Item, error) {
	for _, c := range s.Collections(ctx, false) {
		for _, it := range c.Items {
			if it.ID == id {
				return it, nil
			}
		}
	}
	return Item{}, ErrNotFound
}

// ---- installing ----

// Install copies a mod's folder at its collection's commit, checking every
// file against its git hash: /DATA/.icedash/mods/<id>/<version>.
func (s *Service) Install(ctx context.Context, id string) (Installed, error) {
	it, err := s.find(ctx, id)
	if err != nil {
		return Installed{}, err
	}
	if it.commit == "" || len(it.files) == 0 {
		// Listed from an old cache without file hashes: ask GitHub again.
		s.Collections(ctx, true)
		if it, err = s.find(ctx, id); err != nil {
			return Installed{}, err
		}
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return Installed{}, err
	}
	tmp, err := os.MkdirTemp(s.Dir, ".install-")
	if err != nil {
		return Installed{}, err
	}
	defer os.RemoveAll(tmp)
	var total int64
	for _, f := range it.files {
		rel := strings.TrimPrefix(f.Path, it.Folder+"/")
		clean := filepath.Clean(filepath.FromSlash(rel))
		if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return Installed{}, bad("it has a file outside its folder (%q)", f.Path)
		}
		max := int64(MaxFolder)
		if slices.Contains(WallpaperNames, rel) {
			max = MaxWallpaper
		}
		b, err := s.file(ctx, it.Collection, it.commit, f, max)
		if err != nil {
			return Installed{}, err
		}
		total += int64(len(b))
		if total > MaxFolder {
			return Installed{}, bad("it is larger than 10 MB")
		}
		if want := wallpaperType(rel); want != "" && it.Kind == "theme" && ImageType(b) != want {
			return Installed{}, bad("%s isn't really a %s image", rel, strings.TrimPrefix(want, "image/"))
		}
		dst := filepath.Join(tmp, clean)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return Installed{}, err
		}
		if err := os.WriteFile(dst, b, 0o600); err != nil {
			return Installed{}, err
		}
	}
	modDir := filepath.Join(s.Dir, it.ID)
	if err := os.MkdirAll(modDir, 0o700); err != nil {
		return Installed{}, err
	}
	dest := filepath.Join(modDir, it.Version)
	_ = os.RemoveAll(dest)
	if err := os.Rename(tmp, dest); err != nil {
		return Installed{}, err
	}
	if ents, err := os.ReadDir(modDir); err == nil {
		for _, e := range ents {
			if e.Name() != it.Version {
				_ = os.RemoveAll(filepath.Join(modDir, e.Name()))
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.load()
	in := &Installed{Item: it, Commit: it.commit, Enabled: true, Installed: time.Now().UTC()}
	if old, ok := f.Mods[it.ID]; ok {
		in.Enabled = old.Enabled
	}
	in.files = nil
	f.Mods[it.ID] = in
	return *in, s.save()
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

// List returns the installed mods (each marked when its collection has a
// newer version) and the user's choices.
func (s *Service) List(user string) View {
	s.loadCache()
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.load()
	mods := make([]Installed, 0, len(f.Mods))
	for _, m := range f.Mods {
		cp := *m
		if c, ok := s.cols[m.Collection]; ok {
			for _, it := range c.Items {
				if it.ID == m.ID && it.Version != m.Version {
					cp.Update = true
				}
			}
		}
		if cp.Kind == "theme" && cp.Wallpaper != "" {
			cp.Wallpaper = FilesPrefix + cp.ID + "/wallpaper"
		}
		mods = append(mods, cp)
	}
	p := Prefs{Panel: []string{}}
	if up, ok := f.Users[user]; ok {
		p = Prefs{Theme: up.Theme, Panel: slices.Clone(up.Panel)}
	}
	slices.SortFunc(mods, func(a, b Installed) int { return strings.Compare(a.Name, b.Name) })
	return View{Mods: mods, Prefs: p}
}

// installed returns an installed mod's folder and record.
func (s *Service) installed(id string) (string, Installed, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.load().Mods[id]
	if !ok {
		return "", Installed{}, false
	}
	return filepath.Join(s.Dir, id, m.Version), *m, true
}

// Widget returns an installed, switched-on widget's folder and record.
func (s *Service) Widget(id string) (string, Installed, error) {
	dir, m, ok := s.installed(id)
	if !ok || m.Kind != "widget" || !m.Enabled {
		return "", Installed{}, ErrNotWidget
	}
	return dir, m, nil
}
