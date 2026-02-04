package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"nhooyr.io/websocket"
)

type Server struct {
	hub *Hub
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, World!")
}

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received connection request on /ws from %s\n", r.RemoteAddr)
	userID := r.URL.Query().Get("user")
	if userID == "" {
		fmt.Println("Error: user query parameter is missing")
		http.Error(w, "user query parameter is required", http.StatusBadRequest)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		fmt.Printf("Error accepting websocket for user %s: %v\n", userID, err)
		return
	}
	conn := &connection{ws: c}
	s.hub.register(userID, conn)
	defer s.hub.unregister(userID)
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	fmt.Printf("User %s connected successfully\n", userID)

	ctx := r.Context()
	for {
		_, p, err := c.Read(ctx)
		if err != nil {
			fmt.Printf("User %s disconnected: %v\n", userID, err)
			break
		}

		var msg Message
		if err := json.Unmarshal(p, &msg); err != nil {
			fmt.Printf("Error unmarshaling message from %s: %v\n", userID, err)
			continue
		}

		fmt.Printf("Message from %s to %s: %s\n", msg.Sender, msg.Recipient, msg.Content)

		recipientConn, ok := s.hub.get(msg.Recipient)
		if !ok {
			fmt.Printf("Recipient %s not found for message from %s\n", msg.Recipient, msg.Sender)
			continue
		}

		if err := recipientConn.ws.Write(ctx, websocket.MessageText, p); err != nil {
			fmt.Printf("Error sending message to %s: %v\n", msg.Recipient, err)
		}
	}
}

func SetupRouter(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", helloHandler)
	mux.HandleFunc("/ws", s.wsHandler)
	return mux
}

func main() {
	server := &Server{
		hub: NewHub(),
	}
	mux := SetupRouter(server)
	fmt.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}