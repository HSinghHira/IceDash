package mods

import (
	"bytes"
	_ "embed"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FilesPrefix is where a widget's files are served: <prefix><id>/files/<path>.
const FilesPrefix = "/v2/icedash/mods/"

// SDKName is the bridge every widget page gets (PLAN.md 4.4), served from
// the widget's own folder so its policy allows it.
const SDKName = "icedash-widget.js"

//go:embed sdk/icedash-widget.js
var sdk []byte

// Policy is a widget page's Content-Security-Policy: its own files from its
// own folder, the internet hosts it declared, and nothing else: not IceDash's
// API, not the system's, not other mods.
func Policy(origin, id string, p *Permissions) string {
	own := origin + FilesPrefix + id + "/files/"
	var hosts []string
	if p != nil {
		for _, h := range p.Internet {
			hosts = append(hosts, "https://"+h)
		}
	}
	net := strings.Join(hosts, " ")
	if net == "" {
		net = "'none'"
	}
	return strings.Join([]string{
		"default-src 'none'",
		"script-src " + own + " 'unsafe-inline'",
		"style-src " + own + " 'unsafe-inline'",
		"img-src " + own + " data: " + strings.Join(hosts, " "),
		"font-src " + own + " data:",
		"media-src " + own + " " + strings.Join(hosts, " "),
		"connect-src " + net,
		"frame-src 'none'",
		"worker-src 'none'",
		"form-action 'none'",
		"base-uri 'none'",
	}, "; ")
}

func origin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// ServeFile serves one file of an installed, switched-on widget. It needs no
// sign-in: the widget's iframe can't carry one, and its files hold nothing
// private. The page itself gets its policy as a <meta> written first (the
// gateway may not pass headers on) and as a header with "sandbox".
func (s *Service) ServeFile(w http.ResponseWriter, r *http.Request, id, rel string) {
	dir, m, err := s.Widget(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rel = path.Clean("/" + rel)[1:]
	if rel == "" || strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.") {
		http.NotFound(w, r)
		return
	}
	csp := Policy(origin(r), id, m.Permissions)
	if rel == SDKName {
		h := w.Header()
		h.Set("Content-Security-Policy", csp+"; sandbox allow-scripts")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Type", "text/javascript; charset=utf-8")
		h.Set("Cache-Control", "no-cache")
		_, _ = w.Write(sdk)
		return
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	f, err := root.Open(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Security-Policy", csp+"; sandbox allow-scripts")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-cache")
	h.Set("Referrer-Policy", "no-referrer")
	ext := strings.ToLower(filepath.Ext(rel))
	ctype := mime.TypeByExtension(ext)
	switch ext {
	case ".html", ".htm":
		ctype = "text/html; charset=utf-8"
	case ".js", ".mjs":
		ctype = "text/javascript; charset=utf-8"
	case ".css":
		ctype = "text/css; charset=utf-8"
	case ".json":
		ctype = "application/json"
	case ".svg":
		ctype = "image/svg+xml"
	case ".woff2":
		ctype = "font/woff2"
	}
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	h.Set("Content-Type", ctype)
	if rel != m.Entry {
		http.ServeContent(w, r, rel, info.ModTime(), f)
		return
	}
	body, err := readAll(f, MaxFolder)
	if err != nil {
		http.Error(w, "the widget can't be read", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(withPolicy(body, csp))
}

func readAll(f *os.File, max int64) ([]byte, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(&limited{f: f, n: max})
	return b.Bytes(), err
}

type limited struct {
	f *os.File
	n int64
}

func (l *limited) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, os.ErrInvalid
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.f.Read(p)
	l.n -= int64(n)
	return n, err
}

// withPolicy writes the policy, then the bridge, as the page's very first
// elements, right after a doctype: the parser puts it in the head before any of the widget's
// own markup or scripts, wherever its <head> is.
func withPolicy(page []byte, csp string) []byte {
	meta := []byte(`<meta http-equiv="Content-Security-Policy" content="` + strings.ReplaceAll(csp, `"`, "&quot;") + `">` +
		`<script src="` + SDKName + `"></script>`)
	trimmed := bytes.TrimLeft(page, " \t\r\n\ufeff")
	lead := len(page) - len(trimmed)
	if bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<!doctype")) {
		if j := bytes.IndexByte(trimmed, '>'); j >= 0 {
			at := lead + j + 1
			return append(append(append([]byte{}, page[:at]...), meta...), page[at:]...)
		}
	}
	return append(append([]byte("<!doctype html>"), meta...), page[lead:]...)
}

// ServeWallpaper serves an installed theme's wallpaper. It needs no sign-in
// (a CSS background can't carry one; it's only a picture), and it is only
// served as the image type its first bytes show.
func (s *Service) ServeWallpaper(w http.ResponseWriter, r *http.Request, id string) {
	dir, m, ok := s.installed(id)
	if !ok || m.Kind != "theme" {
		http.NotFound(w, r)
		return
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	for _, name := range WallpaperNames {
		b, err := root.ReadFile(name)
		if err != nil {
			continue
		}
		t := ImageType(b)
		if t == "" || t != wallpaperType(name) {
			break
		}
		w.Header().Set("Content-Type", t)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=86400")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		_, _ = w.Write(b)
		return
	}
	http.NotFound(w, r)
}
