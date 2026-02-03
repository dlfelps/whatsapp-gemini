# Implementation Plan: Core WebSocket Messaging Server

## Phase 1: Project Initialization & Basic Server [checkpoint: 55a23c2]
- [x] Task: Initialize Go module and setup basic HTTP server structure. (72683a7)
    - [x] Run `go mod init whatsapp-clone`.
    - [x] Create `main.go` with a simple "Hello World" HTTP handler.
    - [x] Write a test to verify the HTTP server responds.
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Project Initialization & Basic Server' (Protocol in workflow.md)

## Phase 2: WebSocket Integration & Connection Management
- [x] Task: Implement WebSocket upgrade and basic connection handling. (2310620)
    - [x] Add `nhooyr.io/websocket` dependency.
    - [x] Create a `/ws` endpoint that upgrades connections.
    - [x] Write tests for connection upgrading.
- [x] Task: Implement in-memory connection registry. (4f3ed31)
    - [x] Define a `Hub` or `Server` struct to manage active connections.
    - [x] Use `sync.RWMutex` to protect the connection map.
    - [x] Write unit tests for registering and unregistering connections.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: WebSocket Integration & Connection Management' (Protocol in workflow.md)

## Phase 3: Messaging Logic & Broadcasting
- [ ] Task: Define the messaging protocol and implement message broadcasting.
    - [ ] Define the `Message` struct with JSON tags.
    - [ ] Implement a loop to read messages from WebSocket connections.
    - [ ] Implement logic to find the recipient in the registry and send the message.
    - [ ] Write integration tests for message delivery between two virtual clients.
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Messaging Logic & Broadcasting' (Protocol in workflow.md)

## Phase 4: CLI Client Implementation
- [ ] Task: Build a basic CLI client to interact with the server.
    - [ ] Implement dial logic to connect to `ws://localhost:8080/ws`.
    - [ ] Use separate Goroutines for reading from stdin and reading from the WebSocket.
    - [ ] Format and display messages in the terminal.
- [ ] Task: Conductor - User Manual Verification 'Phase 4: CLI Client Implementation' (Protocol in workflow.md)
