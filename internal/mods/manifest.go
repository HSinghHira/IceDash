// Package mods is Phase 4: themes and widgets (PLAN.md, "Phase 4: mods").
//
// A mod is a zip with a mod.json at its root: a theme (theme.json, maybe
// theme.css) or a widget (an HTML page and its files, run in a sandboxed
// iframe). The rules here are the catalog's own (IceDash-Mods,
// tools/build.py): a mod is checked again on install, wherever it came from.
package mods

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// Manifest is mod.json.
type Manifest struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Kind        string       `json:"kind"` // theme, widget
	Version     string       `json:"version"`
	Author      string       `json:"author"`
	Description string       `json:"description"`
	Entry       string       `json:"entry,omitempty"` // widgets
	Size        string       `json:"size,omitempty"`  // widgets: small, medium, large
	Permissions *Permissions `json:"permissions,omitempty"`
	Preview     *Theme       `json:"preview,omitempty"` // in the catalog: the theme's colours
}

// Permissions are what a widget asks to read.
type Permissions struct {
	Internet []string `json:"internet,omitempty"`
	System   bool     `json:"system,omitempty"`
	Apps     bool     `json:"apps,omitempty"`
	Location bool     `json:"location,omitempty"`
}

// Theme is theme.json, plus its CSS once read.
type Theme struct {
	Scheme string            `json:"scheme"` // dark, light
	Colors map[string]string `json:"colors"`
	CSS    string            `json:"css,omitempty"`
}

// ThemeColors are the colours every theme gives; IceDash works out the rest.
var ThemeColors = []string{"bg", "surface", "text", "accent", "accent2", "accentDark", "ink", "warn"}

const (
	maxCSS = 64 << 10
	// MaxZip caps a mod's download and what it unpacks to.
	MaxZip = 5 << 20
)

var (
	idRx      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)
	versionRx = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	hexRx     = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	hostRx    = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
)

// cssBanned: theme CSS may restyle IceDash but never load anything or read
// what is typed (a selector on input values can send it out letter by letter).
var cssBanned = []struct {
	rx   *regexp.Regexp
	what string
}{
	{regexp.MustCompile(`(?i)@import`), "@import"},
	{regexp.MustCompile(`(?i)(input|textarea)[^{]*\[\s*value`), "a selector on typed values"},
	{regexp.MustCompile(`(?i)expression\s*\(`), "expression()"},
	{regexp.MustCompile(`(?i)behavior\s*:`), "behavior:"},
	{regexp.MustCompile(`(?i)-moz-binding`), "-moz-binding"},
	{regexp.MustCompile(`(?i)</style`), "</style"},
}

// ErrInvalid wraps every reason a mod is refused.
var ErrInvalid = errors.New("this mod can't be installed")

func bad(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// CheckCSS refuses theme CSS that could load or leak anything.
func CheckCSS(css string) error {
	if len(css) > maxCSS {
		return bad("theme.css is over 64 KB")
	}
	for _, b := range cssBanned {
		if b.rx.MatchString(css) {
			return bad("theme.css uses %s, which themes may not", b.what)
		}
	}
	// Every url( must hold a data: URI. A CSS escape (\75rl) could hide one,
	// so backslashes are refused altogether.
	low := strings.ToLower(css)
	if strings.Contains(low, `\`) {
		return bad("theme.css uses escapes, which themes may not")
	}
	for i := strings.Index(low, "url("); i >= 0; {
		rest := strings.TrimLeft(low[i+4:], " \t\r\n\"'")
		if !strings.HasPrefix(rest, "data:") {
			return bad("theme.css uses url() other than data:, which themes may not")
		}
		next := strings.Index(low[i+4:], "url(")
		if next < 0 {
			break
		}
		i += 4 + next
	}
	if strings.Contains(low, "image-set(") || strings.Contains(low, "src(") {
		return bad("theme.css uses image-set() or src(), which themes may not")
	}
	return nil
}

// ValidID reports whether id can name a mod.
func ValidID(id string) bool { return idRx.MatchString(id) }

// Check validates a manifest, and for themes their theme.json.
func (m *Manifest) Check() error {
	if !idRx.MatchString(m.ID) {
		return bad("its id must be lowercase letters, digits and dashes")
	}
	for k, v := range map[string]string{"name": m.Name, "author": m.Author, "description": m.Description} {
		if strings.TrimSpace(v) == "" || len(v) > 200 {
			return bad("its %s is missing or too long", k)
		}
	}
	if !versionRx.MatchString(m.Version) {
		return bad("its version must be x.y.z")
	}
	switch m.Kind {
	case "theme":
		if m.Permissions != nil && (len(m.Permissions.Internet) > 0 || m.Permissions.System || m.Permissions.Apps || m.Permissions.Location) {
			return bad("a theme can't ask for permissions")
		}
	case "widget":
		switch m.Size {
		case "small", "medium", "large":
		default:
			return bad("a widget's size must be small, medium or large")
		}
		if !strings.HasSuffix(m.Entry, ".html") || strings.ContainsAny(m.Entry, `/\`) || strings.HasPrefix(m.Entry, ".") {
			return bad("a widget's entry must be an .html file at the top of the mod")
		}
		if m.Permissions == nil {
			m.Permissions = &Permissions{}
		}
		for _, h := range m.Permissions.Internet {
			if err := checkHost(h); err != nil {
				return err
			}
		}
	default:
		return bad("its kind must be theme or widget")
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

// CheckTheme validates theme.json's colours.
func (t *Theme) Check() error {
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
	return CheckCSS(t.CSS)
}

func decode(b []byte, v any) error {
	d := json.NewDecoder(strings.NewReader(string(b)))
	return d.Decode(v)
}
