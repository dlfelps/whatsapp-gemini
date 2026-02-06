package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"nhooyr.io/websocket"
)

type Message struct {
	Type      string `json:"type"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
	Room      string `json:"room,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/client/main.go <username>")
		return
	}
	username := os.Args[1]

	ctx := context.Background()
	url := "ws://localhost:8080/ws?user=" + username
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	fmt.Printf("Connected to server as %s\n", username)
	fmt.Println("Commands:")
	fmt.Println("  <recipient> <message>          - send direct message")
	fmt.Println("  /create <room>                 - create a chat room")
	fmt.Println("  /invite <room> <user>          - invite user to a room")
	fmt.Println("  /room <room> <message>         - send message to a room")

	// Read messages from server in a separate Goroutine
	go func() {
		for {
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

	// Read from stdin and send messages
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for scanner.Scan() {
		line := scanner.Text()
		var msg Message

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
			// Direct message (original behavior)
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

		p, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error encoding message: %v", err)
			continue
		}

		err = c.Write(ctx, websocket.MessageText, p)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			break
		}
		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from stdin: %v", err)
	}
}
