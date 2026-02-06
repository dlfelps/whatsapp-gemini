package main

import (
	"fmt"
	"sync"

	"nhooyr.io/websocket"
)

type connection struct {
	ws *websocket.Conn
}

// Message represents a chat message or command sent between clients and the server.
// The Type field determines how the message is routed:
//   - "" or "dm": direct message to a single recipient (original behavior)
//   - "create_room": create a new chat room (Content = room name)
//   - "invite": invite a user to a room (Recipient = user, Room = room name)
//   - "room_msg": send a message to all members of a room
type Message struct {
	Type      string `json:"type"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
	Room      string `json:"room,omitempty"`
}

// Room represents a chat room with a set of members.
// Only members can send and receive messages in the room.
type Room struct {
	Name    string
	Members map[string]bool
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*connection
	rooms   map[string]*Room
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*connection),
		rooms:   make(map[string]*Room),
	}
}

func (h *Hub) register(id string, conn *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[id] = conn
	fmt.Printf("Registered client: %s (Total: %d)\n", id, len(h.clients))
}

func (h *Hub) unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, id)
	fmt.Printf("Unregistered client: %s (Total: %d)\n", id, len(h.clients))
}

func (h *Hub) get(id string) (*connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.clients[id]
	return conn, ok
}

// createRoom creates a new chat room and adds the creator as the first member.
// Returns an error message if the room already exists.
func (h *Hub) createRoom(name, creator string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.rooms[name]; exists {
		return fmt.Sprintf("room %q already exists", name)
	}
	h.rooms[name] = &Room{
		Name:    name,
		Members: map[string]bool{creator: true},
	}
	fmt.Printf("Room %q created by %s\n", name, creator)
	return ""
}

// addToRoom adds a user to an existing room.
// Returns an error message if the room doesn't exist or the inviter is not a member.
func (h *Hub) addToRoom(roomName, inviter, invitee string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	room, exists := h.rooms[roomName]
	if !exists {
		return fmt.Sprintf("room %q does not exist", roomName)
	}
	if !room.Members[inviter] {
		return fmt.Sprintf("you are not a member of room %q", roomName)
	}
	room.Members[invitee] = true
	fmt.Printf("User %s invited %s to room %q\n", inviter, invitee, roomName)
	return ""
}

// getRoomMembers returns the list of member IDs for a room.
// Returns nil if the room doesn't exist or the requester is not a member.
func (h *Hub) getRoomMembers(roomName, requester string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room, exists := h.rooms[roomName]
	if !exists {
		return nil
	}
	if !room.Members[requester] {
		return nil
	}
	members := make([]string, 0, len(room.Members))
	for m := range room.Members {
		members = append(members, m)
	}
	return members
}
