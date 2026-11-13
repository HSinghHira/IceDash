package host

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDetect(t *testing.T) {
	t.Setenv("ICEDASH_HOST", "")
	dir := t.TempDir()
	zima := filepath.Join(dir, "zima-os-release")
	debian := filepath.Join(dir, "debian-os-release")
	write(t, zima, "NAME=\"ZimaOS\"\nID=zimaos\nVERSION_ID=1.8.0\n")
	write(t, debian, "PRETTY_NAME=\"Debian GNU/Linux 12 (bookworm)\"\nID=debian\n")

	run := filepath.Join(dir, "run")
	if err := os.Mkdir(run, 0o700); err != nil {
		t.Fatal(err)
	}
	// ZimaOS has the gateway file too: os-release decides.
	write(t, filepath.Join(run, "management.url"), "http://127.0.0.1:41623")

	if k := Detect(zima, run); k != ZimaOS {
		t.Errorf("ZimaOS box: %s", k)
	}
	if k := Detect(debian, run); k != CasaOS {
		t.Errorf("Debian with a CasaOS gateway: %s", k)
	}
	if k := Detect(debian, filepath.Join(dir, "none")); k != ZimaOS {
		t.Errorf("no gateway: %s, want the ZimaOS default", k)
	}
	t.Setenv("ICEDASH_HOST", "CasaOS")
	if k := Detect(zima, run); k != CasaOS {
		t.Errorf("ICEDASH_HOST override: %s", k)
	}
	if CasaOS.Name() != "CasaOS" || ZimaOS.Name() != "ZimaOS" {
		t.Error("names")
	}
}
