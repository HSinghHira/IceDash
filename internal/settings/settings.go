// Package settings keeps IceDash's own settings in /DATA/.icedash/settings.json.
// Phase 1 has one: "Check for updates automatically", on by default.
package settings

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/HSinghHira/IceDash/internal/state"
)

// Settings are the values the UI shows and an admin changes.
type Settings struct {
	AutoCheck bool `json:"auto_check"`
}

// Defaults apply to anything not saved yet.
func Defaults() Settings { return Settings{AutoCheck: true} }

// Store is safe for concurrent use.
type Store struct {
	path string
	mu   sync.Mutex
	cur  Settings
}

// Open loads path, or uses the defaults when it's missing or damaged.
func Open(path string) *Store {
	s := &Store{path: path, cur: Defaults()}
	// A missing or unreadable file means the defaults; it's only written when an admin saves.
	if b, err := os.ReadFile(path); err == nil {
		cur := Defaults()
		if json.Unmarshal(b, &cur) == nil {
			s.cur = cur
		}
	}
	return s
}

// Get returns the current settings.
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

// Set saves new settings.
func (s *Store) Set(v Settings) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := state.WriteFile(s.path, b); err != nil {
		return err
	}
	s.cur = v
	return nil
}
