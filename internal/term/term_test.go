package term

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func service(good string) *Service {
	return &Service{Check: func(_ context.Context, user, pw string) (bool, error) { return pw == good, nil }}
}

func TestTicketsAndTheLock(t *testing.T) {
	s := service("right")
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	ctx := context.Background()

	id, err := s.Ticket(ctx, "Hira", "right")
	if err != nil || id == "" {
		t.Fatal(err)
	}
	if u, err := s.redeem(id); err != nil || u != "Hira" {
		t.Fatalf("%q %v", u, err)
	}
	if _, err := s.redeem(id); !errors.Is(err, ErrTicket) {
		t.Fatal("a ticket opens one terminal only")
	}
	id, _ = s.Ticket(ctx, "Hira", "right")
	now = now.Add(2 * time.Minute)
	if _, err := s.redeem(id); !errors.Is(err, ErrTicket) {
		t.Fatal("a ticket lasts a minute")
	}

	for i := 1; i <= 4; i++ {
		if _, err := s.Ticket(ctx, "Hira", "wrong"); !errors.Is(err, ErrPassword) {
			t.Fatalf("wrong %d: %v", i, err)
		}
	}
	if _, err := s.Ticket(ctx, "Hira", "wrong"); !errors.Is(err, ErrLocked) {
		t.Fatalf("fifth wrong: %v", err)
	}
	if _, err := s.Ticket(ctx, "Hira", "right"); !errors.Is(err, ErrLocked) {
		t.Fatal("even the right password waits while locked")
	}
	now = now.Add(11 * time.Minute)
	if _, err := s.Ticket(ctx, "Hira", "right"); err != nil {
		t.Fatalf("after the lock: %v", err)
	}
	// Old failures fall out of the 10-minute window.
	for i := 0; i < 4; i++ {
		s.Ticket(ctx, "Hira", "wrong")
	}
	now = now.Add(11 * time.Minute)
	if _, err := s.Ticket(ctx, "Hira", "wrong"); !errors.Is(err, ErrPassword) {
		t.Fatalf("old failures should have expired: %v", err)
	}
}

func TestGatewayPassword(t *testing.T) {
	// The answers ZimaOS 1.8.0 and CasaOS 0.4.15 gave (HOST-NOTES, Phase 3c).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v1/users/login" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(string(body), `"password":"testmember"`) {
			io.WriteString(w, `{"success":200,"message":"ok","data":{"token":{"access_token":"a.b.c","refresh_token":"r","expires_at":1},"user":{"role":"user"}}}`)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"success":10013,"message":"User does not exist or password is invalid","data":null}`)
	}))
	defer srv.Close()
	check := GatewayPassword(srv.URL)
	if ok, err := check(context.Background(), "testmember", "testmember"); !ok || err != nil {
		t.Fatalf("right: %v %v", ok, err)
	}
	if ok, err := check(context.Background(), "testmember", "nope"); ok || err != nil {
		t.Fatalf("wrong: %v %v", ok, err)
	}
	down := GatewayPassword("http://127.0.0.1:1")
	if _, err := down(context.Background(), "a", "b"); err == nil {
		t.Fatal("an unreachable sign-in is an error, not a wrong password")
	}
}

func TestRefusesOtherOriginsAndBadTickets(t *testing.T) {
	s := service("right")
	srv := httptest.NewServer(s)
	defer srv.Close()
	for _, c := range []struct {
		origin string
		want   int
	}{{"", http.StatusForbidden}, {"http://evil.example", http.StatusForbidden}, {srv.URL, http.StatusUnauthorized}} {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v2/icedash/terminal?ticket=nope", nil)
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != c.want {
			t.Errorf("origin %q: %d, want %d", c.origin, res.StatusCode, c.want)
		}
	}
}

// client is a bare WebSocket client: masked frames out, as a browser sends.
type client struct {
	c net.Conn
	r *bufio.Reader
}

func dial(t *testing.T, base, path string) *client {
	t.Helper()
	host := strings.TrimPrefix(base, "http://")
	c, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatal(err)
	}
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	io.WriteString(c, "GET "+path+" HTTP/1.1\r\nHost: "+host+"\r\nOrigin: "+base+
		"\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: "+key+"\r\n\r\n")
	r := bufio.NewReader(c)
	res, err := http.ReadResponse(r, nil)
	if err != nil || res.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade: %v %v", res, err)
	}
	return &client{c: c, r: r}
}

func (c *client) send(op byte, p []byte) {
	h := []byte{0x80 | op, 0x80 | byte(len(p))}
	m := make([]byte, 4)
	rand.Read(m)
	h = append(h, m...)
	for i, b := range p {
		h = append(h, b^m[i%4])
	}
	c.c.Write(h)
}

// readUntil collects output until it contains want or the deadline passes.
func (c *client) readUntil(t *testing.T, want string) string {
	t.Helper()
	c.c.SetReadDeadline(time.Now().Add(10 * time.Second))
	var got strings.Builder
	for !strings.Contains(got.String(), want) {
		var h [2]byte
		if _, err := io.ReadFull(c.r, h[:]); err != nil {
			t.Fatalf("waiting for %q, got %q: %v", want, got.String(), err)
		}
		n := int(h[1] & 0x7F)
		if n == 126 {
			var b [2]byte
			io.ReadFull(c.r, b[:])
			n = int(binary.BigEndian.Uint16(b[:]))
		}
		p := make([]byte, n)
		io.ReadFull(c.r, p)
		if h[0]&0x0F == 0x8 {
			got.WriteString("[closed:" + string(p[2:]) + "]")
			continue
		}
		got.Write(p)
	}
	return got.String()
}

func TestShellOverWebSocket(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("terminals need Linux")
	}
	s := service("right")
	s.Shell, s.Dir = []string{"/bin/sh"}, t.TempDir()
	srv := httptest.NewServer(s)
	defer srv.Close()
	id, err := s.Ticket(context.Background(), "Hira", "right")
	if err != nil {
		t.Fatal(err)
	}
	c := dial(t, srv.URL, "/v2/icedash/terminal?ticket="+id+"&cols=100&rows=30")
	c.send(0x1, []byte("stty size; pwd; echo ice$((40+2))\n"))
	out := c.readUntil(t, "ice42")
	if !strings.Contains(out, "30 100") || !strings.Contains(out, s.Dir) {
		t.Fatalf("output: %q", out)
	}
	c.send(0x1, append([]byte{0}, []byte(`{"cols":120,"rows":40}`)...))
	time.Sleep(100 * time.Millisecond)
	c.send(0x1, []byte("stty size\n"))
	c.readUntil(t, "40 120")
	c.send(0x1, []byte("exit\n"))
	c.readUntil(t, "[closed:the shell ended]")

	// The same ticket can't open a second terminal.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v2/icedash/terminal?ticket="+id, nil)
	req.Header.Set("Origin", srv.URL)
	res, _ := http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused ticket: %d", res.StatusCode)
	}
}
