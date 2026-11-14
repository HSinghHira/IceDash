// Package files is the Files app's backend (PLAN.md, Phase 3c, "Files app").
//
// Everything happens inside one folder, /DATA, through an os.Root: the kernel
// refuses any path that leaves it, symlinks included. Paths in the API are
// relative to that folder and written with "/" ("/Documents/a.txt"; "/" is
// the folder itself).
//
// /DATA is also where the system keeps its own things (measured on ZimaOS
// 1.8.0: root's home, .docker with container mounts, .casaos, .icedash ...),
// so every top-level entry whose name starts with a dot, lost+found, and
// anything on another disk can be looked at but never changed.
package files

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Errors the API words for the UI.
var (
	ErrNotFound  = errors.New("that file or folder isn't there")
	ErrProtected = errors.New("that belongs to the system and can't be changed here")
	ErrExists    = errors.New("something with that name is already there")
	ErrBadName   = errors.New("that name can't be used")
	ErrBadPath   = errors.New("that path is outside /DATA")
	ErrIsDir     = errors.New("that is a folder")
	ErrNotDir    = errors.New("that isn't a folder")
	ErrIntoSelf  = errors.New("a folder can't go inside itself")
	ErrTicket    = errors.New("that link has expired; open it again")
)

// trashDir is the Trash, inside IceDash's own state folder.
const trashDir = ".icedash/trash"

// Service is safe for concurrent use.
type Service struct {
	Root string // /DATA

	mu      sync.Mutex
	tickets map[string]ticket
	jobs    map[string]*job
}

func (s *Service) open() (*os.Root, error) {
	r, err := os.OpenRoot(s.Root)
	if err != nil {
		return nil, fmt.Errorf("can't open %s: %w", s.Root, err)
	}
	return r, nil
}

