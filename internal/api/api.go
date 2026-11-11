// Package api is IceDash's HTTP API under /v2/icedash (PLAN.md, "Backend API").
//
// Every route except /health needs a valid ZimaOS session (session.Middleware);
// routes that change something are admin-only (session.RequireAdmin) and
// same-origin (httpx.CSRF). Errors are JSON with a stable code.
package api

import (
	"net/http"
	"time"

	"github.com/chicohaager/lintux-modkit/httpx"

	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/HSinghHira/IceDash/internal/system"
)

// Route is the gateway prefix IceDash registers (measured working on 1.8.0).
const Route = "/v2/icedash"

// Server holds what the handlers need.
type Server struct {
	Version string
	Arch    string
	Started time.Time
	Verify  session.Verifier
	System  *system.Sampler
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+Route+"/health", s.health)
	mux.Handle("GET "+Route+"/system", signedIn(http.HandlerFunc(s.system)))
	// Anything else under the route still needs a session first, so an
	// unknown path tells nothing to someone who isn't signed in.
	mux.Handle(Route+"/", signedIn(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.NotFound(w)
	})))
	return httpx.CSRF(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, Health{Version: s.Version, Arch: s.Arch, Started: s.Started})
}

func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.System.Snapshot())
}
