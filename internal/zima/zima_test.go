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
