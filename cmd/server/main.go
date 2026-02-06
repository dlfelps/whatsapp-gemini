package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"nhooyr.io/websocket"
)

type Server struct {
	hub *Hub
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, World!")
}

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received connection request on /ws from %s\n", r.RemoteAddr)
	userID := r.URL.Query().Get("user")
	if userID == "" {
		fmt.Println("Error: user query parameter is missing")
		http.Error(w, "user query parameter is required", http.StatusBadRequest)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		fmt.Printf("Error accepting websocket for user %s: %v\n", userID, err)
		return
	}
	conn := &connection{ws: c}
	s.hub.register(userID, conn)
	defer s.hub.unregister(userID)
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	fmt.Printf("User %s connected successfully\n", userID)

	ctx := r.Context()
	for {
		_, p, err := c.Read(ctx)
		if err != nil {
			fmt.Printf("User %s disconnected: %v\n", userID, err)
			break
		}

		var msg Message
		if err := json.Unmarshal(p, &msg); err != nil {
			fmt.Printf("Error unmarshaling message from %s: %v\n", userID, err)
			continue
		}

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
func (s *Server) handleDirectMessage(ctx context.Context, _ string, msg Message, rawPayload []byte) {
	fmt.Printf("Message from %s to %s: %s\n", msg.Sender, msg.Recipient, msg.Content)

	recipientConn, ok := s.hub.get(msg.Recipient)
	if !ok {
		fmt.Printf("Recipient %s not found for message from %s\n", msg.Recipient, msg.Sender)
		return
	}

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

	// Notify invitee if they are online
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

// sendError sends a server error message back to a client.
func sendError(ctx context.Context, c *websocket.Conn, errMsg string) {
	msg := Message{
		Type:    "error",
		Sender:  "server",
		Content: errMsg,
	}
	sendJSON(ctx, c, msg)
}

func SetupRouter(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", helloHandler)
	mux.HandleFunc("/ws", s.wsHandler)
	return mux
}

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
