// Package apps is Phase 2's app handling: set up and install IceStore apps,
// track installs and updates, start, stop, restart, uninstall, logs,
// credentials, backups, the first-launch popup and app notifications
// (PLAN.md, "Phase 2: IceStore inside IceDash").
//
// Every change goes through the system's own app management (package zima on
// ZimaOS, package casa on CasaOS), so
// apps installed here look and behave the same on the stock dashboard.
package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HSinghHira/IceDash/internal/catalog"
	"github.com/HSinghHira/IceDash/internal/compose"
	"github.com/HSinghHira/IceDash/internal/creds"
	"github.com/HSinghHira/IceDash/internal/notify"
	"github.com/HSinghHira/IceDash/internal/state"
	"github.com/HSinghHira/IceDash/internal/zima"
)

// Notifier adds a notification (notify.Store).
type Notifier interface {
	Add(notify.Notification) (notify.Notification, bool, error)
}

// Zima is the part of zima.Client the manager uses (tests fake it).
type Zima interface {
	List(ctx context.Context) (map[string]zima.Installed, error)
	Install(ctx context.Context, yaml []byte, icon string, dryRun bool) (zima.InstallResult, error)
	Task(ctx context.Context, id string) (zima.Task, error)
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Restart(ctx context.Context, name string) error
	Uninstall(ctx context.Context, name string, deleteData bool) error
	Logs(ctx context.Context, name string, lines int) (string, error)
	AppGrid(ctx context.Context) ([]zima.Grid, error)
}

// Credentials reads a container's login (creds.Reader).
type Credentials interface {
	Read(ctx context.Context, container string) (creds.Creds, error)
}

// Manager is safe for concurrent use.
type Manager struct {
	Catalog *catalog.Catalog
	Zima    Zima
	Creds   Credentials
	Notify  Notifier
	State   string // /DATA/.icedash/apps.json
	AppData string // /DATA/AppData
	// Poll is how often a running task is checked (default 2 s).
	Poll time.Duration
	// Docker reads the apps' containers and images (nil: no way back and
	// no night updates).
	Docker Docker
	// Versions holds each app's previous version (default app-versions
	// beside State).
	Versions string
	// SelfBusy reports IceDash's own install, which night updates wait for.
	SelfBusy func() bool
	// NightLock is shared with IceDash's own night install (update.Manager).
	NightLock sync.Locker
	// Steady shortens the health check in tests (0: 3 minutes).
	Steady time.Duration

	mu       sync.Mutex
	jobs     map[string]*Job
	st       *file
	status   map[string]string     // last app grid status per app
	ownStops map[string]time.Time  // stops IceDash asked for, so they aren't "unexpected"
	frames   map[string]frameEntry // Frameable answers, by app
	// refreshed is the night the catalog was last refreshed for night updates.
	refreshed string
}

// file is what the manager keeps in /DATA/.icedash/apps.json.
type file struct {
	// Fresh lists apps installed through IceDash whose first-launch popup may show.
	Fresh []string `json:"fresh"`
	// Dismissed lists, per ZimaOS user, apps whose popup they turned off.
	Dismissed map[string][]string `json:"dismissed"`

	// Night updates and the way back (auto.go).
	Auto    []string            `json:"auto,omitempty"`        // apps that update at night
	Prev    map[string]Prev     `json:"prev,omitempty"`        // kept previous versions
	Held    map[string]Hold     `json:"held,omitempty"`        // versions not installed by themselves
	Skip    map[string]string   `json:"night_skip,omitempty"`  // why last night skipped an app
	Tried   map[string]string   `json:"night_tried,omitempty"` // the night each app was last tried
	Pending map[string]*Pending `json:"pending,omitempty"`     // night updates and go backs in progress
}

