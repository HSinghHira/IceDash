package api

// Phase 2 routes: the IceStore catalog, installs and the installed apps
// (PLAN.md, Phase 2, "Backend API"). Reading is for every signed-in user;
// changes, logs, credentials and backups are admin only.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/chicohaager/lintux-modkit/httpx"

	"github.com/HSinghHira/IceDash/internal/apps"
	"github.com/HSinghHira/IceDash/internal/catalog"
	"github.com/HSinghHira/IceDash/internal/compose"
	"github.com/HSinghHira/IceDash/internal/creds"
	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/HSinghHira/IceDash/internal/zima"
)

func (s *Server) appRoutes(mux *http.ServeMux, user, admin func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET "+Route+"/catalog", user(s.catalogIndex))
	mux.Handle("POST "+Route+"/catalog/refresh", admin(s.catalogRefresh))
	mux.Handle("GET "+Route+"/catalog/{id}", user(s.catalogApp))
	mux.Handle("GET "+Route+"/catalog/{id}/setup", admin(s.catalogSetup))
	mux.Handle("POST "+Route+"/catalog/{id}/check", admin(s.catalogCheck))
	mux.Handle("POST "+Route+"/catalog/{id}/install", admin(s.catalogInstall))

	mux.Handle("GET "+Route+"/jobs", user(s.jobs))
	mux.Handle("GET "+Route+"/apps", user(s.appList))
	mux.Handle("POST "+Route+"/apps/{name}/start", admin(s.appAction))
	mux.Handle("POST "+Route+"/apps/{name}/stop", admin(s.appAction))
	mux.Handle("POST "+Route+"/apps/{name}/restart", admin(s.appAction))
	mux.Handle("POST "+Route+"/apps/{name}/update", admin(s.appUpdate))
	mux.Handle("POST "+Route+"/apps/{name}/uninstall", admin(s.appUninstall))
	mux.Handle("GET "+Route+"/apps/{name}/logs", admin(s.appLogs))
	mux.Handle("GET "+Route+"/apps/{name}/credentials", admin(s.appCredentials))
	mux.Handle("GET "+Route+"/apps/{name}/backup", admin(s.appBackup))
	mux.Handle("GET "+Route+"/apps/{name}/frame", user(s.appFrame))
	mux.Handle("GET "+Route+"/apps/{name}/first-launch", user(s.firstLaunch))
	mux.Handle("POST "+Route+"/apps/{name}/first-launch/dismiss", user(s.firstLaunchDismiss))
}

// withToken passes the user's ZimaOS token on to app management.
func withToken(r *http.Request) context.Context {
	return zima.WithToken(r.Context(), session.Token(r))
}

func (s *Server) appsReady(w http.ResponseWriter) bool {
	if s.Apps == nil || s.Catalog == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "not_ready", "app handling isn't available")
		return false
	}
	return true
}

// writeAppErr maps the app errors to answers the UI can word.
func writeAppErr(w http.ResponseWriter, err error) {
	var ie *apps.InputError
	var ze *zima.Error
	switch {
	case errors.As(err, &ie):
		httpx.WriteError(w, http.StatusBadRequest, "bad_input", ie.Error())
	case errors.Is(err, catalog.ErrUnknownApp):
		httpx.WriteError(w, http.StatusNotFound, "unknown_app", err.Error())
	case errors.Is(err, apps.ErrNotInstalled):
		httpx.WriteError(w, http.StatusNotFound, "not_installed", err.Error())
	case errors.Is(err, apps.ErrBusy):
		httpx.WriteError(w, http.StatusConflict, "busy", err.Error())
	case errors.Is(err, creds.ErrNone):
		httpx.WriteError(w, http.StatusNotFound, "no_credentials", err.Error())
	case errors.Is(err, creds.ErrNotRunning):
		httpx.WriteError(w, http.StatusConflict, "not_running", err.Error())
	case errors.Is(err, apps.ErrNoData):
		httpx.WriteError(w, http.StatusNotFound, "no_data", err.Error())
	case errors.As(err, &ze):
		httpx.WriteError(w, http.StatusBadGateway, "zimaos", ze.Error())
	default:
		httpx.WriteError(w, http.StatusBadGateway, "upstream", err.Error())
	}
}

