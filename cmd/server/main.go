// This file is the entry point for the WebSocket chat server. It sets up HTTP
// routing, handles WebSocket upgrades, and dispatches incoming messages to the
// appropriate handler based on message type.
//
// KEY GO CONCEPTS IN THIS FILE:
//   - net/http for building HTTP servers (Go's powerful standard library)
//   - Methods with pointer receivers on the Server struct
//   - context.Context for request-scoped cancellation and deadlines
//   - encoding/json for JSON serialization/deserialization
//   - The http.HandlerFunc pattern for routing
//   - Infinite loops with break/continue for long-lived connections
package main

import (
	// context provides request-scoped values, cancellation signals, and
	// deadlines across API boundaries and goroutines. It's one of Go's most
	// important packages — nearly every network-facing function accepts a
	// context as its first parameter.
	"context"

	// encoding/json provides JSON encoding and decoding. It uses reflection
	// to map between Go structs and JSON, guided by struct tags (see hub.go).
	// Key functions: json.Marshal (Go -> JSON bytes), json.Unmarshal (JSON bytes -> Go).
	"encoding/json"

	// fmt implements formatted I/O. fmt.Fprint writes to an io.Writer (like
	// http.ResponseWriter), fmt.Printf prints to stdout. It's Go's equivalent
	// of printf/sprintf from C.
	"fmt"

	// net/http provides HTTP client and server implementations. Go's HTTP
	// server is production-grade out of the box — many companies use it
	// directly in production without a framework (unlike Node.js/Express or
	// Python/Flask). The standard library is one of Go's biggest strengths.
	"net/http"

	"nhooyr.io/websocket"
)

// Server holds application-level dependencies. This is a common Go pattern for
// dependency injection without a framework — you group shared dependencies in a
// struct and define HTTP handlers as methods on that struct. This way, handlers
// can access the hub (and any future dependencies) through the receiver.
//
// LEARNING POINT — Dependency Injection in Go:
// Go doesn't have a DI framework like Spring (Java) or Nest (TypeScript).
// Instead, you pass dependencies explicitly through struct fields or function
// parameters. This is intentional: Go values explicitness over magic.
type Server struct {
	hub *Hub
}

// helloHandler is a simple HTTP handler that responds with "Hello, World!".
//
// LEARNING POINT — http.HandlerFunc Signature:
// Any function with the signature func(http.ResponseWriter, *http.Request) can
// be used as an HTTP handler. http.ResponseWriter is an interface that lets you
// write the HTTP response (status code, headers, body). *http.Request contains
// all information about the incoming request (method, URL, headers, body).
//
// fmt.Fprint writes to any io.Writer. Since http.ResponseWriter implements
// io.Writer, we can write the response body directly with fmt.Fprint.
func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, World!")
}

// wsHandler upgrades an HTTP connection to a WebSocket connection and enters
// the main message-processing loop for that client.
//
// LEARNING POINT — WebSocket Upgrade:
// WebSocket connections start as regular HTTP requests. The client sends an
// "Upgrade" header, and the server responds with a 101 status code to switch
// protocols. The websocket.Accept() call handles this entire handshake.
//
// LEARNING POINT — defer for Cleanup:
// Notice the two defers: one to unregister the user from the hub, another to
// close the WebSocket. defers execute in LIFO (last-in, first-out) order when
// the function returns, so the WebSocket closes first, then the user is
// unregistered. This guarantees cleanup even if the loop exits due to an error.
//
// LEARNING POINT — Infinite Loop Pattern:
// The for { ... } loop is Go's "while true" — it reads messages until the
// client disconnects (which causes c.Read to return an error, breaking the
// loop). This is the standard pattern for long-lived connections in Go.
func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received connection request on /ws from %s\n", r.RemoteAddr)

	// Extract the "user" query parameter from the URL (e.g., /ws?user=alice).
	// r.URL.Query() parses the query string into a map of key -> []string,
	// and .Get() returns the first value for the key (or "" if missing).
	userID := r.URL.Query().Get("user")
	if userID == "" {
		fmt.Println("Error: user query parameter is missing")
		// http.Error is a convenience function that writes an error message
		// and sets the appropriate HTTP status code in one call.
		http.Error(w, "user query parameter is required", http.StatusBadRequest)
		return
	}

	// Accept upgrades the HTTP connection to a WebSocket connection.
	// InsecureSkipVerify: true disables origin checking — fine for development,
	// but in production you should validate the Origin header to prevent
	// cross-site WebSocket hijacking (CSWSH).
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		fmt.Printf("Error accepting websocket for user %s: %v\n", userID, err)
		return
	}

	// Wrap the raw WebSocket in our connection struct and register with the hub.
	conn := &connection{ws: c}
	s.hub.register(userID, conn)

	// defer runs these cleanup functions when wsHandler returns (in reverse order).
	defer s.hub.unregister(userID)
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	fmt.Printf("User %s connected successfully\n", userID)

	// r.Context() returns the request's context, which is automatically
	// cancelled when the client disconnects. Passing it to c.Read() means
	// the read will unblock if the HTTP connection is closed.
	ctx := r.Context()

	// Main message loop: read messages until the client disconnects.
	for {
		// c.Read blocks until a message arrives. The first return value is
		// the message type (text or binary); we use _ to discard it since
		// we only expect text messages.
		_, p, err := c.Read(ctx)
		if err != nil {
			fmt.Printf("User %s disconnected: %v\n", userID, err)
			break
		}

		// json.Unmarshal parses the JSON byte slice into a Message struct.
		// If the JSON is malformed, we log the error and continue to the
		// next message (don't disconnect the client for a bad message).
		var msg Message
		if err := json.Unmarshal(p, &msg); err != nil {
			fmt.Printf("Error unmarshaling message from %s: %v\n", userID, err)
			continue
		}

		// LEARNING POINT — Type-based Dispatch with switch:
		// Go's switch statement doesn't need "break" — each case automatically
		// breaks unless you use "fallthrough". The default case handles any
		// unrecognized message type, providing backwards compatibility.
		switch msg.Type {
		case "create_room":
			s.handleCreateRoom(ctx, userID, msg, c)
		case "invite":
			s.handleInvite(ctx, userID, msg, c)
		case "room_msg":
			s.handleRoomMessage(ctx, userID, msg)
		default:
			// Direct message (original behavior, backwards compatible)
			s.handleDirectMessage(ctx, userID, msg, p)
		}
	}
}

