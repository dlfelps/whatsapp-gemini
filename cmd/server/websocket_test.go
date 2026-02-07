// This file contains integration tests for WebSocket functionality.
//
// Unlike the unit tests in hub_test.go and main_test.go, these tests spin up a
// real HTTP server (using httptest.NewServer) and make real WebSocket connections.
// This tests the full request lifecycle: HTTP upgrade -> WebSocket communication
// -> message routing -> cleanup.
//
// KEY GO TESTING CONCEPTS IN THIS FILE:
//   - httptest.NewServer for real HTTP server testing
//   - Integration testing with real WebSocket connections
//   - context.WithTimeout for test timeouts
//   - Testing asynchronous message delivery
//   - strings.Replace for URL scheme conversion
//   - defer server.Close() for test cleanup
package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	// time is used for context.WithTimeout in the non-member delivery test.
	// Setting short timeouts in tests prevents them from hanging forever if
	// something goes wrong.
	"time"

	"nhooyr.io/websocket"
)

// TestWebSocketUpgrade verifies that a client can successfully upgrade an HTTP
// connection to a WebSocket connection.
//
// LEARNING POINT — httptest.NewServer:
// Unlike httptest.NewRecorder (which fakes a response), httptest.NewServer
// starts a REAL HTTP server on a random available port. This is necessary for
// WebSocket testing because WebSockets require a real TCP connection — you
// can't fake the upgrade handshake with a recorder.
//
// httptest.NewServer returns a *httptest.Server with a .URL field containing
// the server's base URL (e.g., "http://127.0.0.1:54321"). The server is
// automatically cleaned up when you call server.Close().
//
// LEARNING POINT — URL Scheme Conversion:
// httptest.NewServer gives us an "http://" URL, but WebSocket requires "ws://".
// strings.Replace converts the scheme. In production, you'd use "wss://" for
// secure WebSockets (analogous to "https://").
func TestWebSocketUpgrade(t *testing.T) {
	s := &Server{hub: NewHub()}
	mux := SetupRouter(s)

	// Start a real HTTP server on a random port.
	server := httptest.NewServer(mux)
	defer server.Close() // Shut down the server when the test ends.

	// Convert http:// to ws:// for WebSocket dialing.
	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/ws?user=testuser"

	// context.Background() as the root context for the WebSocket connection.
	ctx := context.Background()
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// If we reached here, the upgrade was successful — no assertion needed.
	// The test passes by not failing. This is a valid Go testing pattern for
	// "does this operation succeed?" tests.
}

// TestMessageDelivery verifies that a direct message from one user is delivered
// to the recipient via WebSocket.
//
// LEARNING POINT — Multi-Client Integration Testing:
// This test connects two WebSocket clients (alice and bob) to the same server
// and verifies end-to-end message delivery. This is an integration test because
// it exercises the full stack: WebSocket read -> JSON parse -> hub lookup ->
// WebSocket write.
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

	// Alice sends message to Bob.
	// Note: this is raw JSON, not using json.Marshal. Both approaches work —
	// raw JSON is sometimes used in tests for brevity and to test exact
	// wire format. json.Marshal is used in later tests for type safety.
	msg := `{"sender": "alice", "recipient": "bob", "content": "Hello Bob!"}`
	err = c1.Write(ctx, websocket.MessageText, []byte(msg))
	if err != nil {
		t.Fatalf("alice failed to write: %v", err)
	}

	// Bob should receive the message.
	// c2.Read blocks until a message arrives. In a production test you'd want
	// a timeout (see TestRoomMessageNotDeliveredToNonMembers below) to prevent
	// the test from hanging if the message is never delivered.
	typ, p, err := c2.Read(ctx)
	if err != nil {
		t.Fatalf("bob failed to read: %v", err)
	}

	// Verify the WebSocket message type is text (not binary).
	if typ != websocket.MessageText {
		t.Errorf("expected text message, got %v", typ)
	}

	// Verify the raw payload matches exactly. The server forwards the original
	// bytes for direct messages, so the payload should be identical.
	expected := `{"sender": "alice", "recipient": "bob", "content": "Hello Bob!"}`
	if string(p) != expected {
		t.Errorf("expected %s, got %s", expected, string(p))
	}
}

