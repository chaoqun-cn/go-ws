package core

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
)

type Server struct {
	socketChan chan *Socket
}

func NewServer() *Server {
	return &Server{
		socketChan: make(chan *Socket, 1),
	}
}

func (s *Server) Accept() (*Socket, error) {
	socket := <-s.socketChan
	if socket.conn == nil {
		return socket, io.EOF
	}

	return socket, nil
}

func isWsRequest(r *http.Request) bool {
	return r.Header.Get("Upgrade") == "websocket"
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if !isWsRequest(r) {
		// pass
		http.Error(w, "Only support websocket protocol", http.StatusBadRequest)
		return
	}

	socket, err := newSocket(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.handshake(socket)

	s.socketChan <- socket

}

/*
* Once we know that it’s a websocket request, the server needs to reply back with a handshake response.
* But we can’t write back the response using the http.ResponseWriter as it will also close the underlying tcp connection once we start sending the response.
* What we need is called HTTP Hijacking. Hijacking allows us to take over the underlying tcp connection handler and bufioWriter.
* This gives us the freedom to read and write data at will without closing the tcp connection.
 */
func (s *Server) handshake(socket *Socket) error {

	hash := func(swk string) string {
		h := sha1.New()
		h.Write([]byte(swk))
		h.Write([]byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		return base64.StdEncoding.EncodeToString(h.Sum(nil))
	}(socket.header.Get("Sec-Websocket-key"))

	lines := []string{
		"HTTP/1.1 101 Web Socket Protocol Handshake",
		"Server: chaoqun@go-ws",
		"Upgrade: WebSocket",
		"Connection: Upgrade",
		"Sec-WebSocket-Accept: " + hash,
		"", // blank line after all header
		"", // response body
	}

	return socket.write([]byte(strings.Join(lines, "\r\n")))
}

type Socket struct {
	conn   net.Conn
	bufrw  *bufio.ReadWriter
	header http.Header
}

func (socket *Socket) write(data []byte) error {
	if _, err := socket.conn.Write(data); err != nil {
		return err
	}

	return socket.bufrw.Flush()
}

func (socket *Socket) read(size int64) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := io.CopyN(&buf, socket.conn, size); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (socket *Socket) Recv() (Frame, error) {
	frame := Frame{}

	head, err := socket.read(2)
	if err != nil {
		return frame, err
	}

	frame.Header.Fin = (head[0] & 0x80) == 0x00
	frame.Header.Rsv = head[0] & 0x70
	frame.Header.Opcode = Opcode(head[0] & 0x0F)

	frame.Header.Masked = (head[1] & 0x80) == 0x00

	if len := uint64(head[1] & 0x7F); len == 126 {
		data, err := socket.read(2)
		if err != nil {
			return frame, err
		}
		frame.Header.Length = uint64(binary.BigEndian.Uint16(data))
	} else if len == 127 {
		data, err := socket.read(8)
		if err != nil {
			return frame, err
		}
		frame.Header.Length = uint64(binary.BigEndian.Uint64(data))
	} else {
		frame.Header.Length = len
	}

	mask, err := socket.read(4)
	if err != nil {
		return frame, err
	}
	copy(frame.Header.Mask[:], mask)

	payload, err := socket.read(int64(frame.Header.Length))
	if err != nil {
		return frame, err
	}

	for i := uint64(0); i < frame.Header.Length; i++ {
		payload[i] ^= mask[i%4]
	}

	frame.Payload = payload

	return frame, nil
}

func (socket *Socket) Send(frame *Frame) {
	socket.write(frame.Bytes())
}

func (socket *Socket) Close() {
	socket.conn.Close()
}

func (socket *Socket) GetConn() net.Conn {
	return socket.conn
}

func newSocket(w http.ResponseWriter, r *http.Request) (*Socket, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("hijacker type assert failed")
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	return &Socket{conn, bufrw, r.Header}, nil
}
