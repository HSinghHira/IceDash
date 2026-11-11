// Package update checks GitHub for a newer IceDash and installs it with one
// click, putting the previous version back when the new one doesn't start
// (PLAN.md, "Self-update"). It follows Module Manager's self-update
// (internal/catalog/self.go, MIT, measured on ZimaOS 1.8.0), and adds one
// thing: the swap script waits until the new version says it started, so a
// broken image that installs fine but can't run is rolled back too.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/HSinghHira/IceDash/internal/notify"
)

// States of the updater, as GET /v2/icedash/update reports them.
const (
	StateIdle       = "idle"
	StateChecking   = "checking"
	StateAvailable  = "available"
	StateInstalling = "installing"
	StateFailed     = "failed"
)

// Step states.
const (
	StepRunning = "running"
	StepDone    = "done"
	StepFailed  = "failed"
)

// Errors for the API.
var (
	ErrBusy        = errors.New("an update is already being installed")
	ErrNothingToDo = errors.New("no newer version is waiting")
	ErrChecksum    = errors.New("the download doesn't match the release's checksum")
	ErrCollision   = errors.New("the new image would replace files that belong to ZimaOS")
	ErrNoImage     = errors.New("the release has no image for this machine")
	ErrNoInstalled = errors.New("the installed IceDash image wasn't found")
	errTooLarge    = errors.New("the download is larger than an IceDash image can be")
)

const (
	resultPrefix          = "result "
	defaultMaxBytes int64 = 200 << 20
	// defaultStartWait is how long (s) the swap script waits for a new version to start.
	defaultStartWait = 90
)

// Step is one line of the install progress the UI shows.
type Step struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	State string `json:"state"`
}

