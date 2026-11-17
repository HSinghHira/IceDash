package files

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// newDisks is newData plus a mount table shaped like ZimaOS 1.8.0's (/media
// a bind of /DATA/.media, /mnt/boot the system disk's boot partition), with a
// USB stick, an NVMe drive and two network shares, and things that must not show.
func newDisks(t *testing.T) (*Service, string, string) {
	t.Helper()
	s, data := newData(t)
	base := filepath.ToSlash(filepath.Dir(data))
	data = filepath.ToSlash(data)
	usb := base + "/media/USB Stick"
	for _, d := range []string{usb, base + "/mnt/fast", base + "/mnt/boot", base + "/media/ram", base + "/media/img",
		base + "/media/MEDIAS", base + "/mnt/nas", base + "/media/part"} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(usb+"/song.mp3", []byte("la la"), 0o644); err != nil {
		t.Fatal(err)
	}
	esc := func(p string) string { return strings.ReplaceAll(p, " ", `\040`) }
	table := strings.Join([]string{
		"26 1 0:23 / / ro - squashfs /dev/sda5 ro",
		"30 26 8:8 / " + esc(data) + " rw,relatime - ext4 /dev/sda8 rw",
		"31 26 8:8 /.media " + esc(base) + "/media rw - ext4 /dev/sda8 rw",
		"32 26 8:1 / " + esc(base) + "/mnt/boot rw - vfat /dev/sda1 rw",
		"40 31 8:17 / " + esc(usb) + " rw,nosuid shared:9 - vfat /dev/sdb1 rw,uid=0",
		"41 31 0:50 / " + esc(base) + "/media/ram rw - tmpfs tmpfs rw",
		"42 31 7:3 / " + esc(base) + "/media/img rw - ext4 /dev/loop3 rw",
		"43 26 259:1 / " + esc(base) + "/mnt/fast rw - ext4 /dev/nvme0n1p1 rw",
		"44 31 8:17 / " + esc(base) + "/media/again rw - vfat /dev/sdb1 rw",
		// A share from another server through ZimaOS Files (rclone), an NFS
		// export's subfolder, and a bind of a disk's subfolder (not a disk).
		"50 31 0:61 / " + esc(base) + "/media/MEDIAS rw,nosuid,nodev - fuse.rclone MEDIAS: rw,user_id=0,group_id=0",
		"51 26 0:62 /exports/tv " + esc(base) + "/mnt/nas rw - nfs4 nas:/exports/tv rw",
		"52 31 8:33 /sub " + esc(base) + "/media/part rw - ext4 /dev/sdc1 rw",
	}, "\n") + "\n"
	info := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(info, []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Root, s.MountInfo, s.MountDirs = data, info, []string{base + "/media", base + "/mnt"}
	return s, usb, base + "/mnt/fast"
}

func TestDisksFound(t *testing.T) {
	s, _, _ := newDisks(t)
	var got []string
	for _, d := range s.Disks() {
		got = append(got, d.Name+"="+d.Path)
	}
	want := []string{"DATA=/", "USB Stick=USB_Stick:/", "fast=fast:/", "MEDIAS=MEDIAS:/", "nas=nas:/"}
	if !slices.Equal(got, want) {
		t.Fatalf("Disks() = %v, want %v", got, want)
	}
	if d := s.Disks(); !d[0].Main || !d[0].Trash || d[1].Main || d[1].Trash {
		t.Errorf("main/trash flags wrong: %+v", d)
	}
}

func TestBaseDisk(t *testing.T) {
	for in, want := range map[string]string{
		"/dev/sda8": "sda", "/dev/sdb": "sdb", "/dev/nvme0n1p2": "nvme0n1", "/dev/mmcblk0p1": "mmcblk0",
		"/dev/vdc3": "vdc", "//nas/share": "", "tmpfs": "",
	} {
		if got := baseDisk(in); got != want {
			t.Errorf("baseDisk(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDiskPaths(t *testing.T) {
	s, usb, _ := newDisks(t)
	l, err := s.List("USB_Stick:/", false)
	if err != nil {
		t.Fatal(err)
	}
	if l.Path != "USB_Stick:/" || len(l.Entries) != 1 || l.Entries[0].Path != "USB_Stick:/song.mp3" || l.Entries[0].Protected {
		t.Fatalf("listing = %+v", l)
	}
	if _, err := s.List("nope:/", false); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown disk: %v", err)
	}
	// Climbing out of a disk stays inside it, as for /DATA.
	if l, err := s.List("USB_Stick:/../../DATA", false); err == nil {
		t.Errorf("escaped the disk: %+v", l)
	}
	// A /DATA path is never read as a disk's.
	if l, err := s.List("/Documents", false); err != nil || l.Path != "/Documents" {
		t.Errorf("/Documents: %+v, %v", l, err)
	}

	// Upload, rename and delete on the stick; delete is for good.
	if _, _, err := s.Upload("USB_Stick:/", "new.txt", ClashAsk, strings.NewReader("hi")); err != nil {
		t.Fatal(err)
	}
	if e, err := s.Rename("USB_Stick:/new.txt", "renamed.txt"); err != nil || e.Path != "USB_Stick:/renamed.txt" {
		t.Fatalf("rename: %+v, %v", e, err)
	}
	if err := s.Delete([]string{"USB_Stick:/renamed.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(usb + "/renamed.txt"); !os.IsNotExist(err) {
		t.Errorf("still there: %v", err)
	}
	if items, _ := s.Trash(); len(items) != 0 {
		t.Errorf("went to the Trash: %+v", items)
	}
	// One request can't mix places.
	if err := s.Delete([]string{"USB_Stick:/song.mp3", "/Documents/notes.txt"}); !errors.Is(err, ErrBadPath) {
		t.Errorf("mixed delete: %v", err)
	}
	// The disk's top can't be changed itself.
	if _, err := s.Rename("USB_Stick:/", "x"); !errors.Is(err, ErrProtected) {
		t.Errorf("rename top: %v", err)
	}
}

func TestTransferBetweenDisks(t *testing.T) {
	s, usb, fast := newDisks(t)
	j, err := s.Transfer("copy", []string{"/Documents/notes.txt"}, "USB_Stick:/", ClashAsk)
	if err != nil {
		t.Fatal(err)
	}
	if got := waitJob(t, s, j.ID); got.State != "done" || got.To != "USB_Stick:/" {
		t.Fatalf("copy: %+v", got)
	}
	if b, err := os.ReadFile(usb + "/notes.txt"); err != nil || string(b) != "hello" {
		t.Fatalf("copied file: %q, %v", b, err)
	}
	j, err = s.Transfer("move", []string{"USB_Stick:/song.mp3"}, "fast:/", ClashAsk)
	if err != nil {
		t.Fatal(err)
	}
	if got := waitJob(t, s, j.ID); got.State != "done" {
		t.Fatalf("move: %+v", got)
	}
	if _, err := os.Stat(path.Join(fast, "song.mp3")); err != nil {
		t.Errorf("not moved in: %v", err)
	}
	if _, err := os.Stat(usb + "/song.mp3"); !os.IsNotExist(err) {
		t.Errorf("original left: %v", err)
	}
	// A clash across disks is asked about first.
	if _, err := s.Transfer("copy", []string{"/Documents/notes.txt"}, "USB_Stick:/", ClashAsk); !errors.Is(err, ErrExists) {
		t.Errorf("clash: %v", err)
	}
}

func TestDiskTicketsAndZip(t *testing.T) {
	s, _, _ := newDisks(t)
	id, err := s.Ticket([]string{"USB_Stick:/song.mp3"}, false)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := s.Redeem(id, false)
	if err != nil || len(paths) != 1 || paths[0] != "USB_Stick:/song.mp3" {
		t.Fatalf("redeem: %v, %v", paths, err)
	}
	f, _, err := s.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if n := ZipName([]string{"USB_Stick:/"}); n != "files.zip" {
		t.Errorf("ZipName(top) = %q", n)
	}
	if n := ZipName([]string{"USB_Stick:/Music"}); n != "Music.zip" {
		t.Errorf("ZipName = %q", n)
	}
	var buf strings.Builder
	if err := s.WriteZip(&buf, []string{"USB_Stick:/song.mp3", "/Documents/notes.txt"}); err != nil || buf.Len() == 0 {
		t.Errorf("zip: %v", err)
	}
}
