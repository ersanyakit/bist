package wsclient

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type Conn struct {
	conn net.Conn
	r    *bufio.Reader
	mu   sync.Mutex
}

func Dial(ctx context.Context, rawURL string) (*Conn, error) {
	return DialHeaders(ctx, rawURL, nil)
}

func DialHeaders(ctx context.Context, rawURL string, headers map[string]string) (*Conn, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return nil, fmt.Errorf("unsupported websocket scheme %q", parsed.Scheme)
	}

	host := parsed.Host
	if !strings.Contains(host, ":") {
		if parsed.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	dialer := &net.Dialer{Timeout: 20 * time.Second}
	rawConn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}

	conn := rawConn
	if parsed.Scheme == "wss" {
		tlsConn := tls.Client(rawConn, &tls.Config{ServerName: parsed.Hostname(), MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return nil, err
		}
		conn = tlsConn
	}

	key, err := websocketKey()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	path := parsed.RequestURI()
	if path == "" {
		path = "/"
	}
	var request strings.Builder
	fmt.Fprintf(&request, "GET %s HTTP/1.1\r\n", path)
	fmt.Fprintf(&request, "Host: %s\r\n", parsed.Host)
	request.WriteString("Upgrade: websocket\r\n")
	request.WriteString("Connection: Upgrade\r\n")
	fmt.Fprintf(&request, "Sec-WebSocket-Key: %s\r\n", key)
	request.WriteString("Sec-WebSocket-Version: 13\r\n")
	if _, ok := headers["User-Agent"]; !ok {
		request.WriteString("User-Agent: hissebot-go/1.0\r\n")
	}
	for name, value := range headers {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		fmt.Fprintf(&request, "%s: %s\r\n", name, value)
	}
	request.WriteString("\r\n")
	if _, err := io.WriteString(conn, request.String()); err != nil {
		_ = conn.Close()
		return nil, err
	}

	reader := bufio.NewReader(conn)
	req := &http.Request{Method: http.MethodGet}
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
		return nil, fmt.Errorf("websocket upgrade status %d", resp.StatusCode)
	}
	if !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") {
		_ = conn.Close()
		return nil, errors.New("websocket upgrade header missing")
	}
	if got, want := resp.Header.Get("Sec-WebSocket-Accept"), acceptKey(key); got != want {
		_ = conn.Close()
		return nil, errors.New("websocket accept key mismatch")
	}

	return &Conn{conn: conn, r: reader}, nil
}

func (c *Conn) ReadText(ctx context.Context) (string, error) {
	var message []byte
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		_ = c.conn.SetReadDeadline(time.Now().Add(time.Second))
		frame, err := c.readFrame()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return "", err
		}

		switch frame.opcode {
		case 0x0, 0x1:
			message = append(message, frame.payload...)
			if frame.fin {
				return string(message), nil
			}
		case 0x8:
			return "", io.EOF
		case 0x9:
			_ = c.writeFrame(0xA, frame.payload)
		case 0xA:
			continue
		default:
			continue
		}
	}
}

func (c *Conn) WriteText(payload string) error {
	return c.writeFrame(0x1, []byte(payload))
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

type frame struct {
	fin     bool
	opcode  byte
	payload []byte
}

func (c *Conn) readFrame() (frame, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.r, header); err != nil {
		return frame{}, err
	}

	fin := header[0]&0x80 != 0
	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7F)

	switch length {
	case 126:
		var buf [2]byte
		if _, err := io.ReadFull(c.r, buf[:]); err != nil {
			return frame{}, err
		}
		length = uint64(binary.BigEndian.Uint16(buf[:]))
	case 127:
		var buf [8]byte
		if _, err := io.ReadFull(c.r, buf[:]); err != nil {
			return frame{}, err
		}
		length = binary.BigEndian.Uint64(buf[:])
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.r, mask[:]); err != nil {
			return frame{}, err
		}
	}

	if length > 16*1024*1024 {
		return frame{}, fmt.Errorf("websocket frame too large: %d", length)
	}

	payload := make([]byte, int(length))
	if _, err := io.ReadFull(c.r, payload); err != nil {
		return frame{}, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return frame{fin: fin, opcode: opcode, payload: payload}, nil
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}

	header := []byte{0x80 | opcode}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, 0x80|byte(length))
	case length <= 0xFFFF:
		header = append(header, 0x80|126, byte(length>>8), byte(length))
	default:
		header = append(header, 0x80|127)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(length))
		header = append(header, buf[:]...)
	}
	header = append(header, mask[:]...)

	masked := make([]byte, len(payload))
	copy(masked, payload)
	for i := range masked {
		masked[i] ^= mask[i%4]
	}

	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	_, err := c.conn.Write(masked)
	return err
}

func websocketKey() (string, error) {
	var key [16]byte
	if _, err := rand.Read(key[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key[:]), nil
}

func acceptKey(key string) string {
	sum := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}
