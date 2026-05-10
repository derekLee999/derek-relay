package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"derek-relay/internal/auth"
	"derek-relay/internal/config"
	"derek-relay/internal/message"
)

type Server struct {
	cfg config.Config
	hub *Hub
}

func New(cfg config.Config) *Server {
	return &Server{
		cfg: cfg,
		hub: NewHub(cfg.QueueSize, cfg.MessageTTL),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/verify", s.handleVerify)
	mux.HandleFunc("POST /api/messages", s.handleMessages)
	mux.HandleFunc("GET /api/poll", s.handlePoll)
	mux.HandleFunc("GET /api/ws", s.handleWebSocket)
	return cors(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"name": "derek-relay",
	})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	if !auth.Authorized(r, s.cfg.Secret) {
		writeJSON(w, http.StatusUnauthorized, message.SendResponse{OK: false, Error: "unauthorized"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"name": "derek-relay",
	})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	var payload message.IncomingPayload
	if err := readJSON(r, s.cfg.MaxBodyBytes, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, message.SendResponse{OK: false, Error: "bad_json"})
		return
	}

	if !payloadAuthorized(r, payload.Secret, s.cfg.Secret) {
		writeJSON(w, http.StatusUnauthorized, message.SendResponse{OK: false, Error: "unauthorized"})
		return
	}

	text := strings.TrimSpace(payload.MessageText())
	code := strings.TrimSpace(payload.Code)
	deviceID := strings.TrimSpace(payload.ID)
	if text == "" && code == "" {
		writeJSON(w, http.StatusBadRequest, message.SendResponse{OK: false, Error: "empty_message"})
		return
	}
	if deviceID == "" {
		writeJSON(w, http.StatusBadRequest, message.SendResponse{OK: false, Error: "missing_id"})
		return
	}

	msg := message.Message{
		RelayID:    newRelayID(),
		Sender:     strings.TrimSpace(payload.Sender),
		Text:       text,
		Code:       code,
		ID:         deviceID,
		ReceivedAt: time.Now().UTC(),
		RemoteAddr: remoteIP(r),
	}
	s.hub.Add(msg)
	writeJSON(w, http.StatusOK, message.SendResponse{OK: true, RelayID: msg.RelayID})
}

func (s *Server) handlePoll(w http.ResponseWriter, r *http.Request) {
	if !auth.Authorized(r, s.cfg.Secret) {
		writeJSON(w, http.StatusUnauthorized, message.SendResponse{OK: false, Error: "unauthorized"})
		return
	}

	after := parseTimeQuery(r.URL.Query().Get("after"))
	messages, _ := s.hub.Wait(r.Context(), after, s.cfg.PollTimeout)
	if messages == nil {
		messages = []message.Message{}
	}
	writeJSON(w, http.StatusOK, message.PollResponse{Messages: messages})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !auth.Authorized(r, s.cfg.Secret) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		http.Error(w, "bad websocket upgrade", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	stopReader := make(chan struct{})
	defer close(stopReader)
	go keepWebSocketReadable(conn, stopReader)

	after := parseTimeQuery(r.URL.Query().Get("after"))
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	for {
		messages, _ := s.hub.Wait(ctx, after, 24*time.Hour)
		if len(messages) == 0 {
			if ctx.Err() != nil {
				return
			}
			continue
		}

		payload, err := json.Marshal(message.PollResponse{Messages: messages})
		if err != nil {
			log.Printf("marshal websocket payload: %v", err)
			return
		}
		if err := writeWebSocketText(conn, payload); err != nil {
			return
		}
		after = messages[len(messages)-1].ReceivedAt
	}
}

func payloadAuthorized(r *http.Request, payloadSecret string, expected string) bool {
	if auth.Authorized(r, expected) {
		return true
	}

	cloned := r.Clone(r.Context())
	query := cloned.URL.Query()
	query.Set("secret", payloadSecret)
	cloned.URL.RawQuery = query.Encode()
	return auth.Authorized(cloned, expected)
}

func readJSON(r *http.Request, maxBytes int64, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("multiple json values")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "authorization,content-type,x-relay-secret")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseTimeQuery(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func remoteIP(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value != "" {
			return value
		}
	}
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func newRelayID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(bytes[:])
}
