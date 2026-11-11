package notify

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "notifications.json")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC) }
	return s, p
}

func add(t *testing.T, s *Store, n Notification) Notification {
	t.Helper()
	got, _, err := s.Add(n)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestNewestFirstWithUnreadCount(t *testing.T) {
	s, _ := open(t)
	a := add(t, s, Notification{Kind: KindUpdateAvailable, Title: "a"})
	b := add(t, s, Notification{Kind: KindUpdated, Title: "b"})
	list, unread := s.List("Hira", 0)
	if len(list) != 2 || list[0].ID != b.ID || list[1].ID != a.ID || unread != 2 {
		t.Fatalf("list %+v unread %d", list, unread)
	}
	// since: only newer ones, but the unread count covers all
	list, unread = s.List("Hira", a.ID)
	if len(list) != 1 || list[0].ID != b.ID || unread != 2 {
		t.Fatalf("since: %+v unread %d", list, unread)
	}
}

func TestReadStateIsPerUser(t *testing.T) {
	s, _ := open(t)
	a := add(t, s, Notification{Title: "a"})
	add(t, s, Notification{Title: "b"})
	if err := s.MarkRead("Hira", a.ID); err != nil {
		t.Fatal(err)
	}
	if _, unread := s.List("Hira", 0); unread != 1 {
		t.Fatalf("Hira unread %d, want 1", unread)
	}
	if _, unread := s.List("testmember", 0); unread != 2 {
		t.Fatalf("member unread %d, want 2", unread)
	}
	s.MarkRead("Hira", 0)
	list, unread := s.List("Hira", 0)
	if unread != 0 || !list[0].Read || !list[1].Read {
		t.Fatalf("after read all: %+v unread %d", list, unread)
	}
}

func TestClearHidesOnlyWhatExists(t *testing.T) {
	s, _ := open(t)
	add(t, s, Notification{Title: "a"})
	s.Clear("Hira")
	if list, _ := s.List("Hira", 0); len(list) != 0 {
		t.Fatalf("after clear: %+v", list)
	}
	if list, _ := s.List("testmember", 0); len(list) != 1 {
		t.Fatal("clear reached another user")
	}
	add(t, s, Notification{Title: "new"})
	if list, unread := s.List("Hira", 0); len(list) != 1 || unread != 1 {
		t.Fatalf("a new one after clear: %+v unread %d", list, unread)
	}
}

func TestKeyStoresOnce(t *testing.T) {
	s, _ := open(t)
	first, added, _ := s.Add(Notification{Kind: KindUpdateAvailable, Key: "update:v0.2.0"})
	if !added {
		t.Fatal("first not added")
	}
	again, added, _ := s.Add(Notification{Kind: KindUpdateAvailable, Key: "update:v0.2.0"})
	if added || again.ID != first.ID {
		t.Fatal("same key stored twice")
	}
	if _, added, _ := s.Add(Notification{Kind: KindUpdateAvailable, Key: "update:v0.3.0"}); !added {
		t.Fatal("another version not added")
	}
}

func TestKeepsTheLastHundred(t *testing.T) {
	s, _ := open(t)
	for i := 0; i < Keep+20; i++ {
		add(t, s, Notification{Title: "x"})
	}
	list, _ := s.List("Hira", 0)
	if len(list) != Keep || list[0].ID != Keep+20 || list[Keep-1].ID != 21 {
		t.Fatalf("%d kept, newest %d, oldest %d", len(list), list[0].ID, list[len(list)-1].ID)
	}
}

func TestSurvivesARestart(t *testing.T) {
	s, p := open(t)
	n := add(t, s, Notification{Kind: KindUpdated, Version: "v0.2.0", Title: "Updated"})
	s.MarkRead("Hira", n.ID)

	again, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	list, unread := again.List("Hira", 0)
	if len(list) != 1 || list[0].Version != "v0.2.0" || !list[0].Read || unread != 0 {
		t.Fatalf("after reopen: %+v unread %d", list, unread)
	}
	if m := add(t, again, Notification{Title: "next"}); m.ID != n.ID+1 {
		t.Fatalf("ids restarted: %d", m.ID)
	}
}

func TestDamagedFileIsSetAside(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notifications.json")
	os.WriteFile(p, []byte("{not json"), 0o600)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List("Hira", 0); len(list) != 0 {
		t.Fatal("damaged file gave entries")
	}
	if _, err := os.Stat(p + ".damaged"); err != nil {
		t.Fatal("damaged file not kept aside")
	}
}
