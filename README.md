# WhatsApp Clone (Go Learning Project)

A functional, WhatsApp-inspired messaging service built to demonstrate **Go (Golang)** programming concepts, **System Design** principles, and **Real-time Communication** using WebSockets.

## 🚀 Features

-   **Real-time Messaging:** Instant one-on-one messaging between users.
-   **WebSocket Powered:** Uses persistent WebSocket connections for low-latency communication.
-   **Concurrent Architecture:** leveraged Go's Goroutines and Channels to handle multiple clients simultaneously.
-   **In-Memory State:** Thread-safe connection management using `sync.RWMutex`.
-   **CLI Client:** A simple command-line interface to interact with the server.

## 📋 Prerequisites

-   **Go 1.21** or higher installed on your machine.

## 🛠️ Getting Started

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd whatsapp
    ```

2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

## 🏃‍♂️ Usage

To use the messaging system, you need to run the server in one terminal and multiple client instances in separate terminals.

### 1. Start the Server

The server listens on port `8080` and manages all active connections.

```bash
go run ./cmd/server
```
*You should see: `Server starting on :8080`*

### 2. Start Clients

Open a new terminal for each user you want to connect.

**Terminal A (Alice):**
```bash
go run ./cmd/client alice
```

**Terminal B (Bob):**
```bash
go run ./cmd/client bob
```

### 3. Send Messages

In the client CLI, the format to send a message is:
```text
<recipient_username> <message_content>
```

**Example (In Alice's terminal):**
```text
> bob Hello Bob, how are you?
```

**Result (In Bob's terminal):**
```text
[alice]: Hello Bob, how are you?
>
```

## 📂 Project Structure

-   **`cmd/server/`**: Contains the main server logic, WebSocket handling, and connection registry (`Hub`).
-   **`cmd/client/`**: Contains the CLI client implementation.
-   **`conductor/`**: Project management artifacts (plans, specs, guides).

## 🧪 Testing

The project follows Test-Driven Development (TDD) principles. You can run the test suite with:

```bash
go test -v ./...
```
