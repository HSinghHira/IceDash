package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/HSinghHira/IceDash/internal/media"
	"github.com/HSinghHira/IceDash/internal/prefs"
	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/chicohaager/lintux-modkit/httpx"
)

// Account is one of the system's users, for the Users page (PLAN.md 5a.5).
type Account struct {
	Name   string `json:"name"`
	Admin  bool   `json:"admin"`
	Avatar bool   `json:"avatar,omitempty"` // has a picture of their own
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
	if s.Media != nil && s.Prefs != nil {
		for _, k := range []media.Kind{media.Wallpaper, media.Avatar} {
			mux.Handle("GET "+Route+"/me/"+string(k), user(func(w http.ResponseWriter, r *http.Request) { s.servePicture(w, r, username(r), k) }))
			mux.Handle("POST "+Route+"/me/"+string(k), user(func(w http.ResponseWriter, r *http.Request) { s.putPicture(w, r, k) }))
			mux.Handle("POST "+Route+"/me/"+string(k)+"/remove", user(func(w http.ResponseWriter, r *http.Request) { s.dropPicture(w, r, k) }))
		}
		mux.Handle("GET "+Route+"/users/{name}/avatar", user(func(w http.ResponseWriter, r *http.Request) {
			s.servePicture(w, r, r.PathValue("name"), media.Avatar)
		}))
	}
}

// servePicture sends a user's wallpaper or avatar. Everyone signed in may see
// avatars (the Users page shows them); a wallpaper only its owner asks for.
func (s *Server) servePicture(w http.ResponseWriter, r *http.Request, who string, k media.Kind) {
	b, typ, mod, err := s.Media.Get(who, k)
	if err != nil {
		httpx.NotFound(w)
		return
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, "", mod, bytes.NewReader(b))
}

// putPicture keeps the body as the user's picture and points their choices at it.
func (s *Server) putPicture(w http.ResponseWriter, r *http.Request, k media.Kind) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, media.Max(k)+1))
	if err != nil || int64(len(b)) > media.Max(k) {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "too_big", media.ErrTooBig.Error())
		return
	}
	if err := s.Media.Put(username(r), k, b); err != nil {
		if errors.Is(err, media.ErrNotImage) {
			httpx.WriteError(w, http.StatusBadRequest, "not_image", err.Error())
			return
		}
		httpx.WriteErr(w, err)
		return
	}
	rev := time.Now().UnixMilli()
	got, err := s.Prefs.Update(username(r), func(p *prefs.Prefs) {
		if k == media.Avatar {
			p.Avatar = rev
		} else {
			p.Wallpaper.Kind, p.Wallpaper.Rev = "image", rev
		}
	})
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, got)
}

func (s *Server) dropPicture(w http.ResponseWriter, r *http.Request, k media.Kind) {
	if err := s.Media.Delete(username(r), k); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	got, err := s.Prefs.Update(username(r), func(p *prefs.Prefs) {
		if k == media.Avatar {
			p.Avatar = 0
			return
		}
		p.Wallpaper.Rev = 0
		if p.Wallpaper.Kind == "image" {
			p.Wallpaper.Kind = ""
		}
	})
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, got)
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
	if s.Media != nil {
		for i := range list {
			list[i].Avatar = s.Media.Has(list[i].Name, media.Avatar)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

// UsersFunc reads the system's accounts with the signed-in user's token.
type UsersFunc func(ctx context.Context, token string) ([]Account, error)
