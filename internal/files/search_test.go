package files

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func paths(f Found) []string {
	out := []string{}
	for _, e := range f.Entries {
		out = append(out, e.Path)
	}
	slices.Sort(out)
	return out
}

func TestSearchFindsNamesWithEveryWord(t *testing.T) {
	s, root := newData(t)
	os.WriteFile(filepath.Join(root, "Documents", "Holiday Notes 2026.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, ".docker", "overlay2", "notes.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "lost+found", "notes"), []byte("x"), 0o644)
	ctx := context.Background()
	f, err := s.Search(ctx, "/", "NOTES", false, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(f); !slices.Equal(got, []string{"/Documents/Holiday Notes 2026.md", "/Documents/notes.txt"}) || f.More {
		t.Fatalf("found %v more=%v", got, f.More)
	}
	if f, _ := s.Search(ctx, "/", "notes 2026", false, DefaultLimits); len(f.Entries) != 1 {
		t.Fatalf("two words: %v", paths(f))
	}
	// Hidden ones only when asked; lost+found never.
	f, _ = s.Search(ctx, "/", "notes", true, DefaultLimits)
	if got := paths(f); !slices.Contains(got, "/.docker/overlay2/notes.txt") || slices.Contains(got, "/lost+found/notes") {
		t.Fatalf("hidden %v", got)
	}
	// A folder matches too, and the search stays under the folder given.
	if f, _ := s.Search(ctx, "/Gallery", "o", false, DefaultLimits); !slices.Equal(paths(f), []string{"/Gallery/photo.jpg"}) {
		t.Fatalf("under Gallery: %v", paths(f))
	}
	if _, err := s.Search(ctx, "/../etc", "passwd", false, DefaultLimits); err == nil {
		t.Fatal("climbed out")
	}
	if _, err := s.Search(ctx, "/nope", "x", false, DefaultLimits); err == nil {
		t.Fatal("missing folder")
	}
}

func TestSearchStopsAtItsLimits(t *testing.T) {
	s, root := newData(t)
	for i := range 30 {
		os.WriteFile(filepath.Join(root, "Documents", "file"+string(rune('a'+i%26))+string(rune('a'+i/26))+".txt"), []byte("x"), 0o644)
	}
	f, err := s.Search(context.Background(), "/", "file", false, SearchLimits{Results: 5, Visit: 1000, Time: time.Second})
	if err != nil || len(f.Entries) != 5 || !f.More {
		t.Fatalf("results: %d more=%v %v", len(f.Entries), f.More, err)
	}
	f, _ = s.Search(context.Background(), "/", "file", false, SearchLimits{Results: 100, Visit: 3, Time: time.Second})
	if !f.More || len(f.Entries) > 3 {
		t.Fatalf("visits: %d more=%v", len(f.Entries), f.More)
	}
}
