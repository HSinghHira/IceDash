package mods

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

// The official collections (PLAN.md 4.2).
const (
	OfficialThemes  = "github.com/HSinghHira/IceColors"
	OfficialWidgets = "github.com/HSinghHira/IceCards"
)

// Official lists the collections every IceDash shows.
var Official = []string{OfficialThemes, OfficialWidgets}

// Errors the API words for the UI.
var (
	ErrNotFound    = errors.New("there's no such mod")
	ErrBadAddress  = errors.New("write the collection's address as github.com/owner/repo")
	ErrNoRepo      = errors.New("that repository can't be found on GitHub")
	ErrEmpty       = errors.New("that repository has no themes or widgets in it")
	ErrTampered    = errors.New("a file doesn't match the commit it should come from, so nothing was installed")
	ErrUnreachable = errors.New("GitHub can't be reached right now")
	ErrNotWidget   = errors.New("that isn't an installed widget")
	ErrOfficial    = errors.New("the official collections can't be removed")
)

const collectionAge = 6 * time.Hour

var ghRx = regexp.MustCompile(`^(?:https?://)?(?:www\.)?(?:github\.com/)?([A-Za-z0-9-]{1,39})/([A-Za-z0-9._-]{1,100}?)(?:\.git)?/?$`)

// ParseCollection turns what was typed into github.com/owner/repo.
func ParseCollection(src string) (string, error) {
	m := ghRx.FindStringSubmatch(strings.TrimSpace(src))
	if m == nil || m[2] == "." || m[2] == ".." {
		return "", ErrBadAddress
	}
	return "github.com/" + m[1] + "/" + m[2], nil
}

// IsOfficial reports whether a collection is one of IceDash's own.
func IsOfficial(src string) bool {
	return slices.ContainsFunc(Official, func(o string) bool { return strings.EqualFold(o, src) })
}

// ModID names a mod by its collection and folder: owner--repo--folder.
func ModID(src, folder string) string {
	slug := func(s string) string {
		s = strings.ToLower(s)
		return strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
				return r
			}
			return '-'
		}, s)
	}
	parts := strings.Split(strings.TrimPrefix(src, "github.com/"), "/")
	return slug(parts[0]) + "--" + slug(parts[1]) + "--" + folder
}

var idRx = regexp.MustCompile(`^[a-z0-9-]{1,39}--[a-z0-9-]{1,100}--[a-z0-9][a-z0-9-]{0,39}$`)

// ValidID reports whether id can name a mod.
func ValidID(id string) bool { return idRx.MatchString(id) }

// Item is a mod in a collection, as it is at the collection's commit.
type Item struct {
	ID          string            `json:"id"`
	Collection  string            `json:"collection"`
	Folder      string            `json:"folder"`
	Official    bool              `json:"official"`
	Kind        string            `json:"kind"` // theme, widget
	Name        string            `json:"name"`
	Author      string            `json:"author"`
	Description string            `json:"description"`
	Version     string            `json:"version"` // the folder's git tree hash, short
	Scheme      string            `json:"scheme,omitempty"`
	Colors      map[string]string `json:"colors,omitempty"`
	Wallpaper   string            `json:"wallpaper,omitempty"` // a preview address on GitHub
	Entry       string            `json:"entry,omitempty"`
	Size        string            `json:"size,omitempty"`
	Permissions *Permissions      `json:"permissions,omitempty"`

	commit string
	files  []treeFile
}

// Collection is a repo's mods at its latest commit.
type Collection struct {
	Source   string    `json:"source"`
	Official bool      `json:"official"`
	Commit   string    `json:"commit,omitempty"`
	Items    []Item    `json:"items"`
	Refused  []Refused `json:"refused,omitempty"`
	Error    string    `json:"error,omitempty"`
	At       time.Time `json:"at"`
}

// Refused is a folder that looks like a mod but breaks a rule, with why.
type Refused struct {
	Folder string `json:"folder"`
	Reason string `json:"reason"`
}

type treeFile struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

func (s *Service) api() string {
	if s.GitHubAPI != "" {
		return strings.TrimRight(s.GitHubAPI, "/")
	}
	return "https://api.github.com"
}

func (s *Service) raw() string {
	if s.RawBase != "" {
		return strings.TrimRight(s.RawBase, "/")
	}
	return "https://raw.githubusercontent.com"
}

func (s *Service) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (s *Service) fetch(ctx context.Context, u string, max int64, api bool) ([]byte, error) {
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
	if api {
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	res, err := s.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusConflict: // 409: an empty repo
		return nil, ErrNoRepo
	case res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("%w (GitHub's limit for this hour is used up; try again later)", ErrUnreachable)
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w (%s)", ErrUnreachable, res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, bad("a file is larger than allowed")
	}
	return b, nil
}

