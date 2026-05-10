package server

import (
	"context"
	"sync"
	"time"

	"derek-relay/internal/message"
)

type Hub struct {
	mu        sync.Mutex
	cond      *sync.Cond
	messages  []message.Message
	queueSize int
	ttl       time.Duration
	version   uint64
}

func NewHub(queueSize int, ttl time.Duration) *Hub {
	hub := &Hub{
		queueSize: queueSize,
		ttl:       ttl,
	}
	hub.cond = sync.NewCond(&hub.mu)
	return hub
}

func (hub *Hub) Add(msg message.Message) uint64 {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	hub.pruneLocked(time.Now())
	hub.version++
	hub.messages = append(hub.messages, msg)
	if len(hub.messages) > hub.queueSize {
		hub.messages = hub.messages[len(hub.messages)-hub.queueSize:]
	}
	hub.cond.Broadcast()
	return hub.version
}

func (hub *Hub) Snapshot(after time.Time) ([]message.Message, uint64) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	hub.pruneLocked(time.Now())
	return hub.messagesAfterLocked(after), hub.version
}

func (hub *Hub) Wait(ctx context.Context, after time.Time, timeout time.Duration) ([]message.Message, uint64) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	hub.pruneLocked(time.Now())
	if messages := hub.messagesAfterLocked(after); len(messages) > 0 {
		return messages, hub.version
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			hub.mu.Lock()
			hub.cond.Broadcast()
			hub.mu.Unlock()
		case <-done:
		}
	}()
	defer close(done)

	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 || ctx.Err() != nil {
			return nil, hub.version
		}

		timer := time.AfterFunc(remaining, func() {
			hub.mu.Lock()
			hub.cond.Broadcast()
			hub.mu.Unlock()
		})
		hub.cond.Wait()
		timer.Stop()

		hub.pruneLocked(time.Now())
		if messages := hub.messagesAfterLocked(after); len(messages) > 0 {
			return messages, hub.version
		}
	}
}

func (hub *Hub) messagesAfterLocked(after time.Time) []message.Message {
	if after.IsZero() {
		return append([]message.Message(nil), hub.messages...)
	}

	messages := make([]message.Message, 0, len(hub.messages))
	for _, msg := range hub.messages {
		if msg.ReceivedAt.After(after) {
			messages = append(messages, msg)
		}
	}
	return messages
}

func (hub *Hub) pruneLocked(now time.Time) {
	if hub.ttl <= 0 {
		return
	}

	cutoff := now.Add(-hub.ttl)
	keepFrom := 0
	for keepFrom < len(hub.messages) && hub.messages[keepFrom].ReceivedAt.Before(cutoff) {
		keepFrom++
	}
	if keepFrom > 0 {
		hub.messages = append([]message.Message(nil), hub.messages[keepFrom:]...)
	}
}
