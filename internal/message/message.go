package message

import "time"

type IncomingPayload struct {
	Sender string `json:"sender,omitempty"`
	Text   string `json:"text,omitempty"`
	Body   string `json:"message,omitempty"`
	Code   string `json:"code,omitempty"`
	ID     string `json:"id,omitempty"`
	Secret string `json:"secret,omitempty"`
}

type Message struct {
	RelayID    string    `json:"relay_id"`
	Sender     string    `json:"sender,omitempty"`
	Text       string    `json:"text"`
	Code       string    `json:"code,omitempty"`
	ID         string    `json:"id"`
	ReceivedAt time.Time `json:"received_at"`
	RemoteAddr string    `json:"remote_addr"`
}

func (payload IncomingPayload) MessageText() string {
	if payload.Text != "" {
		return payload.Text
	}
	return payload.Body
}

type PollResponse struct {
	Messages []Message `json:"messages"`
}

type SendResponse struct {
	OK      bool   `json:"ok"`
	RelayID string `json:"relay_id,omitempty"`
	Error   string `json:"error,omitempty"`
}
