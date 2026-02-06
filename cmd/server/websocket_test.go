package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestCreateRoomViaWebSocket(t *testing.T) {
	s := &Server{hub: NewHub()}
	mux := SetupRouter(s)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/ws?user=alice"
	ctx := context.Background()

	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// Create a room
	createMsg := Message{
		Type:    "create_room",
		Sender:  "alice",
		Content: "general",
	}
	data, _ := json.Marshal(createMsg)
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send create_room: %v", err)
	}

	// Read the acknowledgment
	_, p, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read ack: %v", err)
	}

	var ack Message
	if err := json.Unmarshal(p, &ack); err != nil {
		t.Fatalf("failed to unmarshal ack: %v", err)
	}

	if ack.Type != "room_created" {
		t.Errorf("expected type 'room_created', got %q", ack.Type)
	}
	if ack.Room != "general" {
		t.Errorf("expected room 'general', got %q", ack.Room)
	}
}

func TestInviteAndRoomMessage(t *testing.T) {
	s := &Server{hub: NewHub()}
	mux := SetupRouter(s)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/ws"
	ctx := context.Background()

	// Connect Alice
	alice, _, err := websocket.Dial(ctx, wsURL+"?user=alice", nil)
	if err != nil {
		t.Fatalf("alice failed to dial: %v", err)
	}
	defer alice.Close(websocket.StatusNormalClosure, "")

	// Connect Bob
	bob, _, err := websocket.Dial(ctx, wsURL+"?user=bob", nil)
	if err != nil {
		t.Fatalf("bob failed to dial: %v", err)
	}
	defer bob.Close(websocket.StatusNormalClosure, "")

	// Alice creates a room
	createMsg := Message{Type: "create_room", Sender: "alice", Content: "devteam"}
	data, _ := json.Marshal(createMsg)
	alice.Write(ctx, websocket.MessageText, data)

	// Read Alice's room_created ack
	alice.Read(ctx)

	// Alice invites Bob
	inviteMsg := Message{Type: "invite", Sender: "alice", Room: "devteam", Recipient: "bob"}
	data, _ = json.Marshal(inviteMsg)
	alice.Write(ctx, websocket.MessageText, data)

	// Read Alice's invite_sent ack
	alice.Read(ctx)

	// Bob should receive an invitation notification
	_, p, err := bob.Read(ctx)
	if err != nil {
		t.Fatalf("bob failed to read invite notification: %v", err)
	}
	var notification Message
	json.Unmarshal(p, &notification)
	if notification.Type != "invited" {
		t.Errorf("expected type 'invited', got %q", notification.Type)
	}
	if notification.Room != "devteam" {
		t.Errorf("expected room 'devteam', got %q", notification.Room)
	}

	// Alice sends a room message
	roomMsg := Message{Type: "room_msg", Sender: "alice", Room: "devteam", Content: "Hello team!"}
	data, _ = json.Marshal(roomMsg)
	alice.Write(ctx, websocket.MessageText, data)

	// Bob should receive the room message
	_, p, err = bob.Read(ctx)
	if err != nil {
		t.Fatalf("bob failed to read room message: %v", err)
	}
	var received Message
	json.Unmarshal(p, &received)
	if received.Type != "room_msg" {
		t.Errorf("expected type 'room_msg', got %q", received.Type)
	}
	if received.Room != "devteam" {
		t.Errorf("expected room 'devteam', got %q", received.Room)
	}
	if received.Content != "Hello team!" {
		t.Errorf("expected content 'Hello team!', got %q", received.Content)
	}
	if received.Sender != "alice" {
		t.Errorf("expected sender 'alice', got %q", received.Sender)
	}
}

func TestRoomMessageNotDeliveredToNonMembers(t *testing.T) {
	s := &Server{hub: NewHub()}
	mux := SetupRouter(s)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/ws"
	ctx := context.Background()

	// Connect Alice and Charlie
	alice, _, _ := websocket.Dial(ctx, wsURL+"?user=alice", nil)
	defer alice.Close(websocket.StatusNormalClosure, "")

	charlie, _, _ := websocket.Dial(ctx, wsURL+"?user=charlie", nil)
	defer charlie.Close(websocket.StatusNormalClosure, "")

	// Alice creates a room (only Alice is a member)
	createMsg := Message{Type: "create_room", Sender: "alice", Content: "private"}
	data, _ := json.Marshal(createMsg)
	alice.Write(ctx, websocket.MessageText, data)
	alice.Read(ctx) // ack

	// Alice sends a room message
	roomMsg := Message{Type: "room_msg", Sender: "alice", Room: "private", Content: "secret"}
	data, _ = json.Marshal(roomMsg)
	alice.Write(ctx, websocket.MessageText, data)

	// Charlie should NOT receive anything — use a short timeout to verify
	readCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_, _, err := charlie.Read(readCtx)
	if err == nil {
		t.Error("expected charlie NOT to receive a message, but got one")
	}
}
