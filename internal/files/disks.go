package files

// Other disks (after the release, 2026-11-16): besides /DATA, Files shows
// each disk mounted in its own folder under /media or /mnt (USB sticks, a
// second drive, a network share). Measured on ZimaOS 1.8.0: /media is a bind
// of /DATA/.media holding "ZimaOS-HD -> /DATA", and /mnt/boot is the vfat
// boot partition of the system disk; neither is shown.
//
// API paths for /DATA stay "/rel". A disk's are "<id>:/rel": a /DATA path
// always starts with "/", a disk's never does, so the two can't be confused.
// Each disk is opened as its own os.Root, so nothing can leave it either.

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Disk is one place Files can show: /DATA first, then the other disks.
type Disk struct {
	ID    string `json:"id"`    // "" for /DATA
	Name  string `json:"name"`  // "DATA", or the folder's name ("USB-Stick")
	Path  string `json:"path"`  // its top as an API path: "/" or "<id>:/"
	Main  bool   `json:"main"`  // /DATA
	Trash bool   `json:"trash"` // deleting goes to the Trash (only /DATA has one)
	Size  uint64 `json:"size"`
	Free  uint64 `json:"free"`
}

// place is a disk's folder on the host and how the API names it.
type place struct {
	id  string // "" for /DATA
	dir string
}

// api writes rel (relative to the place) as an API path.
func (p place) api(rel string) string {
	if p.id == "" {
		return "/" + rel
	}
	return p.id + ":/" + rel
}

func (p place) open() (*os.Root, error) {
	r, err := os.OpenRoot(p.dir)
	if err != nil {
		return nil, notFound(err)
	}
	return r, nil
}

func (s *Service) main() place { return place{dir: s.Root} }

var diskID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// resolve splits an API path into its place and the path inside it.
func (s *Service) resolve(p string) (place, string, error) {
	if id, rest, ok := strings.Cut(p, ":/"); ok && !strings.HasPrefix(p, "/") && diskID.MatchString(id) {
		for _, d := range s.mounted() {
			if d.id == id {
				rel, err := Clean(rest)
				return d.place, rel, err
			}
		}
		return place{}, "", ErrNotFound
	}
	rel, err := Clean(p)
	return s.main(), rel, err
}

// Disks lists /DATA and the other disks, with how full they are.
func (s *Service) Disks() []Disk {
	out := []Disk{{Name: "DATA", Path: "/", Main: true, Trash: true}}
	out[0].Size, out[0].Free = usage(s.Root)
	for _, d := range s.mounted() {
		dk := Disk{ID: d.id, Name: d.name, Path: d.api("")}
		dk.Size, dk.Free = usage(d.dir)
		out = append(out, dk)
	}
	return out
}

// mount is a disk found in the mount table.
type mount struct {
	place
	name string
}

// diskTypes are the file systems shown: disks and network shares. tmpfs,
// overlay, squashfs (system images) and the like are not.
var diskTypes = map[string]bool{
	"ext2": true, "ext3": true, "ext4": true, "xfs": true, "btrfs": true, "f2fs": true, "zfs": true,
	"vfat": true, "exfat": true, "ntfs": true, "ntfs3": true, "fuseblk": true, "hfsplus": true,
	"cifs": true, "smb3": true, "nfs": true, "nfs4": true, "fuse.mergerfs": true,
}

// mounted reads the mount table for disks in a folder of their own right
// under one of MountDirs. A disk is left out when it is /DATA's own, or
// another part of the system disk (the boot partition).
func (s *Service) mounted() []mount {
	info := s.MountInfo
	if info == "" {
		info = "/proc/self/mountinfo"
	}
	dirs := s.MountDirs
	if dirs == nil {
		dirs = []string{"/media", "/mnt"}
	}
	f, err := os.Open(info)
	if err != nil {
		return nil
	}
	defer f.Close()
	type row struct{ dev, root, at, fstype, source string }
	var rows []row
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		// 36 35 98:0 /mnt1 /mnt2 rw,noatime master:1 - ext3 /dev/root rw
		pre, post, ok := strings.Cut(sc.Text(), " - ")
		a, b := strings.Fields(pre), strings.Fields(post)
		if !ok || len(a) < 5 || len(b) < 2 {
			continue
		}
		rows = append(rows, row{dev: a[2], root: unescape(a[3]), at: unescape(a[4]), fstype: b[0], source: b[1]})
	}
	var dataDev, dataDisk string
	for _, r := range rows {
		if r.at == s.Root {
			dataDev, dataDisk = r.dev, baseDisk(r.source)
		}
	}
	var out []mount
	seen := map[string]bool{}
	for _, r := range rows {
		if !slices.Contains(dirs, path.Dir(r.at)) || r.root != "/" || !diskTypes[r.fstype] {
			continue
		}
		if r.dev == dataDev || seen[r.dev] || strings.HasPrefix(r.source, "/dev/loop") {
			continue
		}
		if dataDisk != "" && baseDisk(r.source) == dataDisk {
			continue
		}
		seen[r.dev] = true
		name := path.Base(r.at)
		id := idFor(name)
		for n := 2; slices.ContainsFunc(out, func(m mount) bool { return m.id == id }); n++ {
			id = fmt.Sprintf("%s-%d", idFor(name), n)
		}
		out = append(out, mount{place: place{id: id, dir: r.at}, name: name})
	}
	return out
}

// idFor makes a folder's name usable in an API path.
func idFor(name string) string {
	b := []byte(name)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			b[i] = '_'
		}
	}
	id := strings.Trim(string(b), ".")
	if len(id) > 60 {
		id = id[:60]
	}
	if id == "" {
		id = "disk"
	}
	return id
}

var partition = regexp.MustCompile(`^((?:nvme|mmcblk|loop)\d+(?:n\d+)?)p\d+$|^((?:sd|vd|hd|xvd)[a-z]+)\d+$`)

// baseDisk is the whole disk a partition is on ("/dev/sda8" → "sda",
// "/dev/nvme0n1p2" → "nvme0n1"); "" for sources that aren't devices.
func baseDisk(source string) string {
	if !strings.HasPrefix(source, "/dev/") {
		return ""
	}
	name := path.Base(source)
	if m := partition.FindStringSubmatch(name); m != nil {
		return m[1] + m[2]
	}
	return name
}

// unescape undoes the mount table's octal escapes ("\040" is a space).
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && isOct(s[i+1]) && isOct(s[i+2]) && isOct(s[i+3]) {
			b.WriteByte((s[i+1]-'0')<<6 | (s[i+2]-'0')<<3 | (s[i+3] - '0'))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isOct(c byte) bool { return c >= '0' && c <= '7' }
