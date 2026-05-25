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
	RelayID    string    `json:"relayId"`
	Sender     string    `json:"sender,omitempty"`
	Text       string    `json:"text"`
	Code       string    `json:"code,omitempty"`
	ID         string    `json:"id"`
	ReceivedAt time.Time `json:"receivedAt"`
	RemoteAddr string    `json:"remoteAddr"`
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
	RelayID string `json:"relayId,omitempty"`
	Error   string `json:"error,omitempty"`
}
