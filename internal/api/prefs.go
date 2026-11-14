package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/HSinghHira/IceDash/internal/prefs"
	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/chicohaager/lintux-modkit/httpx"
)

// Account is one of the system's users, for the Users page (PLAN.md 5a.5).
type Account struct {
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
}

// prefsRoutes are Phase 5a's: each user's own choices, and the accounts.
func (s *Server) prefsRoutes(mux *http.ServeMux, user func(http.HandlerFunc) http.Handler) {
	if s.Prefs != nil {
		mux.Handle("GET "+Route+"/prefs", user(s.getPrefs))
		mux.Handle("POST "+Route+"/prefs", user(s.setPrefs))
	}
	if s.Users != nil {
		mux.Handle("GET "+Route+"/users", user(s.users))
	}
}

func (s *Server) getPrefs(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.Prefs.Get(username(r)))
}

// setPrefs takes the user's whole set of choices, or part of it: what isn't
// sent keeps its saved value.
func (s *Server) setPrefs(w http.ResponseWriter, r *http.Request) {
	p := s.Prefs.Get(username(r))
	if !httpx.Decode(w, r, &p) {
		return
	}
	got, err := s.Prefs.Set(username(r), p)
	if errors.Is(err, prefs.ErrInvalid) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, got)
}

func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	list, err := s.Users(r.Context(), session.Token(r))
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "unreachable", "the list of users can't be read right now")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

// UsersFunc reads the system's accounts with the signed-in user's token.
type UsersFunc func(ctx context.Context, token string) ([]Account, error)
