//go:build !linux

package files

import "os"

// otherDisk can't tell disks apart off Linux (tests on a PC).
func otherDisk(*os.Root, string) bool { return false }
