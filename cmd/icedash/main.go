//go:build linux

// Command icedash is IceDash's backend: a small service on loopback behind
// the system's gateway. On ZimaOS it is a module beside the stock dashboard
// (route /v2/icedash); on CasaOS it takes over the main address and keeps the
// stock UI at /casaos/ (PLAN.md, Phase 3a).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/chicohaager/lintux-modkit/auth"
	"github.com/chicohaager/lintux-modkit/gateway"
	"github.com/chicohaager/lintux-modkit/httpx"
	"github.com/chicohaager/lintux-modkit/watchdog"
	"github.com/chicohaager/lintux-modkit/ysk"

	"github.com/HSinghHira/IceDash/internal/api"
	"github.com/HSinghHira/IceDash/internal/apps"
	"github.com/HSinghHira/IceDash/internal/casa"
	"github.com/HSinghHira/IceDash/internal/catalog"
	"github.com/HSinghHira/IceDash/internal/creds"
	"github.com/HSinghHira/IceDash/internal/host"
	"github.com/HSinghHira/IceDash/internal/notify"
	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/HSinghHira/IceDash/internal/settings"
	"github.com/HSinghHira/IceDash/internal/state"
	"github.com/HSinghHira/IceDash/internal/system"
	"github.com/HSinghHira/IceDash/internal/update"
	"github.com/HSinghHira/IceDash/internal/zima"
)

// Set at build time (Makefile LDFLAGS).
var (
	version = "0.0.0-dev"
	// broken=1 makes a test image for the rollback check (acceptance check 9):
	// the service exits at once, so the updater must put the previous one back.
	broken = ""
)

const (
	service            = "icedash"
	binary             = "/usr/bin/icedash"
	staticDir          = "/usr/share/casaos/www/modules/icedash"
	defaultRuntimePath = "/var/run/casaos"
	// CasaOS layout (PLAN.md, Phase 3a, "Install, update and uninstall").
	casaHome = "/opt/icedash"
)

func main() {
	log.SetFlags(0) // journald adds the time
	runtimePath := defaultRuntimePath
	if p := os.Getenv("CASAOS_RUNTIME_PATH"); p != "" {
		runtimePath = p
	}
	kind := host.Detect("/etc/os-release", runtimePath)
	if len(os.Args) == 2 && os.Args[1] == "--uninstall" {
		uninstall(kind, runtimePath)
		return
	}
	log.Printf("[icedash] starting v%s on %s", version, kind.Name())
	if broken == "1" {
		log.Fatalf("[icedash] this is a deliberately broken test build; exiting")
	}

	// Everything IceDash writes lives here; without a safe folder, stop.
	if err := state.Ensure(state.Dir); err != nil {
		log.Fatalf("[icedash] %v", err)
	}
	// A sysext service isn't started at boot on its own (measured on 1.8.0):
	// the watchdog units in /etc/systemd/system start it. On CasaOS the unit
	// is enabled normally.
	if kind == host.ZimaOS {
		if err := watchdog.Install(watchdog.Options{Service: service, Binary: binary}); err != nil {
			log.Printf("[icedash] boot watchdog: %v", err)
		}
	}
	var verify session.Verifier = auth.NewVerifier(auth.JWKSResolver(runtimePath))
	if os.Getenv("ICEDASH_DISABLE_AUTH") == "1" {
		log.Printf("[icedash] WARNING: sign-in checks disabled by ICEDASH_DISABLE_AUTH")
		verify = session.Disabled{}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	sampler := system.New()
	go sampler.Run(ctx, 2*time.Second)

	notes, err := notify.Open(filepath.Join(state.Dir, "notifications.json"))
	if err != nil {
		log.Fatalf("[icedash] notifications: %v", err)
	}
	prefs := settings.Open(filepath.Join(state.Dir, "settings.json"))
	web := &http.Client{Timeout: 10 * time.Minute}
	upd := &update.Manager{
		Version: version,
		Arch:    runtime.GOARCH,
		Dir:     filepath.Join(state.Dir, "self-update"),
		ExtDir:  "/var/lib/extensions",
		Root:    "/",
		GitHub:  update.GitHub{API: "https://api.github.com", Repo: update.Repo, HTTP: &http.Client{Timeout: 30 * time.Second}},
		HTTP:    web,
		Run:     run,
		Notify:  notes,
	}
	if kind == host.CasaOS {
		// No module system and no dashboard card on CasaOS (measured on 0.4.15).
		upd.Layout, upd.Home = update.LayoutFolder, casaHome
	} else {
		upd.Card = dashCard{ysk.Notifier{BusURLFile: ysk.DefaultBusURLFile}}
	}

	// Phase 2: IceStore inside IceDash.
	cat := &catalog.Catalog{HTTP: &http.Client{Timeout: 30 * time.Second}, Cache: filepath.Join(state.Dir, "catalog.json")}
	var appMgmt apps.Zima = &zima.Client{}
	if kind == host.CasaOS {
		c := casa.New()
		go c.Listen(ctx)
		appMgmt = c
	}
	mgr := &apps.Manager{
		Catalog: cat,
		Zima:    appMgmt,
		Creds:   &creds.Reader{},
		Notify:  notes,
		State:   filepath.Join(state.Dir, "apps.json"),
	}
	go mgr.Watch(ctx)

	srv := &api.Server{
		Version:  version,
		Arch:     runtime.GOARCH,
		Started:  time.Now().UTC(),
		Verify:   verify,
		System:   sampler,
		Update:   upd,
		Notes:    notes,
		Settings: prefs,
		Catalog:  cat,
		Apps:     mgr,
		Email:    accountEmail,
		Host:     string(kind),
		HostName: kind.Name(),
		StockURL: "/",
	}
	if kind == host.CasaOS {
		srv.StockURL = casa.StockPath
		srv.SessionOn = session.Options{System: "CasaOS", EveryoneAdmin: true}
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("[icedash] %v", err)
	}
	log.Printf("[icedash] listening on http://%s", listener.Addr())
	target := "http://" + listener.Addr().String()
	// Only once the gateway forwards to this process does it count as
	// started for a waiting swap script; then the script's result is read.
	var keeper *casa.Keeper
	if kind == host.CasaOS {
		keeper = &casa.Keeper{RuntimePath: runtimePath, Target: target}
		go keeper.Run(ctx, func() { go upd.Resume(ctx) })
	} else {
		go func() {
			if registerRoute(ctx, runtimePath, target) {
				upd.Resume(ctx)
			}
		}()
	}
	// First check 2 minutes after start (the gateway and /DATA come late at
	// boot), then every 6 hours, while "Check for updates automatically" is on.
	go upd.Loop(ctx, func() bool { return prefs.Get().AutoCheck }, 2*time.Minute, 6*time.Hour)

	dir := staticDir
	if kind == host.CasaOS {
		dir = filepath.Join(casaHome, "current", "web")
	}
	if d := os.Getenv("ICEDASH_STATIC_DIR"); d != "" {
		dir = d
	}
	handler := httpx.Logging(service, 1<<20)(httpx.Static("/modules/icedash/", dir, srv.Handler()))
	if kind == host.CasaOS {
		// The stock UI's own files pass through unlogged.
		handler = &casa.Front{RuntimePath: runtimePath, Next: handler}
	}
	hs := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		log.Printf("[icedash] shutting down")
		if keeper != nil {
			// Give "/" back to the stock UI; the next start takes it again.
			rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := keeper.Restore(rctx); err != nil {
				log.Printf("[icedash] giving / back to CasaOS: %v", err)
			}
			cancel()
		}
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	if err := hs.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("[icedash] %v", err)
	}
}

