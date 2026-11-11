package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsSaveAndReload(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	s := Open(p)
	if !s.Get().AutoCheck {
		t.Fatal("automatic checks are off by default")
	}
	if err := s.Set(Settings{AutoCheck: false}); err != nil {
		t.Fatal(err)
	}
	if Open(p).Get().AutoCheck {
		t.Fatal("the switch didn't survive a restart")
	}
}

func TestDamagedFileUsesDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(p, []byte("{nope"), 0o600)
	if !Open(p).Get().AutoCheck {
		t.Fatal("damaged file didn't fall back to the defaults")
	}
}
