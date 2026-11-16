//go:build !linux

package files

import "os"

// otherDisk can't tell disks apart off Linux (tests on a PC).
func otherDisk(*os.Root, string) bool { return false }

// usage can't be measured off Linux.
func usage(string) (size, free uint64) { return 0, 0 }