// CatalogList is the answer of GET /catalog.
type CatalogList struct {
	UpdatedAt time.Time     `json:"updated_at"`
	FetchedAt time.Time     `json:"fetched_at"`
	Apps      []catalog.App `json:"apps"`
	// Error says the catalog couldn't be fetched; Apps may be an older copy.
	Error string `json:"error,omitempty"`
}

func (s *Server) catalogIndex(w http.ResponseWriter, r *http.Request) { s.writeCatalog(w, r, false) }

func (s *Server) catalogRefresh(w http.ResponseWriter, r *http.Request) {
	s.writeCatalog(w, r, true)
}

func (s *Server) writeCatalog(w http.ResponseWriter, r *http.Request, force bool) {
	if !s.appsReady(w) {
		return
	}
	idx, err := s.Catalog.Index(r.Context(), force)
	if err != nil && len(idx.Apps) == 0 {
		writeAppErr(w, err)
		return
	}
	out := CatalogList{UpdatedAt: idx.UpdatedAt, FetchedAt: idx.FetchedAt, Apps: idx.Apps}
	if out.Apps == nil {
		out.Apps = []catalog.App{}
	}
	if err != nil {
		out.Error = err.Error()
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// AppPage is the answer of GET /catalog/{id}.
type AppPage struct {
	App  catalog.App  `json:"app"`
	Meta catalog.Meta `json:"meta"`
}

func (s *Server) catalogApp(w http.ResponseWriter, r *http.Request) {
	if !s.appsReady(w) {
		return
	}
	a, err := s.Catalog.App(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAppErr(w, err)
		return
	}
	m, err := s.Catalog.Meta(r.Context(), a.ID)
	if err != nil {
		writeAppErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, AppPage{App: a, Meta: m})
}

func (s *Server) catalogSetup(w http.ResponseWriter, r *http.Request) {
	if !s.appsReady(w) {
		return
	}
	c := compose.Context{Host: compose.HostOnly(r.Host)}
	if s.Email != nil {
		c.Email = s.Email(r.Context(), session.Token(r))
	}
	v, err := s.Apps.Setup(withToken(r), r.PathValue("id"), c)
	if err != nil {
		writeAppErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store") // generated secrets
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (s *Server) catalogCheck(w http.ResponseWriter, r *http.Request) {
	if !s.appsReady(w) {
		return
	}
	var ch compose.Choices
	if !httpx.Decode(w, r, &ch) {
		return
	}
	res, err := s.Apps.Check(withToken(r), r.PathValue("id"), ch)
	if err != nil {
		writeAppErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (s *Server) catalogInstall(w http.ResponseWriter, r *http.Request) {
	if !s.appsReady(w) {
		return
	}
	var ch compose.Choices
	if !httpx.Decode(w, r, &ch) {
		return
	}
	j, err := s.Apps.Install(withToken(r), r.PathValue("id"), ch)
	if err != nil {
		writeAppErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, j)
}

func (s *Server) jobs(w http.ResponseWriter, r *http.Request) {
	if !s.appsReady(w) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": s.Apps.Jobs()})
}

func (s *Server) appList(w http.ResponseWriter, r *http.Request) {
	if !s.appsReady(w) {
		return
	}
	list, err := s.Apps.List(withToken(r), username(r))
	if err != nil {
		writeAppErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

// appName reads and checks the {name} path value.
func appName(w http.ResponseWriter, r *http.Request) (string, bool) {
	n := r.PathValue("name")
	if !catalog.ValidName(n) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_name", "that isn't an app name")
		return "", false
	}
	return n, true
}

func (s *Server) appAction(w http.ResponseWriter, r *http.Request) {
	name, ok := appName(w, r)
	if !ok || !s.appsReady(w) {
		return
	}
	ctx := withToken(r)
	var err error
	switch lastSegment(r.URL.Path) {
	case "start":
		err = s.Apps.Start(ctx, name)
	case "stop":
		err = s.Apps.Stop(ctx, name)
	case "restart":
		err = s.Apps.Restart(ctx, name)
	default:
		httpx.NotFound(w)
		return
	}
	if err != nil {
		writeAppErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"name": name})
}

func lastSegment(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

func (s *Server) appUpdate(w http.ResponseWriter, r *http.Request) {
	name, ok := appName(w, r)
	if !ok || !s.appsReady(w) {
		return
	}
	j, err := s.Apps.Update(withToken(r), name)
	if err != nil {
		writeAppErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, j)
}

func (s *Server) appUninstall(w http.ResponseWriter, r *http.Request) {
	name, ok := appName(w, r)
	if !ok || !s.appsReady(w) {
		return
	}
	var body struct {
		DeleteData bool `json:"delete_data"`
	}
	if r.ContentLength != 0 && !httpx.Decode(w, r, &body) {
		return
	}
	if err := s.Apps.Uninstall(withToken(r), name, body.DeleteData); err != nil {
		writeAppErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"name": name, "data_kept": !body.DeleteData, "data": s.Apps.DataDir(name)})
}

func (s *Server) appLogs(w http.ResponseWriter, r *http.Request) {
	name, ok := appName(w, r)
	if !ok || !s.appsReady(w) {
		return
	}
	lines := 500
	if v := r.URL.Query().Get("lines"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 5000 {
			httpx.WriteError(w, http.StatusBadRequest, "bad_lines", "lines must be 1 to 5000")
			return
		}
		lines = n
	}
	text, err := s.Apps.Logs(withToken(r), name, lines)
	if err != nil {
		writeAppErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"name": name, "text": text})
}

func (s *Server) appCredentials(w http.ResponseWriter, r *http.Request) {
	name, ok := appName(w, r)
	if !ok || !s.appsReady(w) {
		return
	}
	c, err := s.Apps.Credentials(withToken(r), name)
	if err != nil {
		writeAppErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, c)
}

func (s *Server) appBackup(w http.ResponseWriter, r *http.Request) {
	name, ok := appName(w, r)
	if !ok || !s.appsReady(w) {
		return
	}
	if !s.Apps.HasData(name) {
		writeAppErr(w, apps.ErrNoData)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.tar.gz"`, name, time.Now().UTC().Format("20060102-1504")))
	w.Header().Set("Cache-Control", "no-store")
	// Headers are out once the archive starts; a failure then cuts it short,
	// which the browser shows as a failed download.
	_ = s.Apps.Backup(withToken(r), name, w)
}

func (s *Server) firstLaunch(w http.ResponseWriter, r *http.Request) {
	name, ok := appName(w, r)
	if !ok || !s.appsReady(w) {
		return
	}
	c, _ := session.FromContext(r.Context())
	fl, err := s.Apps.FirstLaunchFor(withToken(r), name, c.Admin())
	if err != nil {
		writeAppErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, fl)
}

func (s *Server) firstLaunchDismiss(w http.ResponseWriter, r *http.Request) {
	name, ok := appName(w, r)
	if !ok || !s.appsReady(w) {
		return
	}
	s.Apps.Dismiss(username(r), name)
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"name": name})
}

func (s *Server) appFrame(w http.ResponseWriter, r *http.Request) {
	name, ok := appName(w, r)
	if !ok || !s.appsReady(w) {
		return
	}
	f, err := s.Apps.Frameable(withToken(r), name)
	if err != nil {
		writeAppErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, f)
}
