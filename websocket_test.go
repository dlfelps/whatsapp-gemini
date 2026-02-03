package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"nhooyr.io/websocket"
)

func TestWebSocketUpgrade(t *testing.T) {
	s := &Server{hub: NewHub()}
	mux := SetupRouter(s)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/ws?user=testuser"

	ctx := context.Background()
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// If we reached here, the upgrade was successful.
}
