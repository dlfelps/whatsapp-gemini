// Package main implements a command-line WebSocket chat client.
//
// This client connects to the chat server via WebSocket and provides an
// interactive terminal interface for sending direct messages, creating rooms,
// inviting users, and sending room messages.
//
// KEY GO CONCEPTS IN THIS FILE:
//   - os.Args for command-line argument parsing
//   - Goroutines for concurrent read/write on the same connection
//   - bufio.Scanner for line-by-line stdin reading
//   - strings package for text parsing and manipulation
//   - The log package vs fmt for error output
//   - context.Background() as the root context
package main

import (
	// bufio provides buffered I/O. bufio.Scanner is the idiomatic way to read
	// input line-by-line from any io.Reader (here, os.Stdin). It handles
	// buffering and newline splitting automatically.
	"bufio"

	// context.Background() creates a "root" context that is never cancelled.
	// It's used when there's no parent context to derive from — typically at
	// the top level of main() or in background goroutines.
	"context"

	// encoding/json handles JSON serialization. Note that this client defines
	// its own Message struct (same as the server's). In a larger project, you'd
	// put shared types in a separate package to avoid duplication. This is fine
	// for a small project but would become a maintenance burden at scale.
	"encoding/json"
	"fmt"

	// log provides simple logging with timestamps and automatic newlines.
	// log.Fatalf logs a message and then calls os.Exit(1), which is useful for
	// unrecoverable errors during startup. log.Printf is like fmt.Printf but
	// adds a timestamp prefix.
	"log"

	// os provides platform-independent OS functionality. os.Args contains
	// command-line arguments (os.Args[0] is the program name). os.Stdin is
	// the standard input stream.
	"os"

	// strings provides functions for manipulating UTF-8 encoded strings.
	// Common functions used here: HasPrefix (check prefix), TrimPrefix (remove
	// prefix), SplitN (split into at most N parts), TrimSpace (strip whitespace).
	"strings"

	"nhooyr.io/websocket"
)

