// Package catalog reads the IceStore catalog (PLAN.md, Phase 2, "IceStore app").
//
// The index is https://icestore.hsinghhira.me/index.json (84 KB for 122 apps,
// measured 2026-11-12), not store.zip (142 MB). Each app's details come from
// its meta.json, and its full compose file (with the help text for every
// setting) from the IceStore repo on GitHub.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/HSinghHira/IceDash/internal/state"
)

// Where the catalog lives.
const (
	Site = "https://icestore.hsinghhira.me"
	Raw  = "https://raw.githubusercontent.com/HSinghHira/IceStore/main"
)

// IDPrefix marks IceStore apps (x-casaos.id), measured on the box.
const IDPrefix = "com.icestore."

// MaxAge is how long a fetched index is used before it's fetched again.
const MaxAge = 6 * time.Hour

// App is one entry of index.json, with its links made absolute.
type App struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"` // the id without com.icestore.: the compose name and Apps/<name>
	Title         string   `json:"title"`
	Tagline       string   `json:"tagline"`
	Category      string   `json:"category"`
	Categories    []string `json:"categories"`
	Author        string   `json:"author"`
	Developer     string   `json:"developer"`
	Architectures []string `json:"architectures"`
	Icon          string   `json:"icon"`
	Thumbnail     string   `json:"thumbnail"`
	ComposeURL    string   `json:"compose_url"`
	MetaURL       string   `json:"meta_url"`
	ContentHash   string   `json:"content_hash"`
	Version       string   `json:"version"`
}

// Index is index.json.
type Index struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	AppCount  int       `json:"app_count"`
	BaseURL   string    `json:"base_url"`
	Apps      []App     `json:"apps"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Meta is an app's meta.json: the page's details.
type Meta struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Tagline     string   `json:"tagline"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Developer   string   `json:"developer"`
	Author      string   `json:"author"`
	Version     string   `json:"version"`
	UpdateAt    string   `json:"update_at"`
	ReleaseNote string   `json:"release_note"`
	Screenshots []string `json:"screenshot_link"`
	Thumbnail   string   `json:"thumbnail"`
	Tips        struct {
		BeforeInstall string `json:"before_install"`
	} `json:"tips"`
	Repo          string           `json:"repo"`
	Support       string           `json:"support"`
	Docs          string           `json:"docs"`
	MinMemory     int64            `json:"min_memory"`
	MinImageSize  map[string]int64 `json:"min_image_size"`
	Architectures []string         `json:"architectures"`
}

// Catalog fetches and caches the catalog. Safe for concurrent use.
type Catalog struct {
	Site  string // default Site
	Raw   string // default Raw
	HTTP  *http.Client
	Cache string // the cached index file, in /DATA/.icedash/catalog
	Now   func() time.Time

	mu    sync.Mutex
	idx   *Index
	metas map[string]metaEntry
}

type metaEntry struct {
	hash string
	meta Meta
}

const maxIndex = 8 << 20
const maxFile = 2 << 20

var nameOK = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// ValidName reports whether s is safe as an app name in paths and URLs.
func ValidName(s string) bool { return nameOK.MatchString(s) && !strings.Contains(s, "..") }

func (c *Catalog) site() string {
	if c.Site != "" {
		return strings.TrimRight(c.Site, "/")
	}
	return Site
}

func (c *Catalog) raw() string {
	if c.Raw != "" {
		return strings.TrimRight(c.Raw, "/")
	}
	return Raw
}

func (c *Catalog) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// ErrUnknownApp is returned for an id that isn't in the index.
var ErrUnknownApp = errors.New("that app isn't in IceStore")

// Index returns the index, fetching it when there's none or it's older than
// MaxAge (or force is set). When fetching fails, a cached copy is returned
// with the error, so the window still works offline.
func (c *Catalog) Index(ctx context.Context, force bool) (Index, error) {
	c.mu.Lock()
	if c.idx == nil && c.Cache != "" {
		if b, err := os.ReadFile(c.Cache); err == nil {
			var idx Index
			if json.Unmarshal(b, &idx) == nil {
				c.idx = &idx
			}
		}
	}
	cur := c.idx
	c.mu.Unlock()
	if cur != nil && !force && c.now().Sub(cur.FetchedAt) < MaxAge {
		return *cur, nil
	}

	idx, err := c.fetchIndex(ctx)
	if err != nil {
		if cur != nil {
			return *cur, err
		}
		return Index{}, err
	}
	c.mu.Lock()
	c.idx = &idx
	c.mu.Unlock()
	if c.Cache != "" {
		if b, err := json.Marshal(idx); err == nil {
			_ = state.WriteFile(c.Cache, b)
		}
	}
	return idx, nil
}