// Status is the answer of GET /v2/icedash/update.
type Status struct {
	Current   string     `json:"current"`
	Latest    string     `json:"latest,omitempty"`
	Notes     string     `json:"notes,omitempty"`
	State     string     `json:"state"`
	Error     string     `json:"error,omitempty"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
	Steps     []Step     `json:"steps"`
}

// Runner runs a host command and returns its combined output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Notifier stores a notification (notify.Store).
type Notifier interface {
	Add(notify.Notification) (notify.Notification, bool, error)
}

// Card shows and withdraws the "update available" card on the stock ZimaOS dashboard.
type Card interface {
	Show(ctx context.Context, version string) error
	Withdraw(ctx context.Context) error
}

// Manager is the updater. Fill the fields, then call Resume once at start and Run.
type Manager struct {
	Version string // the running version, without "v"
	Arch    string // amd64
	Dir     string // /DATA/.icedash/self-update
	ExtDir  string // /var/lib/extensions, where zpkg keeps installed images
	Root    string // "/": the host, for the collision check
	GitHub  GitHub
	HTTP    *http.Client // downloads
	Run     Runner
	Notify  Notifier
	Card    Card
	// AllowURL vets each download URL; nil means AllowedURL (GitHub only).
	AllowURL func(string) error
	MaxBytes int64

	mu     sync.Mutex
	st     Status
	rel    *Release
	shown  bool // the dashboard card is up
	inited bool
}

func (m *Manager) init() {
	if m.inited {
		return
	}
	m.inited = true
	m.st = Status{Current: m.Version, State: StateIdle, Steps: []Step{}}
}

// Status returns a copy of the current state.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	st := m.st
	st.Steps = append([]Step{}, m.st.Steps...)
	return st
}

// Check asks GitHub for the latest release now.
func (m *Manager) Check(ctx context.Context) Status {
	m.mu.Lock()
	m.init()
	if m.st.State == StateInstalling || m.st.State == StateChecking {
		m.mu.Unlock()
		return m.Status()
	}
	before := m.st.State
	m.st.State = StateChecking
	m.mu.Unlock()

	rel, err := m.GitHub.Latest(ctx)
	now := time.Now().UTC()

	m.mu.Lock()
	m.st.CheckedAt = &now
	switch {
	case errors.Is(err, ErrNoRelease):
		m.st.State, m.st.Error, m.st.Latest, m.rel = StateIdle, "", "", nil
	case err != nil:
		// The check failed; what was known before still holds.
		m.st.Error = err.Error()
		m.st.State = before
		if m.st.State == StateChecking || m.st.State == StateFailed {
			m.st.State = StateIdle
		}
	case Compare(m.Version, rel.Tag) < 0:
		m.st.State, m.st.Error, m.st.Latest, m.st.Notes, m.rel = StateAvailable, "", rel.Tag, strings.TrimSpace(rel.Body), rel
		m.st.Steps = []Step{}
	default:
		m.st.State, m.st.Error, m.st.Latest, m.st.Notes, m.rel = StateIdle, "", rel.Tag, "", nil
	}
	available := m.st.State == StateAvailable
	m.mu.Unlock()

	if available {
		m.notify(notify.Notification{
			Kind: notify.KindUpdateAvailable, Version: rel.Tag, Key: "update:" + rel.Tag,
			Title:  "Update available",
			Body:   fmt.Sprintf("IceDash %s is ready to install.", rel.Tag),
			Action: &notify.Action{Label: "Update now", Kind: "update"},
		})
		m.syncCard(ctx)
	}
	return m.Status()
}

// Apply starts installing the waiting release. Progress shows in Status().Steps.
func (m *Manager) Apply() error {
	m.mu.Lock()
	m.init()
	switch {
	case m.st.State == StateInstalling:
		m.mu.Unlock()
		return ErrBusy
	case m.rel == nil || (m.st.State != StateAvailable && m.st.State != StateFailed):
		m.mu.Unlock()
		return ErrNothingToDo
	}
	rel := m.rel
	m.st.State, m.st.Error, m.st.Steps = StateInstalling, "", []Step{}
	m.mu.Unlock()
	go m.install(context.Background(), rel)
	return nil
}

func (m *Manager) step(id, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n := len(m.st.Steps); n > 0 && m.st.Steps[n-1].State == StepRunning {
		m.st.Steps[n-1].State = StepDone
	}
	m.st.Steps = append(m.st.Steps, Step{ID: id, Text: text, State: StepRunning})
}

func (m *Manager) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n := len(m.st.Steps); n > 0 {
		m.st.Steps[n-1].State = StepFailed
	}
	m.st.State, m.st.Error = StateFailed, err.Error()
}

// install runs the steps up to the handover. Anything that fails before the
// handover changes nothing on the box.
func (m *Manager) install(ctx context.Context, rel *Release) {
	if err := m.prepare(ctx, rel); err != nil {
		m.fail(err)
	}
}

func (m *Manager) prepare(ctx context.Context, rel *Release) error {
	img, err := m.pick(rel)
	if err != nil {
		m.step("download", "Finding the image for this machine")
		return err
	}
	for _, sub := range []string{"new", "previous"} {
		if err := os.RemoveAll(filepath.Join(m.Dir, sub)); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(m.Dir, sub), 0o700); err != nil {
			return err
		}
	}
	// zpkg takes the module name from the file name, so both are icedash.raw.
	newRaw := filepath.Join(m.Dir, "new", "icedash.raw")
	prevRaw := filepath.Join(m.Dir, "previous", "icedash.raw")

	m.step("checksum", "Getting the release's checksum")
	want, err := m.checksum(ctx, rel, img)
	if err != nil {
		return err
	}

	m.step("download", fmt.Sprintf("Downloading %s (%s)", img.Name, rel.Tag))
	got, err := m.download(ctx, img.URL, newRaw)
	if err != nil {
		return err
	}
	m.step("verify", "Checking the download")
	if got != want {
		return fmt.Errorf("%w (expected %s…, got %s…)", ErrChecksum, want[:12], got[:12])
	}

	installed := filepath.Join(m.ExtDir, "icedash.raw")
	if _, err := os.Stat(installed); err != nil {
		return fmt.Errorf("%w (%s)", ErrNoInstalled, installed)
	}
	m.step("files", "Checking the new image's files against ZimaOS")
	if err := m.checkCollisions(ctx, newRaw, installed); err != nil {
		return err
	}

	m.step("keep", "Keeping a copy of this version for the way back")
	if err := copyFile(installed, prevRaw); err != nil {
		return fmt.Errorf("no copy of the installed image for a rollback: %w", err)
	}

	m.step("handover", "Installing: IceDash restarts in a few seconds")
	script := filepath.Join(m.Dir, "run.sh")
	body := SwapScript(m.Dir, strings.TrimPrefix(rel.Tag, "v"), m.Version)
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(m.Dir, "log"))
	_ = os.Remove(filepath.Join(m.Dir, "handled"))
	if out, err := m.Run(ctx, "systemd-run", "--unit=icedash-self-update", "--collect", "--no-block", "/bin/sh", script); err != nil {
		return fmt.Errorf("systemd-run: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// pick chooses icedash-<arch>.raw, or icedash.raw (the amd64 copy) on amd64.
func (m *Manager) pick(rel *Release) (Asset, error) {
	names := []string{"icedash-" + m.Arch + ".raw"}
	if m.Arch == "amd64" {
		names = append(names, "icedash.raw")
	}
	for _, n := range names {
		for _, a := range rel.Assets {
			if a.Name == n {
				return a, nil
			}
		}
	}
	return Asset{}, fmt.Errorf("%w (%s)", ErrNoImage, m.Arch)
}

// checksum reads <image>.sha256 ("<hex>  <file>" or just "<hex>").
func (m *Manager) checksum(ctx context.Context, rel *Release, img Asset) (string, error) {
	var u string
	for _, a := range rel.Assets {
		if a.Name == img.Name+".sha256" {
			u = a.URL
		}
	}
	if u == "" {
		return "", fmt.Errorf("the release has no %s.sha256", img.Name)
	}
	resp, err := m.get(ctx, u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	f := strings.Fields(string(b))
	if len(f) == 0 || len(f[0]) != 64 {
		return "", fmt.Errorf("%s.sha256 isn't a checksum", img.Name)
	}
	if _, err := hex.DecodeString(f[0]); err != nil {
		return "", fmt.Errorf("%s.sha256 isn't a checksum", img.Name)
	}
	return strings.ToLower(f[0]), nil
}

func (m *Manager) get(ctx context.Context, u string) (*http.Response, error) {
	allow := m.AllowURL
	if allow == nil {
		allow = AllowedURL
	}
	if err := allow(u); err != nil {
		return nil, err
	}
	c := *m.HTTP
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return allow(req.URL.String())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "IceDash-updater")
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: %s", u, resp.Status)
	}
	return resp, nil
}

// download saves u to dst and returns the file's sha256.
func (m *Manager) download(ctx context.Context, u, dst string) (string, error) {
	resp, err := m.get(ctx, u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	max := m.MaxBytes
	if max <= 0 {
		max = defaultMaxBytes
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, max+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if n > max {
		return "", errTooLarge
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// checkCollisions refuses an image carrying a file the host already has,
// apart from the installed IceDash's own files: a sysext overlays /usr, so
// such a file would replace ZimaOS's (ground rule 2).
func (m *Manager) checkCollisions(ctx context.Context, img, installed string) error {
	files, err := m.imageFiles(ctx, img)
	if err != nil {
		return err
	}
	old, err := m.imageFiles(ctx, installed)
	if err != nil {
		return err
	}
	own := map[string]bool{}
	for _, f := range old {
		own[f] = true
	}
	var hit []string
	for _, f := range files {
		if own[f] || strings.HasPrefix(f, "/usr/lib/extension-release.d/") {
			continue
		}
		fi, err := os.Lstat(filepath.Join(m.Root, f))
		if err != nil || fi.IsDir() {
			continue
		}
		hit = append(hit, f)
	}
	if len(hit) == 0 {
		return nil
	}
	if len(hit) > 5 {
		hit = append(hit[:5], fmt.Sprintf("and %d more", len(hit)-5))
	}
	return fmt.Errorf("%w: %s", ErrCollision, strings.Join(hit, ", "))
}

// imageFiles lists a squashfs image with the host's unsquashfs (on ZimaOS 1.8.0):
// "squashfs-root/usr/bin/icedash" -> "/usr/bin/icedash".
func (m *Manager) imageFiles(ctx context.Context, img string) ([]string, error) {
	out, err := m.Run(ctx, "unsquashfs", "-lc", img)
	if err != nil {
		return nil, fmt.Errorf("list the image's files (unsquashfs): %w", err)
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(l), "squashfs-root/"); ok && rest != "" {
			files = append(files, "/"+rest)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s lists no files", filepath.Base(img))
	}
	return files, nil
}

// swapScript runs in a transient unit (systemd-run), outside IceDash's own
// service, which it restarts. One `zpkg install --force` swaps the image (one
// sysext refresh); --force doesn't restart the service, so the script does
// (both measured by Module Manager on 1.8.0). Then it waits until the new
// version writes its version to $D/started: an image that installs but can't
// run (acceptance check 9) is rolled back the same way. The last line of the
// log is the result: ok, rolled-back or rollback-failed.
const swapScript = `#!/bin/sh
# Written by IceDash: installs the new image, puts the previous one back on failure.
D=%q
WANT=%q
OLD=%q
exec >"$D/log" 2>&1
echo "swap $(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ) from $OLD to $WANT"
sleep 2
started() {
  i=0
  while [ "$i" -lt %d ]; do
    [ "$(cat "$D/started" 2>/dev/null)" = "$1" ] && return 0
    sleep 1; i=$((i + 1))
  done
  echo "version $1 did not start"
  return 1
}
swap() {
  rm -f "$D/started"
  zpkg install --force "$1" && systemctl daemon-reload && systemctl restart icedash.service && started "$2"
}
if swap "$D/new/icedash.raw" "$WANT"; then
  rm -f "$D/new/icedash.raw"  # zpkg copied it to /var/lib/extensions
  echo "result ok"
  exit 0
fi
echo "the new version failed, putting $OLD back"
if swap "$D/previous/icedash.raw" "$OLD"; then
  echo "result rolled-back"
  exit 0
fi
echo "result rollback-failed: run 'sudo zpkg install --force $D/previous/icedash.raw' then 'sudo systemctl restart icedash'"
exit 1
`

// SwapScript fills in the swap script for dir, the new version and the
// running one (both without "v", as /health and $D/started report them).
func SwapScript(dir, want, old string) string {
	return fmt.Sprintf(swapScript, dir, want, old, startWait)
}

// Resume runs once at start: it tells a waiting swap script that this version
// started, then reads the script's result and raises the matching notification.
func (m *Manager) Resume(ctx context.Context) {
	if err := os.MkdirAll(m.Dir, 0o700); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(m.Dir, "started"), []byte(m.Version), 0o600)

	// The script writes its result just after it sees "started".
	deadline := time.Now().Add(2 * time.Minute)
	for {
		first, result, ok := readLog(filepath.Join(m.Dir, "log"))
		if !ok {
			return // no update ran
		}
		if result != "" {
			m.finish(first, result)
			return
		}
		if time.Now().After(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// finish raises one notification per swap: "handled" remembers which swap's
// result was already announced, so a later restart doesn't repeat it.
func (m *Manager) finish(first, result string) {
	handled := filepath.Join(m.Dir, "handled")
	if b, err := os.ReadFile(handled); err == nil && strings.TrimSpace(string(b)) == first {
		return
	}
	v := "v" + m.Version
	var n notify.Notification
	switch {
	case result == "ok":
		n = notify.Notification{Kind: notify.KindUpdated, Version: v, Title: "Updated to " + v,
			Body: "IceDash is now running " + v + "."}
	case result == "rolled-back":
		n = notify.Notification{Kind: notify.KindUpdateFailed, Version: v, Title: "Update failed, previous version restored",
			Body: "The new version didn't start, so IceDash put " + v + " back. Nothing else changed."}
	case strings.HasPrefix(result, "rollback-failed"):
		fix := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(result, "rollback-failed"), ":"))
		n = notify.Notification{Kind: notify.KindRollbackFailed, Title: "Rollback failed",
			Body: "IceDash couldn't put the previous version back. To fix it: " + fix}
	default:
		return
	}
	m.notify(n)
	_ = os.WriteFile(handled, []byte(first+"\n"), 0o600)
}

// readLog returns the log's first line and its result ("ok", "rolled-back",
// "rollback-failed: …"), if the log exists.
func readLog(path string) (first, result string, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if first == "" {
			first = line
		}
		if strings.HasPrefix(line, resultPrefix) {
			result = strings.TrimPrefix(line, resultPrefix)
		}
	}
	return first, result, true
}

// Loop checks 2 minutes after start, then every 6 hours while auto() is true,
// and keeps the dashboard card up while an update waits: ZimaOS doesn't store
// these cards, so it's re-sent every minute and withdrawn after.
func (m *Manager) Loop(ctx context.Context, auto func() bool, first, every time.Duration) {
	next := time.Now().Add(first)
	card := time.NewTicker(time.Minute)
	defer card.Stop()
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-card.C:
			m.syncCard(ctx)
		case now := <-tick.C:
			if now.After(next) {
				next = now.Add(every)
				if auto() {
					cctx, cancel := context.WithTimeout(ctx, time.Minute)
					m.Check(cctx)
					cancel()
				}
			}
		}
	}
}

func (m *Manager) syncCard(ctx context.Context) {
	if m.Card == nil {
		return
	}
	st := m.Status()
	m.mu.Lock()
	shown := m.shown
	m.mu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	switch {
	case st.State == StateAvailable || st.State == StateFailed && st.Latest != "":
		if m.Card.Show(cctx, st.Latest) == nil {
			shown = true
		}
	case shown:
		if m.Card.Withdraw(cctx) == nil {
			shown = false
		}
	}
	m.mu.Lock()
	m.shown = shown
	m.mu.Unlock()
}

func (m *Manager) notify(n notify.Notification) {
	if m.Notify != nil {
		_, _, _ = m.Notify.Add(n)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// startWait is defaultStartWait; tests shorten it.
var startWait = defaultStartWait
