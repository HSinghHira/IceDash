package api

// Phase 3c routes: the Files app and the Terminal (PLAN.md, Phase 3c,
// "Backend API"). All admin only: on ZimaOS a member gets 403; on CasaOS the
// one account is the admin.

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/chicohaager/lintux-modkit/httpx"

	"github.com/HSinghHira/IceDash/internal/files"
	"github.com/HSinghHira/IceDash/internal/session"
	"github.com/HSinghHira/IceDash/internal/term"
)

// UploadPath is the one route whose body may be larger than 1 MB.
const UploadPath = Route + "/files/upload"

// TerminalPath is the Terminal's WebSocket; its ticket is its sign-in.
const TerminalPath = Route + "/terminal"

func (s *Server) fileRoutes(mux *http.ServeMux, admin func(http.HandlerFunc) http.Handler) {
	if s.Files != nil {
		mux.Handle("GET "+Route+"/files/list", admin(s.filesList))
		mux.Handle("POST "+Route+"/files/ticket", admin(s.filesTicket))
		mux.HandleFunc("GET "+Route+"/files/raw", s.filesRaw) // the ticket is the sign-in
		mux.HandleFunc("GET "+Route+"/files/zip", s.filesZip)
		mux.Handle("POST "+UploadPath, admin(s.filesUpload))
		mux.Handle("POST "+Route+"/files/folder", admin(s.filesFolder))
		mux.Handle("POST "+Route+"/files/rename", admin(s.filesRename))
		mux.Handle("POST "+Route+"/files/move", admin(s.filesTransfer("move")))
		mux.Handle("POST "+Route+"/files/copy", admin(s.filesTransfer("copy")))
		mux.Handle("POST "+Route+"/files/delete", admin(s.filesDelete))
		mux.Handle("GET "+Route+"/files/jobs", admin(s.filesJobs))
		mux.Handle("POST "+Route+"/files/jobs/{id}/cancel", admin(s.filesCancel))
		mux.Handle("GET "+Route+"/files/trash", admin(s.trashList))
		mux.Handle("POST "+Route+"/files/trash/restore", admin(s.trashRestore))
		mux.Handle("POST "+Route+"/files/trash/delete", admin(s.trashDelete))
		mux.Handle("POST "+Route+"/files/trash/empty", admin(s.trashEmpty))
	}
	if s.Term != nil {
		mux.Handle("POST "+Route+"/terminal/ticket", admin(s.terminalTicket))
		mux.Handle("GET "+TerminalPath, s.Term)
	}
}

func writeFileErr(w http.ResponseWriter, err error) {
	var ce *files.ClashError
	switch {
	case errors.As(err, &ce):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{"code": "exists", "error": err.Error(), "names": ce.Names})
	case errors.Is(err, files.ErrExists):
		httpx.WriteError(w, http.StatusConflict, "exists", err.Error())
	case errors.Is(err, files.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, files.ErrProtected):
		httpx.WriteError(w, http.StatusForbidden, "protected", err.Error())
	case errors.Is(err, files.ErrBadName), errors.Is(err, files.ErrBadPath), errors.Is(err, files.ErrIsDir),
		errors.Is(err, files.ErrNotDir), errors.Is(err, files.ErrIntoSelf):
		httpx.WriteError(w, http.StatusBadRequest, "bad_path", err.Error())
	case errors.Is(err, files.ErrTicket), errors.Is(err, term.ErrTicket):
		httpx.WriteError(w, http.StatusUnauthorized, "ticket", err.Error())
	case errors.Is(err, term.ErrPassword):
		httpx.WriteError(w, http.StatusUnauthorized, "wrong_password", err.Error())
	case errors.Is(err, term.ErrLocked):
		httpx.WriteError(w, http.StatusTooManyRequests, "locked", err.Error())
	default:
		httpx.WriteErr(w, err)
	}
}

func (s *Server) filesList(w http.ResponseWriter, r *http.Request) {
	l, err := s.Files.List(r.URL.Query().Get("path"), r.URL.Query().Get("hidden") == "1")
	if err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, l)
}