// handleDirectMessage routes a message to a single recipient (original behavior).
//
// LEARNING POINT — Blank Identifier:
// The second parameter is "_ string" (the userID). The underscore (_) tells Go
// "I'm intentionally not using this parameter." This is required because Go
// doesn't allow unused variables — it's a compile error. The blank identifier
// is a signal to readers that the parameter exists for interface conformity but
// isn't needed in this particular implementation.
func (s *Server) handleDirectMessage(ctx context.Context, _ string, msg Message, rawPayload []byte) {
	fmt.Printf("Message from %s to %s: %s\n", msg.Sender, msg.Recipient, msg.Content)

	// Look up the recipient's connection in the hub using the comma-ok idiom.
	recipientConn, ok := s.hub.get(msg.Recipient)
	if !ok {
		fmt.Printf("Recipient %s not found for message from %s\n", msg.Recipient, msg.Sender)
		return
	}

	// Write the raw JSON payload directly to the recipient's WebSocket.
	// We reuse rawPayload (the original bytes) instead of re-marshaling,
	// which avoids unnecessary serialization work.
	if err := recipientConn.ws.Write(ctx, websocket.MessageText, rawPayload); err != nil {
		fmt.Printf("Error sending message to %s: %v\n", msg.Recipient, err)
	}
}

// handleCreateRoom creates a new chat room with the sender as the first member.
// Sends an acknowledgment or error back to the creator.
func (s *Server) handleCreateRoom(ctx context.Context, userID string, msg Message, c *websocket.Conn) {
	roomName := msg.Content
	if roomName == "" {
		roomName = msg.Room
	}
	if roomName == "" {
		sendError(ctx, c, "room name is required")
		return
	}

	if errMsg := s.hub.createRoom(roomName, userID); errMsg != "" {
		sendError(ctx, c, errMsg)
		return
	}

	// LEARNING POINT — Struct Literals:
	// Go allows you to create struct values inline with named fields.
	// This is more readable than positional arguments, especially for
	// structs with many fields. You only need to specify the fields you
	// want to set — unspecified fields get their zero values.
	ack := Message{
		Type:    "room_created",
		Sender:  "server",
		Room:    roomName,
		Content: fmt.Sprintf("room %q created successfully", roomName),
	}
	sendJSON(ctx, c, ack)
}

// handleInvite adds a user to a chat room and notifies both the inviter and invitee.
func (s *Server) handleInvite(ctx context.Context, userID string, msg Message, c *websocket.Conn) {
	roomName := msg.Room
	invitee := msg.Recipient
	if roomName == "" || invitee == "" {
		sendError(ctx, c, "room and recipient are required for invite")
		return
	}

	if errMsg := s.hub.addToRoom(roomName, userID, invitee); errMsg != "" {
		sendError(ctx, c, errMsg)
		return
	}

	// Acknowledge to inviter
	ack := Message{
		Type:    "invite_sent",
		Sender:  "server",
		Room:    roomName,
		Content: fmt.Sprintf("user %q invited to room %q", invitee, roomName),
	}
	sendJSON(ctx, c, ack)

	// Notify invitee if they are online.
	// This is a "best effort" notification — if the invitee is offline,
	// they simply won't receive the notification. A production system
	// might store pending notifications for delivery when the user reconnects.
	if inviteeConn, ok := s.hub.get(invitee); ok {
		notify := Message{
			Type:    "invited",
			Sender:  userID,
			Room:    roomName,
			Content: fmt.Sprintf("you have been invited to room %q by %s", roomName, userID),
		}
		sendJSON(ctx, inviteeConn.ws, notify)
	}
}

