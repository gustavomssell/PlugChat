package realtime

import (
	"encoding/json"
	"sync"
)

type Event struct {
	Name string
	Data []byte
}

type Hub struct {
	mu     sync.RWMutex
	topics map[string]map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{topics: map[string]map[chan Event]struct{}{}}
}

func (h *Hub) Subscribe(topic string) (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	channel := make(chan Event, 16)
	if h.topics[topic] == nil {
		h.topics[topic] = map[chan Event]struct{}{}
	}
	h.topics[topic][channel] = struct{}{}

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.topics[topic], channel)
		if len(h.topics[topic]) == 0 {
			delete(h.topics, topic)
		}
	}

	return channel, cancel
}

func (h *Hub) Publish(topic, name string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	event := Event{Name: name, Data: data}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for channel := range h.topics[topic] {
		select {
		case channel <- event:
		default:
		}
	}
}
