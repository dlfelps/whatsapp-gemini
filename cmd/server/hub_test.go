// This file contains unit tests for the Hub type (defined in hub.go).
//
// KEY GO TESTING CONCEPTS IN THIS FILE:
//   - The "testing" package and the *testing.T type
//   - Test function naming convention: TestXxx(t *testing.T)
//   - t.Fatal vs t.Error: Fatal stops the test, Error continues
//   - t.Fatalf/t.Errorf for formatted messages
//   - Testing unexported methods (possible because tests are in the same package)
//   - Direct struct field access in tests for assertions
//
// LEARNING POINT — Go's Testing Philosophy:
// Go has a built-in test runner (go test) and a minimal testing package. There
// are no assertions library, no test classes, no setUp/tearDown methods. Tests
// are just functions that call methods on *testing.T. This simplicity is
// intentional — Go's philosophy is that tests are just code, and you don't need
// a framework to write them. Run tests with: go test ./cmd/server/
package main

import (
	"testing"
)

// TestHubRegisterUnregister verifies that clients can be added to and removed
// from the hub.
//
// LEARNING POINT — Test Function Naming:
// Test functions MUST start with "Test" followed by an uppercase letter. The
// go test tool uses this convention to discover test functions. The name after
// "Test" should describe what's being tested, not how (e.g., TestHubRegister,
// not TestCallsRegisterMethod).
//
// LEARNING POINT — Same-Package Testing:
// This test file uses "package main" (same as hub.go), which gives it access
// to unexported (lowercase) types like connection and unexported struct fields
// like h.clients. This is called "white-box testing." If you used
// "package main_test" (note the _test suffix), you'd only have access to
// exported identifiers, which is "black-box testing." Both are valid — white-box
// is common for unit tests, black-box for integration tests.
func TestHubRegisterUnregister(t *testing.T) {
	h := NewHub()

	clientID := "test-user"

	// &connection{} creates a connection with zero-value fields (ws is nil).
	// This works for testing register/unregister because those methods only
	// store/remove the connection in the map — they never use the ws field.
	// This is a simple form of mocking in Go.
	conn := &connection{} // Mock connection

	h.register(clientID, conn)

	// LEARNING POINT — Direct Map Access in Tests:
	// Accessing h.clients directly is only possible because this test is in
	// the same package. The comma-ok idiom checks both existence and value.
	if _, ok := h.clients[clientID]; !ok {
		t.Errorf("expected client %s to be registered", clientID)
	}

	h.unregister(clientID)

	if _, ok := h.clients[clientID]; ok {
		t.Errorf("expected client %s to be unregistered", clientID)
	}
}

// TestCreateRoom verifies that a room can be created and the creator becomes
// a member.
//
// LEARNING POINT — t.Fatal vs t.Error:
// t.Fatal (and t.Fatalf) immediately stops the current test function. Use it
// when a failure makes subsequent assertions meaningless (e.g., "room doesn't
// exist" means there's no point checking room members).
// t.Error (and t.Errorf) records the failure but continues the test. Use it
// when you want to report multiple failures in one test run.
func TestCreateRoom(t *testing.T) {
	h := NewHub()

	errMsg := h.createRoom("general", "alice")
	if errMsg != "" {
		t.Fatalf("unexpected error creating room: %s", errMsg)
	}

	// Room should exist with alice as a member.
	// The if-init statement (if x, ok := ...; !ok) is idiomatic Go — it limits
	// the scope of the variables to the if block.
	if room, ok := h.rooms["general"]; !ok {
		t.Fatal("expected room 'general' to exist")
	} else if !room.Members["alice"] {
		t.Error("expected alice to be a member of 'general'")
	}
}

// TestCreateRoomDuplicate verifies that creating a room with a duplicate name
// returns an error.
//
// LEARNING POINT — Testing Error Paths:
// It's just as important to test that errors are returned when expected as it
// is to test the happy path. This test verifies the guard clause in createRoom.
func TestCreateRoomDuplicate(t *testing.T) {
	h := NewHub()

	h.createRoom("general", "alice")
	errMsg := h.createRoom("general", "bob")
	if errMsg == "" {
		t.Fatal("expected error when creating duplicate room")
	}
}

// TestAddToRoom verifies that a member can invite another user to a room.
func TestAddToRoom(t *testing.T) {
	h := NewHub()
	h.createRoom("general", "alice")

	errMsg := h.addToRoom("general", "alice", "bob")
	if errMsg != "" {
		t.Fatalf("unexpected error adding bob: %s", errMsg)
	}

	// Verify bob was added using the map[string]bool set pattern.
	if !h.rooms["general"].Members["bob"] {
		t.Error("expected bob to be a member of 'general'")
	}
}

// TestAddToRoomNonExistent verifies the error when adding to a room that
// doesn't exist.
func TestAddToRoomNonExistent(t *testing.T) {
	h := NewHub()

	errMsg := h.addToRoom("nonexistent", "alice", "bob")
	if errMsg == "" {
		t.Fatal("expected error when adding to nonexistent room")
	}
}

// TestAddToRoomNotAMember verifies that a non-member cannot invite others.
func TestAddToRoomNotAMember(t *testing.T) {
	h := NewHub()
	h.createRoom("general", "alice")

	// Bob is not a member, so he shouldn't be able to invite charlie.
	errMsg := h.addToRoom("general", "bob", "charlie")
	if errMsg == "" {
		t.Fatal("expected error when non-member tries to invite")
	}
}

// TestGetRoomMembers verifies that room members are correctly returned.
//
// LEARNING POINT — Testing Unordered Collections:
// Since map iteration order is random in Go, getRoomMembers returns members in
// an unpredictable order. To test membership without depending on order, this
// test converts the slice to a map (set) and checks for specific keys. This is
// a common pattern when testing functions that return unordered results.
func TestGetRoomMembers(t *testing.T) {
	h := NewHub()
	h.createRoom("general", "alice")
	h.addToRoom("general", "alice", "bob")

	members := h.getRoomMembers("general", "alice")
	if members == nil {
		t.Fatal("expected non-nil members list")
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	// Convert slice to map for order-independent comparison.
	memberSet := map[string]bool{}
	for _, m := range members {
		memberSet[m] = true
	}
	if !memberSet["alice"] || !memberSet["bob"] {
		t.Errorf("expected alice and bob in members, got %v", members)
	}
}

// TestGetRoomMembersNonMember verifies that non-members cannot see room members.
func TestGetRoomMembersNonMember(t *testing.T) {
	h := NewHub()
	h.createRoom("general", "alice")

	members := h.getRoomMembers("general", "bob")
	if members != nil {
		t.Error("expected nil when non-member requests room members")
	}
}

// TestGetRoomMembersNonExistent verifies the response for a non-existent room.
func TestGetRoomMembersNonExistent(t *testing.T) {
	h := NewHub()

	members := h.getRoomMembers("nonexistent", "alice")
	if members != nil {
		t.Error("expected nil for nonexistent room")
	}
}
