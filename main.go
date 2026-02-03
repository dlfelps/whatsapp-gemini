package main

import (
	"fmt"
	"net/http"

	"nhooyr.io/websocket"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, World!")
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		fmt.Printf("Error accepting websocket: %v\n", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	ctx := r.Context()
	// For now, just keep the connection open until the client closes it or the context is cancelled.
	for {
		_, _, err := c.Read(ctx)
		if err != nil {
			break
		}
	}
}

func SetupRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", helloHandler)
	mux.HandleFunc("/ws", wsHandler)
	return mux
}

func main() {
	mux := SetupRouter()
	fmt.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
