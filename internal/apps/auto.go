package apps

// App updates at night and the way back to an app's previous version
// (site/roadmap.json, "app auto-updates and roll back").
//
// Before every update IceDash keeps the installed file with each image
// pinned to the digest it runs (IceStore files often say :latest), and the
// file as it reads right after the update. Going back reapplies the old file
// with the settings changed since the update. A night update that doesn't
// come up and stay running goes back by itself; the version it went back
// from is held, so it isn't installed by itself again.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/HSinghHira/IceDash/internal/catalog"
	"github.com/HSinghHira/IceDash/internal/compose"
	"github.com/HSinghHira/IceDash/internal/notify"
	"github.com/HSinghHira/IceDash/internal/state"
	"github.com/HSinghHira/IceDash/internal/update"
	"github.com/HSinghHira/IceDash/internal/zima"
)

// Night skip reasons (Info.NightSkip).
const (
	SkipStopped    = "stopped"
	SkipUnhealthy  = "unhealthy"
	SkipNoCopy     = "no_copy"
	SkipCantChange = "cant_change"
)

var (
	// ErrNoPrevious: no earlier version is kept for that app.
	ErrNoPrevious = errors.New("there's no earlier version to go back to")
	// errNoCopy: the running images' digests couldn't be read.
	errNoCopy = errors.New("IceDash couldn't save this version to go back to")
	// errGone: the older image is neither here nor in its registry.
	errGone = errors.New("the older version can't be downloaded any more")
)

// Hold is a version that isn't installed by itself: the user went back from
// it (By "user") or it didn't start after a night update (By "night").
type Hold struct {
	Version string `json:"version"`
	By      string `json:"by"`
}

// Prev says which version the kept file is (From) and which version it was
// kept for (To, the version IceDash updated to).
type Prev struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Pending is a night update or a go back in progress, kept so a restart of
// IceDash or the server picks it up again.
type Pending struct {
	Name   string    `json:"name"`
	Title  string    `json:"title"`
	Icon   string    `json:"icon"`
	Kind   string    `json:"kind"` // update (night only), rollback
	Night  bool      `json:"night"`
	Phase  string    `json:"phase"` // applying, checking
	TaskID string    `json:"task_id"`
	From   string    `json:"from"`          // the version before
	To     string    `json:"to"`            // the version applied
	Hold   string    `json:"hold"`          // rollback: who holds the version gone back from ("" none)
	Why    string    `json:"why,omitempty"` // night rollback: "failed" (the change didn't finish) or "unhealthy"
	Before []Ref     `json:"before,omitempty"`
	Start  time.Time `json:"start"`
}

// Ref is a container, the image it ran and its state, to see whether an
// update changed the app.
type Ref struct {
	ID      string `json:"id"`
	ImageID string `json:"image_id"`
	State   string `json:"state,omitempty"`
}

// ---- state ----

func (m *Manager) versionsDir() string {
	if m.Versions != "" {
		return m.Versions
	}
	if m.State == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(m.State), "app-versions")
}

// keepNext stores the installed version's file before an update (it holds
// passwords: 0600, in a 0700 folder). It becomes the previous version only
// once the update was applied (promote).
func (m *Manager) keepNext(name string, old []byte) error {
	dir := m.versionsDir()
	if dir == "" {
		return errors.New("no state folder")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return state.WriteFile(filepath.Join(dir, name+".next.yml"), old)
}

// promote makes the file keepNext stored the previous version, with after,
// the file right after the update.
func (m *Manager) promote(name string, after []byte) error {
	dir := m.versionsDir()
	if dir == "" {
		return errors.New("no state folder")
	}
	if _, err := os.Stat(filepath.Join(dir, name+".next.yml")); err != nil {
		return err
	}
	if len(after) == 0 {
		_ = os.Remove(filepath.Join(dir, name+".after.yml"))
	} else if err := state.WriteFile(filepath.Join(dir, name+".after.yml"), after); err != nil {
		return err
	}
	return os.Rename(filepath.Join(dir, name+".next.yml"), filepath.Join(dir, name+".yml"))
}

func (m *Manager) dropNext(name string) {
	if dir := m.versionsDir(); dir != "" {
		_ = os.Remove(filepath.Join(dir, name+".next.yml"))
	}
}

func (m *Manager) readFiles(name string) (old, after []byte, err error) {
	dir := m.versionsDir()
	if dir == "" {
		return nil, nil, ErrNoPrevious
	}
	if old, err = os.ReadFile(filepath.Join(dir, name+".yml")); err != nil {
		return nil, nil, ErrNoPrevious
	}
	// "after" may be missing (kept without it after a restart): nil then.
	if after, err = os.ReadFile(filepath.Join(dir, name+".after.yml")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrNoPrevious
	}
	return old, after, nil
}

// keepAfter saves the installed file as "after" for a copy kept without one.
func (m *Manager) keepAfter(ctx context.Context, name string) {
	dir := m.versionsDir()
	in, err := m.installed(ctx, name)
	if dir == "" || err != nil {
		return
	}
	if b, err := m.installedFile(ctx, name, in); err == nil && len(b) > 0 {
		_ = state.WriteFile(filepath.Join(dir, name+".after.yml"), b)
	}
}

// errNoAfter: the copy was kept without the file right after the update, so
// the settings changed since can't be told apart from the new version's.
var errNoAfter = errors.New("IceDash can't tell which settings changed since the update, so it won't go back by hand")

// dropPrev forgets the previous version (m.mu held).
func (m *Manager) dropPrev(name string) {
	f := m.load()
	delete(f.Prev, name)
	if dir := m.versionsDir(); dir != "" {
		_ = os.Remove(filepath.Join(dir, name+".yml"))
		_ = os.Remove(filepath.Join(dir, name+".after.yml"))
		_ = os.Remove(filepath.Join(dir, name+".next.yml"))
	}
}

// forget drops everything auto-update keeps for an app (m.mu held).
func (m *Manager) forget(name string, auto bool) {
	f := m.load()
	m.dropPrev(name)
	delete(f.Held, name)
	delete(f.Skip, name)
	if auto {
		f.Auto = slices.DeleteFunc(f.Auto, func(s string) bool { return s == name })
		delete(f.Tried, name)
	}
}

func (m *Manager) setPending(p *Pending) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.load()
	if p.Phase == "" {
		delete(f.Pending, p.Name)
	} else {
		cp := *p
		f.Pending[p.Name] = &cp
	}
	m.save()
}

