// Package media keeps each user's own pictures: a wallpaper and an avatar
// (PLAN.md 5a.8). They live in /DATA/.icedash/media, one file each, named by
// the hex of the user name so no name can reach outside the folder. Only
// JPEG, PNG and WebP are kept, checked by their first bytes, not their name.
package media

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/HSinghHira/IceDash/internal/mods"
	"github.com/HSinghHira/IceDash/internal/state"
)

// Kind is which picture.
type Kind string

const (
	Wallpaper Kind = "wallpaper"
	Avatar    Kind = "avatar"
)

// Limits: the UI shrinks larger pictures before sending them.
const (
	MaxWallpaper = 15 << 20
	MaxAvatar    = 1 << 20
)

var (
	ErrNotImage = errors.New("that isn't a JPEG, PNG or WebP picture")
	ErrTooBig   = errors.New("that picture is too large")
	ErrNone     = errors.New("there's no picture")
	ErrKind     = errors.New("there's no such picture")
	ErrUser     = errors.New("no user")
)

// Store is safe for concurrent use: each write replaces a file whole.
type Store struct {
	Dir string // /DATA/.icedash/media
}

// Max is the size limit of a kind.
func Max(k Kind) int64 {
	if k == Avatar {
		return MaxAvatar
	}
	return MaxWallpaper
}

func (s *Store) path(user string, k Kind) (string, error) {
	if user == "" || len(user) > 128 {
		return "", ErrUser
	}
	if k != Wallpaper && k != Avatar {
		return "", ErrKind
	}
	return filepath.Join(s.Dir, hex.EncodeToString([]byte(user))+"."+string(k)), nil
}

// Put keeps a user's picture, replacing the one before.
func (s *Store) Put(user string, k Kind, b []byte) error {
	p, err := s.path(user, k)
	if err != nil {
		return err
	}
	if int64(len(b)) > Max(k) {
		return ErrTooBig
	}
	if mods.ImageType(b) == "" {
		return ErrNotImage
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	return state.WriteFile(p, b)
}

// Get returns a user's picture, its type and when it was set.
func (s *Store) Get(user string, k Kind) ([]byte, string, time.Time, error) {
	p, err := s.path(user, k)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", time.Time{}, ErrNone
	}
	if err != nil {
		return nil, "", time.Time{}, err
	}
	t := mods.ImageType(b)
	if t == "" {
		return nil, "", time.Time{}, ErrNone
	}
	var mod time.Time
	if fi, err := os.Stat(p); err == nil {
		mod = fi.ModTime()
	}
	return b, t, mod, nil
}

// Has reports whether a user has a picture of kind k.
func (s *Store) Has(user string, k Kind) bool {
	p, err := s.path(user, k)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Delete forgets a user's picture; none is fine.
func (s *Store) Delete(user string, k Kind) error {
	p, err := s.path(user, k)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