// uninstall undoes what IceDash set up outside its own files. On ZimaOS: the
// boot watchdog units in /etc/systemd/system, which `zpkg remove` doesn't
// know about, so the README runs `icedash --uninstall` first. On CasaOS:
// "/" goes back to the stock UI (uninstall.sh runs this).
func uninstall(kind host.Kind, runtimePath string) {
	if kind == host.CasaOS {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := (&casa.Keeper{RuntimePath: runtimePath}).Restore(ctx); err != nil {
			log.Fatalf("[icedash] giving / back to CasaOS: %v", err)
		}
		log.Printf("[icedash] the CasaOS dashboard is back at /")
		return
	}
	if err := watchdog.Uninstall(watchdog.Options{Service: service, Binary: binary}); err != nil {
		log.Fatalf("[icedash] removing the boot watchdog: %v", err)
	}
	log.Printf("[icedash] boot watchdog removed; now run: zpkg remove icedash")
}

// registerRoute retries for two minutes: the gateway needs ~45 s after boot,
// and a route posted before it listens is lost (lintux-modkit, measured on
// 1.7.1). The route list lives in /run, so this runs on every start.
func registerRoute(ctx context.Context, runtimePath, target string) bool {
	for i := 1; i <= 60; i++ {
		err := gateway.Register(ctx, runtimePath, api.Route, target, 10*time.Second)
		if err == nil {
			log.Printf("[icedash] gateway route registered: %s -> %s", api.Route, target)
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		log.Printf("[icedash] gateway route attempt %d: %v", i, err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Second):
		}
	}
	log.Printf("[icedash] gave up registering the gateway route")
	return false
}

// run runs a host command for the updater (unsquashfs, systemd-run).
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// dashCard is the "update available" card on the stock ZimaOS dashboard,
// through lintux-modkit's ysk (the message bus). Its button opens IceDash.
type dashCard struct{ n ysk.Notifier }

const cardID = "icedash-update"

func (c dashCard) Show(ctx context.Context, v string) error {
	return c.n.Show(ctx, ysk.NewCard(cardID, "IceDash", "/modules/icedash/appicon.svg",
		"IceDash "+v+" is ready to install.", "Open IceDash", "modules/icedash/index.html"))
}

func (c dashCard) Withdraw(ctx context.Context) error { return c.n.Withdraw(ctx, cardID) }

// accountEmail reads the signed-in user's email from ZimaOS (empty on the
// measured box, so it often fills in nothing).
func accountEmail(ctx context.Context, token string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1/v1/users/current", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	var v struct {
		Data struct {
			Email string `json:"email"`
		} `json:"data"`
	}
	if res.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&v) != nil {
		return ""
	}
	return v.Data.Email
}
