package api

// Phase 4 routes: themes and widgets (PLAN.md, Phase 4, "Backend API").
// Everyone signed in chooses their own theme and panel; installing,
// removing and switching widgets is admin only. A widget's own files need no
// sign-in: its sandboxed iframe can't carry one, and they hold nothing private.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/chicohaager/lintux-modkit/httpx"

	"github.com/HSinghHira/IceDash/internal/mods"
)

func (s *Server) modRoutes(mux *http.ServeMux, user, admin func(http.HandlerFunc) http.Handler) {
	if s.Mods == nil {
		return
	}
	mux.Handle("GET "+Route+"/mods", user(s.modsList))
	mux.Handle("GET "+Route+"/mods/catalog", user(s.modsCatalog))
	mux.Handle("POST "+Route+"/mods/catalog/refresh", admin(s.modsCatalogRefresh))
	mux.Handle("POST "+Route+"/mods/inspect", admin(s.modsInspect))
	mux.Handle("POST "+Route+"/mods/install", admin(s.modsInstall))
	mux.Handle("POST "+Route+"/mods/theme", user(s.modsTheme))
	mux.Handle("POST "+Route+"/mods/panel", user(s.modsPanel))
	mux.Handle("POST "+Route+"/mods/{id}/remove", admin(s.modsRemove))
	mux.Handle("POST "+Route+"/mods/{id}/enabled", admin(s.modsEnabled))
	mux.HandleFunc("GET "+Route+"/mods/{id}/files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		s.Mods.ServeFile(w, r, r.PathValue("id"), r.PathValue("path"))
	})
}

func writeModErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mods.ErrInvalid), errors.Is(err, mods.ErrBadAddress):
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
	case errors.Is(err, mods.ErrNotFound), errors.Is(err, mods.ErrNoRelease), errors.Is(err, mods.ErrNotWidget):
		httpx.WriteError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, mods.ErrChecksum):
		httpx.WriteError(w, http.StatusBadGateway, "checksum", err.Error())
	case errors.Is(err, mods.ErrUnreachable):
		httpx.WriteError(w, http.StatusBadGateway, "unreachable", err.Error())
	default:
		httpx.WriteErr(w, err)
	}
}

func (s *Server) modsList(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.Mods.List(username(r)))
}

func (s *Server) modsCatalog(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	c, err := s.Mods.Catalog(ctx, false)
	resp := map[string]any{"mods": c.Mods}
	if err != nil {
		resp["error"] = err.Error() // the cached copy, if any, is still shown
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) modsCatalogRefresh(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	c, err := s.Mods.Catalog(ctx, true)
	if err != nil {
		writeModErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"mods": c.Mods})
}

type sourceBody struct {
	Source string `json:"source"`
}

func (s *Server) modsInspect(w http.ResponseWriter, r *http.Request) {
	var body sourceBody
	if !httpx.Decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	o, err := s.Mods.Inspect(ctx, body.Source)
	if err != nil {
		writeModErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

func (s *Server) modsInstall(w http.ResponseWriter, r *http.Request) {
	var body sourceBody
	if !httpx.Decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	in, err := s.Mods.Install(ctx, body.Source)
	if err != nil {
		writeModErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, in)
}

func (s *Server) modsRemove(w http.ResponseWriter, r *http.Request) {
	if err := s.Mods.Remove(r.PathValue("id")); err != nil {
		writeModErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s.Mods.List(username(r)))
}

func (s *Server) modsEnabled(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if err := s.Mods.SetEnabled(r.PathValue("id"), body.Enabled); err != nil {
		writeModErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s.Mods.List(username(r)))
}

func (s *Server) modsTheme(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if err := s.Mods.SetTheme(username(r), body.ID); err != nil {
		writeModErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s.Mods.List(username(r)))
}

func (s *Server) modsPanel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if err := s.Mods.SetPanel(username(r), body.IDs); err != nil {
		writeModErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s.Mods.List(username(r)))
}