// Message mirrors the server's Message struct. Both client and server must
// agree on this JSON format to communicate.
//
// LEARNING POINT — Duplicate Types Across Packages:
// In Go, types are package-scoped. The client and server are separate packages
// (both "package main" but in different directories), so they can't share
// types directly. Solutions for larger projects:
//   - Create a shared package (e.g., "pkg/models") with common types
//   - Use code generation (protobuf, OpenAPI) to generate types for both
//   - For small projects like this, duplicating the struct is acceptable
type Message struct {
	Type      string `json:"type"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
	Room      string `json:"room,omitempty"`
}

// main is the entry point for the chat client. It connects to the server,
// starts a goroutine for reading incoming messages, and processes user input
// from stdin in the main goroutine.
//
// LEARNING POINT — Program Structure:
// This follows a common Go CLI pattern:
//  1. Parse arguments and validate input
//  2. Establish connections / initialize resources
//  3. Start background goroutines for async work
//  4. Run the main event loop in the foreground
//  5. Clean up with defer statements
func main() {
	// LEARNING POINT — os.Args:
	// os.Args is a []string slice. os.Args[0] is the program name, and
	// os.Args[1:] are the user-provided arguments. Unlike flags.Parse(),
	// this is manual argument handling — suitable for simple CLIs with
	// one or two positional arguments. For complex CLIs, use the "flag"
	// package or third-party libraries like cobra or urfave/cli.
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/client/main.go <username>")
		return
	}
	username := os.Args[1]

	// LEARNING POINT — context.Background():
	// context.Background() returns an empty, non-nil context. It's the
	// conventional root context when you don't have a parent context (like
	// an HTTP request context). It's never cancelled, has no deadline, and
	// carries no values. All other contexts in your program should derive
	// from this one (or from an HTTP request context).
	ctx := context.Background()

	// LEARNING POINT — WebSocket Dial:
	// websocket.Dial initiates a WebSocket connection. It performs the HTTP
	// upgrade handshake and returns a *websocket.Conn. The second return
	// value (*http.Response) contains the server's upgrade response — we
	// discard it here with _ since we don't need the response headers.
	url := "ws://localhost:8080/ws?user=" + username
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		// log.Fatalf logs the error message and immediately exits with
		// status code 1. Use it for fatal startup errors where continuing
		// is impossible (can't connect, can't open files, etc.).
		log.Fatalf("failed to dial: %v", err)
	}

	// LEARNING POINT — defer for Connection Cleanup:
	// This ensures the WebSocket connection is properly closed when main()
	// returns, regardless of how it returns (normal exit, error, etc.).
	// StatusNormalClosure (1000) tells the server this was an intentional
	// close, not an error.
	defer c.Close(websocket.StatusNormalClosure, "")

	fmt.Printf("Connected to server as %s\n", username)
	fmt.Println("Commands:")
	fmt.Println("  <recipient> <message>          - send direct message")
	fmt.Println("  /create <room>                 - create a chat room")
	fmt.Println("  /invite <room> <user>          - invite user to a room")
	fmt.Println("  /room <room> <message>         - send message to a room")

	// LEARNING POINT — Goroutines:
	// "go func() { ... }()" launches a new goroutine — a lightweight thread
	// managed by the Go runtime. Goroutines are extremely cheap (~2KB stack)
	// and multiplexed onto OS threads by Go's scheduler. You can run thousands
	// (or millions) of goroutines without issues.
	//
	// Here we use a goroutine for the "read loop" because WebSocket
	// communication is full-duplex: we need to read incoming messages AND
	// read user input simultaneously. The goroutine handles reading from the
	// server, while the main goroutine handles reading from stdin.
	//
	// LEARNING POINT — Anonymous Functions:
	// The func() { ... }() syntax defines and immediately invokes an anonymous
	// function (closure). It captures variables from the enclosing scope (ctx, c)
	// by reference. This is the standard way to launch goroutines with access
	// to local variables.
	go func() {
		for {
			// Block until a message arrives from the server.
			_, p, err := c.Read(ctx)
			if err != nil {
				log.Printf("Disconnected from server: %v", err)
				return
			}
			var msg Message
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Printf("Error decoding message: %v", err)
				continue
			}

			// LEARNING POINT — switch without a Condition:
			// Go's switch can match on any expression, not just a single
			// variable. Here we switch on msg.Type to format different
			// message types differently. Each case could also contain
			// complex logic — switch is often preferred over if/else chains
			// in Go for readability.
			//
			// The "\n> " at the end of each Printf restores the input prompt
			// after printing the incoming message, since the message
			// interrupts the user's typing line.
			switch msg.Type {
			case "room_msg":
				fmt.Printf("\n[%s][%s]: %s\n> ", msg.Room, msg.Sender, msg.Content)
			case "room_created", "invite_sent":
				fmt.Printf("\n[server]: %s\n> ", msg.Content)
			case "invited":
				fmt.Printf("\n[server]: %s\n> ", msg.Content)
			case "error":
				fmt.Printf("\n[error]: %s\n> ", msg.Content)
			default:
				fmt.Printf("\n[%s]: %s\n> ", msg.Sender, msg.Content)
			}
		}
	}()

	// LEARNING POINT — bufio.Scanner for Line Input:
	// bufio.NewScanner(os.Stdin) creates a scanner that reads stdin line by
	// line. scanner.Scan() returns true if a line was read, false on EOF or
	// error. scanner.Text() returns the line without the trailing newline.
	// This is the idiomatic way to read interactive terminal input in Go.
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for scanner.Scan() {
		line := scanner.Text()
		var msg Message

		// LEARNING POINT — switch with Conditions (Expression Switch):
		// "switch { case condition: ... }" is Go's version of if/else if chains,
		// but more readable. Each case is evaluated top-to-bottom, and the first
		// matching case executes. This is preferred over long if/else if chains
		// when you have multiple conditions to check.
		//
		// LEARNING POINT — strings.HasPrefix / strings.TrimPrefix:
		// These are from the standard "strings" package. HasPrefix checks if a
		// string starts with a given prefix. TrimPrefix removes the prefix.
		// Using these together is a common pattern for simple command parsing.
		switch {
		case strings.HasPrefix(line, "/create "):
			roomName := strings.TrimPrefix(line, "/create ")
			roomName = strings.TrimSpace(roomName)
			if roomName == "" {
				fmt.Println("Usage: /create <room>")
				fmt.Print("> ")
				continue
			}
			msg = Message{
				Type:    "create_room",
				Sender:  username,
				Content: roomName,
			}

		case strings.HasPrefix(line, "/invite "):
			// LEARNING POINT — strings.SplitN:
			// SplitN splits a string into at most N substrings. Using N=2 means
			// "split on the first space only", which is perfect for parsing
			// "<room> <user>" where the user might have spaces (though unlikely).
			// SplitN is safer than Split when you want to limit the number of parts.
			parts := strings.SplitN(strings.TrimPrefix(line, "/invite "), " ", 2)
			if len(parts) < 2 {
				fmt.Println("Usage: /invite <room> <user>")
				fmt.Print("> ")
				continue
			}
			msg = Message{
				Type:      "invite",
				Sender:    username,
				Room:      strings.TrimSpace(parts[0]),
				Recipient: strings.TrimSpace(parts[1]),
			}

		case strings.HasPrefix(line, "/room "):
			parts := strings.SplitN(strings.TrimPrefix(line, "/room "), " ", 2)
			if len(parts) < 2 {
				fmt.Println("Usage: /room <room> <message>")
				fmt.Print("> ")
				continue
			}
			msg = Message{
				Type:    "room_msg",
				Sender:  username,
				Room:    strings.TrimSpace(parts[0]),
				Content: parts[1],
			}

		default:
			// Direct message (original behavior): "<recipient> <message>"
			parts := strings.SplitN(line, " ", 2)
			if len(parts) < 2 {
				fmt.Println("Invalid format. Use: <recipient> <message>")
				fmt.Print("> ")
				continue
			}
			msg = Message{
				Sender:    username,
				Recipient: parts[0],
				Content:   parts[1],
			}
		}

		// LEARNING POINT — json.Marshal:
		// json.Marshal converts a Go struct to JSON bytes ([]byte). It uses the
		// struct tags we defined on Message to determine the JSON field names.
		// It returns ([]byte, error) — the error is non-nil if the struct contains
		// types that can't be serialized to JSON (channels, functions, etc.).
		p, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error encoding message: %v", err)
			continue
		}

		// Write sends the JSON message over the WebSocket. If writing fails
		// (server disconnected), we break out of the input loop.
		err = c.Write(ctx, websocket.MessageText, p)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			break
		}
		fmt.Print("> ")
	}

	// LEARNING POINT — scanner.Err():
	// After the scan loop ends, always check scanner.Err(). scanner.Scan()
	// returns false for both EOF (normal) and errors (abnormal). scanner.Err()
	// returns nil for EOF and the actual error otherwise. This is a commonly
	// missed check in Go — always verify after a scanner loop.
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from stdin: %v", err)
	}
}