func (c *Catalog) fetchIndex(ctx context.Context) (Index, error) {
	b, err := c.get(ctx, c.site()+"/index.json", maxIndex)
	if err != nil {
		return Index{}, err
	}
	var idx Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return Index{}, fmt.Errorf("IceStore index: %w", err)
	}
	base := c.site()
	apps := idx.Apps[:0]
	for _, a := range idx.Apps {
		a.Name = strings.TrimPrefix(a.ID, IDPrefix)
		if !strings.HasPrefix(a.ID, IDPrefix) || !ValidName(a.Name) {
			continue
		}
		a.Icon = abs(base, a.Icon)
		a.Thumbnail = abs(base, a.Thumbnail)
		a.ComposeURL = abs(base, a.ComposeURL)
		a.MetaURL = abs(base, a.MetaURL)
		apps = append(apps, a)
	}
	idx.Apps = apps
	idx.FetchedAt = c.now().UTC()
	return idx, nil
}

// App finds an app by its id (com.icestore.x) or name (x).
func (c *Catalog) App(ctx context.Context, id string) (App, error) {
	idx, err := c.Index(ctx, false)
	if len(idx.Apps) == 0 && err != nil {
		return App{}, err
	}
	for _, a := range idx.Apps {
		if a.ID == id || a.Name == id {
			return a, nil
		}
	}
	return App{}, ErrUnknownApp
}

// Meta returns the app's meta.json, cached until its content hash changes.
func (c *Catalog) Meta(ctx context.Context, id string) (Meta, error) {
	a, err := c.App(ctx, id)
	if err != nil {
		return Meta{}, err
	}
	c.mu.Lock()
	if e, ok := c.metas[a.ID]; ok && e.hash == a.ContentHash {
		c.mu.Unlock()
		return e.meta, nil
	}
	c.mu.Unlock()
	b, err := c.get(ctx, a.MetaURL, maxFile)
	if err != nil {
		return Meta{}, err
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, fmt.Errorf("IceStore app details: %w", err)
	}
	base := c.site()
	m.Thumbnail = abs(base, m.Thumbnail)
	for i, s := range m.Screenshots {
		m.Screenshots[i] = abs(base, s)
	}
	c.mu.Lock()
	if c.metas == nil {
		c.metas = map[string]metaEntry{}
	}
	c.metas[a.ID] = metaEntry{a.ContentHash, m}
	c.mu.Unlock()
	return m, nil
}

// Compose returns the app's full compose file from the IceStore repo, which
// has the per-setting help text; the site's trimmed copy is the fallback.
func (c *Catalog) Compose(ctx context.Context, id string) ([]byte, error) {
	a, err := c.App(ctx, id)
	if err != nil {
		return nil, err
	}
	b, err := c.get(ctx, c.raw()+"/Apps/"+url.PathEscape(a.Name)+"/docker-compose.yml", maxFile)
	if err == nil {
		return b, nil
	}
	if b2, err2 := c.get(ctx, a.ComposeURL, maxFile); err2 == nil {
		return b2, nil
	}
	return nil, err
}

// AllowedURL keeps fetches to the catalog's hosts.
func (c *Catalog) allowed(u string) bool {
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "https" {
		return false
	}
	for _, base := range []string{c.site(), c.raw()} {
		if b, err := url.Parse(base); err == nil && strings.EqualFold(b.Host, p.Host) {
			return true
		}
	}
	return false
}

func (c *Catalog) get(ctx context.Context, u string, max int64) ([]byte, error) {
	if !c.allowed(u) {
		return nil, fmt.Errorf("refusing to fetch %s: not an IceStore address", u)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't reach IceStore: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IceStore answered %s for %s", res.Status, u)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("%s is too large", u)
	}
	return b, nil
}

func abs(base, p string) string {
	if p == "" || strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://") {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return base + p
}
