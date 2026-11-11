// Package state guards /DATA/.icedash, the one place IceDash writes to.
//
// The backend runs as root and later installs images it finds there, so the
// folder must be one nobody else can change: a real directory (no symlink),
// owned by root, mode 0700, in a parent that only root can write to.
// /DATA is 0755 root:root on ZimaOS 1.8.0 (docs/HOST-NOTES.md). This follows
// Module Manager's internal/state (MIT, github.com/chicohaager/modmanager).
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Dir is IceDash's state folder on ZimaOS.
const Dir = "/DATA/.icedash"

// owner is the uid the folder must belong to: root in the service, the test's
// own user in tests.
var owner = os.Geteuid()

// Ensure creates dir (0700) when it is missing and refuses to go on when dir
// or its parent can be changed by anyone but root or this process' user.
func Ensure(dir string) error {
	parent := filepath.Dir(dir)
	pfi, err := os.Lstat(parent)
	if err != nil {
		return fmt.Errorf("state folder: %w", err)
	}
	if !pfi.IsDir() {
		return fmt.Errorf("state folder: %s is not a directory", parent)
	}
	if u, ok := uidOf(pfi); ok && u != 0 && u != owner {
		return fmt.Errorf("state folder: %s belongs to uid %d", parent, u)
	}
	if checkModes && pfi.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("state folder: others can write to %s (mode %v)", parent, pfi.Mode().Perm())
	}

	fi, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return fmt.Errorf("state folder: %w", err)
		}
		fi, err = os.Lstat(dir)
	}
	if err != nil {
		return fmt.Errorf("state folder: %w", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return fmt.Errorf("state folder: %s is not a plain directory", dir)
	}
	if u, ok := uidOf(fi); ok && u != owner {
		return fmt.Errorf("state folder: %s belongs to uid %d", dir, u)
	}
	if checkModes && fi.Mode().Perm() != 0o700 {
		return os.Chmod(dir, 0o700)
	}
	return nil
}

// WriteFile replaces path atomically (temp file, then rename) with mode 0600,
// so a crash never leaves half a file behind.
func WriteFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // a no-op after the rename
	if err := tmp.Chmod(0o600); err != nil && checkModes {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
