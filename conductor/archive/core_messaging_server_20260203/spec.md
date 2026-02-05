# Specification: Core WebSocket Messaging Server

## Overview
Implement a real-time messaging server using Go and WebSockets, accompanied by a simple CLI client. This track focuses on the fundamental architecture required for two-way communication and concurrent message handling.

## Requirements
- **Server:**
    - Listen for HTTP connections and upgrade to WebSockets using `nhooyr.io/websocket`.
    - Handle multiple concurrent connections using Goroutines.
    - Manage active connections in an in-memory map guarded by a mutex.
    - Broadcast messages to the intended recipient based on a simple JSON protocol.
- **Client:**
    - Connect to the server via WebSocket.
    - Accept user input from the terminal and send it as a JSON message.
    - Display incoming messages in real-time.
- **Protocol (JSON):**
    - Message format: `{"sender": "string", "recipient": "string", "content": "string"}`

## Technical Constraints
- Language: Go 1.21+
- WebSocket Library: `nhooyr.io/websocket`
- Serialization: `encoding/json`
- State: `sync.RWMutex` and `map[string]*connection`