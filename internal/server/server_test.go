package server

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"derek-relay/internal/config"
	"derek-relay/internal/message"
)

func TestSendAndPollMessage(t *testing.T) {
	secret := "test-secret"
	relay := New(testConfig(secret))
	server := httptest.NewServer(relay.Routes())
	defer server.Close()

	sendBody := `{"text":"您的验证码是 135790，5 分钟内有效","id":"1234567","secret":"test-secret"}`
	resp, err := http.Post(server.URL+"/api/messages", "application/json", strings.NewReader(sendBody))
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("post message status = %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/poll", nil)
	if err != nil {
		t.Fatalf("new poll request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	pollResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	defer pollResp.Body.Close()

	var payload message.PollResponse
	if err := json.NewDecoder(pollResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode poll response: %v", err)
	}
	if len(payload.Messages) != 1 {
		t.Fatalf("poll message count = %d", len(payload.Messages))
	}
	if payload.Messages[0].Text != "您的验证码是 135790，5 分钟内有效" {
		t.Fatalf("poll text = %q", payload.Messages[0].Text)
	}
	if payload.Messages[0].ID != "1234567" {
		t.Fatalf("poll id = %q", payload.Messages[0].ID)
	}
}

func TestUnauthorizedMessageIsRejected(t *testing.T) {
	relay := New(testConfig("test-secret"))
	server := httptest.NewServer(relay.Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/messages", "application/json", strings.NewReader(`{"text":"hello","id":"1234567","secret":"bad"}`))
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post message status = %d", resp.StatusCode)
	}
}

func TestWebSocketReceivesMessage(t *testing.T) {
	secret := "test-secret"
	relay := New(testConfig(secret))
	server := httptest.NewServer(relay.Routes())
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	key := base64.StdEncoding.EncodeToString([]byte("derek-relay-test1"))
	handshake := fmt.Sprintf("GET /api/ws?secret=%s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", secret, host, key)
	if _, err := io.WriteString(conn, handshake); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read handshake status: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("handshake status = %q", status)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read handshake header: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	sendBody := `{"text":"您的验证码是 975310，5 分钟内有效","id":"7654321","secret":"test-secret"}`
	resp, err := http.Post(server.URL+"/api/messages", "application/json", strings.NewReader(sendBody))
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("post message status = %d", resp.StatusCode)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	payload, err := readWebSocketTextFrame(reader)
	if err != nil {
		t.Fatalf("read websocket frame: %v", err)
	}

	var received message.PollResponse
	if err := json.Unmarshal(payload, &received); err != nil {
		t.Fatalf("decode websocket payload: %v", err)
	}
	if len(received.Messages) != 1 {
		t.Fatalf("websocket message count = %d", len(received.Messages))
	}
	if received.Messages[0].ID != "7654321" {
		t.Fatalf("websocket id = %q", received.Messages[0].ID)
	}
}

func testConfig(secret string) config.Config {
	return config.Config{
		Listen:       ":0",
		Secret:       secret,
		PollTimeout:  200 * time.Millisecond,
		MessageTTL:   10 * time.Minute,
		QueueSize:    100,
		MaxBodyBytes: 32 * 1024,
	}
}

func readWebSocketTextFrame(reader *bufio.Reader) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if header[0] != 0x81 {
		return nil, fmt.Errorf("unexpected opcode byte: %x", header[0])
	}

	length := int(header[1] & 0x7f)
	switch length {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return nil, err
		}
		length = int(extended[0])<<8 | int(extended[1])
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return nil, err
		}
		if extended[0] != 0 || extended[1] != 0 || extended[2] != 0 || extended[3] != 0 {
			return nil, fmt.Errorf("frame too large")
		}
		length = int(extended[4])<<24 | int(extended[5])<<16 | int(extended[6])<<8 | int(extended[7])
	}

	payload := make([]byte, length)
	_, err := io.ReadFull(reader, payload)
	return payload, err
}