// handleRoomMessage broadcasts a message to all online members of a room
// except the sender.
//
// LEARNING POINT — Fan-out Pattern:
// This function demonstrates a simple fan-out: one incoming message is sent to
// multiple recipients. The for loop iterates over all room members and writes
// to each one individually. In a high-throughput system, you might use
// goroutines for parallel writes, but for simplicity this does them sequentially.
func (s *Server) handleRoomMessage(ctx context.Context, userID string, msg Message) {
	roomName := msg.Room
	if roomName == "" {
		fmt.Printf("Room message from %s missing room name\n", userID)
		return
	}

	members := s.hub.getRoomMembers(roomName, userID)
	if members == nil {
		fmt.Printf("User %s cannot send to room %q (not a member or room doesn't exist)\n", userID, roomName)
		return
	}

	fmt.Printf("Room message from %s in %q: %s\n", userID, roomName, msg.Content)

	// Build the outgoing message once and marshal it once, then send the
	// same bytes to every recipient. This is more efficient than marshaling
	// per-recipient.
	outMsg := Message{
		Type:    "room_msg",
		Sender:  userID,
		Room:    roomName,
		Content: msg.Content,
	}
	data, err := json.Marshal(outMsg)
	if err != nil {
		fmt.Printf("Error marshaling room message: %v\n", err)
		return
	}

	for _, memberID := range members {
		// Skip the sender — they already know what they sent.
		if memberID == userID {
			continue
		}
		if memberConn, ok := s.hub.get(memberID); ok {
			if err := memberConn.ws.Write(ctx, websocket.MessageText, data); err != nil {
				fmt.Printf("Error sending room message to %s: %v\n", memberID, err)
			}
		}
	}
}

// sendJSON marshals a message and writes it to the WebSocket connection.
//
// LEARNING POINT — Helper Functions:
// Small utility functions like sendJSON reduce repetition and centralize error
// handling. In Go, it's idiomatic to keep helpers in the same file where they're
// used, rather than creating a separate "utils" package. Go favors flat package
// structures over deep hierarchies.
func sendJSON(ctx context.Context, c *websocket.Conn, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Error marshaling message: %v\n", err)
		return
	}
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		fmt.Printf("Error writing message: %v\n", err)
	}
}

// sendError sends a server error message back to a client. This is a thin
// wrapper around sendJSON that constructs an error-typed Message.
func sendError(ctx context.Context, c *websocket.Conn, errMsg string) {
	msg := Message{
		Type:    "error",
		Sender:  "server",
		Content: errMsg,
	}
	sendJSON(ctx, c, msg)
}

// SetupRouter creates and configures the HTTP request multiplexer (router).
//
// LEARNING POINT — http.ServeMux:
// http.ServeMux is Go's built-in HTTP request router. It matches incoming
// request URLs to registered handler functions. While simple, it's sufficient
// for many applications. For more advanced routing (path parameters, middleware
// chains, regex patterns), third-party routers like chi or gorilla/mux are popular.
//
// LEARNING POINT — Exported vs Unexported:
// SetupRouter starts with an uppercase letter, making it "exported" (public).
// This is intentional — it allows test files to call SetupRouter to create a
// testable server instance without starting a real HTTP listener. This is a
// common Go testing pattern: export the router setup, test against it with
// httptest.NewServer.
func SetupRouter(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", helloHandler)
	mux.HandleFunc("/ws", s.wsHandler)
	return mux
}

// main is the entry point of the program. It creates the server, sets up
// routes, and starts listening for HTTP connections.
//
// LEARNING POINT — http.ListenAndServe:
// This single function call starts a production-capable HTTP server. It binds
// to the given address (":8080" means all interfaces, port 8080) and serves
// requests using the provided handler (our mux). It blocks forever, only
// returning if the server encounters a fatal error (e.g., port already in use).
//
// In Go, the main function takes no arguments and returns no value. The program
// exits when main returns. Command-line arguments are accessed via os.Args,
// and exit codes are set with os.Exit().
func main() {
	server := &Server{
		hub: NewHub(),
	}
	mux := SetupRouter(server)
	fmt.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