func (s *Server) filesTicket(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Paths []string `json:"paths"`
		Zip   bool     `json:"zip"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	id, err := s.Files.Ticket(body.Paths, body.Zip)
	if err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"ticket": id})
}

// attachment names a download, safely for any file name.
func attachment(name string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(name))
}

// filesRaw serves one file for a ticket: inline for previews, or as a
// download with ?download=1. Ranges work, so videos seek and big downloads
// resume.
func (s *Server) filesRaw(w http.ResponseWriter, r *http.Request) {
	paths, err := s.Files.Redeem(r.URL.Query().Get("ticket"), false)
	if err != nil {
		writeFileErr(w, err)
		return
	}
	f, info, err := s.Files.Open(paths[0])
	if err != nil {
		writeFileErr(w, err)
		return
	}
	defer f.Close()
	name := path.Base(paths[0])
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, no-store")
	// Nothing served from /DATA may run in IceDash's page.
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'; sandbox")
	if r.URL.Query().Get("download") == "1" {
		h.Set("Content-Type", "application/octet-stream")
		h.Set("Content-Disposition", attachment(name))
	} else {
		h.Set("Content-Type", files.ContentType(name))
		if files.KindOf(name, false) == "pdf" {
			// The browser's PDF viewer doesn't work in a sandbox; a PDF runs no page scripts.
			h.Set("Content-Security-Policy", "default-src 'none'; object-src 'self'; plugin-types application/pdf")
		}
	}
	http.ServeContent(w, r, name, info.ModTime(), f)
}

func (s *Server) filesZip(w http.ResponseWriter, r *http.Request) {
	paths, err := s.Files.Redeem(r.URL.Query().Get("ticket"), true)
	if err != nil {
		writeFileErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", attachment(files.ZipName(paths)))
	w.Header().Set("Cache-Control", "no-store")
	// Headers are out once the archive starts; a failure then cuts it short.
	_ = s.Files.WriteZip(w, paths)
}

func (s *Server) filesUpload(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	e, skipped, err := s.Files.Upload(q.Get("path"), q.Get("name"), files.Clash(q.Get("clash")), r.Body)
	if err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"entry": e, "skipped": skipped})
}

func (s *Server) filesFolder(w http.ResponseWriter, r *http.Request) {
	var body struct{ Path, Name string }
	if !httpx.Decode(w, r, &body) {
		return
	}
	e, err := s.Files.Mkdir(body.Path, body.Name)
	if err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, e)
}

func (s *Server) filesRename(w http.ResponseWriter, r *http.Request) {
	var body struct{ Path, Name string }
	if !httpx.Decode(w, r, &body) {
		return
	}
	e, err := s.Files.Rename(body.Path, body.Name)
	if err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, e)
}

func (s *Server) filesTransfer(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Paths []string `json:"paths"`
			To    string   `json:"to"`
			Clash string   `json:"clash"`
		}
		if !httpx.Decode(w, r, &body) {
			return
		}
		j, err := s.Files.Transfer(kind, body.Paths, body.To, files.Clash(body.Clash))
		if err != nil {
			writeFileErr(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusAccepted, j)
	}
}

func (s *Server) filesDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Paths []string `json:"paths"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if err := s.Files.Delete(body.Paths); err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int{"deleted": len(body.Paths)})
}

func (s *Server) filesJobs(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": s.Files.Jobs()})
}

func (s *Server) filesCancel(w http.ResponseWriter, r *http.Request) {
	if !s.Files.Cancel(r.PathValue("id")) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "no such job")
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]bool{"cancelled": true})
}

func (s *Server) trashList(w http.ResponseWriter, r *http.Request) {
	items, err := s.Files.Trash()
	if err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

type idsBody struct {
	IDs []string `json:"ids"`
}

func (s *Server) trashRestore(w http.ResponseWriter, r *http.Request) {
	var body idsBody
	if !httpx.Decode(w, r, &body) {
		return
	}
	got, err := s.Files.Restore(body.IDs)
	if err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"restored": got})
}

func (s *Server) trashDelete(w http.ResponseWriter, r *http.Request) {
	var body idsBody
	if !httpx.Decode(w, r, &body) {
		return
	}
	if len(body.IDs) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "bad_ids", "say which items")
		return
	}
	if err := s.Files.Purge(body.IDs); err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int{"deleted": len(body.IDs)})
}

func (s *Server) trashEmpty(w http.ResponseWriter, r *http.Request) {
	if err := s.Files.Purge(nil); err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"emptied": true})
}

func (s *Server) terminalTicket(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	c, _ := session.FromContext(r.Context())
	id, err := s.Term.Ticket(r.Context(), c.Username, body.Password)
	if err != nil {
		writeFileErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"ticket": id})
}
