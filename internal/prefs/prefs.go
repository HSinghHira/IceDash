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
}

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
