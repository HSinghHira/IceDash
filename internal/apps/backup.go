package apps

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoData: the app has no folder under /DATA/AppData.
var ErrNoData = errors.New("this app has no data folder")

// DataDir is the app's folder under /DATA/AppData.
func (m *Manager) DataDir(name string) string {
	base := m.AppData
	if base == "" {
		base = "/DATA/AppData"
	}
	return filepath.Join(base, name)
}

// Backup writes a .tar.gz of the app's data folder to w (PLAN.md, Phase 2,
// "Back up data"). Links are stored as links, never followed; sockets and
// devices are skipped. The app keeps running: a database mid-write may need
// the app's own export instead, which the UI says.
func (m *Manager) Backup(ctx context.Context, name string, w io.Writer) error {
	if _, err := m.installed(ctx, name); err != nil {
		return err
	}
	return TarGz(ctx, m.DataDir(name), name, w)
}

// HasData reports whether the app has a data folder to back up.
func (m *Manager) HasData(name string) bool {
	fi, err := os.Lstat(m.DataDir(name))
	return err == nil && fi.IsDir()
}

// TarGz archives dir with its entries under prefix/.
func TarGz(ctx context.Context, dir, prefix string, w io.Writer) error {
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() {
		return ErrNoData
	}
	gz, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
	tw := tar.NewWriter(gz)
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable: skip, keep going
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, _ := filepath.Rel(dir, p)
		name := prefix
		if rel != "." {
			name = prefix + "/" + filepath.ToSlash(rel)
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		link := ""
		switch mode := info.Mode(); {
		case mode&fs.ModeSymlink != 0:
			link, _ = os.Readlink(p)
		case mode.IsDir(), mode.IsRegular():
		default:
			return nil // sockets, pipes, devices
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return nil
		}
		hdr.Name = name
		if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(p)
			if err != nil {
				return nil
			}
			n, err := io.CopyN(tw, f, hdr.Size)
			f.Close()
			if errors.Is(err, io.EOF) {
				// It shrank while being read: pad to the size in the header.
				_, err = io.CopyN(tw, zeros{}, hdr.Size-n)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
