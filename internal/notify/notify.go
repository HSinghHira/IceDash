// Package notify keeps IceDash's notifications in /DATA/.icedash/notifications.json:
// the last 100, with read and cleared state per ZimaOS user, so they survive
// reloads and show on every device (PLAN.md, "Notifications").
//
// Phase 1 sources are IceDash itself (update available, updated, update failed,
// rollback failed). Phase 2 adds app events through the same Add.
package notify

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/HSinghHira/IceDash/internal/state"
)

// Keep is how many notifications the store holds.
const Keep = 100

// Kinds IceDash raises in Phase 1. The UI picks its words by kind (web/src/strings.ts);
// Title and Body are the English fallback for any other client.
const (
	KindUpdateAvailable = "update_available"
	KindUpdated         = "updated"
	KindUpdateFailed    = "update_failed"
	KindRollbackFailed  = "rollback_failed"
	KindNightInstall    = "night_install"

	// Phase 2: app events. App names the app (its compose name).
	KindAppInstalled       = "app_installed"
	KindAppInstallFailed   = "app_install_failed"
	KindAppUpdated         = "app_updated"
	KindAppUpdateFailed    = "app_update_failed"
	KindAppUpdateAvailable = "app_update_available"
	KindAppStopped         = "app_stopped"
)

// Action is the optional button on a notification.
type Action struct {
	Label string `json:"label"`
	// Kind tells the UI what the button does: "update" starts Update now;
	// "open_app" opens the notification's app; "app_update" updates it.
	Kind string `json:"kind"`
}

// Notification is one entry.
type Notification struct {
	ID      int64     `json:"id"`
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"`
	Version string    `json:"version,omitempty"`
	App     string    `json:"app,omitempty"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Action  *Action   `json:"action,omitempty"`
	// Key, when set, makes Add skip a notification whose twin is already
	// stored (one "update available" per version, however often it's checked).
	Key string `json:"key,omitempty"`
}

// View is a notification as one user sees it.
type View struct {
	Notification
	Read bool `json:"read"`
}

type userState struct {
	ReadUpTo    int64   `json:"read_up_to"`    // everything up to here is read
	Read        []int64 `json:"read"`          // and these single ones above it
	ClearedUpTo int64   `json:"cleared_up_to"` // nothing up to here is shown
}

type file struct {
	NextID int64                 `json:"next_id"`
	Items  []Notification        `json:"items"` // oldest first
	Users  map[string]*userState `json:"users"`
}

// Store is safe for concurrent use.
type Store struct {
	path string
	now  func() time.Time
	mu   sync.Mutex
	f    file
}

// Open loads the store at path, or starts an empty one if the file is missing.
func Open(path string) (*Store, error) {
	s := &Store{path: path, now: time.Now, f: file{NextID: 1, Users: map[string]*userState{}}}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return s, nil
	case err != nil:
		return nil, err
	}
	if err := json.Unmarshal(b, &s.f); err != nil {
		// A damaged file mustn't keep IceDash from starting; keep it aside for a look.
		_ = os.Rename(path, path+".damaged")
		s.f = file{NextID: 1, Users: map[string]*userState{}}
		return s, nil
	}
	if s.f.Users == nil {
		s.f.Users = map[string]*userState{}
	}
	if s.f.NextID < 1 {
		s.f.NextID = 1
	}
	return s, nil
}

// Add stores n (ID and Time are set here) and returns it. With a Key that is
// already stored, it stores nothing and returns the existing one, false.
func (s *Store) Add(n Notification) (Notification, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.Key != "" {
		for _, old := range s.f.Items {
			if old.Key == n.Key {
				return old, false, nil
			}
		}
	}
	n.ID = s.f.NextID
	n.Time = s.now().UTC()
	s.f.NextID++
	s.f.Items = append(s.f.Items, n)
	if len(s.f.Items) > Keep {
		s.f.Items = s.f.Items[len(s.f.Items)-Keep:]
	}
	return n, true, s.save()
}

// List returns user's notifications newer than since, newest first, and how
// many of all their visible ones are unread.
func (s *Store) List(user string, since int64) ([]View, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.user(user)
	out := []View{}
	unread := 0
	for i := len(s.f.Items) - 1; i >= 0; i-- {
		n := s.f.Items[i]
		if n.ID <= u.ClearedUpTo {
			break
		}
		read := n.ID <= u.ReadUpTo || slices.Contains(u.Read, n.ID)
		if !read {
			unread++
		}
		if n.ID > since {
			out = append(out, View{Notification: n, Read: read})
		}
	}
	return out, unread
}

// MarkRead marks one notification (id > 0) or all of them (id == 0) read for user.
func (s *Store) MarkRead(user string, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.ensureUser(user)
	if id <= 0 {
		u.ReadUpTo, u.Read = s.f.NextID-1, nil
	} else if id > u.ReadUpTo && !slices.Contains(u.Read, id) {
		u.Read = append(u.Read, id)
	}
	s.compact(u)
	return s.save()
}

// Clear hides everything there is now from user's list.
func (s *Store) Clear(user string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.ensureUser(user)
	u.ClearedUpTo = s.f.NextID - 1
	u.ReadUpTo, u.Read = u.ClearedUpTo, nil
	return s.save()
}

// compact drops single read marks that ReadUpTo covers or that point at
// notifications no longer stored, so the per-user list stays small.
func (s *Store) compact(u *userState) {
	oldest := int64(0)
	if len(s.f.Items) > 0 {
		oldest = s.f.Items[0].ID
	}
	u.Read = slices.DeleteFunc(u.Read, func(id int64) bool { return id <= u.ReadUpTo || id < oldest })
}

func (s *Store) user(name string) userState {
	if u, ok := s.f.Users[name]; ok {
		return *u
	}
	return userState{}
}

func (s *Store) ensureUser(name string) *userState {
	u, ok := s.f.Users[name]
	if !ok {
		u = &userState{}
		s.f.Users[name] = u
	}
	return u
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.f, "", " ")
	if err != nil {
		return err
	}
	return state.WriteFile(s.path, b)
}
