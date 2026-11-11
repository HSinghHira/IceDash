//go:build unix

package state

import (
	"os"
	"syscall"
)

// checkModes: Unix permission bits mean something here.
const checkModes = true

func uidOf(fi os.FileInfo) (int, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(st.Uid), true
}