// TestCreateRoomViaWebSocket tests the create_room command through a real
// WebSocket connection, verifying the server sends a room_created acknowledgment.
//
// LEARNING POINT — Testing Request/Response Over WebSocket:
// This follows a pattern of: send a command, read the response, unmarshal it,
// and verify the fields. This is analogous to HTTP request/response testing
// but over a persistent WebSocket connection.
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

	// Create a room by sending a create_room message.
	// Here we use json.Marshal for type-safe serialization.
	createMsg := Message{
		Type:    "create_room",
		Sender:  "alice",
		Content: "general",
	}
	data, _ := json.Marshal(createMsg)
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send create_room: %v", err)
	}

	// Read the acknowledgment from the server.
	_, p, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read ack: %v", err)
	}

	// Unmarshal and verify the acknowledgment message.
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

// TestInviteAndRoomMessage is an end-to-end test of the room workflow:
// create room -> invite user -> send room message -> verify delivery.
//
// LEARNING POINT — Multi-Step Integration Tests:
// This test exercises a complete user story: Alice creates a room, invites Bob,
// then sends a message that Bob receives. Each step depends on the previous one
// succeeding, which is why t.Fatalf is used — if room creation fails, there's
// no point testing invites.
//
// The test reads intermediate messages (acks) to keep the WebSocket in sync.
// If you don't read acks, subsequent reads would return the ack instead of
// the expected message, causing confusing test failures.
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

	// Step 1: Alice creates a room.
	createMsg := Message{Type: "create_room", Sender: "alice", Content: "devteam"}
	data, _ := json.Marshal(createMsg)
	alice.Write(ctx, websocket.MessageText, data)

	// Read Alice's room_created ack (must consume it to keep the stream in order).
	alice.Read(ctx)

	// Step 2: Alice invites Bob.
	inviteMsg := Message{Type: "invite", Sender: "alice", Room: "devteam", Recipient: "bob"}
	data, _ = json.Marshal(inviteMsg)
	alice.Write(ctx, websocket.MessageText, data)

	// Read Alice's invite_sent ack.
	alice.Read(ctx)

	// Bob should receive an invitation notification.
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

	// Step 3: Alice sends a room message.
	roomMsg := Message{Type: "room_msg", Sender: "alice", Room: "devteam", Content: "Hello team!"}
	data, _ = json.Marshal(roomMsg)
	alice.Write(ctx, websocket.MessageText, data)

	// Bob should receive the room message (since he was invited).
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

// TestRoomMessageNotDeliveredToNonMembers verifies that room messages are NOT
// sent to users who aren't members of the room.
//
// LEARNING POINT — Testing Negative Cases with Timeouts:
// Testing "this should NOT happen" is tricky — how long do you wait before
// concluding it won't happen? The solution is context.WithTimeout: set a short
// deadline and verify that Read returns an error (context deadline exceeded)
// rather than a message.
//
// LEARNING POINT — context.WithTimeout:
// context.WithTimeout(parent, duration) creates a child context that
// automatically cancels after the duration. Any blocking operation using this
// context (like c.Read) will return an error when the timeout expires. The
// cancel function should always be deferred to release resources, even if the
// timeout fires first.
func TestRoomMessageNotDeliveredToNonMembers(t *testing.T) {
	s := &Server{hub: NewHub()}
	mux := SetupRouter(s)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/ws"
	ctx := context.Background()

	// Connect Alice and Charlie.
	alice, _, _ := websocket.Dial(ctx, wsURL+"?user=alice", nil)
	defer alice.Close(websocket.StatusNormalClosure, "")

	charlie, _, _ := websocket.Dial(ctx, wsURL+"?user=charlie", nil)
	defer charlie.Close(websocket.StatusNormalClosure, "")

	// Alice creates a room (only Alice is a member, Charlie is NOT invited).
	createMsg := Message{Type: "create_room", Sender: "alice", Content: "private"}
	data, _ := json.Marshal(createMsg)
	alice.Write(ctx, websocket.MessageText, data)
	alice.Read(ctx) // consume the ack

	// Alice sends a room message.
	roomMsg := Message{Type: "room_msg", Sender: "alice", Room: "private", Content: "secret"}
	data, _ = json.Marshal(roomMsg)
	alice.Write(ctx, websocket.MessageText, data)

	// Charlie should NOT receive anything — use a short timeout to verify.
	// 200ms is enough time for the message to arrive if it's going to,
	// but short enough that the test doesn't slow down the suite.
	readCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_, _, err := charlie.Read(readCtx)
	if err == nil {
		t.Error("expected charlie NOT to receive a message, but got one")
	}
	// If err is non-nil (timeout), the test passes — Charlie correctly
	// did not receive the private room message.
}
