// Package creds reads an IceStore app's generated login from inside its
// running container (PLAN.md, Phase 2, "Credentials panel").
//
// IceStore images write it to /run/icestore/credentials (or $AIO_CRED_FILE),
// one line each: "K<TAB>label<TAB>value" or "N<TAB>note" (docker/_common/lib.sh
// in the IceStore repo). IceDash finds the container's main process through
// the Docker API and reads the file through /proc/<pid>/root, so nothing in
// the container changes. Values are never stored or logged.
package creds

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

// DefaultFile is where IceStore images write the login.
const DefaultFile = "/run/icestore/credentials"

// DefaultSocket is the Docker socket (root:docker 0660, measured).
const DefaultSocket = "/var/run/docker.sock"

// Field is one labelled value. Fields with the same Group belong together
// (an address with its own login, like Documenso's MAIL VIEWER).
type Field struct {
	Group  string `json:"group,omitempty"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
	URL    bool   `json:"url"`
}

// Creds is the parsed file.
type Creds struct {
	Fields []Field  `json:"fields"`
	Notes  []string `json:"notes"`
}

var (
	// ErrNotRunning: the container isn't running, so there's nothing to read.
	ErrNotRunning = errors.New("the app isn't running")
	// ErrNone: the app keeps no generated login.
	ErrNone = errors.New("this app has no saved login")
)

var secretLabel = regexp.MustCompile(`(?i)pass|secret|token|key`)

// urlWithLogin is "http://host/path  user / password" in one value (measured
// in Documenso's MAIL VIEWER line).
var urlWithLogin = regexp.MustCompile(`^(https?://\S+)\s+(\S+)\s*/\s*(\S+)$`)

// withSlash ends a folder-like address with "/": "http://box:6142/mail" made
// the app redirect to "/mail/" on its own inside port (8080, measured with
// Documenso's mail viewer), which the browser can't reach. Root, files
// ("/a.html"), queries and fragments are left alone.
func withSlash(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(raw, "?") || strings.HasSuffix(raw, "#") {
		return raw
	}
	p := u.EscapedPath()
	if p == "" || strings.HasSuffix(p, "/") || strings.Contains(path.Base(p), ".") {
		return raw
	}
	return raw + "/"
}

// Parse reads the file's lines; anything else is skipped.
func Parse(b []byte) Creds {
	c := Creds{Fields: []Field{}, Notes: []string{}}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 3)
		switch {
		case parts[0] == "K" && len(parts) == 3:
			label, value := strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
			if label == "" {
				continue
			}
			isURL := strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
			if m := urlWithLogin.FindStringSubmatch(value); m != nil {
				c.Fields = append(c.Fields,
					Field{Group: label, Label: "Address", Value: withSlash(m[1]), URL: true},
					Field{Group: label, Label: "User", Value: m[2]},
					Field{Group: label, Label: "Password", Value: m[3], Secret: true})
				continue
			}
			if isURL {
				c.Fields = append(c.Fields, Field{Group: label, Label: "Address", Value: withSlash(value), URL: true})
				continue
			}
			c.Fields = append(c.Fields, Field{Label: label, Value: value, Secret: secretLabel.MatchString(label)})
		case parts[0] == "N" && len(parts) >= 2:
			if n := strings.TrimSpace(strings.Join(parts[1:], "\t")); n != "" {
				c.Notes = append(c.Notes, n)
			}
		}
	}
	return c
}

// Reader reads credentials from containers.
type Reader struct {
	Socket string // default DefaultSocket
	Proc   string // default "/proc"; tests point it at a folder
	// Inspect, when set, replaces the Docker API (tests).
	Inspect func(ctx context.Context, container string) (Container, error)
}

// Container is what Reader needs from docker inspect.
type Container struct {
	Pid     int
	Running bool
	Env     []string
}

// Read returns the container's credentials.
func (r *Reader) Read(ctx context.Context, container string) (Creds, error) {
	inspect := r.Inspect
	if inspect == nil {
		inspect = r.inspect
	}
	c, err := inspect(ctx, container)
	if err != nil {
		return Creds{}, err
	}
	if !c.Running || c.Pid <= 0 {
		return Creds{}, ErrNotRunning
	}
	file := DefaultFile
	for _, e := range c.Env {
		if v, ok := strings.CutPrefix(e, "AIO_CRED_FILE="); ok && strings.HasPrefix(v, "/") {
			file = v
		}
	}
	proc := r.Proc
	if proc == "" {
		proc = "/proc"
	}
	b, err := readInRoot(fmt.Sprintf("%s/%d/root", proc, c.Pid), file)
	if errors.Is(err, fs.ErrNotExist) {
		return Creds{}, ErrNone
	}
	if err != nil {
		return Creds{}, err
	}
	creds := Parse(b)
	if len(creds.Fields) == 0 && len(creds.Notes) == 0 {
		return Creds{}, ErrNone
	}
	return creds, nil
}

// readInRoot reads root+file, refusing symlinks on the way: through
// /proc/<pid>/root an absolute link would point into the host.
func readInRoot(root, file string) ([]byte, error) {
	clean := path.Clean("/" + file)
	cur := root
	for _, part := range strings.Split(strings.TrimPrefix(clean, "/"), "/") {
		cur += "/" + part
		fi, err := os.Lstat(cur)
		if err != nil {
			return nil, err
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing to follow a link at %s", strings.TrimPrefix(cur, root))
		}
	}
	f, err := os.Open(cur)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("the credentials file isn't a plain file")
	}
	return io.ReadAll(io.LimitReader(f, 64<<10))
}

func (r *Reader) inspect(ctx context.Context, container string) (Container, error) {
	sock := r.Socket
	if sock == "" {
		sock = DefaultSocket
	}
	hc := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		}},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/"+url.PathEscape(container)+"/json", nil)
	if err != nil {
		return Container{}, err
	}
	res, err := hc.Do(req)
	if err != nil {
		return Container{}, fmt.Errorf("docker: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return Container{}, ErrNotRunning
	}
	if res.StatusCode != http.StatusOK {
		return Container{}, fmt.Errorf("docker answered %s", res.Status)
	}
	var v struct {
		State struct {
			Pid     int  `json:"Pid"`
			Running bool `json:"Running"`
		} `json:"State"`
		Config struct {
			Env []string `json:"Env"`
		} `json:"Config"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&v); err != nil {
		return Container{}, fmt.Errorf("docker: %w", err)
	}
	return Container{Pid: v.State.Pid, Running: v.State.Running, Env: v.Config.Env}, nil
}