// SetAuto turns night updates on or off for an installed IceStore app.
func (m *Manager) SetAuto(ctx context.Context, name string, on bool) error {
	in, err := m.installed(ctx, name)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(in.StoreID, catalog.IDPrefix) {
		return &InputError{errors.New("only apps from IceStore can update by themselves")}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.load()
	has := slices.Contains(f.Auto, name)
	switch {
	case on && !has:
		f.Auto = append(f.Auto, name)
		slices.Sort(f.Auto)
	case !on && has:
		f.Auto = slices.DeleteFunc(f.Auto, func(s string) bool { return s == name })
	default:
		return nil
	}
	m.save()
	return nil
}

// ---- docker ----

// installedFile is the app's file as app management keeps it (unexpanded).
// A system that can say (zima and casa can) must: its app list holds an
// expanded copy, which would compare wrongly with a kept file. Only without
// such a reader (tests) does the list's copy stand in.
func (m *Manager) installedFile(ctx context.Context, name string, in zima.Installed) ([]byte, error) {
	if r, ok := m.Zima.(interface {
		ComposeFile(ctx context.Context, name string) ([]byte, error)
	}); ok {
		b, err := r.ComposeFile(ctx, name)
		if err == nil && len(b) == 0 {
			err = errors.New("app management sent an empty file")
		}
		if err != nil {
			return nil, fmt.Errorf("the installed file can't be read: %w", err)
		}
		return b, nil
	}
	return in.Compose, nil
}

// snapshot pins each service of the installed file to the image digest it
// runs, and notes the containers.
func (m *Manager) snapshot(ctx context.Context, name string, in zima.Installed) ([]byte, []Ref, error) {
	if m.Docker == nil {
		return nil, nil, errNoCopy
	}
	raw, err := m.installedFile(ctx, name, in)
	if err != nil {
		log.Printf("[icedash] keeping %s's version: %v", name, err)
		return nil, nil, errNoCopy
	}
	d, err := compose.Parse(raw)
	if err != nil {
		return nil, nil, errNoCopy
	}
	cs, err := m.Docker.Containers(ctx, name)
	if err != nil || len(cs) == 0 {
		return nil, nil, errNoCopy
	}
	for svc, img := range d.Images() {
		if compose.Pinned(img) {
			continue
		}
		i := slices.IndexFunc(cs, func(c Container) bool { return c.Service == svc })
		if i < 0 {
			return nil, nil, errNoCopy
		}
		ds, err := m.Docker.ImageDigests(ctx, cs[i].ImageID)
		ref := compose.DigestRef(img, ds)
		if err != nil || ref == "" {
			return nil, nil, errNoCopy
		}
		d.SetImage(svc, ref)
	}
	b, err := d.Bytes()
	if err != nil {
		return nil, nil, errNoCopy
	}
	refs := []Ref{}
	for _, c := range cs {
		refs = append(refs, Ref{c.ID, c.ImageID, c.State})
	}
	return b, refs, nil
}

// changedBy reports whether a failed update changed the app: its containers
// (ID, image, state) differ from before. A stopped or broken app, or a
// one-shot that had finished, counts as unchanged when it is as it was.
// Without the containers (no Docker, or none noted before), only a new
// installed version counts as a change.
func (m *Manager) changedBy(ctx context.Context, p *Pending) bool {
	if m.Docker != nil && len(p.Before) > 0 {
		if cs, err := m.Docker.Containers(ctx, p.Name); err == nil {
			if len(cs) != len(p.Before) {
				return true
			}
			for _, c := range cs {
				if !slices.Contains(p.Before, Ref{c.ID, c.ImageID, c.State}) {
					return true
				}
			}
			return false
		}
	}
	in, err := m.installed(ctx, p.Name)
	return err == nil && in.Version != "" && in.Version != p.From
}

// pull downloads the images of the file IceDash is about to apply, so the
// update really gets a newer :latest. A failed download changes nothing.
func (m *Manager) pull(ctx context.Context, d *compose.Doc) error {
	if m.Docker == nil {
		return nil
	}
	for _, svc := range d.Services() {
		img := d.Images()[svc]
		if img == "" || strings.Contains(img, "$") || compose.Pinned(img) {
			continue
		}
		if err := m.Docker.Pull(ctx, img); err != nil {
			return err
		}
	}
	return nil
}

// ensureImages makes sure the pinned images of an older file are here,
// downloading them by digest when they are not.
func (m *Manager) ensureImages(ctx context.Context, d *compose.Doc) error {
	if m.Docker == nil {
		return nil
	}
	for _, img := range d.Images() {
		if !compose.Pinned(img) {
			continue
		}
		if ok, err := m.Docker.HasImage(ctx, img); err == nil && ok {
			continue
		}
		if err := m.Docker.Pull(ctx, img); err != nil {
			log.Printf("[icedash] go back: %v", err)
			return errGone
		}
	}
	return nil
}

// ---- health ----

// Health check timing (PLAN: 3 minutes steady, 10 minutes at most, or the
// longest start_period and 5 minutes when that is longer).
var (
	healthSteady = 3 * time.Minute
	healthMax    = 10 * time.Minute
	healthPoll   = 10 * time.Second
)

// healthy watches the app's containers until they ran steadily (every
// service running, healthy or without a healthcheck, or a one-shot that
// exited 0, with no new restarts) or it is clear they don't.
func (m *Manager) healthy(ctx context.Context, name string, services []string) bool {
	if m.Docker == nil {
		return true
	}
	steady, most, poll := healthSteady, healthMax, healthPoll
	if m.Steady > 0 {
		steady, most, poll = m.Steady, m.Steady*4, m.Steady/10
		if poll <= 0 {
			poll = time.Millisecond
		}
	}
	start := time.Now()
	base := map[string]int{}
	last := -1
	var since time.Time
	for {
		cs, err := m.Docker.Containers(ctx, name)
		restarts := 0
		good := err == nil && len(cs) > 0
		if err == nil {
			for _, c := range cs {
				if _, ok := base[c.ID]; !ok {
					base[c.ID] = c.Restarts
				}
				restarts += c.Restarts - base[c.ID]
				if sp := c.StartPeriod + 5*time.Minute; m.Steady == 0 && sp > most {
					most = sp
				}
				up := c.State == "running" && (c.Health == "" || c.Health == "healthy")
				done := c.State == "exited" && c.ExitCode == 0
				if !up && !done {
					good = false
				}
			}
			for _, s := range services {
				if !slices.ContainsFunc(cs, func(c Container) bool { return c.Service == s }) {
					good = false
				}
			}
		}
		if restarts >= 3 {
			return false
		}
		if restarts != last {
			since, last = time.Time{}, restarts
		}
		switch {
		case !good:
			since = time.Time{}
		case since.IsZero():
			since = time.Now()
		case time.Since(since) >= steady:
			return true
		}
		if time.Since(start) > most {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(poll):
		}
	}
}

// ---- jobs ----

func (m *Manager) phase(name, phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.jobs[name]; j != nil {
		j.Phase = phase
	}
}

func (m *Manager) kind(name, kind string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.jobs[name]; j != nil {
		j.Kind = kind
	}
}

func (m *Manager) end(name, errText string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.jobs[name]
	if j == nil {
		return
	}
	j.Ended = time.Now().UTC()
	if errText == "" {
		j.State = "succeeded"
	} else {
		j.State, j.Error = "failed", errText
	}
}

func (m *Manager) note(n notify.Notification) {
	if m.Notify != nil {
		_, _, _ = m.Notify.Add(n)
	}
}

// waitTask follows a task to its end (at most 30 minutes). With tries > 0 it
// gives up after that many failed reads (a task from before a restart the
// system may not know any more) and reports ok=false.
func (m *Manager) waitTask(ctx context.Context, name, id string, tries int) (zima.Task, bool) {
	poll := m.Poll
	if poll == 0 {
		poll = 2 * time.Second
	}
	if id == "" {
		return zima.Task{State: "succeeded"}, true
	}
	deadline := time.Now().Add(30 * time.Minute)
	for {
		if ctx.Err() != nil {
			return zima.Task{}, false
		}
		t, err := m.Zima.Task(ctx, id)
		if err == nil {
			m.phase(name, t.Phase)
			if t.Done() {
				return t, true
			}
		} else if tries--; tries == 0 {
			return zima.Task{}, false
		}
		if time.Now().After(deadline) {
			return zima.Task{State: "failed", Error: "it took more than 30 minutes"}, true
		}
		select {
		case <-ctx.Done():
			return zima.Task{}, false
		case <-time.After(poll):
		}
	}
}

func taskErr(t zima.Task) string {
	if t.Error != "" {
		return t.Error
	}
	return "the task stopped (" + t.State + ")"
}

// ---- update ----

// Update rebuilds an installed IceStore app with the catalog's newer file,
// keeping the env values and ports it has now. The installed version is kept
// first, so the user can go back.
func (m *Manager) Update(ctx context.Context, name string) (Job, error) {
	return m.update(ctx, name, false)
}

func (m *Manager) update(ctx context.Context, name string, night bool) (Job, error) {
	list, err := m.Zima.List(ctx)
	if err != nil {
		return Job{}, err
	}
	inst, ok := list[name]
	if !ok || !strings.HasPrefix(inst.StoreID, catalog.IDPrefix) {
		return Job{}, &InputError{errors.New("only apps from IceStore can be updated here")}
	}
	a, err := m.Catalog.App(ctx, inst.StoreID)
	if err != nil {
		return Job{}, err
	}
	doc, err := m.doc(ctx, a)
	if err != nil {
		return Job{}, err
	}
	if old, err := compose.Parse(inst.Compose); err == nil {
		doc.KeepFrom(old)
	}
	b, err := doc.Bytes()
	if err != nil {
		return Job{}, err
	}
	// The catalog's version: the one List calls latest and a hold is
	// compared with.
	to := a.Version
	if to == "" {
		to = doc.XString("version")
	}
	if !m.claim(name) {
		return Job{}, ErrBusy
	}
	old, before, snapErr := m.snapshot(ctx, name, inst)
	if snapErr == nil {
		if err := m.keepNext(name, old); err != nil {
			log.Printf("[icedash] keeping %s's version: %v", name, err)
			snapErr = errNoCopy
		}
	}
	if night && snapErr != nil {
		m.release(name)
		return Job{}, snapErr
	}
	if snapErr != nil {
		m.dropNext(name)
	}
	j := m.start(name, a.Title, a.Icon, "update", "")
	m.phase(name, "pulling_images")
	p := &Pending{Name: name, Title: a.Title, Icon: a.Icon, Kind: "update", Night: night,
		From: inst.Version, To: to, Before: before, Start: time.Now().UTC()}
	go m.runUpdate(context.WithoutCancel(ctx), p, doc, b)
	return j, nil
}

// start registers a claimed job; with a task it is tracked as an install.
func (m *Manager) start(name, title, icon, kind, task string) Job {
	m.mu.Lock()
	j := m.jobs[name]
	j.Title, j.Icon, j.Kind, j.TaskID, j.Phase = title, icon, kind, task, "accepted"
	cp := *j
	m.mu.Unlock()
	if kind == "install" {
		go m.track(name)
	}
	return cp
}

func (m *Manager) runUpdate(ctx context.Context, p *Pending, doc *compose.Doc, b []byte) {
	fail := func(err error, cant bool) {
		if p.Night {
			if cant {
				m.cantUpdate(p, p.To, err.Error())
			} else {
				m.note(notify.Notification{Kind: notify.KindAppUpdateFailed, App: p.Name,
					Key:   "app-night-failed:" + p.Name + ":" + p.To,
					Title: p.Title + " didn't update overnight",
					Body:  "You're still on " + p.From + ". IceDash tries again tomorrow night. " + err.Error()})
			}
		} else {
			m.note(notify.Notification{Kind: notify.KindAppUpdateFailed, App: p.Name, Title: p.Title + " didn't update", Body: err.Error()})
		}
		m.dropNext(p.Name)
		m.end(p.Name, err.Error())
	}
	if err := m.pull(ctx, doc); err != nil {
		if p.Night || !errors.Is(err, ErrNoDocker) {
			fail(err, false)
			return
		}
		// No Docker socket: update as before, through app management alone.
		log.Printf("[icedash] updating %s without pulling first: %v", p.Name, err)
	}
	if p.Night {
		p.Phase = "applying"
		m.setPending(p)
	}
	res, err := m.Zima.Install(ctx, b, p.Icon, false)
	if err != nil {
		if p.Night {
			p.Phase = ""
			m.setPending(p)
		}
		var ie *InputError
		fail(err, errors.Is(err, zima.ErrOldInstall) || errors.As(err, &ie))
		return
	}
	p.TaskID = res.TaskID
	m.mu.Lock()
	if j := m.jobs[p.Name]; j != nil {
		j.TaskID = res.TaskID
	}
	m.mu.Unlock()
	if p.Night {
		m.setPending(p)
	}
	t, _ := m.waitTask(ctx, p.Name, p.TaskID, 0)
	if ctx.Err() != nil {
		return // shutting down: the pending job resumes at the next start
	}
	m.applied(ctx, p, t.State == "succeeded", taskErr(t), b)
}

// applied goes on once the update's task ended (ok: it succeeded).
func (m *Manager) applied(ctx context.Context, p *Pending, ok bool, errText string, sent []byte) {
	if ctx.Err() != nil {
		return
	}
	// A failed task may still have changed the app (new containers, a new
	// version): then the kept file is the way back, for manual updates too.
	changed := !ok && m.changedBy(ctx, p)
	if ctx.Err() != nil {
		return
	}
	noAfter := false
	if ok || changed {
		after := sent // the file IceDash sent, unexpanded
		prevTo := p.To
		if in, err := m.installed(ctx, p.Name); err == nil {
			if b, err := m.installedFile(ctx, p.Name, in); err == nil {
				after = b
			} else {
				log.Printf("[icedash] after updating %s: %v", p.Name, err)
			}
			if in.Version != "" {
				prevTo = in.Version // what the system reports, which List compares
				if ok {
					p.To = in.Version
				}
			}
		}
		m.mu.Lock()
		f := m.load()
		if ok {
			delete(f.Held, p.Name)
			delete(f.Skip, p.Name)
		}
		// Without a readable "after" (a restart lost the sent file) the copy
		// is still the way back: kept without it (see runRollback).
		noAfter = len(after) == 0
		if m.promote(p.Name, after) == nil {
			f.Prev[p.Name] = Prev{From: p.From, To: prevTo}
		} else if f.Prev[p.Name] != (Prev{From: p.From, To: prevTo}) {
			// No usable copy for this change; an older one no longer applies.
			m.dropPrev(p.Name)
		}
		m.save()
		m.mu.Unlock()
	} else {
		m.dropNext(p.Name)
	}
	if !p.Night {
		if ok {
			m.note(notify.Notification{Kind: notify.KindAppUpdated, App: p.Name, Title: p.Title + " is updated", Body: p.Title + " now runs the newest version from IceStore."})
			m.end(p.Name, "")
		} else {
			m.note(notify.Notification{Kind: notify.KindAppUpdateFailed, App: p.Name, Title: p.Title + " didn't update", Body: errText})
			m.end(p.Name, errText)
		}
		return
	}
	if !ok && !changed {
		m.note(notify.Notification{Kind: notify.KindAppUpdateFailed, App: p.Name,
			Key:   "app-night-failed:" + p.Name + ":" + p.To,
			Title: p.Title + " didn't update overnight",
			Body:  "You're still on " + p.From + ". IceDash tries again tomorrow night. " + errText})
		p.Phase = ""
		m.setPending(p)
		m.end(p.Name, errText)
		return
	}
	if ok {
		p.Phase = "checking"
		m.setPending(p)
		m.phase(p.Name, "checking")
		good := m.healthy(ctx, p.Name, m.servicesOf(ctx, p.Name))
		if ctx.Err() != nil {
			return // shutting down: not a verdict on the app
		}
		if good {
			if noAfter {
				m.keepAfter(ctx, p.Name) // read it now, so a later Go back keeps settings
			}
			m.note(notify.Notification{Kind: notify.KindAppUpdated, App: p.Name,
				Title:  p.Title + " was updated overnight",
				Body:   "It now runs " + p.To + " (was " + p.From + "). If something is wrong, you can go back from its menu.",
				Action: &notify.Action{Label: "Open", Kind: "open_app"}})
			p.Phase = ""
			m.setPending(p)
			m.end(p.Name, "")
			return
		}
	}
	// Changed and not working: back to the kept version. The change reached
	// the app (its images are here), so trying it again each night would
	// repeat this: hold it.
	hold := "night"
	why := "unhealthy"
	if !ok {
		why = "failed"
	}
	r := &Pending{Name: p.Name, Title: p.Title, Icon: p.Icon, Kind: "rollback", Night: true, From: p.To, Start: time.Now().UTC(), Hold: hold, Why: why}
	m.mu.Lock()
	kept := m.load().Prev[p.Name].From
	m.mu.Unlock()
	if kept != p.From {
		// The kept file isn't the version the app came from (lost in a
		// restart): putting an older one back could do more harm.
		m.holdVersion(p.Name, p.To, hold)
		m.note(notify.Notification{Kind: notify.KindAppRollbackFailed, App: p.Name,
			Title: p.Title + " couldn't go back to " + p.From,
			Body:  "IceDash has no copy of " + p.From + " to put back. " + p.To + " won't install by itself again. Look at the app's logs."})
		p.Phase = ""
		m.setPending(p)
		m.end(p.Name, "no copy of "+p.From)
		return
	}
	r.To = kept
	m.kind(p.Name, "rollback")
	m.runRollback(ctx, r)
}

// servicesOf lists the installed app's services.
func (m *Manager) servicesOf(ctx context.Context, name string) []string {
	in, err := m.installed(ctx, name)
	if err != nil {
		return nil
	}
	raw, err := m.installedFile(ctx, name, in)
	if err != nil {
		return nil
	}
	d, err := compose.Parse(raw)
	if err != nil {
		return nil
	}
	return d.Services()
}

// cantUpdate notes, once per version, that the night run can't update the app.
func (m *Manager) cantUpdate(p *Pending, version, reason string) {
	m.note(notify.Notification{Kind: notify.KindAppUpdateFailed, App: p.Name,
		Key:   "app-auto-cant:" + p.Name + ":" + version,
		Title: p.Title + " can't update by itself", Body: reason})
}

// ---- go back ----

// Rollback puts the app's previous version back, keeping the settings
// changed since the update. It holds the version it goes back from.
func (m *Manager) Rollback(ctx context.Context, name string) (Job, error) {
	in, err := m.installed(ctx, name)
	if err != nil {
		return Job{}, err
	}
	if !strings.HasPrefix(in.StoreID, catalog.IDPrefix) {
		return Job{}, &InputError{errors.New("only apps from IceStore can go back here")}
	}
	m.mu.Lock()
	pv, ok := m.load().Prev[name]
	m.mu.Unlock()
	if !ok || pv.To != in.Version {
		return Job{}, ErrNoPrevious
	}
	if _, after, err := m.readFiles(name); err != nil {
		return Job{}, err
	} else if after == nil {
		return Job{}, &InputError{errNoAfter}
	}
	if !m.claim(name) {
		return Job{}, ErrBusy
	}
	title, icon := in.Title, ""
	if a, err := m.Catalog.App(ctx, in.StoreID); err == nil {
		icon = a.Icon
		if title == "" {
			title = a.Title
		}
	}
	if title == "" {
		title = name
	}
	j := m.start(name, title, icon, "rollback", "")
	r := &Pending{Name: name, Title: title, Icon: icon, Kind: "rollback", From: pv.To, To: pv.From, Hold: "user", Start: time.Now().UTC()}
	go m.runRollback(context.WithoutCancel(ctx), r)
	return j, nil
}

func (m *Manager) runRollback(ctx context.Context, r *Pending) {
	cant := func(err error) {
		m.note(notify.Notification{Kind: notify.KindAppRollbackFailed, App: r.Name,
			Title: r.Title + " couldn't go back to " + r.To,
			Body:  err.Error() + ". Look at the app's logs."})
		if r.Hold == "night" {
			m.holdVersion(r.Name, r.From, "night")
		}
		r.Phase = ""
		m.setPending(r)
		m.end(r.Name, err.Error())
	}
	oldB, afterB, err := m.readFiles(r.Name)
	if err != nil {
		cant(err)
		return
	}
	old, err1 := compose.Parse(oldB)
	var after *compose.Doc
	var err2 error
	switch {
	case afterB != nil:
		after, err2 = compose.Parse(afterB)
	case !r.Night:
		cant(errNoAfter)
		return
	}
	if err1 != nil || err2 != nil {
		cant(errors.New("the kept file can't be read"))
		return
	}
	// Settings changed since the update come along; if they can't be read,
	// stop rather than drop them. Only an app that is gone has none.
	// Without "after" (a night update that broke right after a restart) no
	// one changed settings since: the old file goes back as kept.
	in, err := m.installed(ctx, r.Name)
	switch {
	case ctx.Err() != nil:
		return
	case after == nil:
	case errors.Is(err, ErrNotInstalled):
	case err != nil:
		cant(fmt.Errorf("the app's current settings can't be read: %w", err))
		return
	default:
		raw, err := m.installedFile(ctx, r.Name, in)
		if err != nil {
			cant(err)
			return
		}
		now, err := compose.Parse(raw)
		if err != nil {
			cant(fmt.Errorf("the app's current settings can't be read: %w", err))
			return
		}
		old.KeepChanges(after, now)
	}
	m.phase(r.Name, "pulling_images")
	if err := m.ensureImages(ctx, old); err != nil {
		if ctx.Err() != nil {
			return
		}
		cant(err)
		return
	}
	b, err := old.Bytes()
	if err != nil {
		cant(err)
		return
	}
	if ctx.Err() != nil {
		return
	}
	r.Phase = "applying"
	m.setPending(r)
	res, err := m.Zima.Install(ctx, b, r.Icon, false)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		cant(err)
		return
	}
	r.TaskID = res.TaskID
	m.setPending(r)
	t, _ := m.waitTask(ctx, r.Name, r.TaskID, 0)
	if ctx.Err() != nil {
		return
	}
	if t.State != "succeeded" {
		cant(errors.New(strings.TrimSuffix(taskErr(t), ".")))
		return
	}
	m.rolledBack(ctx, r)
}

