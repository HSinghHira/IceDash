// Package api is IceDash's HTTP API under /v2/icedash (PLAN.md, "Backend API").
//
// Every route except /health needs a valid ZimaOS session (session.Middleware);
// routes that change the box are admin-only (session.RequireAdmin), and every
// POST must be same-origin (httpx.CSRF). Errors are JSON with a stable code.
package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/chicohaager/lintux-modkit/httpx"

	"github.com/HSinghHira/IceDash/internal/notify"
	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/HSinghHira/IceDash/internal/settings"
	"github.com/HSinghHira/IceDash/internal/system"
	"github.com/HSinghHira/IceDash/internal/update"
)

// Route is the gateway prefix IceDash registers (measured working on 1.8.0).
const Route = "/v2/icedash"

// Updater is the self-updater (update.Manager).
type Updater interface {
	Status() update.Status
	Check(ctx context.Context) update.Status
	Apply() error
}

// Notes is the notification store (notify.Store).
type Notes interface {
	List(user string, since int64) ([]notify.View, int)
	MarkRead(user string, id int64) error
	Clear(user string) error
}

// Settings is the settings store (settings.Store).
type Settings interface {
	Get() settings.Settings
	Set(settings.Settings) error
}

// Server holds what the handlers need.
type Server struct {
	Version  string
	Arch     string
	Started  time.Time
	Verify   session.Verifier
	System   *system.Sampler
	Update   Updater
	Notes    Notes
	Settings Settings
}

// Health is the answer of GET /v2/icedash/health. The updater and the UI poll
// it to see which version runs.
type Health struct {
	Version string    `json:"version"`
	Arch    string    `json:"arch"`
	Started time.Time `json:"started"`
}

// Handler returns the API with its checks in place.
func (s *Server) Handler() http.Handler {
	signedIn := session.Middleware(s.Verify)
	user := func(h http.HandlerFunc) http.Handler { return signedIn(h) }
	admin := func(h http.HandlerFunc) http.Handler { return signedIn(session.RequireAdmin(h)) }

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+Route+"/health", s.health)
	mux.Handle("GET "+Route+"/system", user(s.system))

	mux.Handle("GET "+Route+"/notifications", user(s.notifications))
	mux.Handle("POST "+Route+"/notifications/read", user(s.markRead))
	mux.Handle("POST "+Route+"/notifications/clear", user(s.clear))

	mux.Handle("GET "+Route+"/update", user(s.updateStatus))
	mux.Handle("POST "+Route+"/update/check", admin(s.updateCheck))
	mux.Handle("POST "+Route+"/update/apply", admin(s.updateApply))

	mux.Handle("GET "+Route+"/settings", user(s.getSettings))
	mux.Handle("POST "+Route+"/settings", admin(s.setSettings))

	// Anything else under the route still needs a session first, so an
	// unknown path tells nothing to someone who isn't signed in.
	mux.Handle(Route+"/", user(func(w http.ResponseWriter, r *http.Request) { httpx.NotFound(w) }))
	return httpx.CSRF(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, Health{Version: s.Version, Arch: s.Arch, Started: s.Started})
}

func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.System.Snapshot())
}

// NotificationList is the answer of GET /v2/icedash/notifications.
type NotificationList struct {
	Items  []notify.View `json:"items"`
	Unread int           `json:"unread"`
}

func username(r *http.Request) string {
	c, _ := session.FromContext(r.Context())
	return c.Username
}

func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	var since int64
	if v := r.URL.Query().Get("since"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			httpx.WriteError(w, http.StatusBadRequest, "bad_since", "since must be a notification id")
			return
		}
		since = n
	}
	items, unread := s.Notes.List(username(r), since)
	httpx.WriteJSON(w, http.StatusOK, NotificationList{Items: items, Unread: unread})
}

func (s *Server) markRead(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID int64 `json:"id"` // 0 or missing: all
	}
	if r.ContentLength != 0 && !httpx.Decode(w, r, &body) {
		return
	}
	if err := s.Notes.MarkRead(username(r), body.ID); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	s.notifications(w, r)
}

func (s *Server) clear(w http.ResponseWriter, r *http.Request) {
	if err := s.Notes.Clear(username(r)); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, NotificationList{Items: []notify.View{}})
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.Update.Status())
}

func (s *Server) updateCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	httpx.WriteJSON(w, http.StatusOK, s.Update.Check(ctx))
}

func (s *Server) updateApply(w http.ResponseWriter, r *http.Request) {
	switch err := s.Update.Apply(); {
	case errors.Is(err, update.ErrBusy):
		httpx.WriteError(w, http.StatusConflict, "busy", err.Error())
	case errors.Is(err, update.ErrNothingToDo):
		httpx.WriteError(w, http.StatusConflict, "nothing_to_install", err.Error())
	case err != nil:
		httpx.WriteErr(w, err)
	default:
		httpx.WriteJSON(w, http.StatusAccepted, s.Update.Status())
	}
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.Settings.Get())
}

func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	v := s.Settings.Get()
	if !httpx.Decode(w, r, &v) {
		return
	}
	if err := s.Settings.Set(v); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}
