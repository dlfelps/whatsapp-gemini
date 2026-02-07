// Package main implements a WebSocket chat server.
//
// This file defines the Hub, which is the central coordination point for all
// connected clients and chat rooms. The Hub pattern is a common Go idiom for
// managing shared state in concurrent applications — instead of letting each
// goroutine access shared data directly, a single "hub" (or "broker") owns the
// data and provides thread-safe methods for access.
//
// KEY GO CONCEPTS IN THIS FILE:
//   - sync.RWMutex for concurrent map access (read-heavy workloads)
//   - Struct embedding and composition over inheritance
//   - Constructor functions (NewHub) — Go's replacement for constructors
//   - The "comma ok" idiom for map lookups
//   - defer for automatic resource cleanup (mutex unlocking)
package main

import (
	"fmt"
	"sync"

	// nhooyr.io/websocket is a popular, minimal WebSocket library for Go.
	// It's preferred over the older gorilla/websocket for new projects because
	// it has a smaller API surface, supports context.Context natively, and
	// works with net/http without needing a separate upgrader.
	"nhooyr.io/websocket"
)

// connection wraps a WebSocket connection. This thin wrapper struct is a common
// Go pattern — it lets you attach additional per-connection state later (e.g.,
// send channels, metadata) without changing the Hub's interface.
//
// In Go, lowercase struct names (unexported) are only visible within the same
// package. This is intentional: connection is an internal implementation detail.
type connection struct {
	ws *websocket.Conn
}

// Message represents a chat message or command sent between clients and the server.
//
// LEARNING POINT — Struct Tags:
// The `json:"..."` annotations are "struct tags". They tell the encoding/json
// package how to serialize/deserialize this struct. For example:
//   - `json:"type"` maps the Go field "Type" to JSON key "type"
//   - `json:"room,omitempty"` omits the "room" key entirely when Room is empty
//
// This is how Go handles the mismatch between Go's PascalCase convention and
// JSON's camelCase/lowercase convention.
//
// The Type field determines how the message is routed:
//   - "" (empty) or unrecognized: direct message to a single recipient
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
//
// LEARNING POINT — map[string]bool as a Set:
// Go doesn't have a built-in Set type. The idiomatic workaround is to use
// map[string]bool (or map[string]struct{} for zero-memory overhead). With
// map[string]bool, checking membership is simply: if room.Members["alice"] { ... }
// A missing key returns the zero value (false), so it naturally reads as
// "alice is not a member."
type Room struct {
	Name    string
	Members map[string]bool
}

// Hub is the central registry that tracks all connected clients and chat rooms.
//
// LEARNING POINT — sync.RWMutex:
// A sync.RWMutex (read-write mutex) allows multiple concurrent readers OR one
// exclusive writer. This is more efficient than a plain sync.Mutex when reads
// vastly outnumber writes — which is typical for a chat server where message
// routing (reads) happens far more often than connect/disconnect (writes).
//
// The mutex protects BOTH the clients and rooms maps. In Go, maps are NOT safe
// for concurrent use. Any concurrent read + write (or write + write) to a map
// will cause a runtime panic. The mutex prevents this.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*connection
	rooms   map[string]*Room
}

// NewHub creates and returns a new Hub with initialized maps.
//
// LEARNING POINT — Constructor Functions:
// Go doesn't have constructors. Instead, the convention is to provide a
// NewXxx() function. This is important here because Go's zero value for a map
// is nil, and writing to a nil map causes a runtime panic. NewHub ensures the
// maps are properly initialized with make().
//
// Returning a pointer (*Hub) is idiomatic when the struct will be shared and
// mutated by multiple goroutines.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*connection),
		rooms:   make(map[string]*Room),
	}
}

// register adds a client connection to the hub, keyed by their user ID.
//
// LEARNING POINT — Method Receivers:
// The (h *Hub) before the function name is a "pointer receiver". This means
// register is a method on *Hub that can modify the Hub's state. If we used a
// value receiver (h Hub), Go would pass a copy and our changes would be lost.
//
// LEARNING POINT — defer:
// "defer h.mu.Unlock()" schedules the Unlock to run when this function returns,
// no matter how it returns (normal return, panic, etc.). This pattern of
// Lock + defer Unlock is the standard way to use mutexes in Go — it guarantees
// the lock is always released, even if a panic occurs between Lock and Unlock.
func (h *Hub) register(id string, conn *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[id] = conn
	fmt.Printf("Registered client: %s (Total: %d)\n", id, len(h.clients))
}

// unregister removes a client connection from the hub.
//
// LEARNING POINT — delete() built-in:
// delete(map, key) removes a key from a map. It's a no-op if the key doesn't
// exist (no error, no panic). This is safe to call without checking existence.
func (h *Hub) unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, id)
	fmt.Printf("Unregistered client: %s (Total: %d)\n", id, len(h.clients))
}

// get retrieves a client connection by user ID.
//
// LEARNING POINT — RLock for Read-Only Access:
// We use RLock/RUnlock (read lock) instead of Lock/Unlock because this method
// only reads from the map. Multiple goroutines can hold a read lock
// simultaneously, which gives better performance under concurrent load.
//
// LEARNING POINT — The "Comma Ok" Idiom:
// The two-value map lookup (conn, ok := h.clients[id]) is one of Go's most
// common patterns. 'ok' is true if the key exists, false otherwise. This lets
// callers distinguish between "key exists with zero value" and "key missing".
func (h *Hub) get(id string) (*connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.clients[id]
	return conn, ok
}

// createRoom creates a new chat room and adds the creator as the first member.
// Returns an empty string on success, or an error message string on failure.
//
// LEARNING POINT — Error Handling Style:
// This function returns a string error message instead of the standard error
// interface. While Go convention typically favors returning an error type
// (e.g., func createRoom(...) error), returning a string is sometimes used
// for simple cases where the error message will be sent directly to a user.
// In production code, you'd more commonly see: func createRoom(...) error
// and use fmt.Errorf("room %q already exists", name) to create the error.
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

// addToRoom adds a user to an existing room. Only existing members can invite.
// Returns an empty string on success, or an error message string on failure.
//
// LEARNING POINT — Guard Clauses:
// The early returns for "room doesn't exist" and "not a member" are called
// "guard clauses." This is idiomatic Go style — handle error cases first and
// return early, keeping the "happy path" at the lowest indentation level.
// This avoids deeply nested if/else chains and makes code easier to read.
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
//
// LEARNING POINT — Slice Pre-allocation with make():
// make([]string, 0, len(room.Members)) creates a slice with length 0 but
// capacity equal to the number of members. This is a performance optimization:
// append() won't need to reallocate and copy the underlying array as it grows,
// because we already reserved enough space. This matters when you know the
// final size ahead of time.
//
// LEARNING POINT — Iterating Maps with range:
// "for m := range room.Members" iterates over map keys. When you only need
// keys (not values), you can omit the second variable. The iteration order is
// intentionally randomized by Go's runtime to prevent code from depending on
// a specific order.
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
