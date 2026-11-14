// Package term is the Terminal's backend (PLAN.md, Phase 3c, "Terminal"): a
// root shell in a pseudo-terminal, over a WebSocket, for admins who typed
// their password again.
//
// The password is checked with the system's own sign-in (POST
// /v1/users/login on the gateway): a second sign-in leaves the user's session
// alone, and a wrong password answers 400 (measured on ZimaOS 1.8.0 and
// CasaOS 0.4.15). That gives a ticket for one connection within a minute; the
// browser's WebSocket can't carry the sign-in header, so the ticket is what
// lets it in.
package term

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Errors the API words for the UI.
var (
	ErrPassword = errors.New("that password didn't work")
	ErrLocked   = errors.New("too many wrong passwords; the Terminal opens again in 10 minutes")
	ErrTicket   = errors.New("that terminal link has expired; open the Terminal again")
)

const (
	ticketLife  = time.Minute
	maxFailures = 5
	failWindow  = 10 * time.Minute
	lockFor     = 10 * time.Minute
	// IdleLimit ends a terminal with no input or output for this long.
	IdleLimit = 30 * time.Minute
)

// CheckPassword asks the system whether user's password is right.
type CheckPassword func(ctx context.Context, user, password string) (bool, error)

// Service hands out terminal tickets and runs the terminals.
type Service struct {
	Check CheckPassword
	Shell []string // default /bin/bash -l
	Dir   string   // the shell's folder, default /DATA

	mu       sync.Mutex
	tickets  map[string]grant
	failures []time.Time
	locked   time.Time
	now      func() time.Time
}

type grant struct {
	user    string
	expires time.Time
}

func (s *Service) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// Ticket checks the password and returns a ticket for one terminal.
func (s *Service) Ticket(ctx context.Context, user, password string) (string, error) {
	s.mu.Lock()
	if s.clock().Before(s.locked) {
		s.mu.Unlock()
		return "", ErrLocked
	}
	s.mu.Unlock()

	ok, err := s.Check(ctx, user, password)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	if !ok {
		keep := s.failures[:0]
		for _, t := range s.failures {
			if now.Sub(t) < failWindow {
				keep = append(keep, t)
			}
		}
		s.failures = append(keep, now)
		if len(s.failures) >= maxFailures {
			s.locked, s.failures = now.Add(lockFor), nil
			log.Printf("[icedash] terminal: locked for 10 minutes after %d wrong passwords (last: %s)", maxFailures, user)
			return "", ErrLocked
		}
		return "", ErrPassword
	}
	s.failures = nil
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	if s.tickets == nil {
		s.tickets = map[string]grant{}
	}
	for k, g := range s.tickets {
		if now.After(g.expires) {
			delete(s.tickets, k)
		}
	}
	s.tickets[id] = grant{user: user, expires: now.Add(ticketLife)}
	return id, nil
}

// redeem uses a ticket up: it opens one terminal only.
func (s *Service) redeem(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.tickets[id]
	delete(s.tickets, id)
	if !ok || s.clock().After(g.expires) {
		return "", ErrTicket
	}
	return g.user, nil
}

// sameOrigin: the page that opens the socket must be IceDash's own.
func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return false
	}
	u, err := url.Parse(o)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

// ServeHTTP opens a terminal: GET /v2/icedash/terminal?ticket=...&cols=&rows=
// upgraded to a WebSocket. Text messages from the browser are keystrokes;
// a text message starting with "\x00" is a resize {"cols":..,"rows":..}.
// Output goes back as binary messages.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "cross-origin request rejected", http.StatusForbidden)
		return
	}
	user, err := s.redeem(r.URL.Query().Get("ticket"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	ws, err := upgrade(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer ws.Close()

	shell := s.Shell
	if len(shell) == 0 {
		shell = []string{"/bin/bash", "-l"}
	}
	cmd := exec.Command(shell[0], shell[1:]...)
	cmd.Dir = s.Dir
	if cmd.Dir == "" {
		cmd.Dir = "/DATA"
	}
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "LANG=C.UTF-8")
	ptm, err := startPTY(cmd)
	if err != nil {
		ws.CloseWith(1011, "the terminal couldn't start: "+err.Error())
		return
	}
	defer ptm.Close()
	if c, rr := atoi16(r.URL.Query().Get("cols")), atoi16(r.URL.Query().Get("rows")); c > 0 && rr > 0 {
		_ = resize(ptm, c, rr)
	}
	started := time.Now()
	log.Printf("[icedash] terminal opened for %s (pid %d)", user, cmd.Process.Pid)

	var mu sync.Mutex
	last := time.Now()
	reason := "the shell ended"
	touch := func() { mu.Lock(); last = time.Now(); mu.Unlock() }
	setReason := func(r string) {
		mu.Lock()
		if reason == "the shell ended" {
			reason = r
		}
		mu.Unlock()
	}
	done := make(chan struct{})

	go func() { // output
		defer close(done)
		buf := make([]byte, 32<<10)
		for {
			n, err := ptm.Read(buf)
			if n > 0 {
				touch()
				if ws.WriteBinary(buf[:n]) != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { // input
		for {
			msg, text, err := ws.Read()
			if err != nil {
				setReason("closed")
				_ = cmd.Process.Kill()
				return
			}
			touch()
			if text && len(msg) > 0 && msg[0] == 0 {
				var sz struct{ Cols, Rows uint16 }
				if json.Unmarshal(msg[1:], &sz) == nil && sz.Cols > 0 && sz.Rows > 0 {
					_ = resize(ptm, sz.Cols, sz.Rows)
				}
				continue
			}
			if _, err := ptm.Write(msg); err != nil {
				return
			}
		}
	}()
	idle := time.NewTicker(time.Minute)
	defer idle.Stop()
loop:
	for {
		select {
		case <-done:
			break loop
		case <-idle.C:
			mu.Lock()
			quiet := time.Since(last)
			mu.Unlock()
			if quiet > IdleLimit {
				setReason("closed after 30 minutes without use")
				_ = cmd.Process.Kill()
			}
		}
	}
	_ = cmd.Wait()
	mu.Lock()
	why := reason
	mu.Unlock()
	ws.CloseWith(1000, why)
	log.Printf("[icedash] terminal closed for %s after %s (%s)", user, time.Since(started).Round(time.Second), why)
}

func atoi16(s string) uint16 {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n <= 0 || n > 1000 {
		return 0
	}
	return uint16(n)
}

// GatewayPassword checks a password with the system's sign-in on the
// gateway at base (http://127.0.0.1).
func GatewayPassword(base string) CheckPassword {
	hc := &http.Client{Timeout: 15 * time.Second}
	return func(ctx context.Context, user, password string) (bool, error) {
		body, _ := json.Marshal(map[string]string{"username": user, "password": password})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/users/login", bytes.NewReader(body))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := hc.Do(req)
		if err != nil {
			return false, fmt.Errorf("the sign-in service didn't answer: %w", err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
		var v struct {
			Data struct {
				Token struct {
					AccessToken string `json:"access_token"`
				} `json:"token"`
			} `json:"data"`
		}
		_ = json.Unmarshal(raw, &v)
		switch {
		case res.StatusCode == http.StatusOK && v.Data.Token.AccessToken != "":
			return true, nil
		case res.StatusCode == http.StatusBadRequest || res.StatusCode == http.StatusUnauthorized:
			return false, nil
		}
		return false, fmt.Errorf("the sign-in service answered %s", res.Status)
	}
}