// rolledBack checks the app once the older file was applied and says how it went.
func (m *Manager) rolledBack(ctx context.Context, r *Pending) {
	r.Phase = "checking"
	m.setPending(r)
	m.phase(r.Name, "checking")
	ok := m.healthy(ctx, r.Name, m.servicesOf(ctx, r.Name))
	if ctx.Err() != nil {
		return // shutting down: the check runs again at the next start
	}
	m.mu.Lock()
	m.dropPrev(r.Name)
	delete(m.load().Skip, r.Name)
	m.save()
	m.mu.Unlock()
	if r.Hold != "" && (ok || r.Hold == "night") {
		m.holdVersion(r.Name, r.From, r.Hold)
	}
	held := ""
	if r.Hold == "night" {
		held = " " + r.From + " won't install by itself again."
	}
	switch {
	case !ok:
		m.note(notify.Notification{Kind: notify.KindAppRollbackFailed, App: r.Name,
			Title:  r.Title + " isn't running",
			Body:   "IceDash put " + r.To + " back, but it isn't running properly either." + held + " The app's data may have changed. Look at its logs.",
			Action: &notify.Action{Label: "Start", Kind: "start_app"}})
	case r.Night && r.Why == "failed":
		m.note(notify.Notification{Kind: notify.KindAppRolledBack, App: r.Name,
			Title:  r.Title + " went back to " + r.To,
			Body:   "The update to " + r.From + " didn't finish, so IceDash put " + r.To + " back." + held + " The app's data wasn't rolled back.",
			Action: &notify.Action{Label: "Open", Kind: "open_app"}})
	case r.Night:
		m.note(notify.Notification{Kind: notify.KindAppRolledBack, App: r.Name,
			Title:  r.Title + " went back to " + r.To,
			Body:   r.From + " didn't start properly, so IceDash put " + r.To + " back." + held + " The app's data wasn't rolled back.",
			Action: &notify.Action{Label: "Open", Kind: "open_app"}})
	default:
		m.note(notify.Notification{Kind: notify.KindAppRolledBack, App: r.Name,
			Title:  r.Title + " is back on " + r.To,
			Body:   r.From + " won't install by itself again.",
			Action: &notify.Action{Label: "Open", Kind: "open_app"}})
	}
	r.Phase = ""
	m.setPending(r)
	if ok {
		m.end(r.Name, "")
	} else {
		m.end(r.Name, r.Title+" isn't running")
	}
}

