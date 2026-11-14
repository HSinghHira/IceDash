//go:build linux

package files

import (
	"os"
	"syscall"
)

// otherDisk reports whether rel sits on another disk than /DATA itself: a
// mount (Docker's overlays live under /DATA/.docker, measured on ZimaOS).
func otherDisk(r *os.Root, rel string) bool {
	root, err := r.Lstat(".")
	if err != nil {
		return false
	}
	here, err := r.Lstat(rel)
	if err != nil {
		return false
	}
	a, ok1 := root.Sys().(*syscall.Stat_t)
	b, ok2 := here.Sys().(*syscall.Stat_t)
	return ok1 && ok2 && a.Dev != b.Dev
}