func (f *file) init() {
	if f.Dismissed == nil {
		f.Dismissed = map[string][]string{}
	}
	if f.Prev == nil {
		f.Prev = map[string]Prev{}
	}
	if f.Held == nil {
		f.Held = map[string]Hold{}
	}
	if f.Skip == nil {
		f.Skip = map[string]string{}
	}
	if f.Tried == nil {
		f.Tried = map[string]string{}
	}
	if f.Pending == nil {
		f.Pending = map[string]*Pending{}
	}
}

// Job is an install or update in progress or just finished.
type Job struct {
	Name    string    `json:"name"`
	Title   string    `json:"title"`
	Icon    string    `json:"icon"`
	Kind    string    `json:"kind"` // install, update, rollback
	TaskID  string    `json:"task_id"`
	State   string    `json:"state"` // running, succeeded, failed
	Phase   string    `json:"phase"`
	Error   string    `json:"error,omitempty"`
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitzero"`
}

func (m *Manager) load() *file {
	if m.st != nil {
		return m.st
	}
	m.st = &file{}
	if m.State != "" {
		if b, err := os.ReadFile(m.State); err == nil {
			var f file
			if json.Unmarshal(b, &f) == nil {
				m.st = &f
			}
		}
	}
	m.st.init()
	return m.st
}

func (m *Manager) save() {
	if m.State == "" || m.st == nil {
		return
	}
	b, err := json.Marshal(m.st)
	if err == nil {
		err = state.WriteFile(m.State, b)
	}
	if err != nil {
		log.Printf("[icedash] apps state: %v", err)
	}
}

// ---- setup and install ----

// SetupView is the setup popup's content.
type SetupView struct {
	App           catalog.App `json:"app"`
	Installed     bool        `json:"installed"`
	BeforeInstall string      `json:"before_install,omitempty"`
	compose.Setup
}

// Setup works out the popup for an IceStore app.
func (m *Manager) Setup(ctx context.Context, id string, c compose.Context) (SetupView, error) {
	a, err := m.Catalog.App(ctx, id)
	if err != nil {
		return SetupView{}, err
	}
	doc, err := m.doc(ctx, a)
	if err != nil {
		return SetupView{}, err
	}
	v := SetupView{App: a, Setup: doc.Setup(c)}
	if meta, err := m.Catalog.Meta(ctx, a.ID); err == nil {
		v.BeforeInstall = meta.Tips.BeforeInstall
	}
	if list, err := m.Zima.List(ctx); err == nil {
		_, v.Installed = list[doc.Name()]
	}
	return v, nil
}