func (m *Manager) holdVersion(name, version, by string) {
	if version == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load().Held[name] = Hold{Version: version, By: by}
	m.save()
}

// ---- after a restart ----

// Resume picks up night updates and go backs a restart cut short.
func (m *Manager) Resume(ctx context.Context) {
	m.mu.Lock()
	var ps []Pending
	for _, p := range m.load().Pending {
		ps = append(ps, *p)
	}
	m.mu.Unlock()
	for _, p := range ps {
		if !m.claim(p.Name) {
			continue
		}
		m.start(p.Name, p.Title, p.Icon, p.Kind, p.TaskID)
		go m.resume(ctx, p)
	}
}

func (m *Manager) resume(ctx context.Context, p Pending) {
	// At boot app management comes up late: wait for it (up to 10 minutes)
	// before judging anything.
	for deadline := time.Now().Add(10 * time.Minute); time.Now().Before(deadline); {
		if _, err := m.Zima.List(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		if ctx.Err() != nil {
			return // shutting down: the pending job stays for the next start
		}
	}
	ok := p.Phase == "checking"
	errText := ""
	if p.Phase == "applying" {
		t, known := m.waitTask(ctx, p.Name, p.TaskID, 3)
		if ctx.Err() != nil {
			return
		}
		if known {
			ok, errText = t.State == "succeeded", taskErr(t)
		} else {
			// The system doesn't know the task any more: the version says.
			in, err := m.installed(ctx, p.Name)
			ok = err == nil && in.Version == p.To
			errText = "IceDash restarted during the change"
		}
	}
	if p.Kind == "rollback" {
		if !ok {
			m.note(notify.Notification{Kind: notify.KindAppRollbackFailed, App: p.Name,
				Title: p.Title + " couldn't go back to " + p.To, Body: errText + ". Look at the app's logs."})
			p.Phase = ""
			m.setPending(&p)
			m.end(p.Name, errText)
			return
		}
		m.rolledBack(ctx, &p)
		return
	}
	m.applied(ctx, &p, ok, errText, nil)
}

// ---- the night run ----

// NightLoop updates apps that have night updates on, between 02:00 and
// 05:00 in zone(), one at a time, at most once each a night.
func (m *Manager) NightLoop(ctx context.Context, zone func() string, now func() time.Time) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			m.NightStep(ctx, zone(), now())
		}
	}
}

