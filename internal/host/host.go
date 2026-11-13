// Package host tells which system IceDash runs on (PLAN.md, Phase 3a, "Host
// adapter"). ZimaOS names itself in /etc/os-release (ID=zimaos, measured on
// 1.8.0); CasaOS runs on a plain Debian or Ubuntu and is known by its gateway
// (/var/run/casaos/management.url, measured on 0.4.15).
package host

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// Kind is a supported system.
type Kind string

const (
	ZimaOS Kind = "zimaos"
	CasaOS Kind = "casaos"
)

// Name is the system's name as the UI writes it.
func (k Kind) Name() string {
	if k == CasaOS {
		return "CasaOS"
	}
	return "ZimaOS"
}

// Valid reports whether k is a known system.
func (k Kind) Valid() bool { return k == ZimaOS || k == CasaOS }

// Detect works out the system: ICEDASH_HOST if set (tests, development),
// then ID=zimaos in osRelease, then a CasaOS gateway under runtimePath.
// Anything else is taken as ZimaOS, the system IceDash started on.
func Detect(osRelease, runtimePath string) Kind {
	if k := Kind(strings.ToLower(strings.TrimSpace(os.Getenv("ICEDASH_HOST")))); k.Valid() {
		return k
	}
	if osID(osRelease) == "zimaos" {
		return ZimaOS
	}
	if _, err := os.Stat(filepath.Join(runtimePath, "management.url")); err == nil {
		return CasaOS
	}
	return ZimaOS
}

// osID reads ID= from an os-release file.
func osID(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "ID="); ok {
			return strings.ToLower(strings.Trim(v, `"'`))
		}
	}
	return ""
}