func (m *Manager) doc(ctx context.Context, a catalog.App) (*compose.Doc, error) {
	b, err := m.Catalog.Compose(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	d, err := compose.Parse(b)
	if err != nil {
		return nil, err
	}
	if d.Name() == "" || !catalog.ValidName(d.Name()) {
		return nil, fmt.Errorf("%s's compose file has no usable name", a.Title)
	}
	return d, nil
}

// build fills the file with the user's choices; the result is ready to install.
func (m *Manager) build(ctx context.Context, id string, ch compose.Choices) (catalog.App, *compose.Doc, []byte, error) {
	a, err := m.Catalog.App(ctx, id)
	if err != nil {
		return a, nil, nil, err
	}
	doc, err := m.doc(ctx, a)
	if err != nil {
		return a, nil, nil, err
	}
	if err := doc.Apply(ch); err != nil {
		return a, nil, nil, &InputError{err}
	}
	b, err := doc.Bytes()
	return a, doc, b, err
}

// InputError is a problem with what the user entered.
type InputError struct{ Err error }

func (e *InputError) Error() string { return e.Err.Error() }
func (e *InputError) Unwrap() error { return e.Err }

// CheckResult answers the popup's port check.
type CheckResult struct {
	OK         bool              `json:"ok"`
	PortsInUse []string          `json:"ports_in_use"`
	Suggest    map[string]string `json:"suggest"` // port key → a free published port
	// Taken are ports in use that the setup has no field for, as with an
	// app on the host network: the install goes ahead, with a warning.
	Taken []string `json:"taken"`
	Error string   `json:"error,omitempty"`
}

func inUse(res zima.InstallResult) []string {
	return append(append([]string{}, res.PortsInUse["TCP"]...), res.PortsInUse["UDP"]...)
}

// outsideClash reports a port clash the setup can't fix: ports in use that
// no port field names, or ZimaOS's "install validation failed: port
// conflict" (HOST-NOTES, 2026-11-17) for an app with no port fields.
func outsideClash(doc *compose.Doc, res zima.InstallResult, err error) bool {
	if errors.Is(err, zima.ErrPortsInUse) {
		return !fixable(doc, inUse(res))
	}
	var ze *zima.Error
	return errors.As(err, &ze) && strings.Contains(strings.ToLower(ze.Message), "port conflict") && len(doc.Ports()) == 0
}

// fixable reports whether a clash is on a port the setup can change.
func fixable(doc *compose.Doc, used []string) bool {
	for _, p := range doc.Ports() {
		if slices.Contains(used, p.Published) {
			return true
		}
	}
	return false
}

// Check is a dry run of the install; clashing ports get a free suggestion.
func (m *Manager) Check(ctx context.Context, id string, ch compose.Choices) (CheckResult, error) {
	_, doc, b, err := m.build(ctx, id, ch)
	if err != nil {
		return CheckResult{}, err
	}
	res, err := m.Zima.Install(ctx, b, "", true)
	out := CheckResult{PortsInUse: []string{}, Suggest: map[string]string{}, Taken: []string{}}
	if outsideClash(doc, res, err) {
		// Nothing in the popup could free them (WatchYourLAN on the host
		// network, community feedback 2026-11-20): check the rest without.
		out.Taken = inUse(res)
		if pm := doc.XString("port_map"); len(out.Taken) == 0 && pm != "" {
			out.Taken = []string{pm}
		}
		res, err = m.Zima.Install(zima.WithoutPortCheck(ctx), b, "", true)
	}
	switch {
	case errors.Is(err, zima.ErrPortsInUse):
		inUse := inUse(res)
		out.PortsInUse = inUse
		for _, p := range doc.Ports() {
			if slices.Contains(inUse, p.Published) {
				if free, ok := m.freePort(ctx, id, ch, p); ok {
					out.Suggest[compose.PortKey(p)] = free
				}
			}
		}
		return out, nil
	case err != nil:
		var ze *zima.Error
		if errors.As(err, &ze) {
			out.Error = ze.Message
			return out, nil
		}
		return out, err
	}
	out.OK = true
	return out, nil
}

// freePort tries the next 30 numbers with dry runs.
func (m *Manager) freePort(ctx context.Context, id string, ch compose.Choices, p compose.Port) (string, bool) {
	start, err := strconv.Atoi(p.Published)
	if err != nil {
		return "", false
	}
	for n := start + 1; n <= start+30 && n <= 65535; n++ {
		try := compose.Choices{Values: ch.Values, Ports: map[string]string{}, Folders: ch.Folders, Add: ch.Add}
		for k, v := range ch.Ports {
			try.Ports[k] = v
		}
		try.Ports[compose.PortKey(p)] = strconv.Itoa(n)
		_, _, b, err := m.build(ctx, id, try)
		if err != nil {
			return "", false
		}
		res, err := m.Zima.Install(ctx, b, "", true)
		if err == nil {
			return strconv.Itoa(n), true
		}
		if !errors.Is(err, zima.ErrPortsInUse) || slices.Contains(res.PortsInUse["TCP"], strconv.Itoa(n)) || slices.Contains(res.PortsInUse["UDP"], strconv.Itoa(n)) {
			continue
		}
		return strconv.Itoa(n), true // the clash left is another port's
	}
	return "", false
}

// ErrBusy: that app already has an install or update running.
var ErrBusy = errors.New("that app is already being installed or updated")

// Install starts installing an IceStore app with the user's choices.
func (m *Manager) Install(ctx context.Context, id string, ch compose.Choices) (Job, error) {
	a, doc, b, err := m.build(ctx, id, ch)
	if err != nil {
		return Job{}, err
	}
	name := doc.Name()
	if !m.claim(name) {
		return Job{}, ErrBusy
	}
	res, err := m.Zima.Install(ctx, b, a.Icon, false)
	if outsideClash(doc, res, err) {
		res, err = m.Zima.Install(zima.WithoutPortCheck(ctx), b, a.Icon, false)
	}
	if err != nil {
		m.release(name)
		if errors.Is(err, zima.ErrPortsInUse) {
			return Job{}, &InputError{fmt.Errorf("port %s is in use now; check again", strings.Join(append(res.PortsInUse["TCP"], res.PortsInUse["UDP"]...), ", "))}
		}
		if errors.Is(err, zima.ErrOldInstall) {
			return Job{}, &InputError{err}
		}
		return Job{}, err
	}
	m.mu.Lock()
	m.forget(name, false) // installed again from the popup: an earlier copy no longer applies
	m.save()
	m.mu.Unlock()
	j := m.start(name, a.Title, a.Icon, "install", res.TaskID)
	return j, nil
}

// claim marks name busy unless a job for it is running.
func (m *Manager) claim(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.jobs == nil {
		m.jobs = map[string]*Job{}
	}
	if j, ok := m.jobs[name]; ok && j.State == "running" {
		return false
	}
	m.jobs[name] = &Job{Name: name, State: "running", Started: time.Now().UTC()}
	return true
}

func (m *Manager) release(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, name)
}