// Busy reports whether any install, update or go back runs (IceDash's own
// night install waits for it).
func (m *Manager) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.State == "running" {
			return true
		}
	}
	return false
}

// NightStep is one minute of NightLoop.
func (m *Manager) NightStep(ctx context.Context, zone string, now time.Time) {
	due, night := update.NightDue(now, zone)
	if !due {
		return
	}
	if m.NightLock != nil {
		// Shared with IceDash's own night install: deciding and starting
		// happen under it, so the two never start together.
		m.NightLock.Lock()
		defer m.NightLock.Unlock()
	}
	if m.Busy() || (m.SelfBusy != nil && m.SelfBusy()) {
		return
	}
	m.mu.Lock()
	auto := slices.Clone(m.load().Auto)
	m.mu.Unlock()
	if len(auto) == 0 {
		return
	}
	if m.refreshed != night {
		if _, err := m.Catalog.Index(ctx, true); err != nil {
			return // try again next minute
		}
		m.refreshed = night
	}
	list, err := m.List(ctx, "")
	if err != nil {
		return
	}
	for _, a := range list {
		if !a.Auto || !a.Update || a.Held == a.Latest {
			continue
		}
		m.mu.Lock()
		f := m.load()
		tried := f.Tried[a.Name] == night
		if !tried {
			f.Tried[a.Name] = night
			m.save()
		}
		m.mu.Unlock()
		if tried {
			continue
		}
		if why := m.nightReady(ctx, a.Name); why != "" {
			m.skip(a.Name, why)
			continue
		}
		if m.SelfBusy != nil && m.SelfBusy() {
			return
		}
		_, err := m.update(ctx, a.Name, true)
		switch {
		case err == nil:
			return // one at a time
		case errors.Is(err, errNoCopy):
			m.skip(a.Name, SkipNoCopy)
			m.cantUpdate(&Pending{Name: a.Name, Title: m.titleOf(ctx, a.Name)}, a.Latest, err.Error()+".")
		default:
			var ie *InputError
			if errors.As(err, &ie) || errors.Is(err, zima.ErrOldInstall) {
				m.skip(a.Name, SkipCantChange)
				m.cantUpdate(&Pending{Name: a.Name, Title: m.titleOf(ctx, a.Name)}, a.Latest, err.Error())
			} else {
				log.Printf("[icedash] night update of %s: %v", a.Name, err)
			}
		}
	}
}

