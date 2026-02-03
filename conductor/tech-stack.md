# Tech Stack - WhatsApp Clone (Learning Project)

## Language & Runtime
- **Language:** Go (Golang)
- **Version:** 1.21+ (utilizing `slog` for structured logging)

## Core Libraries
- **Networking:** Standard library `net/http` for the server.
- **WebSockets:** `nhooyr.io/websocket` for idiomatic WebSocket management.
- **Serialization:** Standard library `encoding/json` for message encoding/decoding.

## State Management
- **In-Memory Storage:** Go `map` guarded by `sync.RWMutex` for tracking active user connections and recent message history.

## Testing & Tooling
- **Unit Testing:** Standard library `testing` package.
- **Formatting:** `gofmt` or `goimports`.
- **Linting:** `golangci-lint` (recommended).
