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

func TestCreateRoom(t *testing.T) {
	h := NewHub()

	errMsg := h.createRoom("general", "alice")
	if errMsg != "" {
		t.Fatalf("unexpected error creating room: %s", errMsg)
	}

	// Room should exist with alice as a member
	if room, ok := h.rooms["general"]; !ok {
		t.Fatal("expected room 'general' to exist")
	} else if !room.Members["alice"] {
		t.Error("expected alice to be a member of 'general'")
	}
}

func TestCreateRoomDuplicate(t *testing.T) {
	h := NewHub()

	h.createRoom("general", "alice")
	errMsg := h.createRoom("general", "bob")
	if errMsg == "" {
		t.Fatal("expected error when creating duplicate room")
	}
}

func TestAddToRoom(t *testing.T) {
	h := NewHub()
	h.createRoom("general", "alice")

	errMsg := h.addToRoom("general", "alice", "bob")
	if errMsg != "" {
		t.Fatalf("unexpected error adding bob: %s", errMsg)
	}

	if !h.rooms["general"].Members["bob"] {
		t.Error("expected bob to be a member of 'general'")
	}
}

func TestAddToRoomNonExistent(t *testing.T) {
	h := NewHub()

	errMsg := h.addToRoom("nonexistent", "alice", "bob")
	if errMsg == "" {
		t.Fatal("expected error when adding to nonexistent room")
	}
}

func TestAddToRoomNotAMember(t *testing.T) {
	h := NewHub()
	h.createRoom("general", "alice")

	errMsg := h.addToRoom("general", "bob", "charlie")
	if errMsg == "" {
		t.Fatal("expected error when non-member tries to invite")
	}
}

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

	memberSet := map[string]bool{}
	for _, m := range members {
		memberSet[m] = true
	}
	if !memberSet["alice"] || !memberSet["bob"] {
		t.Errorf("expected alice and bob in members, got %v", members)
	}
}

func TestGetRoomMembersNonMember(t *testing.T) {
	h := NewHub()
	h.createRoom("general", "alice")

	members := h.getRoomMembers("general", "bob")
	if members != nil {
		t.Error("expected nil when non-member requests room members")
	}
}

func TestGetRoomMembersNonExistent(t *testing.T) {
	h := NewHub()

	members := h.getRoomMembers("nonexistent", "alice")
	if members != nil {
		t.Error("expected nil for nonexistent room")
	}
}
