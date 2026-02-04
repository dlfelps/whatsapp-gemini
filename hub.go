package main

import (
	"sync"

	"nhooyr.io/websocket"
)

type connection struct {
	ws *websocket.Conn
}

type Message struct {
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*connection
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*connection),
	}
}

func (h *Hub) register(id string, conn *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[id] = conn
}

func (h *Hub) unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, id)
}

func (h *Hub) get(id string) (*connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.clients[id]
	return conn, ok
}
