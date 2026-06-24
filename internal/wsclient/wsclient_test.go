package wsclient

import (
	"bufio"
	"bytes"
	"net"
	"testing"
	"time"
)

func TestAcceptKeyMatchesWebSocketRFCExample(t *testing.T) {
	got := acceptKey("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Fatalf("acceptKey() = %q, want %q", got, want)
	}
}

func TestReadFrameReadsUnmaskedServerFrame(t *testing.T) {
	conn := &Conn{r: bufio.NewReader(bytes.NewReader([]byte{0x81, 0x02, 'o', 'k'}))}

	frame, err := conn.readFrame()
	if err != nil {
		t.Fatalf("readFrame() error = %v", err)
	}
	if !frame.fin || frame.opcode != 0x1 || string(frame.payload) != "ok" {
		t.Fatalf("readFrame() = %#v", frame)
	}
}

func TestWriteFrameMasksClientPayload(t *testing.T) {
	raw := &recordingConn{}
	conn := &Conn{conn: raw}

	if err := conn.writeFrame(0x1, []byte("hello")); err != nil {
		t.Fatalf("writeFrame() error = %v", err)
	}

	data := raw.Bytes()
	if len(data) != 2+4+5 {
		t.Fatalf("writeFrame() wrote %d bytes, want 11", len(data))
	}
	if data[0] != 0x81 {
		t.Fatalf("frame first byte = %#x, want 0x81", data[0])
	}
	if data[1]&0x80 == 0 || int(data[1]&0x7F) != 5 {
		t.Fatalf("frame length/mask byte = %#x, want masked len 5", data[1])
	}

	mask := data[2:6]
	payload := append([]byte(nil), data[6:]...)
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	if string(payload) != "hello" {
		t.Fatalf("unmasked payload = %q, want hello", string(payload))
	}
}

type recordingConn struct {
	bytes.Buffer
}

func (c *recordingConn) Read(_ []byte) (int, error)         { return 0, nil }
func (c *recordingConn) Close() error                       { return nil }
func (c *recordingConn) LocalAddr() net.Addr                { return dummyAddr("local") }
func (c *recordingConn) RemoteAddr() net.Addr               { return dummyAddr("remote") }
func (c *recordingConn) SetDeadline(_ time.Time) error      { return nil }
func (c *recordingConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *recordingConn) SetWriteDeadline(_ time.Time) error { return nil }

type dummyAddr string

func (a dummyAddr) Network() string { return string(a) }
func (a dummyAddr) String() string  { return string(a) }
