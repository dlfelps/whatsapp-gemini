# Specification: Core WebSocket Messaging Server

## Overview
Implement a real-time messaging server using Go and WebSockets, accompanied by a simple CLI client. This track focuses on the fundamental architecture required for two-way communication and concurrent message handling.

## Requirements
- **Server:**
    - Listen for HTTP connections and upgrade to WebSockets using 
hooyr.io/websocket.
    - Handle multiple concurrent connections using Goroutines.
    - Manage active connections in an in-memory map guarded by a mutex.
    - Broadcast messages to the intended recipient based on a simple JSON protocol.
- **Client:**
    - Connect to the server via WebSocket.
    - Accept user input from the terminal and send it as a JSON message.
    - Display incoming messages in real-time.
- **Protocol (JSON):**
    - Message format: {"sender": "string", "recipient": "string", "content": "string"}

## Technical Constraints
- Language: Go 1.21+
- WebSocket Library: 
hooyr.io/websocket
- Serialization: encoding/json
- State: sync.RWMutex and map[string]*connection
"@

 = @"
# Implementation Plan: Core WebSocket Messaging Server

## Phase 1: Project Initialization & Basic Server
- [ ] Task: Initialize Go module and setup basic HTTP server structure.
    - [ ] Run go mod init whatsapp-clone.
    - [ ] Create main.go with a simple "Hello World" HTTP handler.
    - [ ] Write a test to verify the HTTP server responds.
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Project Initialization & Basic Server' (Protocol in workflow.md)

## Phase 2: WebSocket Integration & Connection Management
- [ ] Task: Implement WebSocket upgrade and basic connection handling.
    - [ ] Add 
hooyr.io/websocket dependency.
    - [ ] Create a /ws endpoint that upgrades connections.
    - [ ] Write tests for connection upgrading.
- [ ] Task: Implement in-memory connection registry.
    - [ ] Define a Hub or Server struct to manage active connections.
    - [ ] Use sync.RWMutex to protect the connection map.
    - [ ] Write unit tests for registering and unregistering connections.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: WebSocket Integration & Connection Management' (Protocol in workflow.md)

## Phase 3: Messaging Logic & Broadcasting
- [ ] Task: Define the messaging protocol and implement message broadcasting.
    - [ ] Define the Message struct with JSON tags.
    - [ ] Implement a loop to read messages from WebSocket connections.
    - [ ] Implement logic to find the recipient in the registry and send the message.
    - [ ] Write integration tests for message delivery between two virtual clients.
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Messaging Logic & Broadcasting' (Protocol in workflow.md)

## Phase 4: CLI Client Implementation
- [ ] Task: Build a basic CLI client to interact with the server.
    - [ ] Implement dial logic to connect to ws://localhost:8080/ws.
    - [ ] Use separate Goroutines for reading from stdin and reading from the WebSocket.
    - [ ] Format and display messages in the terminal.
- [ ] Task: Conductor - User Manual Verification 'Phase 4: CLI Client Implementation' (Protocol in workflow.md)
