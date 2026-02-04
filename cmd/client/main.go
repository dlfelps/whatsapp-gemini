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
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
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
	fmt.Println("Format: <recipient> <message>")

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
			fmt.Printf("\n[%s]: %s\n> ", msg.Sender, msg.Content)
		}
	}()

	// Read from stdin and send messages
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			fmt.Println("Invalid format. Use: <recipient> <message>")
			fmt.Print("> ")
			continue
		}

		msg := Message{
			Sender:    username,
			Recipient: parts[0],
			Content:   parts[1],
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
