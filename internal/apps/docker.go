package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Docker is what the auto-update and the way back need from the Docker
// API: the app's containers, its images' digests, and pulls. Both ZimaOS and
// CasaOS run their apps as compose projects named after the app.
type Docker interface {
	Containers(ctx context.Context, project string) ([]Container, error)
	ImageDigests(ctx context.Context, image string) ([]string, error)
	HasImage(ctx context.Context, ref string) (bool, error)
	Pull(ctx context.Context, ref string) error
}

// Container is one container of an app, as docker inspect tells it.
type Container struct {
	ID       string
	Service  string // com.docker.compose.service
	ImageID  string // sha256:… of the image it runs
	State    string // running, exited, restarting, created…
	ExitCode int
	Restarts int
	Health   string // "", starting, healthy, unhealthy
	// StartPeriod is the healthcheck's start_period (0 without one).
	StartPeriod time.Duration
}

// DockerAPI talks to the Docker socket (root:docker 0660, measured on both systems).
type DockerAPI struct {
	Socket string // default /var/run/docker.sock
}

func (d *DockerAPI) client(timeout time.Duration) *http.Client {
	sock := d.Socket
	if sock == "" {
		sock = "/var/run/docker.sock"
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dl net.Dialer
			return dl.DialContext(ctx, "unix", sock)
		}},
	}
}

// ErrNoDocker: the Docker socket can't be reached at all (as opposed to a
// download Docker tried and failed).
var ErrNoDocker = errors.New("docker isn't reachable")

// errNotFound: Docker has no such container or image.
var errNotFound = errors.New("not found")

func (d *DockerAPI) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return err
	}
	res, err := d.client(30 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoDocker, err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("docker answered %s", res.Status)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(out)
}

// Containers lists the project's containers, stopped ones too.
func (d *DockerAPI) Containers(ctx context.Context, project string) ([]Container, error) {
	f, _ := json.Marshal(map[string][]string{"label": {"com.docker.compose.project=" + project}})
	var list []struct {
		ID string `json:"Id"`
	}
	if err := d.get(ctx, "/containers/json?all=1&filters="+url.QueryEscape(string(f)), &list); err != nil {
		return nil, err
	}
	out := []Container{}
	for _, c := range list {
		var v struct {
			ID           string `json:"Id"`
			Image        string `json:"Image"`
			RestartCount int    `json:"RestartCount"`
			State        struct {
				Status   string `json:"Status"`
				ExitCode int    `json:"ExitCode"`
				Health   *struct {
					Status string `json:"Status"`
				} `json:"Health"`
			} `json:"State"`
			Config struct {
				Labels      map[string]string `json:"Labels"`
				Healthcheck *struct {
					StartPeriod int64 `json:"StartPeriod"`
				} `json:"Healthcheck"`
			} `json:"Config"`
		}
		if err := d.get(ctx, "/containers/"+url.PathEscape(c.ID)+"/json", &v); err != nil {
			if errors.Is(err, errNotFound) {
				continue // removed in between
			}
			return nil, err
		}
		ct := Container{ID: v.ID, Service: v.Config.Labels["com.docker.compose.service"], ImageID: v.Image,
			State: v.State.Status, ExitCode: v.State.ExitCode, Restarts: v.RestartCount}
		if v.State.Health != nil {
			ct.Health = v.State.Health.Status
		}
		if v.Config.Healthcheck != nil {
			ct.StartPeriod = time.Duration(v.Config.Healthcheck.StartPeriod)
		}
		out = append(out, ct)
	}
	return out, nil
}

// ImageDigests returns an image's RepoDigests (repo@sha256:…).
func (d *DockerAPI) ImageDigests(ctx context.Context, image string) ([]string, error) {
	var v struct {
		RepoDigests []string `json:"RepoDigests"`
	}
	err := d.get(ctx, "/images/"+url.PathEscape(image)+"/json", &v)
	return v.RepoDigests, err
}

// HasImage reports whether Docker holds ref.
func (d *DockerAPI) HasImage(ctx context.Context, ref string) (bool, error) {
	var v struct {
		ID string `json:"Id"`
	}
	err := d.get(ctx, "/images/"+url.PathEscape(ref)+"/json", &v)
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Pull downloads ref (repo:tag or repo@sha256:…) and waits for the end.
// Docker answers 200 at once and streams progress; an error comes as a
// line with "error".
func (d *DockerAPI) Pull(ctx context.Context, ref string) error {
	q := url.Values{}
	if i := strings.Index(ref, "@"); i >= 0 {
		q.Set("fromImage", ref)
	} else if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		q.Set("fromImage", ref[:i])
		q.Set("tag", ref[i+1:])
	} else {
		q.Set("fromImage", ref)
		q.Set("tag", "latest")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/images/create?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	res, err := d.client(30 * time.Minute).Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoDocker, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&e)
		if e.Message == "" {
			e.Message = res.Status
		}
		return fmt.Errorf("couldn't download %s: %s", ref, e.Message)
	}
	dec := json.NewDecoder(res.Body)
	for {
		var line struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&line); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("couldn't download %s: %w", ref, err)
		}
		if line.Error != "" {
			return fmt.Errorf("couldn't download %s: %s", ref, line.Error)
		}
	}
}
