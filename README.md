# go-ws
:point_right: A demo implementing WebSocket Protocol in Golang

## Implementation steps
- Opening handshake
- Receive / Send data frames with clients
- Closing handshake

## Core code snippet
```golang
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
```

## References
[RFC6455](https://datatracker.ietf.org/doc/html/rfc6455)