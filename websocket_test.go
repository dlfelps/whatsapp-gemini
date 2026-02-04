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

func TestMessageDelivery(t *testing.T) {
	s := &Server{hub: NewHub()}
	mux := SetupRouter(s)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/ws"

	ctx := context.Background()

	// Connect Alice
	c1, _, err := websocket.Dial(ctx, wsURL+"?user=alice", nil)
	if err != nil {
		t.Fatalf("alice failed to dial: %v", err)
	}
	defer c1.Close(websocket.StatusNormalClosure, "")

	// Connect Bob
	c2, _, err := websocket.Dial(ctx, wsURL+"?user=bob", nil)
	if err != nil {
		t.Fatalf("bob failed to dial: %v", err)
	}
	defer c2.Close(websocket.StatusNormalClosure, "")

	// Alice sends message to Bob
	msg := `{"sender": "alice", "recipient": "bob", "content": "Hello Bob!"}`
	err = c1.Write(ctx, websocket.MessageText, []byte(msg))
	if err != nil {
		t.Fatalf("alice failed to write: %v", err)
	}

	// Bob should receive the message
	typ, p, err := c2.Read(ctx)
	if err != nil {
		t.Fatalf("bob failed to read: %v", err)
	}

	if typ != websocket.MessageText {
		t.Errorf("expected text message, got %v", typ)
	}

	expected := `{"sender": "alice", "recipient": "bob", "content": "Hello Bob!"}`
	if string(p) != expected {
		t.Errorf("expected %s, got %s", expected, string(p))
	}
}
