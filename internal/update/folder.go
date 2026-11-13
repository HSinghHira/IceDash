package update

// Self-update on CasaOS (PLAN.md, Phase 3a, "Install, update and uninstall on
// CasaOS"). There is no module system there: IceDash lives in
// /opt/icedash/releases/<version>/ and /opt/icedash/current points at the
// running one. An update unpacks the new release beside it, and the swap
// script switches the link and restarts the service, switching back when the
// new version doesn't start, the same check as the sysext swap.

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// LayoutFolder selects the CasaOS layout (Manager.Layout); "" is the sysext one.
const LayoutFolder = "folder"

// FolderAsset is the release file for the folder layout.
func FolderAsset(arch string) string { return "icedash-casaos-" + arch + ".tar.gz" }

// maxUnpacked caps what a release may unpack to.
const maxUnpacked = 400 << 20

func (m *Manager) prepareFolder(ctx context.Context, rel *Release, img Asset) error {
	if m.Home == "" {
		return errors.New("the install folder isn't set")
	}
	newDir := filepath.Join(m.Dir, "new")
	if err := os.RemoveAll(newDir); err != nil {
		return err
	}
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return err
	}
	tarball := filepath.Join(newDir, img.Name)

	m.step("checksum", "Getting the release's checksum")
	want, err := m.checksum(ctx, rel, img)
	if err != nil {
		return err
	}
	m.step("download", fmt.Sprintf("Downloading %s (%s)", img.Name, rel.Tag))
	got, err := m.download(ctx, img.URL, tarball)
	if err != nil {
		return err
	}
	m.step("verify", "Checking the download")
	if got != want {
		return fmt.Errorf("%w (expected %s…, got %s…)", ErrChecksum, want[:12], got[:12])
	}

	current := filepath.Join(m.Home, "current")
	old, err := filepath.EvalSymlinks(current)
	if err != nil {
		return fmt.Errorf("%w (%s)", ErrNoInstalled, current)
	}
	releases := filepath.Join(m.Home, "releases")
	ver := strings.TrimPrefix(rel.Tag, "v")
	if ver == "" || strings.ContainsAny(ver, `/\`) || ver == "." || ver == ".." {
		return fmt.Errorf("odd release tag %q", rel.Tag)
	}
	dest := filepath.Join(releases, ver)

	m.step("files", "Unpacking the new version")
	// Keep only the running release; the new one goes beside it.
	if entries, err := os.ReadDir(releases); err == nil {
		for _, e := range entries {
			p := filepath.Join(releases, e.Name())
			if p != old {
				_ = os.RemoveAll(p)
			}
		}
	}
	if dest == old {
		return fmt.Errorf("%s is already the running version", rel.Tag)
	}
	if err := Unpack(tarball, dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	if _, err := os.Stat(filepath.Join(dest, "icedash")); err != nil {
		_ = os.RemoveAll(dest)
		return errors.New("the release has no icedash program")
	}
	_ = os.Remove(tarball)

	m.step("handover", "Installing: IceDash restarts in a few seconds")
	return m.handover(ctx, FolderSwapScript(m.Dir, current, dest, old, ver, m.Version))
}

// Unpack extracts a .tar.gz into dir, refusing anything but plain files and
// folders inside it.
func Unpack(tarball, dir string) error {
	f, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	var total int64
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(strings.TrimPrefix(h.Name, "./"))
		if name == "." {
			continue
		}
		if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("the release has a file outside its folder: %q", h.Name)
		}
		dst := filepath.Join(dir, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			total += h.Size
			if total > maxUnpacked {
				return errTooLarge
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			// The program and the scripts run whatever the archive says (a
			// package made on Windows loses the flag).
			if h.Mode&0o111 != 0 || name == "icedash" || strings.HasSuffix(name, ".sh") {
				mode = 0o755
			}
			out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, io.LimitReader(tr, h.Size))
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("the release has an unexpected entry %q", h.Name)
		}
	}
}

// folderSwapScript is swapScript for the folder layout: the link replaces
// zpkg. "ln -sfn" then "mv -T" makes the switch one rename.
const folderSwapScript = `#!/bin/sh
# Written by IceDash: switches to the new release, back to the old one on failure.
D=%q
CURRENT=%q
NEW=%q
OLDDIR=%q
WANT=%q
OLD=%q
exec >"$D/log" 2>&1
echo "swap $(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ) from $OLD to $WANT"
sleep 2
started() {
  i=0
  while [ "$i" -lt %d ]; do
    [ "$(cat "$D/started" 2>/dev/null)" = "$1" ] && return 0
    sleep 1; i=$((i + 1))
  done
  echo "version $1 did not start"
  return 1
}
swap() {
  rm -f "$D/started"
  ln -sfn "$1" "$CURRENT.next" && mv -T "$CURRENT.next" "$CURRENT" && systemctl restart icedash.service && started "$2"
}
if swap "$NEW" "$WANT"; then
  echo "result ok"
  exit 0
fi
echo "the new version failed, putting $OLD back"
if swap "$OLDDIR" "$OLD"; then
  rm -rf "$NEW"
  echo "result rolled-back"
  exit 0
fi
echo "result rollback-failed: run 'sudo ln -sfn $OLDDIR $CURRENT' then 'sudo systemctl restart icedash'"
exit 1
`

// FolderSwapScript fills in the folder swap script.
func FolderSwapScript(dir, current, newDir, oldDir, want, old string) string {
	return fmt.Sprintf(folderSwapScript, dir, current, newDir, oldDir, want, old, startWait)
}