// Clean turns an API path into one relative to the root: "" is the root.
// It refuses paths that climb out ("/../etc").
func Clean(p string) (string, error) {
	if strings.ContainsRune(p, 0) || strings.Contains(p, `\`) {
		return "", ErrBadPath
	}
	c := path.Clean("/" + p)
	if c == "/" {
		return "", nil
	}
	rel := strings.TrimPrefix(c, "/")
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return "", ErrBadPath
		}
	}
	return rel, nil
}

// apiPath is rel as the API writes it.
func apiPath(rel string) string { return "/" + rel }

// ValidName reports whether n can be a file name.
func ValidName(n string) bool {
	return n != "" && n != "." && n != ".." && len(n) <= 255 &&
		!strings.ContainsAny(n, "/\\\x00")
}

func top(rel string) string {
	t, _, _ := strings.Cut(rel, "/")
	return t
}

// Protected reports whether rel belongs to the system: a dot-entry at the
// top, lost+found, IceDash's state folder, or anything below them.
func Protected(rel string) bool {
	t := top(rel)
	return strings.HasPrefix(t, ".") || t == "lost+found"
}

// writable refuses the root itself, protected paths and other disks.
func (s *Service) writable(r *os.Root, rel string) error {
	if rel == "" || Protected(rel) {
		return ErrProtected
	}
	if otherDisk(r, rel) {
		return ErrProtected
	}
	return nil
}

// dirWritable is whether new things may go into the folder rel ("" is fine).
func (s *Service) dirWritable(r *os.Root, rel string) error {
	if rel == "" {
		return nil
	}
	return s.writable(r, rel)
}

func notFound(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	if strings.Contains(err.Error(), "path escapes") {
		return ErrBadPath
	}
	return err
}

// ---- listing ----

// Entry is one file or folder.
type Entry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Dir       bool      `json:"dir"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
	Mode      string    `json:"mode"`
	Link      bool      `json:"link,omitempty"`
	Broken    bool      `json:"broken,omitempty"` // a link that points nowhere, or out of /DATA
	Protected bool      `json:"protected,omitempty"`
	Kind      string    `json:"kind"` // folder, image, text, pdf, video, audio, other
}

// Listing is a folder's content.
type Listing struct {
	Path      string  `json:"path"`
	Protected bool    `json:"protected"`
	Entries   []Entry `json:"entries"`
	Hidden    int     `json:"hidden"` // entries left out because they start with a dot
}

// List reads the folder p. Dot-entries are left out unless hidden.
func (s *Service) List(p string, hidden bool) (Listing, error) {
	rel, err := Clean(p)
	if err != nil {
		return Listing{}, err
	}
	r, err := s.open()
	if err != nil {
		return Listing{}, err
	}
	defer r.Close()
	d, err := r.Open(dotIfEmpty(rel))
	if err != nil {
		return Listing{}, notFound(err)
	}
	defer d.Close()
	st, err := d.Stat()
	if err != nil {
		return Listing{}, err
	}
	if !st.IsDir() {
		return Listing{}, ErrNotDir
	}
	des, err := d.ReadDir(-1)
	if err != nil {
		return Listing{}, err
	}
	out := Listing{Path: apiPath(rel), Protected: rel != "" && Protected(rel), Entries: []Entry{}}
	for _, de := range des {
		name := de.Name()
		if !hidden && strings.HasPrefix(name, ".") {
			out.Hidden++
			continue
		}
		child := path.Join(rel, name)
		out.Entries = append(out.Entries, s.entry(r, child, de))
	}
	slices.SortFunc(out.Entries, func(a, b Entry) int {
		if a.Dir != b.Dir {
			if a.Dir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

func dotIfEmpty(rel string) string {
	if rel == "" {
		return "."
	}
	return rel
}

func (s *Service) entry(r *os.Root, rel string, de fs.DirEntry) Entry {
	e := Entry{Name: de.Name(), Path: apiPath(rel), Protected: Protected(rel)}
	info, err := de.Info()
	if err != nil {
		e.Broken = true
		return e
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		e.Link = true
		// Follow it only inside /DATA: os.Root refuses a target outside.
		if t, err := r.Stat(rel); err == nil {
			info = t
		} else {
			e.Broken = true
		}
	}
	e.Dir = info.IsDir()
	e.Modified = info.ModTime().UTC()
	e.Mode = info.Mode().Perm().String()
	if !e.Dir {
		e.Size = info.Size()
	}
	if !e.Protected && !e.Link && otherDisk(r, rel) {
		e.Protected = true
	}
	e.Kind = KindOf(e.Name, e.Dir)
	return e
}

// KindOf picks how a file previews, by its extension.
func KindOf(name string, dir bool) string {
	if dir {
		return "folder"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".bmp", ".svg", ".ico":
		return "image"
	case ".pdf":
		return "pdf"
	case ".mp4", ".webm", ".mov", ".m4v", ".mkv", ".ogv":
		return "video"
	case ".mp3", ".m4a", ".aac", ".ogg", ".oga", ".opus", ".wav", ".flac":
		return "audio"
	case ".txt", ".md", ".log", ".json", ".yml", ".yaml", ".toml", ".ini", ".conf", ".cfg", ".env",
		".xml", ".csv", ".tsv", ".sh", ".bash", ".py", ".js", ".ts", ".go", ".rs", ".java", ".c", ".h",
		".cpp", ".css", ".html", ".htm", ".vue", ".sql", ".properties", ".service", ".srt", ".vtt":
		return "text"
	}
	if strings.HasPrefix(strings.ToLower(name), "dockerfile") || name == "Makefile" {
		return "text"
	}
	return "other"
}

// ContentType is what a file is served as inline. Only types a browser shows
// without running anything keep their own type; the rest (HTML, SVG, scripts)
// is text/plain, so a file in /DATA can't run inside IceDash's page.
func ContentType(name string) string {
	switch KindOf(name, false) {
	case "image":
		if strings.EqualFold(filepath.Ext(name), ".svg") {
			return "text/plain; charset=utf-8"
		}
		fallthrough
	case "pdf", "video", "audio":
		ext := strings.ToLower(filepath.Ext(name))
		if t, ok := mediaTypes[ext]; ok {
			return t
		}
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
		return "application/octet-stream"
	}
	return "text/plain; charset=utf-8"
}

// mediaTypes doesn't depend on the host's /etc/mime.types, which Go reads
// for anything beyond its own short list (no video or audio in it).
var mediaTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".gif": "image/gif",
	".webp": "image/webp", ".avif": "image/avif", ".bmp": "image/bmp", ".ico": "image/x-icon",
	".pdf": "application/pdf",
	".mp4": "video/mp4", ".m4v": "video/mp4", ".webm": "video/webm", ".mov": "video/quicktime",
	".mkv": "video/x-matroska", ".ogv": "video/ogg",
	".mp3": "audio/mpeg", ".m4a": "audio/mp4", ".aac": "audio/aac", ".ogg": "audio/ogg", ".oga": "audio/ogg",
	".opus": "audio/ogg", ".wav": "audio/wav", ".flac": "audio/flac",
}

// Open opens a file for reading (downloads and previews).
func (s *Service) Open(p string) (*os.File, fs.FileInfo, error) {
	rel, err := Clean(p)
	if err != nil {
		return nil, nil, err
	}
	r, err := s.open()
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()
	f, err := r.Open(dotIfEmpty(rel))
	if err != nil {
		return nil, nil, notFound(err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if st.IsDir() {
		f.Close()
		return nil, nil, ErrIsDir
	}
	return f, st, nil
}

// ---- creating and changing ----

// Mkdir makes the folder name in p.
func (s *Service) Mkdir(p, name string) (Entry, error) {
	dir, err := Clean(p)
	if err != nil {
		return Entry{}, err
	}
	if !ValidName(name) {
		return Entry{}, ErrBadName
	}
	r, err := s.open()
	if err != nil {
		return Entry{}, err
	}
	defer r.Close()
	if err := s.dirWritable(r, dir); err != nil {
		return Entry{}, err
	}
	rel := path.Join(dir, name)
	if err := r.Mkdir(rel, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return Entry{}, ErrExists
		}
		return Entry{}, notFound(err)
	}
	_ = r.Chmod(rel, 0o755) // the umask may have taken bits off
	return s.stat(r, rel)
}

func (s *Service) stat(r *os.Root, rel string) (Entry, error) {
	info, err := r.Lstat(rel)
	if err != nil {
		return Entry{}, notFound(err)
	}
	return s.entry(r, rel, fs.FileInfoToDirEntry(info)), nil
}

// Rename gives p a new name in the same folder.
func (s *Service) Rename(p, name string) (Entry, error) {
	rel, err := Clean(p)
	if err != nil {
		return Entry{}, err
	}
	if !ValidName(name) {
		return Entry{}, ErrBadName
	}
	r, err := s.open()
	if err != nil {
		return Entry{}, err
	}
	defer r.Close()
	if err := s.writable(r, rel); err != nil {
		return Entry{}, err
	}
	if _, err := r.Lstat(rel); err != nil {
		return Entry{}, notFound(err)
	}
	to := path.Join(path.Dir(rel), name)
	if path.Dir(rel) == "." {
		to = name
	}
	if Protected(to) {
		return Entry{}, ErrProtected
	}
	if to == rel {
		return s.stat(r, rel)
	}
	// A case-only change on a case-blind disk finds itself: allow it.
	if _, err := r.Lstat(to); err == nil && !strings.EqualFold(to, rel) {
		return Entry{}, ErrExists
	}
	if err := r.Rename(rel, to); err != nil {
		return Entry{}, notFound(err)
	}
	return s.stat(r, to)
}

// Clash says what to do when the target name is taken.
type Clash string

const (
	ClashAsk     Clash = ""        // answer ErrExists
	ClashReplace Clash = "replace" // overwrite a file (never a folder)
	ClashKeep    Clash = "keep"    // keep both: "name (2).ext"
	ClashSkip    Clash = "skip"
)

// freeName finds "name (n).ext" not yet in dir.
func freeName(r *os.Root, dir, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if ext == name { // ".env"
		base, ext = name, ""
	}
	for n := 2; ; n++ {
		try := fmt.Sprintf("%s (%d)%s", base, n, ext)
		if _, err := r.Lstat(path.Join(dir, try)); errors.Is(err, fs.ErrNotExist) {
			return try
		}
	}
}

// Upload writes body to name in the folder p. name may hold "/" for a file
// inside a dropped folder ("photos/2024/a.jpg"); the folders are made. The
// file is written beside its target and renamed at the end, so a cancelled
// upload leaves nothing half-written. It reports the entry, or skipped.
func (s *Service) Upload(p, name string, clash Clash, body io.Reader) (Entry, bool, error) {
	dir, err := Clean(p)
	if err != nil {
		return Entry{}, false, err
	}
	sub, err := Clean(name)
	if err != nil || sub == "" {
		return Entry{}, false, ErrBadName
	}
	for _, part := range strings.Split(sub, "/") {
		if !ValidName(part) {
			return Entry{}, false, ErrBadName
		}
	}
	rel := path.Join(dir, sub)
	r, err := s.open()
	if err != nil {
		return Entry{}, false, err
	}
	defer r.Close()
	if err := s.dirWritable(r, dir); err != nil {
		return Entry{}, false, err
	}
	if err := s.writable(r, rel); err != nil {
		return Entry{}, false, err
	}
	parent := path.Dir(rel)
	if parent != "." {
		if err := mkdirAll(r, parent); err != nil {
			return Entry{}, false, err
		}
	} else {
		parent = ""
	}
	if info, err := r.Lstat(rel); err == nil {
		switch {
		case clash == ClashSkip:
			_, _ = io.Copy(io.Discard, body)
			return Entry{}, true, nil
		case clash == ClashKeep:
			rel = path.Join(parent, freeName(r, parent, path.Base(rel)))
		case clash == ClashReplace && !info.IsDir():
		default:
			return Entry{}, false, ErrExists
		}
	}
	tmp := path.Join(parent, ".icedash-upload-"+randomID())
	f, err := r.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return Entry{}, false, notFound(err)
	}
	_, err = io.Copy(f, body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		_ = r.Chmod(tmp, 0o644)
		err = r.Rename(tmp, rel)
	}
	if err != nil {
		_ = r.Remove(tmp)
		return Entry{}, false, err
	}
	e, err := s.stat(r, rel)
	return e, false, err
}

// mkdirAll makes the folders of rel, 0755 like the stock ZimaOS Files.
func mkdirAll(r *os.Root, rel string) error {
	cur := ""
	for _, part := range strings.Split(rel, "/") {
		cur = path.Join(cur, part)
		err := r.Mkdir(cur, 0o755)
		switch {
		case err == nil:
			_ = r.Chmod(cur, 0o755)
		case errors.Is(err, fs.ErrExist):
			info, serr := r.Stat(cur)
			if serr != nil || !info.IsDir() {
				return ErrNotDir
			}
		default:
			return notFound(err)
		}
	}
	return nil
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
