package server

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	if !headerContains(r.Header, "Connection", "upgrade") ||
		!strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, errors.New("not a websocket upgrade")
	}

	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, errors.New("missing websocket key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("websocket hijack unsupported")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	acceptHash := sha1.Sum([]byte(key + websocketGUID))
	accept := base64.StdEncoding.EncodeToString(acceptHash[:])
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := rw.WriteString(response); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func writeWebSocketText(conn net.Conn, payload []byte) error {
	header := []byte{0x81}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length <= 65535:
		header = append(header, 126, byte(length>>8), byte(length))
	default:
		header = append(header, 127,
			byte(uint64(length)>>56),
			byte(uint64(length)>>48),
			byte(uint64(length)>>40),
			byte(uint64(length)>>32),
			byte(uint64(length)>>24),
			byte(uint64(length)>>16),
			byte(uint64(length)>>8),
			byte(uint64(length)),
		)
	}

	if _, err := conn.Write(header); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}

func keepWebSocketReadable(conn net.Conn, stop <-chan struct{}) {
	buffer := make([]byte, 512)
	for {
		select {
		case <-stop:
			return
		default:
			_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			if _, err := conn.Read(buffer); err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}
		}
	}
}

func headerContains(header http.Header, key string, expected string) bool {
	for _, value := range header.Values(key) {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), expected) {
				return true
			}
		}
	}
	return false
}
