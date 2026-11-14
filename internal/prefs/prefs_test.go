package prefs

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

func TestDefaultsAndPerUser(t *testing.T) {
	p := filepath.Join(t.TempDir(), "prefs.json")
	s := Open(p)
	if d := s.Get("hira"); d.Clock != "" || d.HidePanel || d.Dock.Pinned == nil || d.Notify.Off == nil {
		t.Fatalf("defaults %+v", d)
	}
	got, err := s.Set("hira", Prefs{Clock: "24", HidePanel: true, Dock: Dock{Hidden: []string{"terminal", "terminal"}, Pinned: []string{"gitsave", "documenso"}}, Notify: Notify{Off: []string{"apps"}}})
	if err != nil || len(got.Dock.Hidden) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	if o := Open(p).Get("testmember"); o.Clock != "" || o.HidePanel {
		t.Fatalf("another user changed: %+v", o)
	}
	h := Open(p).Get("hira")
	if h.Clock != "24" || !h.HidePanel || !slices.Equal(h.Dock.Pinned, []string{"gitsave", "documenso"}) || h.Notify.Off[0] != "apps" {
		t.Fatalf("not kept: %+v", h)
	}
}

func TestUnknownValuesRefused(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "prefs.json"))
	for _, p := range []Prefs{
		{Clock: "13"},
		{Motion: "wild"},
		{Dock: Dock{Hidden: []string{"settings"}}},
		{Dock: Dock{Pinned: []string{"../etc"}}},
		{Notify: Notify{Off: []string{"everything"}}},
	} {
		if _, err := s.Set("hira", p); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v", p, err)
		}
	}
	if _, err := s.Set("", Prefs{}); err == nil {
		t.Error("no user accepted")
	}
}