// track follows the job's task to its end (at most 30 minutes), then raises
// one notification.
func (m *Manager) track(name string) {
	poll := m.Poll
	if poll == 0 {
		poll = 2 * time.Second
	}
	m.mu.Lock()
	j := *m.jobs[name]
	m.mu.Unlock()

	deadline := time.Now().Add(30 * time.Minute)
	var t zima.Task
	var err error
	for {
		if j.TaskID == "" {
			// A rebuild without a task finished at once.
			t = zima.Task{State: "succeeded"}
			break
		}
		t, err = m.Zima.Task(context.Background(), j.TaskID)
		if err == nil {
			m.mu.Lock()
			m.jobs[name].Phase = t.Phase
			m.mu.Unlock()
			if t.Done() {
				break
			}
		}
		if time.Now().After(deadline) {
			t = zima.Task{State: "failed", Error: "it took more than 30 minutes"}
			break
		}
		time.Sleep(poll)
	}

	ok := t.State == "succeeded"
	errText := t.Error
	if !ok && errText == "" {
		errText = "the task stopped (" + t.State + ")"
	}
	if ok && j.Kind == "install" {
		m.mu.Lock()
		f := m.load()
		if !slices.Contains(f.Fresh, name) {
			f.Fresh = append(f.Fresh, name)
		}
		m.save()
		m.mu.Unlock()
	}

	// The notification goes first, so whoever sees the job end finds it.
	n := notify.Notification{App: name}
	switch {
	case ok && j.Kind == "install":
		n.Kind, n.Title, n.Body = notify.KindAppInstalled, j.Title+" is ready", j.Title+" is installed and starting."
		n.Action = &notify.Action{Label: "Open", Kind: "open_app"}
	case ok:
		n.Kind, n.Title, n.Body = notify.KindAppUpdated, j.Title+" is updated", j.Title+" now runs the newest version from IceStore."
	case j.Kind == "install":
		n.Kind, n.Title, n.Body = notify.KindAppInstallFailed, j.Title+" didn't install", errText
	default:
		n.Kind, n.Title, n.Body = notify.KindAppUpdateFailed, j.Title+" didn't update", errText
	}
	if m.Notify != nil {
		_, _, _ = m.Notify.Add(n)
	}

	m.mu.Lock()
	jj := m.jobs[name]
	jj.Ended = time.Now().UTC()
	if ok {
		jj.State = "succeeded"
	} else {
		jj.State, jj.Error = "failed", errText
	}
	m.mu.Unlock()
}

