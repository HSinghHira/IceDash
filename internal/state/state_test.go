package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCreatesPrivateFolder(t *testing.T) {
	parent := t.TempDir()
	if checkModes {
		if err := os.Chmod(parent, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(parent, ".icedash")
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("folder not created: %v", err)
	}
	if checkModes && fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode %v, want 0700", fi.Mode().Perm())
	}
	// a second start finds it and leaves it
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureFixesLooseMode(t *testing.T) {
	if !checkModes {
		t.Skip("no Unix modes on this OS")
	}
	parent := t.TempDir()
	os.Chmod(parent, 0o755)
	dir := filepath.Join(parent, ".icedash")
	os.Mkdir(dir, 0o755)
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Lstat(dir)
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode %v, want 0700", fi.Mode().Perm())
	}
}

func TestEnsureRefusesSymlink(t *testing.T) {
	parent := t.TempDir()
	if checkModes {
		os.Chmod(parent, 0o755)
	}
	target := filepath.Join(parent, "elsewhere")
	os.Mkdir(target, 0o700)
	dir := filepath.Join(parent, ".icedash")
	if err := os.Symlink(target, dir); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if err := Ensure(dir); err == nil {
		t.Fatal("a symlinked state folder was accepted")
	}
}

func TestEnsureRefusesWritableParent(t *testing.T) {
	if !checkModes {
		t.Skip("no Unix modes on this OS")
	}
	parent := t.TempDir()
	os.Chmod(parent, 0o777) // like /DATA/AppData on some boxes
	if err := Ensure(filepath.Join(parent, ".icedash")); err == nil {
		t.Fatal("a parent others can write to was accepted")
	}
}

func TestWriteFileReplacesAtomically(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "notifications.json")
	if err := WriteFile(p, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(p, []byte("two")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "two" {
		t.Fatalf("got %q", b)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
	if checkModes {
		fi, _ := os.Stat(p)
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
		}
	}
}
