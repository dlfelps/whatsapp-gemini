package main

import (
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
	userID := r.URL.Query().Get("user")
	if userID == "" {
		http.Error(w, "user query parameter is required", http.StatusBadRequest)
		return
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		fmt.Printf("Error accepting websocket: %v\n", err)
		return
	}
	conn := &connection{ws: c}
	s.hub.register(userID, conn)
	defer s.hub.unregister(userID)
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	ctx := r.Context()
	for {
		_, _, err := c.Read(ctx)
		if err != nil {
			break
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