// Jobs lists installs and updates from the last hour, newest first.
func (m *Manager) Jobs() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Job{}
	for _, j := range m.jobs {
		if j.State == "running" || time.Since(j.Ended) < time.Hour {
			out = append(out, *j)
		}
	}
	slices.SortFunc(out, func(a, b Job) int { return b.Started.Compare(a.Started) })
	return out
}

// ---- installed apps ----

// Info is an installed app with what IceDash knows about it.
type Info struct {
	Name        string `json:"name"`
	StoreID     string `json:"store_id"`
	IceStore    bool   `json:"icestore"`
	Version     string `json:"version"`
	Latest      string `json:"latest,omitempty"`
	Update      bool   `json:"update"`
	FirstLaunch bool   `json:"first_launch"` // the popup should show for this user
	// Night updates and the way back.
	Auto      bool   `json:"auto"`
	Back      string `json:"back,omitempty"`    // the previous version kept to go back to
	Held      string `json:"held,omitempty"`    // a version that isn't installed by itself
	HeldBy    string `json:"held_by,omitempty"` // user, night
	NightSkip string `json:"night_skip,omitempty"`
}

// List returns the installed apps for user.
func (m *Manager) List(ctx context.Context, user string) ([]Info, error) {
	list, err := m.Zima.List(ctx)
	if err != nil {
		return nil, err
	}
	latest := map[string]string{}
	if idx, err := m.Catalog.Index(ctx, false); len(idx.Apps) > 0 || err == nil {
		for _, a := range idx.Apps {
			latest[a.ID] = a.Version
		}
	}
	m.mu.Lock()
	f := m.load()
	fresh, dismissed := slices.Clone(f.Fresh), slices.Clone(f.Dismissed[user])
	auto, held, skip := slices.Clone(f.Auto), maps.Clone(f.Held), maps.Clone(f.Skip)
	prev := map[string]Prev{}
	stale := false
	for n, p := range f.Prev {
		// A kept version applies only while the app runs the version it was
		// kept for (it may have changed in the stock dashboard).
		if in, ok := list[n]; ok && in.Version == p.To {
			prev[n] = p
		} else if _, running := m.jobs[n]; !running || m.jobs[n].State != "running" {
			m.dropPrev(n)
			stale = true
		}
	}
	if stale {
		m.save()
	}
	m.mu.Unlock()

	out := []Info{}
	for name, in := range list {
		i := Info{Name: name, StoreID: in.StoreID, Version: in.Version}
		i.IceStore = strings.HasPrefix(in.StoreID, catalog.IDPrefix)
		if i.IceStore {
			i.Latest = latest[in.StoreID]
			i.Update = i.Latest != "" && i.Version != "" && i.Latest != i.Version
		}
		i.FirstLaunch = slices.Contains(fresh, name) && !slices.Contains(dismissed, name)
		if i.IceStore {
			i.Auto = slices.Contains(auto, name)
			i.Back = prev[name].From
			i.Held, i.HeldBy = held[name].Version, held[name].By
			i.NightSkip = skip[name]
		}
		out = append(out, i)
	}
	slices.SortFunc(out, func(a, b Info) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (m *Manager) installed(ctx context.Context, name string) (zima.Installed, error) {
	list, err := m.Zima.List(ctx)
	if err != nil {
		return zima.Installed{}, err
	}
	in, ok := list[name]
	if !ok {
		return zima.Installed{}, ErrNotInstalled
	}
	return in, nil
}

// ErrNotInstalled: no app of that name.
var ErrNotInstalled = errors.New("that app isn't installed")

// Start, Stop and Restart go through the system's app management.
func (m *Manager) Start(ctx context.Context, name string) error {
	if _, err := m.installed(ctx, name); err != nil {
		return err
	}
	return m.Zima.Start(ctx, name)
}

func (m *Manager) Stop(ctx context.Context, name string) error {
	if _, err := m.installed(ctx, name); err != nil {
		return err
	}
	m.mu.Lock()
	if m.ownStops == nil {
		m.ownStops = map[string]time.Time{}
	}
	m.ownStops[name] = time.Now()
	m.mu.Unlock()
	return m.Zima.Stop(ctx, name)
}

func (m *Manager) Restart(ctx context.Context, name string) error {
	if _, err := m.installed(ctx, name); err != nil {
		return err
	}
	m.mu.Lock()
	if m.ownStops == nil {
		m.ownStops = map[string]time.Time{}
	}
	m.ownStops[name] = time.Now()
	m.mu.Unlock()
	return m.Zima.Restart(ctx, name)
}

// Uninstall removes the app; its data folder stays unless deleteData.
func (m *Manager) Uninstall(ctx context.Context, name string, deleteData bool) error {
	if _, err := m.installed(ctx, name); err != nil {
		return err
	}
	m.mu.Lock()
	if m.ownStops == nil {
		m.ownStops = map[string]time.Time{}
	}
	m.ownStops[name] = time.Now()
	m.mu.Unlock()
	if err := m.Zima.Uninstall(ctx, name, deleteData); err != nil {
		return err
	}
	m.mu.Lock()
	f := m.load()
	f.Fresh = slices.DeleteFunc(f.Fresh, func(s string) bool { return s == name })
	m.forget(name, true)
	for u, l := range f.Dismissed {
		f.Dismissed[u] = slices.DeleteFunc(l, func(s string) bool { return s == name })
	}
	m.save()
	m.mu.Unlock()
	return nil
}

// Logs returns the app's last lines.
func (m *Manager) Logs(ctx context.Context, name string, lines int) (string, error) {
	if _, err := m.installed(ctx, name); err != nil {
		return "", err
	}
	return m.Zima.Logs(ctx, name, lines)
}

// Credentials reads the app's generated login from its main container.
func (m *Manager) Credentials(ctx context.Context, name string) (creds.Creds, error) {
	in, err := m.installed(ctx, name)
	if err != nil {
		return creds.Creds{}, err
	}
	container := name
	if d, err := compose.Parse(in.Compose); err == nil {
		container = d.Container()
	}
	return m.Creds.Read(ctx, container)
}

// ---- first launch ----

// FirstLaunch is the welcome popup's content (mock-up 4).
type FirstLaunch struct {
	Name    string       `json:"name"`
	Title   string       `json:"title"`
	Tagline string       `json:"tagline"`
	Steps   []string     `json:"steps"`
	Creds   *creds.Creds `json:"creds,omitempty"` // admins only
}

var howToHead = regexp.MustCompile(`(?i)\*\*\s*how to use\??\s*\*\*`)
var stepLine = regexp.MustCompile(`^\s*\d+\.\s+(.*)$`)

// Steps reads the numbered "How to use?" list from an IceStore description.
func Steps(desc string) []string {
	loc := howToHead.FindStringIndex(desc)
	if loc == nil {
		return []string{}
	}
	var steps []string
	for _, line := range strings.Split(desc[loc[1]:], "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "**") {
			break // the next heading
		}
		if m := stepLine.FindStringSubmatch(line); m != nil {
			steps = append(steps, strings.TrimSpace(m[1]))
		} else if t := strings.TrimSpace(line); t != "" && len(steps) > 0 {
			steps[len(steps)-1] += " " + t // a wrapped step
		}
	}
	if steps == nil {
		return []string{}
	}
	return steps
}

// FirstLaunchFor builds the popup for an installed IceStore app.
func (m *Manager) FirstLaunchFor(ctx context.Context, name string, admin bool) (FirstLaunch, error) {
	in, err := m.installed(ctx, name)
	if err != nil {
		return FirstLaunch{}, err
	}
	out := FirstLaunch{Name: name, Title: in.Title, Steps: []string{}}
	if meta, err := m.Catalog.Meta(ctx, in.StoreID); err == nil {
		out.Tagline, out.Steps = meta.Tagline, Steps(meta.Description)
		if out.Title == "" {
			out.Title = meta.Title
		}
	}
	if admin {
		if c, err := m.Credentials(ctx, name); err == nil {
			out.Creds = &c
		}
	}
	return out, nil
}

// Dismiss turns the popup off for user and app.
func (m *Manager) Dismiss(user, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.load()
	if !slices.Contains(f.Dismissed[user], name) {
		f.Dismissed[user] = append(f.Dismissed[user], name)
		m.save()
	}
}

// ---- watcher ----

// Watch raises app notifications: an app that stopped without IceDash stopping
// it (every 30 s; a stop from the stock dashboard counts too),
// and IceStore app updates (after 3 minutes, then every 6 hours).
func (m *Manager) Watch(ctx context.Context) {
	status := time.NewTicker(30 * time.Second)
	defer status.Stop()
	updates := time.NewTimer(3 * time.Minute)
	defer updates.Stop()
	m.Resume(ctx)
	m.watchStatus(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-status.C:
			m.watchStatus(ctx)
		case <-updates.C:
			m.CheckUpdates(ctx)
			updates.Reset(catalog.MaxAge)
		}
	}
}

