package apps

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func dockerServer(t *testing.T, h http.HandlerFunc) *DockerAPI {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "d.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Skipf("no unix socket: %v", err)
	}
	srv := &http.Server{Handler: h}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return &DockerAPI{Socket: sock}
}

func TestDockerAPI(t *testing.T) {
	d := dockerServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/containers/json":
			if !strings.Contains(r.URL.Query().Get("filters"), "com.docker.compose.project=gitsave") || r.URL.Query().Get("all") != "1" {
				http.Error(w, "bad filter", 400)
				return
			}
			_, _ = w.Write([]byte(`[{"Id":"abc"}]`))
		case r.URL.Path == "/containers/abc/json":
			_, _ = w.Write([]byte(`{"Id":"abc","Image":"sha256:img","RestartCount":2,"State":{"Status":"running","ExitCode":0,"Health":{"Status":"healthy"}},"Config":{"Labels":{"com.docker.compose.service":"web"},"Healthcheck":{"StartPeriod":60000000000}}}`))
		case r.URL.Path == "/images/sha256:img/json":
			_, _ = w.Write([]byte(`{"Id":"sha256:img","RepoDigests":["ghcr.io/a/b@sha256:dd"]}`))
		case r.URL.Path == "/images/create":
			if r.URL.Query().Get("tag") == "bad" {
				_, _ = w.Write([]byte(`{"status":"Pulling"}` + "\n" + `{"error":"manifest unknown"}` + "\n"))
				return
			}
			_, _ = w.Write([]byte(`{"status":"Pulling"}` + "\n" + `{"status":"Done"}` + "\n"))
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	cs, err := d.Containers(ctx, "gitsave")
	if err != nil || len(cs) != 1 {
		t.Fatalf("containers %v %v", cs, err)
	}
	c := cs[0]
	if c.Service != "web" || c.ImageID != "sha256:img" || c.Restarts != 2 || c.Health != "healthy" || c.StartPeriod != time.Minute {
		t.Errorf("container %+v", c)
	}
	if ds, err := d.ImageDigests(ctx, "sha256:img"); err != nil || len(ds) != 1 {
		t.Errorf("digests %v %v", ds, err)
	}
	if ok, err := d.HasImage(ctx, "ghcr.io/a/b@sha256:ee"); ok || err != nil {
		t.Errorf("missing image: %v %v", ok, err)
	}
	if err := d.Pull(ctx, "ghcr.io/a/b:1"); err != nil {
		t.Errorf("pull %v", err)
	}
	if err := d.Pull(ctx, "ghcr.io/a/b:bad"); err == nil || !strings.Contains(err.Error(), "manifest unknown") {
		t.Errorf("failed pull: %v", err)
	}
}
