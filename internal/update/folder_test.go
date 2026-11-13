package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type entry struct {
	name, body string
	typ        byte
	mode       int64
}

func tarball(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		h := &tar.Header{Name: e.name, Typeflag: typ, Mode: mode, Size: int64(len(e.body)), Linkname: "/etc/passwd"}
		if typ != tar.TypeReg {
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

var release = []entry{
	{name: "icedash", body: "#!/bin/sh\n", mode: 0o755},
	{name: "web/", typ: tar.TypeDir},
	{name: "web/index.html", body: "<!doctype html>"},
	{name: "icedash.service", body: "[Unit]"},
}

func TestUnpack(t *testing.T) {
	dir := t.TempDir()
	tgz := filepath.Join(dir, "r.tar.gz")
	os.WriteFile(tgz, tarball(t, release), 0o600)
	out := filepath.Join(dir, "out")
	if err := Unpack(tgz, out); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "web", "index.html")); string(b) != "<!doctype html>" {
		t.Fatalf("web/index.html: %q", b)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(filepath.Join(out, "icedash")); fi.Mode().Perm() != 0o755 {
			t.Fatalf("icedash mode %v", fi.Mode())
		}
	}

	for name, bad := range map[string][]entry{
		"parent":   {{name: "../evil", body: "x"}},
		"absolute": {{name: "/etc/evil", body: "x"}},
		"symlink":  {{name: "link", typ: tar.TypeSymlink}},
	} {
		os.WriteFile(tgz, tarball(t, bad), 0o600)
		if err := Unpack(tgz, filepath.Join(dir, "bad-"+name)); err == nil {
			t.Errorf("%s: unpacked", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
		t.Fatal("a file landed outside the folder")
	}
}

func TestApplyFolderLayout(t *testing.T) {
	home := t.TempDir()
	old := filepath.Join(home, "releases", "0.1.0")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "icedash"), []byte("old"), 0o755)
	stale := filepath.Join(home, "releases", "0.0.9")
	os.MkdirAll(stale, 0o755)
	if err := os.Symlink(old, filepath.Join(home, "current")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}

	tgz := tarball(t, release)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := FolderAsset("amd64")
		switch r.URL.Path {
		case "/repos/HSinghHira/IceDash/releases/latest":
			json.NewEncoder(w).Encode(Release{Tag: "v0.2.0", Assets: []Asset{
				{Name: "icedash-amd64.raw", URL: srv.URL + "/dl/raw"},
				{Name: name, URL: srv.URL + "/dl/" + name},
				{Name: name + ".sha256", URL: srv.URL + "/dl/" + name + ".sha256"},
			}})
		case "/dl/" + name:
			w.Write(tgz)
		case "/dl/" + name + ".sha256":
			fmt.Fprintf(w, "%s  %s\n", sha(tgz), name)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	f := &fake{t: t, list: map[string]string{}}
	m := &Manager{
		Version: "0.1.0", Arch: "amd64", Layout: LayoutFolder, Home: home,
		Dir:      filepath.Join(home, "state"),
		GitHub:   GitHub{API: srv.URL, Repo: Repo, HTTP: srv.Client()},
		HTTP:     srv.Client(),
		Run:      f.run,
		Notify:   &notes{},
		Card:     &card{},
		AllowURL: func(string) error { return nil },
	}
	m.Check(context.Background())
	if err := m.Apply(); err != nil {
		t.Fatal(err)
	}
	f.waitRun(t, "systemd-run")
	if st := m.Status(); st.State == StateFailed {
		t.Fatalf("%+v", st)
	}
	newDir := filepath.Join(home, "releases", "0.2.0")
	if b, _ := os.ReadFile(filepath.Join(newDir, "web", "index.html")); string(b) != "<!doctype html>" {
		t.Fatalf("not unpacked: %q", b)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("an older release than the running one should be removed")
	}
	if _, err := os.Stat(filepath.Join(old, "icedash")); err != nil {
		t.Fatal("the running release must stay for the way back")
	}
	script, _ := os.ReadFile(filepath.Join(m.Dir, "run.sh"))
	for _, want := range []string{fmt.Sprintf("NEW=%q", newDir), fmt.Sprintf("OLDDIR=%q", old), `WANT="0.2.0"`, `OLD="0.1.0"`} {
		if !strings.Contains(string(script), want) {
			t.Fatalf("script lacks %s:\n%s", want, script)
		}
	}
	if f.ran("unsquashfs") != "" {
		t.Fatal("the folder layout has no image to list")
	}
}

func TestFolderSwapScript(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("needs a Unix sh")
	}
	old := startWait
	startWait = 3
	t.Cleanup(func() { startWait = old })

	for _, c := range []struct {
		name, newKind, result, link string
	}{
		{"good update", "good", "result ok", "new"},
		{"broken release rolls back", "broken", "result rolled-back", "old"},
	} {
		t.Run(c.name, func(t *testing.T) {
			home, bin, state := t.TempDir(), t.TempDir(), t.TempDir()
			oldDir, newDir := filepath.Join(home, "old"), filepath.Join(home, "new")
			os.MkdirAll(oldDir, 0o755)
			os.MkdirAll(newDir, 0o755)
			os.WriteFile(filepath.Join(oldDir, "kind"), []byte("good 0.1.0"), 0o644)
			os.WriteFile(filepath.Join(newDir, "kind"), []byte(c.newKind+" 0.2.0"), 0o644)
			current := filepath.Join(home, "current")
			os.Symlink(oldDir, current)
			// restarting "starts" whatever current points at, if it's good
			os.WriteFile(filepath.Join(bin, "systemctl"), []byte(`#!/bin/sh
read -r kind ver < "`+current+`/kind"
[ "$kind" = good ] && printf %s "$ver" > "`+state+`/started"
exit 0
`), 0o755)
			os.WriteFile(filepath.Join(bin, "sleep"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
			script := filepath.Join(state, "run.sh")
			os.WriteFile(script, []byte(FolderSwapScript(state, current, newDir, oldDir, "0.2.0", "0.1.0")), 0o700)
			cmd := exec.Command(sh, script)
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
			done := make(chan error, 1)
			go func() { done <- cmd.Run() }()
			select {
			case <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("script hung")
			}
			_, result, _ := readLog(filepath.Join(state, "log"))
			logText, _ := os.ReadFile(filepath.Join(state, "log"))
			if !strings.HasPrefix("result "+result, c.result) {
				t.Fatalf("result %q, want %q\n%s", result, c.result, logText)
			}
			target, _ := os.Readlink(current)
			if want := map[string]string{"new": newDir, "old": oldDir}[c.link]; target != want {
				t.Fatalf("current -> %s, want %s", target, want)
			}
		})
	}
}
