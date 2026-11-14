// Package mods is Phase 4: themes and widgets (PLAN.md, "Phase 4: mods").
//
// Mods come from collections: GitHub repos whose top-level folders are
// themes (theme.json and maybe a wallpaper) or widgets (widget.json and an
// HTML page). There is nothing to build or publish: IceDash reads a repo at
// its latest commit and checks every file against its git hash. The
// official collections are HSinghHira/IceColors and HSinghHira/IceCards.
package mods

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// ErrInvalid wraps every reason a mod is refused.
var ErrInvalid = errors.New("this mod can't be installed")

func bad(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Permissions are what a widget asks to read.
type Permissions struct {
	Internet []string `json:"internet,omitempty"`
	System   bool     `json:"system,omitempty"`
	Apps     bool     `json:"apps,omitempty"`
	Location bool     `json:"location,omitempty"`
}

// ThemeFile is theme.json.
type ThemeFile struct {
	Name        string            `json:"name"`
	Author      string            `json:"author"`
	Description string            `json:"description"`
	Scheme      string            `json:"scheme"` // dark, light
	Colors      map[string]string `json:"colors"`
}

// WidgetFile is widget.json.
type WidgetFile struct {
	Name        string      `json:"name"`
	Author      string      `json:"author"`
	Description string      `json:"description"`
	Entry       string      `json:"entry"`
	Size        string      `json:"size"` // small, medium, large
	Permissions Permissions `json:"permissions"`
}

// ThemeColors are the colours every theme gives; IceDash works out the rest.
var ThemeColors = []string{"bg", "surface", "text", "accent", "accent2", "accentDark", "ink", "warn"}

const (
	// MaxFolder caps what one mod may bring, files and wallpaper together.
	MaxFolder = 10 << 20
	// MaxWallpaper caps a wallpaper.
	MaxWallpaper = 5 << 20
	maxFiles     = 200
)

var (
	folderRx = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	hexRx    = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	hostRx   = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
)

// WallpaperNames are the files a theme's wallpaper may be.
var WallpaperNames = []string{"wallpaper.webp", "wallpaper.jpg", "wallpaper.png"}

func checkWords(name, author, desc string) error {
	for k, v := range map[string]string{"name": name, "author": author, "description": desc} {
		if strings.TrimSpace(v) == "" || len(v) > 200 {
			return bad("its %s is missing or too long", k)
		}
	}
	return nil
}

// Check validates a theme.
func (t *ThemeFile) Check() error {
	if err := checkWords(t.Name, t.Author, t.Description); err != nil {
		return err
	}
	if t.Scheme != "dark" && t.Scheme != "light" {
		return bad("theme.json's scheme must be dark or light")
	}
	for _, k := range ThemeColors {
		if !hexRx.MatchString(t.Colors[k]) {
			return bad("theme.json's colors.%s must be #rrggbb", k)
		}
	}
	for k := range t.Colors {
		if !strings.Contains(" "+strings.Join(ThemeColors, " ")+" ", " "+k+" ") {
			return bad("theme.json has an unknown colour %q", k)
		}
	}
	return nil
}

// Check validates a widget.
func (w *WidgetFile) Check() error {
	if err := checkWords(w.Name, w.Author, w.Description); err != nil {
		return err
	}
	if w.Entry == "" {
		w.Entry = "index.html"
	}
	switch w.Size {
	case "small", "medium", "large":
	default:
		return bad("widget.json's size must be small, medium or large")
	}
	if !strings.HasSuffix(w.Entry, ".html") || strings.ContainsAny(w.Entry, `/\`) || strings.HasPrefix(w.Entry, ".") {
		return bad("widget.json's entry must be an .html file in the widget's folder")
	}
	for _, h := range w.Permissions.Internet {
		if err := checkHost(h); err != nil {
			return err
		}
	}
	return nil
}

// checkHost: a widget's internet hosts are public names, never an address,
// localhost or a local name, so it can't reach the box or the LAN.
func checkHost(h string) error {
	switch {
	case net.ParseIP(strings.Trim(h, "[]")) != nil:
		return bad("internet host %q is an address; widgets name public hosts", h)
	case !hostRx.MatchString(h):
		return bad("internet host %q isn't a public host name", h)
	}
	for _, local := range []string{".local", ".lan", ".home", ".internal", ".localhost", ".arpa"} {
		if strings.HasSuffix(h, local) {
			return bad("internet host %q is a local name", h)
		}
	}
	return nil
}

// ImageType tells a wallpaper's real type from its first bytes, or "".
func ImageType(b []byte) string {
	switch {
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return "image/webp"
	case len(b) >= 8 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(b) >= 3 && bytes.Equal(b[:3], []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	}
	return ""
}

// wallpaperType is the type a wallpaper's name promises.
func wallpaperType(name string) string {
	switch name {
	case "wallpaper.webp":
		return "image/webp"
	case "wallpaper.png":
		return "image/png"
	case "wallpaper.jpg":
		return "image/jpeg"
	}
	return ""
}
