// Package prefs keeps each user's own choices (PLAN.md 5a): the clock, the
// widget panel, motion, the dock and which notifications pop up. They live
// in /DATA/.icedash/prefs.json, by user name.
package prefs

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"slices"
	"sync"

	"github.com/HSinghHira/IceDash/internal/state"
)

// Prefs are one user's choices. The zero value is IceDash's default.
type Prefs struct {
	Clock     string `json:"clock"`      // "" (the browser's habit), "12", "24"
	HidePanel bool   `json:"hide_panel"` // the widget panel is hidden
	Motion    string `json:"motion"`     // "" (the system's), "reduce", "full"
	Dock      Dock   `json:"dock"`
	Notify    Notify `json:"notify"`
	// Wallpaper is the user's own desktop background (PLAN.md 5a.8).
	Wallpaper Wallpaper `json:"wallpaper"`
	// Avatar changes each time the user sets a picture; 0 is none.
	Avatar int64 `json:"avatar"`
	// Without a picture, a drawn avatar (DiceBear, drawn in the browser):
	// its style ("" is Critters, "initials" the letters) and seed ("" is the
	// user name; another seed gives another face).
	AvatarStyle string `json:"avatar_style"`
	AvatarSeed  string `json:"avatar_seed"`
}

// AvatarStyles are the drawn styles offered: DiceBear's public domain (CC0) ones.
var AvatarStyles = []string{"", "initials", "critters", "lorelei", "thumbs", "pixel-art", "pixelbot", "voxel-bot", "moods", "sprouts", "clay", "planets", "marbles", "glass", "shapes", "blobs"}

var seedRx = regexp.MustCompile(`^[A-Za-z0-9 ._-]{0,64}$`)

// Wallpaper is what fills the desktop behind the icons.
type Wallpaper struct {
	Kind   string `json:"kind"`   // "" (the theme's), "preset", "color", "image"
	Preset string `json:"preset"` // one of Presets
	Color  string `json:"color"`  // #rrggbb
	Fit    string `json:"fit"`    // "" (cover), "contain", "center", "tile"
	Dim    int    `json:"dim"`    // 0–80: darkens it, in percent
	Blur   int    `json:"blur"`   // 0–30: blurs it, in px
	Rev    int64  `json:"rev"`    // changes with each uploaded picture
}

// Presets are the built-in wallpapers; the UI draws them.
var Presets = []string{"aurora", "glacier", "nightfall", "ember", "lagoon", "dune", "graphite", "mist"}

var colorRx = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Dock is which launchers the dock leaves out and which apps it keeps.
type Dock struct {
	Hidden []string `json:"hidden"` // launcher ids
	Pinned []string `json:"pinned"` // app names, in order
}

// Notify is which notifications pop up; all of them still go to the list.
type Notify struct {
	Off   []string `json:"off"`   // groups with no pop-up
	Quiet bool     `json:"quiet"` // do not disturb: no pop-ups at all
}

// Launchers are the dock entries a user may hide; Settings and Trash stay.
var Launchers = []string{"files", "icestore", "appstore", "terminal"}

// Groups are the kinds of notification a user may silence.
var Groups = []string{"updates", "apps", "stopped"}

// ErrInvalid is any choice IceDash doesn't know.
var ErrInvalid = errors.New("that setting isn't one IceDash knows")

var appRx = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

const maxPinned = 24

// Check refuses unknown values and cleans lists (no repeats, no empties).
func (p *Prefs) Check() error {
	if !slices.Contains([]string{"", "12", "24"}, p.Clock) || !slices.Contains([]string{"", "reduce", "full"}, p.Motion) {
		return ErrInvalid
	}
	var err error
	if p.Dock.Hidden, err = only(p.Dock.Hidden, func(s string) bool { return slices.Contains(Launchers, s) }); err != nil {
		return err
	}
	if p.Dock.Pinned, err = only(p.Dock.Pinned, appRx.MatchString); err != nil {
		return err
	}
	if len(p.Dock.Pinned) > maxPinned {
		return ErrInvalid
	}
	if p.Notify.Off, err = only(p.Notify.Off, func(s string) bool { return slices.Contains(Groups, s) }); err != nil {
		return err
	}
	if !slices.Contains(AvatarStyles, p.AvatarStyle) || !seedRx.MatchString(p.AvatarSeed) {
		return ErrInvalid
	}
	return p.Wallpaper.check()
}

func (w *Wallpaper) check() error {
	if !slices.Contains([]string{"", "preset", "color", "image"}, w.Kind) ||
		!slices.Contains([]string{"", "contain", "center", "tile"}, w.Fit) ||
		w.Dim < 0 || w.Dim > 80 || w.Blur < 0 || w.Blur > 30 || w.Rev < 0 {
		return ErrInvalid
	}
	if w.Preset != "" && !slices.Contains(Presets, w.Preset) {
		return ErrInvalid
	}
	if w.Color != "" && !colorRx.MatchString(w.Color) {
		return ErrInvalid
	}
	if (w.Kind == "preset" && w.Preset == "") || (w.Kind == "color" && w.Color == "") {
		return ErrInvalid
	}
	return nil
}

func only(list []string, ok func(string) bool) ([]string, error) {
	out := []string{}
	for _, s := range list {
		if !ok(s) {
			return nil, ErrInvalid
		}
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (p Prefs) clean() Prefs {
	if p.Dock.Hidden == nil {
		p.Dock.Hidden = []string{}
	}
	if p.Dock.Pinned == nil {
		p.Dock.Pinned = []string{}
	}
	if p.Notify.Off == nil {
		p.Notify.Off = []string{}
	}
	return p
}

// Store is safe for concurrent use.
type Store struct {
	path  string
	mu    sync.Mutex
	users map[string]Prefs
}

// Open loads path; a missing or damaged file means everyone has the defaults.
func Open(path string) *Store {
	s := &Store{path: path, users: map[string]Prefs{}}
	if b, err := os.ReadFile(path); err == nil {
		var f struct {
			Users map[string]Prefs `json:"users"`
		}
		if json.Unmarshal(b, &f) == nil && f.Users != nil {
			s.users = f.Users
		}
	}
	return s
}

// Get returns a user's choices.
func (s *Store) Get(user string) Prefs {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.users[user].clean()
}

// Update changes part of a user's choices and saves them.
func (s *Store) Update(user string, change func(*Prefs)) (Prefs, error) {
	p := s.Get(user)
	change(&p)
	return s.Set(user, p)
}

// Set checks and saves a user's choices.
func (s *Store) Set(user string, p Prefs) (Prefs, error) {
	if user == "" {
		return Prefs{}, ErrInvalid
	}
	if err := p.Check(); err != nil {
		return Prefs{}, err
	}
	p = p.clean()
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make(map[string]Prefs, len(s.users)+1)
	for k, v := range s.users {
		next[k] = v
	}
	next[user] = p
	b, err := json.MarshalIndent(struct {
		Users map[string]Prefs `json:"users"`
	}{next}, "", " ")
	if err != nil {
		return Prefs{}, err
	}
	if err := state.WriteFile(s.path, b); err != nil {
		return Prefs{}, err
	}
	s.users = next
	return p, nil
}
