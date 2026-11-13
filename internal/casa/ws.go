package casa

// A minimal WebSocket client (RFC 6455), enough to read CasaOS message bus
// events: text frames in, pongs and a close out. IceDash has no other use for
// WebSockets, so this avoids a dependency.

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// maxMessage caps one event (the bus sends small JSON objects).
const maxMessage = 1 << 20

type wsConn struct {
	c net.Conn
	r *bufio.Reader
}

// dialWS opens a WebSocket to an http:// URL.
func dialWS(ctx context.Context, raw string) (*wsConn, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" {
		return nil, fmt.Errorf("unsupported message bus address %q", raw)
	}
	addr := u.Host
	if u.Port() == "" {
		addr += ":80"
	}
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	keyRaw := make([]byte, 16)
	if _, err := rand.Read(keyRaw); err != nil {
		c.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyRaw)
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	req := "GET " + u.RequestURI() + " HTTP/1.1\r\n" +
		"Host: " + u.Host + "\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\nSec-WebSocket-Version: 13\r\n\r\n"
	if _, err := io.WriteString(c, req); err != nil {
		c.Close()
		return nil, err
	}
	br := bufio.NewReader(c)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		c.Close()
		return nil, err
	}
	res.Body.Close()
	sum := sha1.Sum([]byte(key + wsGUID))
	if res.StatusCode != http.StatusSwitchingProtocols ||
		!strings.EqualFold(res.Header.Get("Upgrade"), "websocket") ||
		res.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(sum[:]) {
		c.Close()
		return nil, fmt.Errorf("message bus refused the WebSocket (%s)", res.Status)
	}
	_ = c.SetDeadline(time.Time{})
	return &wsConn{c: c, r: br}, nil
}

func (w *wsConn) Close() error { return w.c.Close() }

// errClosed: the server closed the connection.
var errClosed = errors.New("message bus closed the connection")

// Read returns the next text or binary message, answering pings on the way.
func (w *wsConn) Read() ([]byte, error) {
	var msg []byte
	for {
		fin, op, payload, err := w.frame()
		if err != nil {
			return nil, err
		}
		switch op {
		case 0x8: // close
			_ = w.write(0x8, nil)
			return nil, errClosed
		case 0x9: // ping
			if err := w.write(0xA, payload); err != nil {
				return nil, err
			}
			continue
		case 0xA: // pong
			continue
		case 0x0, 0x1, 0x2: // continuation, text, binary
			msg = append(msg, payload...)
			if len(msg) > maxMessage {
				return nil, errors.New("message bus event too large")
			}
			if fin {
				return msg, nil
			}
		default:
			return nil, fmt.Errorf("unexpected WebSocket opcode %d", op)
		}
	}
}

func (w *wsConn) frame() (fin bool, op byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(w.r, h[:]); err != nil {
		return
	}
	fin, op = h[0]&0x80 != 0, h[0]&0x0F
	masked := h[1]&0x80 != 0
	n := uint64(h[1] & 0x7F)
	switch n {
	case 126:
		var b [2]byte
		if _, err = io.ReadFull(w.r, b[:]); err != nil {
			return
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err = io.ReadFull(w.r, b[:]); err != nil {
			return
		}
		n = binary.BigEndian.Uint64(b[:])
	}
	if n > maxMessage {
		err = errors.New("message bus frame too large")
		return
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(w.r, mask[:]); err != nil {
			return
		}
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(w.r, payload); err != nil {
		return
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return
}

// write sends one frame; a client must mask what it sends.
func (w *wsConn) write(op byte, payload []byte) error {
	if len(payload) > 125 {
		payload = payload[:125] // control frames only
	}
	b := []byte{0x80 | op, 0x80 | byte(len(payload))}
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	b = append(b, mask[:]...)
	for i, c := range payload {
		b = append(b, c^mask[i%4])
	}
	_ = w.c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := w.c.Write(b)
	return err
}
