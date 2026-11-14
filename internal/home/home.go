// Package home opens IceDash at ZimaOS's main address when an admin asks
// (PLAN.md 5b.1). ZimaOS's gateway is Caddy: it serves the stock dashboard's
// files from /usr/share/casaos/www for every path no service claimed
// (HOST-NOTES item 3). Pointing the route "/" at IceDash makes every such
// path come here instead, so IceDash serves those same files itself: the
// stock dashboard moves to /zimaos/, its files keep their addresses, and the
// APIs (longer routes) still go straight to ZimaOS.
//
// The gateway can't remove a route (HOST-NOTES, uninstall), so turning it off
// leaves "/" pointing here until the next restart of the box; IceDash then
// serves the stock dashboard at "/" exactly as Caddy did.
package home

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// WWW is where ZimaOS keeps its dashboard's files.
const WWW = "/usr/share/casaos/www"

// StockPath is where the stock dashboard stays while IceDash has "/".
const StockPath = "/zimaos/"

// Marker is the header every answer from here carries, so a check through
// the gateway can tell IceDash answered.
const Marker = "X-IceDash-Front"

// Front serves "/" and the stock dashboard's files. Next gets IceDash's own
// paths; On says whether "/" opens IceDash.
type Front struct {
	WWW  string
	Next http.Handler
	On   func() bool
}

func (f *Front) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if strings.HasPrefix(p, "/modules/icedash") || strings.HasPrefix(p, "/v2/icedash") {
		f.Next.ServeHTTP(w, r)
		return
	}
	w.Header().Set(Marker, "zimaos")
	switch {
	case p == "/" && f.On != nil && f.On():
		http.Redirect(w, r, "/modules/icedash/", http.StatusFound)
		return
	case p == strings.TrimSuffix(StockPath, "/"):
		http.Redirect(w, r, StockPath, http.StatusFound)
		return
	case strings.HasPrefix(p, StockPath):
		p = "/" + strings.TrimPrefix(p, StockPath)
	}
	f.file(w, r, p)
}

// file serves one of the stock files: a folder only by its index.html, never
// a listing; nothing outside the folder (os.Root).
func (f *Front) file(w http.ResponseWriter, r *http.Request, p string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	root, err := os.OpenRoot(f.WWW)
	if err != nil {
		http.Error(w, "the ZimaOS dashboard isn't available right now", http.StatusBadGateway)
		return
	}
	defer root.Close()
	rel := strings.TrimPrefix(path.Clean("/"+p), "/")
	if rel == "" {
		rel = "."
	}
	st, err := root.Stat(rel)
	if err == nil && st.IsDir() {
		rel = path.Join(rel, "index.html")
		st, err = root.Stat(rel)
	}
	if err != nil || st.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	fh, err := root.Open(rel)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer fh.Close()
	if strings.HasSuffix(rel, ".html") {
		// The page itself is never cached, as Caddy's file server does with ETags.
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, path.Base(rel), st.ModTime(), fh)
}

// Ready reports whether the stock dashboard's files are there to serve.
func Ready(www string) error {
	st, err := os.Stat(path.Join(www, "index.html"))
	if err != nil {
		return fmt.Errorf("the ZimaOS dashboard's files aren't in %s: %w", www, err)
	}
	if st.IsDir() {
		return fs.ErrInvalid
	}
	return nil
}

// ErrNotTaken is when the gateway doesn't send "/" here.
var ErrNotTaken = errors.New("ZimaOS's gateway didn't send its main address to IceDash")

// Check asks the gateway for "/" (base is http://127.0.0.1:<port>) and
// reports whether IceDash answered.
func Check(ctx context.Context, base string, c *http.Client) error {
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Second}
	}
	cc := *c
	cc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err != nil {
		return err
	}
	res, err := cc.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.Header.Get(Marker) == "" {
		return ErrNotTaken
	}
	return nil
}
