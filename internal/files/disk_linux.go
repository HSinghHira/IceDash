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

// usage is a folder's disk size and free space, in bytes.
func usage(dir string) (size, free uint64) {
	var st syscall.Statfs_t
	if syscall.Statfs(dir, &st) != nil {
		return 0, 0
	}
	return st.Blocks * uint64(st.Bsize), st.Bavail * uint64(st.Bsize)
}

// sameDisk reports whether two folders are on the same file system: a folder
// under /media that nothing is mounted on is still /DATA's.
func sameDisk(a, b string) bool {
	var x, y syscall.Stat_t
	if syscall.Stat(a, &x) != nil || syscall.Stat(b, &y) != nil {
		return false
	}
	return x.Dev == y.Dev
}
