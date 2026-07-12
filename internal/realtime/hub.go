package realtime

import (
	"sync"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// Hub is a simple in-memory pub/sub for run events.
// It is intended for live SSE streams until a more targeted per-run stream is built.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan model.Event]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[chan model.Event]struct{}),
	}
}

// Subscribe returns a channel of events and a function to unsubscribe.
func (h *Hub) Subscribe() (<-chan model.Event, func()) {
	ch := make(chan model.Event, 16)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subscribers, ch)
		h.mu.Unlock()
		close(ch)
	}
}

// Publish sends an event to all current subscribers (non-blocking).
func (h *Hub) Publish(e model.Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}
