package casa

// Taking over the main address (PLAN.md, Phase 3a, "Taking over the main
// address"). Measured on CasaOS 0.4.15 (HOST-NOTES, "Taking over the main
// address on CasaOS"):
//   - the gateway's route API can point "/" at IceDash and back; CasaOS's own
//     API routes are longer and keep winning;
//   - a gateway restart resets "/" to its static server, on a new port, and
//     moves the management address; routes IceDash added survive;
//   - the stock UI's files are all at root paths (/app.<hash>.js, /js/, ...)
//     and its router is hash-based.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Routes IceDash points at itself on CasaOS.
var Routes = []string{"/", "/modules/icedash", "/v2/icedash"}

// StockPath is where the stock CasaOS UI stays reachable.
const StockPath = "/casaos/"

// Keeper keeps the gateway's routes pointing at IceDash.
type Keeper struct {
	RuntimePath string // /var/run/casaos
	Target      string // http://127.0.0.1:<port>
	HTTP        *http.Client
}

func (k *Keeper) client() *http.Client {
	if k.HTTP != nil {
		return k.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (k *Keeper) read(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(k.RuntimePath, name))
	if err != nil {
		return "", err
	}
	u := strings.TrimRight(strings.TrimSpace(string(b)), "/")
	if !strings.HasPrefix(u, "http://127.0.0.1:") && !strings.HasPrefix(u, "http://localhost:") {
		return "", fmt.Errorf("unexpected address %q in %s", u, name)
	}
	return u, nil
}

// routes reads the gateway's route table (routes.json, path → target).
func (k *Keeper) routes() (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(k.RuntimePath, "routes.json"))
	if err != nil {
		return nil, err
	}
	var r map[string]string
	return r, json.Unmarshal(b, &r)
}

func (k *Keeper) register(ctx context.Context, mgmt, path, target string) error {
	body, _ := json.Marshal(map[string]string{"path": path, "target": target})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mgmt+"/v1/gateway/routes", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := k.client().Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("the gateway answered %s for %s", res.Status, path)
	}
	return nil
}

// Ensure registers every route that doesn't point at IceDash. It reports
// whether it changed anything.
func (k *Keeper) Ensure(ctx context.Context) (bool, error) {
	mgmt, err := k.read("management.url")
	if err != nil {
		return false, err
	}
	have, err := k.routes()
	if err != nil {
		have = map[string]string{} // not written yet: register them all
	}
	changed := false
	for _, p := range Routes {
		if strings.TrimRight(have[p], "/") == k.Target {
			continue
		}
		if err := k.register(ctx, mgmt, p, k.Target); err != nil {
			return changed, err
		}
		changed = true
	}
	return changed, nil
}

// Run keeps the routes in place until ctx ends: every 2 s until the first
// success (the gateway comes late at boot), then every 10 s. ready is called
// once, after the first success.
func (k *Keeper) Run(ctx context.Context, ready func()) {
	every, first := 2*time.Second, true
	for {
		changed, err := k.Ensure(ctx)
		switch {
		case err != nil:
			if ctx.Err() == nil {
				log.Printf("[icedash] gateway routes: %v", err)
			}
		case first:
			log.Printf("[icedash] gateway routes point at %s: %s", k.Target, strings.Join(Routes, ", "))
			first, every = false, 10*time.Second
			if ready != nil {
				ready()
			}
		case changed:
			log.Printf("[icedash] gateway routes were reset (gateway restart?); registered again")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

// Restore points "/" back at the stock UI. The other routes stay until the
// next reboot (the gateway can't remove a route, measured on ZimaOS).
func (k *Keeper) Restore(ctx context.Context) error {
	mgmt, err := k.read("management.url")
	if err != nil {
		return err
	}
	static, err := k.read("static.url")
	if err != nil {
		return err
	}
	return k.register(ctx, mgmt, "/", static)
}

// Front serves the stock UI next to IceDash: /casaos/ is its page, and every
// path IceDash doesn't own goes to the stock static server, since the page's
// files are at root paths. Next gets "/", /modules/icedash/ and /v2/icedash.
type Front struct {
	RuntimePath string
	Next        http.Handler
}

func (f *Front) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	switch {
	case p == "/" || strings.HasPrefix(p, "/modules/icedash") || strings.HasPrefix(p, "/v2/icedash"):
		f.Next.ServeHTTP(w, r)
		return
	case p == strings.TrimSuffix(StockPath, "/"):
		http.Redirect(w, r, StockPath, http.StatusFound)
		return
	}
	static, err := (&Keeper{RuntimePath: f.RuntimePath}).read("static.url")
	if err != nil {
		http.Error(w, "the CasaOS dashboard isn't available right now", http.StatusBadGateway)
		return
	}
	target, err := url.Parse(static)
	if err != nil {
		http.Error(w, "the CasaOS dashboard isn't available right now", http.StatusBadGateway)
		return
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			if pr.In.URL.Path == StockPath {
				pr.Out.URL.Path, pr.Out.URL.RawPath = "/", ""
			}
			pr.Out.Host = pr.In.Host
		},
	}
	rp.ServeHTTP(w, r)
}