func (m *Manager) titleOf(ctx context.Context, name string) string {
	if in, err := m.installed(ctx, name); err == nil && in.Title != "" {
		return in.Title
	}
	return name
}

func (m *Manager) skip(name, why string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load().Skip[name] = why
	m.save()
}

// nightReady says why an app shouldn't update now ("" when it may): a
// stopped app would be started, and an unhealthy one can't be judged.
func (m *Manager) nightReady(ctx context.Context, name string) string {
	// Only an app the grid shows running: one it can't confirm may be stopped.
	grid, err := m.Zima.AppGrid(ctx)
	if err != nil || !slices.ContainsFunc(grid, func(g zima.Grid) bool { return g.Name == name && g.Status == "running" }) {
		return SkipStopped
	}
	if m.Docker == nil {
		return SkipNoCopy
	}
	cs, err := m.Docker.Containers(ctx, name)
	if err != nil || len(cs) == 0 {
		return SkipNoCopy
	}
	for _, c := range cs {
		switch {
		case c.State == "created" || c.State == "paused":
			return SkipStopped
		case c.State == "exited" && c.ExitCode == 0: // a one-shot that finished
		case c.State != "running" || c.Health == "unhealthy" || c.Health == "starting":
			return SkipUnhealthy
		}
	}
	return ""
}

// autoQuiet reports whether the app's "is in IceStore" notice for version
// stays away: the night run installs it, unless it is held or can't.
func (m *Manager) autoQuiet(name, version string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.load()
	if !slices.Contains(f.Auto, name) || f.Held[name].Version == version {
		return false
	}
	s := f.Skip[name]
	return s != SkipNoCopy && s != SkipCantChange
}
