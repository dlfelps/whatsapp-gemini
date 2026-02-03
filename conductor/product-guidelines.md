# Product Guidelines - WhatsApp Clone (Learning Project)

## Technical Philosophy
- **Idiomatic Go First:** Always prefer the "Go way" of doing things (e.g., explicit error handling, using interfaces for abstraction, and concurrency via CSP).
- **Educational Clarity:** The codebase should serve as a learning resource. Logic should be transparent, and complex Go patterns should be explained.
- **Minimal Dependencies:** Prioritize the Go standard library to understand the low-level mechanics of network programming and concurrency.

## Code Guidelines
- **Concurrency:** Use Goroutines for asynchronous tasks and Channels for communication between them. Avoid shared state and mutexes where a CSP (Communicating Sequential Processes) approach is clearer.
- **Error Handling:** Errors must be handled explicitly. Wrap errors with descriptive context to make debugging easier (e.g., `fmt.Errorf("starting server: %w", err)`).
- **Naming:** Follow standard Go naming conventions (e.g., `camelCase` for private, `PascalCase` for public, short but descriptive names for local variables).
- **Comments:** Use "Educational Comments" to explain *why* a specific Go feature or design pattern is being used, especially when it relates to learning System Design or Go idioms.

## Architectural Guidelines
- **Monolithic but Modular:** Keep the code in a single repository and binary for simplicity, but organize it into logical packages (e.g., `messaging`, `network`, `state`).
- **Standard Library Networking:** Use `net/http` for the server and the `crypto/rand` for any necessary unique identifiers.

## User Experience (CLI)
- **Simplicity:** The CLI should be straightforward to start and use.
- **Clear Feedback:** Provide clear logs or messages for connection status, message delivery, and errors.
