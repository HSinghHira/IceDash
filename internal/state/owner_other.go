//go:build !unix

package state

import "os"

// checkModes: Windows has no Unix owners or permission bits. IceDash only
// runs on Linux; this lets the other packages' tests run on a Windows PC.
const checkModes = false

func uidOf(os.FileInfo) (int, bool) { return 0, false }
