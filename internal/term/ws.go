package term

// The server side of a WebSocket (RFC 6455): enough for one terminal. The
// browser's frames come masked; the server's go out unmasked.

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// maxFrame caps one incoming message: keystrokes and pastes, not files.
const maxFrame = 1 << 20

type wsConn struct {
	c  net.Conn
	r  *bufio.Reader
	mu sync.Mutex // one writer at a time
}

var errNotWS = errors.New("not a WebSocket request")

// upgrade answers the handshake and takes over the connection.
func upgrade(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		!strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") ||
		r.Header.Get("Sec-WebSocket-Version") != "13" || r.Header.Get("Sec-WebSocket-Key") == "" {
		return nil, errNotWS
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("this server can't hand over connections")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + wsGUID))
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " +
		base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n"
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write([]byte(resp)); err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return &wsConn{c: conn, r: rw.Reader}, nil
}

func (w *wsConn) Close() error { return w.c.Close() }

// Read returns the next message and whether it was text.
func (w *wsConn) Read() ([]byte, bool, error) {
	var msg []byte
	var text bool
	for {
		var h [2]byte
		if _, err := io.ReadFull(w.r, h[:]); err != nil {
			return nil, false, err
		}
		fin, op := h[0]&0x80 != 0, h[0]&0x0F
		if h[1]&0x80 == 0 {
			return nil, false, errors.New("an unmasked frame from the browser")
		}
		n := uint64(h[1] & 0x7F)
		switch n {
		case 126:
			var b [2]byte
			if _, err := io.ReadFull(w.r, b[:]); err != nil {
				return nil, false, err
			}
			n = uint64(binary.BigEndian.Uint16(b[:]))
		case 127:
			var b [8]byte
			if _, err := io.ReadFull(w.r, b[:]); err != nil {
				return nil, false, err
			}
			n = binary.BigEndian.Uint64(b[:])
		}
		if n > maxFrame || uint64(len(msg))+n > maxFrame {
			return nil, false, errors.New("message too large")
		}
		var mask [4]byte
		if _, err := io.ReadFull(w.r, mask[:]); err != nil {
			return nil, false, err
		}
		p := make([]byte, n)
		if _, err := io.ReadFull(w.r, p); err != nil {
			return nil, false, err
		}
		for i := range p {
			p[i] ^= mask[i%4]
		}
		switch op {
		case 0x8:
			_ = w.write(0x8, nil)
			return nil, false, io.EOF
		case 0x9:
			if err := w.write(0xA, p); err != nil {
				return nil, false, err
			}
			continue
		case 0xA:
			continue
		case 0x1, 0x2:
			text = op == 0x1
			msg = p
		case 0x0:
			msg = append(msg, p...)
		default:
			return nil, false, errors.New("unknown WebSocket frame")
		}
		if fin {
			return msg, text, nil
		}
	}
}

// WriteBinary sends output to the browser.
func (w *wsConn) WriteBinary(p []byte) error { return w.write(0x2, p) }

// CloseWith sends a close frame with a reason the UI shows.
func (w *wsConn) CloseWith(code uint16, reason string) {
	p := make([]byte, 2, 2+len(reason))
	binary.BigEndian.PutUint16(p, code)
	_ = w.write(0x8, append(p, reason...))
	w.c.Close()
}

func (w *wsConn) write(op byte, p []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	h := []byte{0x80 | op}
	switch n := len(p); {
	case n < 126:
		h = append(h, byte(n))
	case n < 1<<16:
		h = append(h, 126, byte(n>>8), byte(n))
	default:
		h = append(h, 127)
		h = binary.BigEndian.AppendUint64(h, uint64(n))
	}
	_ = w.c.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := w.c.Write(h); err != nil {
		return err
	}
	_, err := w.c.Write(p)
	return err
}