// gitHash is the hash git gives a file: sha1("blob <size>\0" + bytes).
func gitHash(b []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// file fetches one file at the commit and checks it against the tree.
func (s *Service) file(ctx context.Context, src, commit string, f treeFile, max int64) ([]byte, error) {
	repo := strings.TrimPrefix(src, "github.com/")
	b, err := s.fetch(ctx, s.raw()+"/"+repo+"/"+commit+"/"+f.Path, max, false)
	if err != nil {
		return nil, err
	}
	if gitHash(b) != f.SHA {
		return nil, ErrTampered
	}
	return b, nil
}

// read lists a collection at its latest commit: 2 API calls, then each
// folder's manifest from raw.githubusercontent.com.
func (s *Service) read(ctx context.Context, src string) (*Collection, error) {
	repo := strings.TrimPrefix(src, "github.com/")
	var commit struct {
		SHA string `json:"sha"`
	}
	b, err := s.fetch(ctx, s.api()+"/repos/"+repo+"/commits/HEAD", 1<<20, true)
	if err != nil {
		return nil, err
	}
	if json.Unmarshal(b, &commit) != nil || len(commit.SHA) != 40 {
		return nil, ErrNoRepo
	}
	var tree struct {
		Tree      []treeFile `json:"tree"`
		Truncated bool       `json:"truncated"`
	}
	b, err = s.fetch(ctx, s.api()+"/repos/"+repo+"/git/trees/"+commit.SHA+"?recursive=1", 8<<20, true)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &tree); err != nil {
		return nil, ErrNoRepo
	}
	c := &Collection{Source: src, Official: IsOfficial(src), Commit: commit.SHA, Items: []Item{}, At: time.Now().UTC()}
	folders := map[string]string{} // folder → its tree hash
	files := map[string][]treeFile{}
	for _, f := range tree.Tree {
		top, rest, nested := strings.Cut(f.Path, "/")
		if !folderRx.MatchString(top) {
			continue
		}
		if !nested {
			if f.Type == "tree" {
				folders[top] = f.SHA
			}
			continue
		}
		if f.Type == "blob" {
			files[top] = append(files[top], treeFile{Path: f.Path, Type: f.Type, SHA: f.SHA, Size: f.Size})
		}
		_ = rest
	}
	names := make([]string, 0, len(folders))
	for k := range folders {
		names = append(names, k)
	}
	slices.Sort(names)
	for _, folder := range names {
		it, err := s.item(ctx, c, folder, folders[folder], files[folder])
		if err != nil {
			reason := strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": ")
			c.Refused = append(c.Refused, Refused{Folder: folder, Reason: reason})
			continue // a broken mod: left out, with the reason shown
		}
		if it == nil {
			continue // not a mod
		}
		c.Items = append(c.Items, *it)
	}
	return c, nil
}

// item reads one folder's manifest; nil when it is neither a theme nor a widget.
func (s *Service) item(ctx context.Context, c *Collection, folder, treeSHA string, files []treeFile) (*Item, error) {
	if len(files) > maxFiles {
		return nil, bad("%s has more than %d files", folder, maxFiles)
	}
	byName := map[string]treeFile{}
	var total int64
	for _, f := range files {
		byName[strings.TrimPrefix(f.Path, folder+"/")] = f
		total += f.Size
	}
	if total > MaxFolder {
		return nil, bad("%s is larger than 10 MB", folder)
	}
	repo := strings.TrimPrefix(c.Source, "github.com/")
	it := &Item{ID: ModID(c.Source, folder), Collection: c.Source, Folder: folder, Official: c.Official,
		Version: treeSHA[:min(12, len(treeSHA))], commit: c.Commit, files: files}
	if f, ok := byName["theme.json"]; ok {
		b, err := s.file(ctx, c.Source, c.Commit, f, 64<<10)
		if err != nil {
			return nil, err
		}
		var t ThemeFile
		if err := json.Unmarshal(b, &t); err != nil {
			return nil, bad("%s/theme.json can't be read", folder)
		}
		if err := t.Check(); err != nil {
			return nil, err
		}
		it.Kind, it.Name, it.Author, it.Description, it.Scheme, it.Colors = "theme", t.Name, t.Author, t.Description, t.Scheme, t.Colors
		for _, w := range WallpaperNames {
			if f, ok := byName[w]; ok && f.Size <= MaxWallpaper {
				it.Wallpaper = s.raw() + "/" + repo + "/" + c.Commit + "/" + path.Join(folder, w)
				break
			}
		}
		return it, nil
	}
	if f, ok := byName["widget.json"]; ok {
		b, err := s.file(ctx, c.Source, c.Commit, f, 64<<10)
		if err != nil {
			return nil, err
		}
		var w WidgetFile
		if err := json.Unmarshal(b, &w); err != nil {
			return nil, bad("%s/widget.json can't be read", folder)
		}
		if err := w.Check(); err != nil {
			return nil, err
		}
		if _, ok := byName[w.Entry]; !ok {
			return nil, bad("%s has no %s", folder, w.Entry)
		}
		perms := w.Permissions
		it.Kind, it.Name, it.Author, it.Description = "widget", w.Name, w.Author, w.Description
		it.Entry, it.Size, it.Permissions = w.Entry, w.Size, &perms
		return it, nil
	}
	return nil, nil
}

// AllowedURL lets the mods service reach only the two GitHub hosts it uses:
// the API (commits and trees) and raw files at a commit, over HTTPS.
func AllowedURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch strings.ToLower(u.Hostname()) {
	case "api.github.com", "raw.githubusercontent.com":
		if u.Scheme == "https" {
			return nil
		}
	}
	return fmt.Errorf("refusing to fetch %s: mods come from GitHub only", u.Redacted())
}
