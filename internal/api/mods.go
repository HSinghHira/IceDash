package api

// Phase 4 routes: themes and widgets from collections (PLAN.md, Phase 4).
// Everyone signed in chooses their own theme and panel; adding collections
// and installing, removing and switching mods is admin only. A widget's own
// files and a theme's wallpaper need no sign-in: an iframe or a CSS
// background can't carry one, and they hold nothing private.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/chicohaager/lintux-modkit/httpx"

	"github.com/HSinghHira/IceDash/internal/mods"
	"github.com/HSinghHira/IceDash/internal/session"
)

func (s *Server) modRoutes(mux *http.ServeMux, user, admin func(http.HandlerFunc) http.Handler) {
	if s.Mods == nil {
		return
	}
	mux.Handle("GET "+Route+"/mods", user(s.modsList))
	mux.Handle("GET "+Route+"/mods/collections", user(s.modsCollections))
	mux.Handle("POST "+Route+"/mods/collections", admin(s.modsAddCollection))
	mux.Handle("POST "+Route+"/mods/collections/remove", admin(s.modsRemoveCollection))
	mux.Handle("POST "+Route+"/mods/install", admin(s.modsInstall))
	mux.Handle("POST "+Route+"/mods/theme", user(s.modsTheme))
	mux.Handle("POST "+Route+"/mods/panel", user(s.modsPanel))
	mux.Handle("POST "+Route+"/mods/{id}/remove", admin(s.modsRemove))
	mux.Handle("POST "+Route+"/mods/{id}/enabled", admin(s.modsEnabled))
	mux.HandleFunc("GET "+Route+"/mods/{id}/files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		s.Mods.ServeFile(w, r, r.PathValue("id"), r.PathValue("path"))
	})
	mux.HandleFunc("GET "+Route+"/mods/{id}/wallpaper", func(w http.ResponseWriter, r *http.Request) {
		s.Mods.ServeWallpaper(w, r, r.PathValue("id"))
	})
}

func writeModErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mods.ErrInvalid), errors.Is(err, mods.ErrBadAddress), errors.Is(err, mods.ErrEmpty), errors.Is(err, mods.ErrOfficial):
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
	case errors.Is(err, mods.ErrNotFound), errors.Is(err, mods.ErrNoRepo), errors.Is(err, mods.ErrNotWidget):
		httpx.WriteError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, mods.ErrTampered):
		httpx.WriteError(w, http.StatusBadGateway, "tampered", err.Error())
	case errors.Is(err, mods.ErrUnreachable):
		httpx.WriteError(w, http.StatusBadGateway, "unreachable", err.Error())
	default:
		if err != nil && err.Error() != "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
		httpx.WriteErr(w, err)
	}
}

func (s *Server) modsList(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.Mods.List(username(r)))
}

func (s *Server) modsCollections(w http.ResponseWriter, r *http.Request) {
	c, _ := session.FromContext(r.Context())
	refresh := r.URL.Query().Get("refresh") == "1" && c.Admin()
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"collections": s.Mods.Collections(ctx, refresh)})
}

type sourceBody struct {
	Source string `json:"source"`
}

func (s *Server) modsAddCollection(w http.ResponseWriter, r *http.Request) {
	var body sourceBody
	if !httpx.Decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	c, err := s.Mods.AddCollection(ctx, body.Source)
	if err != nil {
		writeModErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, c)
}

func (s *Server) modsRemoveCollection(w http.ResponseWriter, r *http.Request) {
	var body sourceBody
	if !httpx.Decode(w, r, &body) {
		return
	}
	if err := s.Mods.RemoveCollection(body.Source); err != nil {
		writeModErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"removed": true})
}

func (s *Server) modsInstall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	in, err := s.Mods.Install(ctx, body.ID)
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
