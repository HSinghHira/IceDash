package files

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// newData makes a /DATA look-alike (shapes measured on ZimaOS 1.8.0).
func newData(t *testing.T) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"Documents", "Gallery", "AppData/gitsave", ".docker/overlay2", ".icedash", "lost+found"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, body string) {
		if err := os.WriteFile(filepath.Join(root, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Documents/notes.txt", "hello")
	write("Documents/.hidden", "x")
	write(".bash_history", "secret")
	write("Gallery/photo.jpg", "jpg")
	return &Service{Root: root}, root
}

func names(l Listing) []string {
	var n []string
	for _, e := range l.Entries {
		n = append(n, e.Name)
	}
	return n
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{"/": "", "": "", "/Documents": "Documents", "Documents/a/../b": "Documents/b", "/a//b/": "a/b"} {
		if got, err := Clean(in); err != nil || got != want {
			t.Errorf("Clean(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{`a\b`, "a\x00b"} {
		if _, err := Clean(bad); err == nil {
			t.Errorf("Clean(%q) accepted", bad)
		}
	}
	// "/../etc" cleans to "/etc": still inside the root, never above it.
	if got, _ := Clean("/../etc"); got != "etc" {
		t.Errorf("Clean(/../etc) = %q", got)
	}
}

func TestListHidesDotsAndMarksTheSystem(t *testing.T) {
	s, _ := newData(t)
	l, err := s.List("/", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(l); !slices.Equal(got, []string{"AppData", "Documents", "Gallery", "lost+found"}) {
		t.Fatalf("top: %v", got)
	}
	if l.Hidden != 3 { // .bash_history, .docker, .icedash
		t.Fatalf("hidden count %d", l.Hidden)
	}
	all, _ := s.List("/", true)
	for _, e := range all.Entries {
		want := strings.HasPrefix(e.Name, ".") || e.Name == "lost+found"
		if e.Protected != want {
			t.Errorf("%s protected=%v", e.Name, e.Protected)
		}
	}
	docs, _ := s.List("/Documents", false)
	if got := names(docs); !slices.Equal(got, []string{"notes.txt"}) || docs.Entries[0].Kind != "text" || docs.Entries[0].Size != 5 {
		t.Fatalf("documents: %+v", docs.Entries)
	}
	if _, err := s.List("/Documents/notes.txt", false); !errors.Is(err, ErrNotDir) {
		t.Fatalf("listing a file: %v", err)
	}
	if _, err := s.List("/nope", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestSymlinkOutOfTheRootIsRefused(t *testing.T) {
	s, root := newData(t)
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "passwd"), []byte("root:x:0:0"), 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "Documents", "etc")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	l, _ := s.List("/Documents", false)
	for _, e := range l.Entries {
		if e.Name == "etc" && (!e.Link || !e.Broken) {
			t.Fatalf("link out of the root shown as usable: %+v", e)
		}
	}
	if _, err := s.List("/Documents/etc", false); err == nil {
		t.Fatal("listed a folder outside the root")
	}
	if _, _, err := s.Open("/Documents/etc/passwd"); err == nil {
		t.Fatal("read a file outside the root")
	}
	if _, err := s.Ticket([]string{"/Documents/etc/passwd"}, false); err == nil {
		t.Fatal("ticket for a file outside the root")
	}
}

func TestSystemFoldersCantChange(t *testing.T) {
	s, root := newData(t)
	if _, err := s.Rename("/.docker", "docker"); !errors.Is(err, ErrProtected) {
		t.Fatalf("rename .docker: %v", err)
	}
	if err := s.Delete([]string{"/.icedash"}); !errors.Is(err, ErrProtected) {
		t.Fatalf("delete .icedash: %v", err)
	}
	if err := s.Delete([]string{"/"}); !errors.Is(err, ErrProtected) {
		t.Fatalf("delete the root: %v", err)
	}
	if _, err := s.Mkdir("/.docker", "x"); !errors.Is(err, ErrProtected) {
		t.Fatalf("mkdir in .docker: %v", err)
	}
	if _, _, err := s.Upload("/lost+found", "a.txt", ClashAsk, strings.NewReader("x")); !errors.Is(err, ErrProtected) {
		t.Fatalf("upload into lost+found: %v", err)
	}
	if _, err := s.Transfer("move", []string{"/Documents/notes.txt"}, "/.icedash", ClashAsk); !errors.Is(err, ErrProtected) {
		t.Fatalf("move into .icedash: %v", err)
	}
	if _, err := s.Rename("/Documents", ".docker"); !errors.Is(err, ErrProtected) {
		t.Fatalf("rename onto a system name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".docker")); err != nil {
		t.Fatal(".docker is gone")
	}
}

func TestMkdirRenameUpload(t *testing.T) {
	s, root := newData(t)
	e, err := s.Mkdir("/Documents", "Taxes")
	if err != nil || !e.Dir || e.Path != "/Documents/Taxes" {
		t.Fatalf("%+v %v", e, err)
	}
	if _, err := s.Mkdir("/Documents", "Taxes"); !errors.Is(err, ErrExists) {
		t.Fatalf("again: %v", err)
	}
	for _, bad := range []string{"", "..", "a/b"} {
		if _, err := s.Mkdir("/Documents", bad); !errors.Is(err, ErrBadName) {
			t.Fatalf("name %q: %v", bad, err)
		}
	}
	e, err = s.Rename("/Documents/notes.txt", "todo.txt")
	if err != nil || e.Path != "/Documents/todo.txt" {
		t.Fatalf("rename: %+v %v", e, err)
	}

	e, skipped, err := s.Upload("/Documents", "photos/2024/a.jpg", ClashAsk, strings.NewReader("jpeg bytes"))
	if err != nil || skipped || e.Path != "/Documents/photos/2024/a.jpg" || e.Size != 10 {
		t.Fatalf("upload with folders: %+v %v", e, err)
	}
	if _, _, err := s.Upload("/Documents", "todo.txt", ClashAsk, strings.NewReader("new")); !errors.Is(err, ErrExists) {
		t.Fatalf("clash: %v", err)
	}
	if e, _, err := s.Upload("/Documents", "todo.txt", ClashKeep, strings.NewReader("new")); err != nil || e.Name != "todo (2).txt" {
		t.Fatalf("keep both: %+v %v", e, err)
	}
	if _, skipped, err := s.Upload("/Documents", "todo.txt", ClashSkip, strings.NewReader("new")); err != nil || !skipped {
		t.Fatalf("skip: %v %v", skipped, err)
	}
	if _, _, err := s.Upload("/Documents", "todo.txt", ClashReplace, strings.NewReader("replaced")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "Documents", "todo.txt")); string(b) != "replaced" {
		t.Fatalf("after replace: %q", b)
	}
	if _, _, err := s.Upload("/Documents", "../escape.txt", ClashAsk, strings.NewReader("x")); err == nil {
		if _, serr := os.Stat(filepath.Join(root, "escape.txt")); serr == nil {
			t.Fatal("upload climbed out of its folder")
		}
	}
	// A failed upload leaves no temporary file.
	if _, _, err := s.Upload("/Documents", "broken.bin", ClashAsk, failing{}); err == nil {
		t.Fatal("a failing body should fail")
	}
	ents, _ := os.ReadDir(filepath.Join(root, "Documents"))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".icedash-upload-") || e.Name() == "broken.bin" {
			t.Fatalf("left behind: %s", e.Name())
		}
	}
}

type failing struct{}

func (failing) Read([]byte) (int, error) { return 0, errors.New("connection lost") }

func waitJob(t *testing.T, s *Service, id string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range s.Jobs() {
			if j.ID == id && j.State != "running" {
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job never ended")
	return Job{}
}

func TestMoveAndCopy(t *testing.T) {
	s, root := newData(t)
	os.MkdirAll(filepath.Join(root, "Documents", "Trip", "day1"), 0o755)
	os.WriteFile(filepath.Join(root, "Documents", "Trip", "day1", "a.txt"), bytes.Repeat([]byte("x"), 3000), 0o640)

	j, err := s.Transfer("copy", []string{"/Documents/Trip"}, "/Gallery", ClashAsk)
	if err != nil || j.Total != 3000 {
		t.Fatalf("%+v %v", j, err)
	}
	if j = waitJob(t, s, j.ID); j.State != "done" || j.Done != 3000 {
		t.Fatalf("copy: %+v", j)
	}
	if b, err := os.ReadFile(filepath.Join(root, "Gallery", "Trip", "day1", "a.txt")); err != nil || len(b) != 3000 {
		t.Fatalf("copied file: %d %v", len(b), err)
	}
	// Copying again asks first, naming what clashes.
	_, err = s.Transfer("copy", []string{"/Documents/Trip"}, "/Gallery", ClashAsk)
	var ce *ClashError
	if !errors.As(err, &ce) || !slices.Equal(ce.Names, []string{"Trip"}) {
		t.Fatalf("clash: %v", err)
	}
	j, _ = s.Transfer("copy", []string{"/Documents/Trip"}, "/Gallery", ClashKeep)
	waitJob(t, s, j.ID)
	if _, err := os.Stat(filepath.Join(root, "Gallery", "Trip (2)")); err != nil {
		t.Fatal("keep both didn't make Trip (2)")
	}

	if _, err := s.Transfer("move", []string{"/Documents/Trip"}, "/Documents/Trip/day1", ClashAsk); !errors.Is(err, ErrIntoSelf) {
		t.Fatalf("into itself: %v", err)
	}
	j, err = s.Transfer("move", []string{"/Documents/notes.txt"}, "/Gallery", ClashAsk)
	if err != nil {
		t.Fatal(err)
	}
	if j = waitJob(t, s, j.ID); j.State != "done" {
		t.Fatalf("move: %+v", j)
	}
	if _, err := os.Stat(filepath.Join(root, "Gallery", "notes.txt")); err != nil {
		t.Fatal("not moved")
	}
	if _, err := os.Stat(filepath.Join(root, "Documents", "notes.txt")); err == nil {
		t.Fatal("still at the old place")
	}
}

func TestTrash(t *testing.T) {
	s, root := newData(t)
	if err := s.Delete([]string{"/Documents/notes.txt", "/Gallery"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Gallery")); err == nil {
		t.Fatal("Gallery still there")
	}
	items, err := s.Trash()
	if err != nil || len(items) != 2 {
		t.Fatalf("%+v %v", items, err)
	}
	var notes TrashItem
	for _, it := range items {
		if it.Name == "notes.txt" {
			notes = it
		}
	}
	if notes.From != "/Documents" || notes.Size != 5 || notes.Kind != "text" {
		t.Fatalf("notes: %+v", notes)
	}
	// The Trash itself isn't reachable through the normal calls.
	if _, err := s.List("/.icedash/trash", true); err != nil {
		t.Log("trash is listable read-only:", err)
	}
	if err := s.Delete([]string{"/.icedash/trash"}); !errors.Is(err, ErrProtected) {
		t.Fatalf("deleting the trash: %v", err)
	}

	// A file of the same name came back meanwhile: the restored one gets "(2)".
	os.WriteFile(filepath.Join(root, "Documents", "notes.txt"), []byte("new"), 0o644)
	got, err := s.Restore([]string{notes.ID})
	if err != nil || len(got) != 1 || got[0].Path != "/Documents/notes (2).txt" {
		t.Fatalf("restore: %+v %v", got, err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "Documents", "notes (2).txt")); string(b) != "hello" {
		t.Fatalf("restored %q", b)
	}
	if err := s.Purge(nil); err != nil {
		t.Fatal(err)
	}
	if items, _ := s.Trash(); len(items) != 0 {
		t.Fatalf("after empty: %+v", items)
	}
	if err := s.Purge([]string{"../../etc"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("purge outside: %v", err)
	}
}

func TestTicketsAndZip(t *testing.T) {
	s, _ := newData(t)
	id, err := s.Ticket([]string{"/Documents/notes.txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Redeem(id, false); err != nil || !slices.Equal(got, []string{"/Documents/notes.txt"}) {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := s.Redeem(id, true); !errors.Is(err, ErrTicket) {
		t.Fatal("a file ticket must not open a zip")
	}
	if _, err := s.Redeem("nope", false); !errors.Is(err, ErrTicket) {
		t.Fatal("unknown ticket")
	}
	if _, err := s.Ticket([]string{"/Documents"}, false); !errors.Is(err, ErrIsDir) {
		t.Fatalf("file ticket for a folder: %v", err)
	}

	var buf bytes.Buffer
	if err := s.WriteZip(&buf, []string{"/Documents", "/Gallery/photo.jpg"}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(b)
	}
	want := map[string]string{"Documents/notes.txt": "hello", "Documents/.hidden": "x", "photo.jpg": "jpg"}
	if len(got) != len(want) {
		t.Fatalf("zip: %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("zip %s = %q", k, got[k])
		}
	}
	// The whole root leaves the system folders out.
	buf.Reset()
	s.WriteZip(&buf, []string{"/"})
	zr, _ = zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, ".") || strings.HasPrefix(f.Name, "lost+found") {
			t.Fatalf("system entry in the zip: %s", f.Name)
		}
	}
	if ZipName([]string{"/Documents"}) != "Documents.zip" || ZipName([]string{"/a", "/b"}) != "files.zip" {
		t.Fatal("zip names")
	}
}

func TestContentTypeNeverRunsCode(t *testing.T) {
	for name, want := range map[string]string{
		"a.png": "image/png", "a.pdf": "application/pdf", "a.mp4": "video/mp4",
		"a.html": "text/plain; charset=utf-8", "a.svg": "text/plain; charset=utf-8", "a.js": "text/plain; charset=utf-8", "a.bin": "text/plain; charset=utf-8",
	} {
		if got := ContentType(name); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}
	}
}
