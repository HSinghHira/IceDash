package zima

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Answers as measured on ZimaOS 1.8.0 (docs/HOST-NOTES.md).
func box(t *testing.T, seen *[]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*seen = append(*seen, r.Method+" "+r.URL.RequestURI()+" "+string(b)+" auth="+r.Header.Get("Authorization"))
		p := strings.TrimPrefix(r.URL.Path, "/v2/app_management")
		switch {
		case p == "/zimaapp/install" && strings.Contains(string(b), `6101`):
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"data":{"ports_in_use":{"TCP":["6101"],"UDP":[]}},"message":"there are ports in use"}`))
		case p == "/zimaapp/install" && strings.Contains(string(b), "broken"):
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"data":{"code":"invalid_request","message":"compose file is invalid"},"message":"x"}`))
		case p == "/zimaapp/install":
			_, _ = w.Write([]byte(`{"data":{"accepted":false,"app_id":"autoxpose","dry_run":true,"port_changes":[],"task_id":"","type":"v2app","zimaapp_id":"zimaapp://v2app/autoxpose"},"message":"only validation"}`))
		case p == "/zimaapp/v2app/task/query":
			_, _ = w.Write([]byte(`{"data":{"app_id":"opensignup","id":"57","phase":"starting","state":"running","type":"start"},"message":"OK"}`))
		case p == "/zimaapp/v2app/stop":
			_, _ = w.Write([]byte(`{"data":{"items":[{"error":"not found"}],"summary":{"accepted":0,"failed":1,"succeeded":0,"total":1}},"message":"batch stop accepted"}`))
		case p == "/zimaapp/v2app/files/compose" && r.Method == http.MethodGet && r.URL.Query().Get("zimaapp_id") == "zimaapp://v2app/gitsave":
			w.Header().Set("Etag", `"sha256:2fc3"`)
			_, _ = w.Write([]byte(`{"name":"gitsave","services":{}}`))
		case p == "/zimaapp/v2app/files/compose" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"package not found"}`))
		case p == "/zimaapp/v2app/files/compose" && r.Method == http.MethodPut && strings.Contains(string(b), "old"):
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"invalid user override: generated override does not reproduce submitted Compose"}`))
		case p == "/zimaapp/v2app/files/compose" && r.Method == http.MethodPut:
			*seen = append(*seen, "if-match="+r.Header.Get("If-Match")+" type="+r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"data":{"accepted":true,"app_id":"gitsave","rebuilt":false,"task_id":"b34b","zimaapp_id":"zimaapp://v2app/gitsave"},"message":"compose rebuild accepted"}`))
		case p == "/compose/gitsave/logs":
			_, _ = w.Write([]byte(`{"data":"gitsave  | Server running on port 3000\n"}`))
		case p == "/compose":
			_, _ = w.Write([]byte(`{"data":{"gitsave":{"compose":{"name":"gitsave","x-casaos":{"id":"com.icestore.gitsave","version":"1.4.0","title":{"en_US":"GitSave"}}}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return &Client{Base: srv.URL}
}

func TestInstallAnswers(t *testing.T) {
	var seen []string
	c := box(t, &seen)
	ctx := WithToken(context.Background(), "tok")
	res, err := c.Install(ctx, []byte("name: autoxpose\n"), "", true)
	if err != nil || res.ZimaAppID != "zimaapp://v2app/autoxpose" {
		t.Fatalf("%+v %v", res, err)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(strings.SplitN(seen[0], " ", 3)[2][:strings.LastIndex(strings.SplitN(seen[0], " ", 3)[2], " auth=")]), &body)
	opts := body["v2app"].(map[string]any)["options"].(map[string]any)
	if opts["dryRun"] != true || opts["checkPortConflict"] != true || !strings.HasSuffix(seen[0], "auth=tok") {
		t.Errorf("request %s", seen[0])
	}
	res, err = c.Install(ctx, []byte(`published: "6101"`), "", true)
	if !errors.Is(err, ErrPortsInUse) || res.PortsInUse["TCP"][0] != "6101" {
		t.Errorf("clash: %+v %v", res, err)
	}
	_, err = c.Install(ctx, []byte("broken"), "", true)
	var ze *Error
	if !errors.As(err, &ze) || ze.Message != "compose file is invalid" {
		t.Errorf("invalid: %v", err)
	}
}

// An installed app is changed through the app settings call, not installed
// over (its own ports would count as in use).
func TestInstallOverInstalledRebuilds(t *testing.T) {
	var seen []string
	c := box(t, &seen)
	ctx := context.Background()
	res, err := c.Install(ctx, []byte("name: gitsave\nservices: {}\n"), "", false)
	if err != nil || res.TaskID != "b34b" || !res.Accepted {
		t.Fatalf("%+v %v", res, err)
	}
	put := seen[len(seen)-2]
	if !strings.HasPrefix(put, "PUT /v2/app_management/zimaapp/v2app/files/compose?zimaapp_id=zimaapp%3A%2F%2Fv2app%2Fgitsave&sync=false name: gitsave") {
		t.Errorf("put %s", put)
	}
	if seen[len(seen)-1] != `if-match="sha256:2fc3" type=application/yaml` {
		t.Errorf("headers %s", seen[len(seen)-1])
	}
	for _, s := range seen {
		if strings.Contains(s, "/zimaapp/install") {
			t.Errorf("installed over: %s", s)
		}
	}
	if _, err := c.Install(ctx, []byte("name: gitsave\n# old\n"), "", false); !errors.Is(err, ErrOldInstall) {
		t.Errorf("old install: %v", err)
	}
	// A new app, and any dry run, still go to install.
	seen = nil
	if _, err := c.Install(ctx, []byte("name: autoxpose\n"), "", false); err != nil || !strings.Contains(seen[len(seen)-1], "/zimaapp/install") {
		t.Errorf("new app: %v %v", err, seen)
	}
	seen = nil
	if _, err := c.Install(ctx, []byte("name: gitsave\n"), "", true); err != nil || len(seen) != 1 || !strings.Contains(seen[0], "/zimaapp/install") {
		t.Errorf("dry run: %v %v", err, seen)
	}
}

func TestOtherCalls(t *testing.T) {
	var seen []string
	c := box(t, &seen)
	ctx := context.Background()
	if tk, err := c.Task(ctx, "57"); err != nil || tk.Phase != "starting" || tk.Done() {
		t.Errorf("task %+v %v", tk, err)
	}
	if err := c.Stop(ctx, "nope"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("stop: %v", err)
	}
	if !strings.Contains(seen[len(seen)-1], `"zimaapp_ids":["zimaapp://v2app/nope"]`) {
		t.Errorf("stop body %s", seen[len(seen)-1])
	}
	if s, err := c.Logs(ctx, "gitsave", 5); err != nil || !strings.Contains(s, "port 3000") {
		t.Errorf("logs %q %v", s, err)
	}
	if !strings.Contains(seen[len(seen)-1], "/compose/gitsave/logs?lines=5") {
		t.Errorf("logs request %s", seen[len(seen)-1])
	}
	l, err := c.List(ctx)
	if err != nil || l["gitsave"].StoreID != "com.icestore.gitsave" || l["gitsave"].Version != "1.4.0" || l["gitsave"].Title != "GitSave" {
		t.Errorf("list %+v %v", l, err)
	}
}

func TestURLFileMustBeLoopback(t *testing.T) {
	f := filepath.Join(t.TempDir(), "url")
	_ = os.WriteFile(f, []byte("http://10.0.0.1:43397\n"), 0o644)
	if _, err := (&Client{URLFile: f}).List(context.Background()); err == nil {
		t.Error("want a refusal for a non-loopback address")
	}
}