func (m *Manager) watchStatus(ctx context.Context) {
	grid, err := m.Zima.AppGrid(ctx)
	if err != nil {
		return
	}
	titles := map[string]string{}
	if list, err := m.Zima.List(ctx); err == nil {
		for n, in := range list {
			titles[n] = in.Title
		}
	}
	m.mu.Lock()
	first := m.status == nil
	if first {
		m.status = map[string]string{}
	}
	var stopped []string
	for _, g := range grid {
		was, seen := m.status[g.Name]
		m.status[g.Name] = g.Status
		own := time.Since(m.ownStops[g.Name]) < 3*time.Minute
		if !first && seen && was == "running" && g.Status != "running" && !own {
			stopped = append(stopped, g.Name)
		}
	}
	// Jobs in progress stop and start their app on purpose.
	stopped = slices.DeleteFunc(stopped, func(n string) bool { j, ok := m.jobs[n]; return ok && j.State == "running" })
	m.mu.Unlock()
	for _, n := range stopped {
		t := titles[n]
		if t == "" {
			t = n
		}
		if m.Notify != nil {
			_, _, _ = m.Notify.Add(notify.Notification{
				Kind: notify.KindAppStopped, App: n, Title: t + " stopped",
				Body:   t + " isn't running now. Start it from its menu, or look at its logs to see why.",
				Action: &notify.Action{Label: "Start", Kind: "start_app"},
			})
		}
	}
}

// CheckUpdates refreshes the catalog and raises one notification per new
// version of each installed IceStore app.
func (m *Manager) CheckUpdates(ctx context.Context) {
	if _, err := m.Catalog.Index(ctx, true); err != nil {
		log.Printf("[icedash] IceStore catalog: %v", err)
	}
	list, err := m.List(ctx, "")
	if err != nil {
		return
	}
	for _, a := range list {
		if !a.Update || m.Notify == nil || m.autoQuiet(a.Name, a.Latest) {
			continue
		}
		title := a.Name
		if in, err := m.installed(ctx, a.Name); err == nil && in.Title != "" {
			title = in.Title
		}
		_, _, _ = m.Notify.Add(notify.Notification{
			Kind: notify.KindAppUpdateAvailable, App: a.Name, Version: a.Latest,
			Key:    "app-update:" + a.Name + ":" + a.Latest,
			Title:  title + " " + a.Latest + " is in IceStore",
			Body:   "You have " + a.Version + ". Updating keeps your settings and data.",
			Action: &notify.Action{Label: "Update", Kind: "app_update"},
		})
	}
}
