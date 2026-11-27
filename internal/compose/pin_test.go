package compose

import (
	"os"
	"strings"
	"testing"
)

func TestRepoAndDigestRef(t *testing.T) {
	for in, want := range map[string]string{
		"nginx":                             "nginx",
		"docker.io/library/nginx:1.27":      "nginx",
		"ghcr.io/hsinghhira/x:latest":       "ghcr.io/hsinghhira/x",
		"localhost:5000/app":                "localhost:5000/app",
		"localhost:5000/app:2":              "localhost:5000/app",
		"ghcr.io/a/b@sha256:abc":            "ghcr.io/a/b",
		"mostafawahied/autoxpose:0.6.0":     "mostafawahied/autoxpose",
		"index.docker.io/mostafawahied/x:1": "mostafawahied/x",
	} {
		if got := Repo(in); got != want {
			t.Errorf("Repo(%q) = %q, want %q", in, got, want)
		}
	}
	ds := []string{"mirror.example/x@sha256:aa", "nginx@sha256:bb"}
	if got := DigestRef("docker.io/library/nginx:latest", ds); got != "nginx@sha256:bb" {
		t.Errorf("DigestRef %q", got)
	}
	if got := DigestRef("other:1", ds); got != "" {
		t.Errorf("no match should give nothing, got %q", got)
	}
}

func TestSetImageAndServices(t *testing.T) {
	b, _ := os.ReadFile("testdata/autoxpose.yml")
	d, _ := Parse(b)
	if s := d.Services(); len(s) != 1 || s[0] != "autoxpose" {
		t.Fatalf("services %v", s)
	}
	if !d.SetImage("autoxpose", "mostafawahied/autoxpose@sha256:cc") || d.Images()["autoxpose"] != "mostafawahied/autoxpose@sha256:cc" {
		t.Errorf("images %v", d.Images())
	}
	if d.SetImage("nope", "x") {
		t.Error("set an image on a missing service")
	}
}

// Going back keeps what the user changed after the update, and only that.
func TestKeepChanges(t *testing.T) {
	old, _ := Parse([]byte(`name: a
services:
  web:
    image: a@sha256:11
    environment:
      - MODE=old
      - USER_SET=x
      - UNTOUCHED=1
    ports:
      - target: 80
        published: "8080"
    volumes:
      - type: bind
        source: /DATA/AppData/a
        target: /data
`))
	after, _ := Parse([]byte(`name: a
services:
  web:
    image: a:2
    environment:
      - MODE=new-default
      - USER_SET=x
      - UNTOUCHED=1
      - NEW_ONLY=y
    ports:
      - target: 80
        published: "8080"
    volumes:
      - type: bind
        source: /DATA/AppData/a
        target: /data
      - type: bind
        source: /DATA/AppData/a/cache
        target: /cache
`))
	now, _ := Parse([]byte(`name: a
services:
  web:
    image: a:2
    environment:
      - MODE=new-default
      - USER_SET=changed
      - UNTOUCHED=1
      - NEW_ONLY=y
      - ADDED_BY_USER=z
    ports:
      - target: 80
        published: "9090"
      - target: 443
        published: "9443"
    volumes:
      - type: bind
        source: /DATA/Media
        target: /data
      - type: bind
        source: /DATA/AppData/a/cache
        target: /cache
      - type: bind
        source: /DATA/Downloads
        target: /downloads
`))
	old.KeepChanges(after, now)
	b, _ := old.Bytes()
	got := string(b)
	for _, want := range []string{"MODE=old", "USER_SET=changed", "UNTOUCHED=1", `published: "9090"`, "source: /DATA/Media", "/DATA/Downloads", "image: a@sha256:11", "ADDED_BY_USER=z", `published: "9443"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	for _, bad := range []string{"new-default", "NEW_ONLY", "/cache"} {
		if strings.Contains(got, bad) {
			t.Errorf("the newer version's %q leaked into\n%s", bad, got)
		}
	}
}
