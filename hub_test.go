package main

import (
	"testing"
)

func TestHubRegisterUnregister(t *testing.T) {
	h := NewHub()

	clientID := "test-user"
	conn := &connection{} // Mock connection

	h.register(clientID, conn)

	if _, ok := h.clients[clientID]; !ok {
		t.Errorf("expected client %s to be registered", clientID)
	}

	h.unregister(clientID)

	if _, ok := h.clients[clientID]; ok {
		t.Errorf("expected client %s to be unregistered", clientID)
	}
}
