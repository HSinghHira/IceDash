package files

import (
	"archive/zip"
	"compress/flate"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

// A ticket lets the browser fetch a file with a plain URL (a link, an <img>,
// a <video>), which can't carry the sign-in header. It names what it is for,
// lives 10 minutes and is kept in memory only.
type ticket struct {
	paths   []string
	zip     bool
	expires time.Time
}

const ticketLife = 10 * time.Minute

// Ticket checks the paths and returns a ticket for them: one file to read,
// or (zip) files and folders to download as one archive.
func (s *Service) Ticket(paths []string, asZip bool) (string, error) {
	if len(paths) == 0 || (!asZip && len(paths) != 1) {
		return "", ErrBadPath
	}
	r, err := s.open()
	if err != nil {
		return "", err
	}
	defer r.Close()
	var rels []string
	for _, p := range paths {
		rel, err := Clean(p)
		if err != nil {
			return "", err
		}
		info, err := r.Stat(dotIfEmpty(rel))
		if err != nil {
			return "", notFound(err)
		}
		if !asZip && info.IsDir() {
			return "", ErrIsDir
		}
		rels = append(rels, rel)
	}
	id := randomID() + randomID()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tickets == nil {
		s.tickets = map[string]ticket{}
	}
	for k, t := range s.tickets {
		if time.Now().After(t.expires) {
			delete(s.tickets, k)
		}
	}
	s.tickets[id] = ticket{paths: rels, zip: asZip, expires: time.Now().Add(ticketLife)}
	return id, nil
}

// Redeem returns a ticket's paths (as API paths) if it is valid.
func (s *Service) Redeem(id string, asZip bool) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[id]
	if !ok || time.Now().After(t.expires) || t.zip != asZip {
		return nil, ErrTicket
	}
	out := make([]string, len(t.paths))
	for i, p := range t.paths {
		out[i] = apiPath(p)
	}
	return out, nil
}

// ZipName is the download's name: the item's own for one, else "files".
func ZipName(paths []string) string {
	if len(paths) == 1 {
		if n := path.Base(paths[0]); n != "/" && n != "." {
			return n + ".zip"
		}
	}
	return "files.zip"
}

// WriteZip streams the paths as a .zip. Links and special files are left
// out; a read error ends the archive early (the headers are out by then).
func (s *Service) WriteZip(w io.Writer, paths []string) error {
	r, err := s.open()
	if err != nil {
		return err
	}
	defer r.Close()
	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestSpeed)
	})
	for _, p := range paths {
		rel, err := Clean(p)
		if err != nil {
			return err
		}
		base := path.Dir(rel)
		if rel == "" {
			base = ""
		}
		err = fs.WalkDir(r.FS(), dotIfEmpty(rel), func(p string, d fs.DirEntry, werr error) error {
			if werr != nil {
				return nil // unreadable: left out
			}
			if rel == "" && p != "." && Protected(p) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			name := strings.TrimPrefix(strings.TrimPrefix(p, base), "/")
			if p == "." {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if d.IsDir() {
				h := &zip.FileHeader{Name: name + "/", Modified: info.ModTime()}
				h.SetMode(info.Mode())
				_, err := zw.CreateHeader(h)
				return err
			}
			if !d.Type().IsRegular() {
				return nil
			}
			h, err := zip.FileInfoHeader(info)
			if err != nil {
				return nil
			}
			h.Name, h.Method = name, zip.Deflate
			if k := KindOf(name, false); k == "image" || k == "video" || k == "audio" || strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".gz") {
				h.Method = zip.Store // already compressed
			}
			out, err := zw.CreateHeader(h)
			if err != nil {
				return err
			}
			f, err := r.Open(p)
			if err != nil {
				return nil
			}
			defer f.Close()
			_, err = io.Copy(out, f)
			return err
		})
		if err != nil {
			return err
		}
	}
	return zw.Close()
}
