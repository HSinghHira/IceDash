package update

import "strings"

// Compare orders two versions semver-style and returns -1, 0 or 1. A leading
// "v" and build metadata ("+abc") are ignored, and a pre-release sorts before
// its release (0.3.0-dev7 < 0.3.0), with trailing numbers compared as numbers
// (dev10 > dev7). Same rules as Module Manager's CompareVersions.
func Compare(a, b string) int {
	a, b = strings.TrimPrefix(a, "v"), strings.TrimPrefix(b, "v")
	a, _, _ = strings.Cut(a, "+")
	b, _, _ = strings.Cut(b, "+")
	ac, apre, _ := strings.Cut(a, "-")
	bc, bpre, _ := strings.Cut(b, "-")
	as, bs := strings.Split(ac, "."), strings.Split(bc, ".")
	for i := 0; i < 3; i++ {
		if c := cmpInt(numAt(as, i), numAt(bs, i)); c != 0 {
			return c
		}
	}
	switch {
	case apre == bpre:
		return 0
	case apre == "":
		return 1
	case bpre == "":
		return -1
	}
	ap, bp := strings.Split(apre, "."), strings.Split(bpre, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		aw, an := splitNum(ap[i])
		bw, bn := splitNum(bp[i])
		if c := strings.Compare(aw, bw); c != 0 {
			return c
		}
		if c := cmpInt(an, bn); c != 0 {
			return c
		}
	}
	return cmpInt(len(ap), len(bp))
}

func cmpInt(x, y int) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

func numAt(p []string, i int) int {
	if i >= len(p) {
		return 0
	}
	n := 0
	for _, r := range p[i] {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func splitNum(s string) (string, int) {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	n := 0
	for _, r := range s[i:] {
		n = n*10 + int(r-'0')
	}
	return s[:i], n
}
