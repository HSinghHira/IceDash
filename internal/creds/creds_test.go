package creds

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const sample = "K\tURL\thttp://192.168.68.17:6142\nK\tUser\tadmin\nK\tPassword\tx7-secret\nN\tCreate your account on first visit.\njunk line\nK\t\tnolabel\n"

func TestParse(t *testing.T) {
	c := Parse([]byte(sample))
	if len(c.Fields) != 3 || len(c.Notes) != 1 {
		t.Fatalf("%+v", c)
	}
	if !c.Fields[0].URL || c.Fields[0].Group != "URL" || c.Fields[0].Label != "Address" || c.Fields[0].Secret || c.Fields[1].Secret || !c.Fields[2].Secret || c.Fields[1].Group != "" {
		t.Errorf("marks %+v", c.Fields)
	}
	if c.Notes[0] != "Create your account on first visit." {
		t.Errorf("note %q", c.Notes[0])
	}
}

func fakeRoot(t *testing.T, pid int, file, content string) string {
	t.Helper()
	proc := t.TempDir()
	p := filepath.Join(proc, "42", "root", filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return proc
}

func reader(proc string, c Container, err error) *Reader {
	return &Reader{Proc: proc, Inspect: func(context.Context, string) (Container, error) { return c, err }}
}

func TestReadFromTheContainerRoot(t *testing.T) {
	proc := fakeRoot(t, 42, "/run/icestore/credentials", sample)
	c, err := reader(proc, Container{Pid: 42, Running: true}, nil).Read(context.Background(), "documenso")
	if err != nil || len(c.Fields) != 3 {
		t.Fatalf("%+v %v", c, err)
	}
	// AIO_CRED_FILE moves it.
	proc = fakeRoot(t, 42, "/tmp/icestore-credentials", sample)
	c, err = reader(proc, Container{Pid: 42, Running: true, Env: []string{"AIO_CRED_FILE=/tmp/icestore-credentials"}}, nil).Read(context.Background(), "x")
	if err != nil || len(c.Fields) != 3 {
		t.Fatalf("AIO_CRED_FILE: %+v %v", c, err)
	}
}

func TestReadNoneOrStopped(t *testing.T) {
	proc := t.TempDir()
	_ = os.MkdirAll(filepath.Join(proc, "42", "root"), 0o700)
	if _, err := reader(proc, Container{Pid: 42, Running: true}, nil).Read(context.Background(), "x"); !errors.Is(err, ErrNone) {
		t.Errorf("no file: %v", err)
	}
	if _, err := reader(proc, Container{Pid: 0}, nil).Read(context.Background(), "x"); !errors.Is(err, ErrNotRunning) {
		t.Errorf("stopped: %v", err)
	}
	empty := fakeRoot(t, 42, "/run/icestore/credentials", "")
	if _, err := reader(empty, Container{Pid: 42, Running: true}, nil).Read(context.Background(), "x"); !errors.Is(err, ErrNone) {
		t.Errorf("empty file: %v", err)
	}
}

func TestReadRefusesLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	proc := t.TempDir()
	root := filepath.Join(proc, "42", "root")
	_ = os.MkdirAll(filepath.Join(root, "run"), 0o700)
	secret := filepath.Join(t.TempDir(), "host-secret")
	_ = os.WriteFile(secret, []byte("K\tHost\tshadow\n"), 0o600)
	if err := os.Symlink(filepath.Dir(secret), filepath.Join(root, "run", "icestore")); err != nil {
		t.Fatal(err)
	}
	_ = os.Rename(secret, filepath.Join(filepath.Dir(secret), "credentials"))
	if _, err := reader(proc, Container{Pid: 42, Running: true}, nil).Read(context.Background(), "x"); err == nil || errors.Is(err, ErrNone) {
		t.Fatalf("want a refusal, got %v", err)
	}
}

func TestParseSplitsAnAddressWithItsLogin(t *testing.T) {
	c := Parse([]byte("K\tMAIL VIEWER\thttp://192.168.68.17:6142/mail  admin / s3cr3t-value\n"))
	want := []Field{
		{Group: "MAIL VIEWER", Label: "Address", Value: "http://192.168.68.17:6142/mail", URL: true},
		{Group: "MAIL VIEWER", Label: "User", Value: "admin"},
		{Group: "MAIL VIEWER", Label: "Password", Value: "s3cr3t-value", Secret: true},
	}
	if len(c.Fields) != 3 {
		t.Fatalf("%+v", c.Fields)
	}
	for i := range want {
		if c.Fields[i] != want[i] {
			t.Errorf("field %d: %+v, want %+v", i, c.Fields[i], want[i])
		}
	}
}
