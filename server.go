package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	// Allow all origins for this demo
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func startServer(config Config) error {
	// Create a new ServeMux to handle routes
	mux := http.NewServeMux()

	// Add health check endpoint for load balancer
	mux.HandleFunc("/ping", handleHealthCheck)

	// Set the default handler for all other paths to be the WebSocket handler
	mux.HandleFunc("/", handleWebSocket)

	log.Printf("WebSocket server listening on %s", config.Addr)
	log.Printf("Health check endpoint available at http://%s/ping", config.Addr)

	return http.ListenAndServe(config.Addr, mux)
}

// handleHealthCheck responds to health check requests from load balancers
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
	// log.Printf("Health check requested from %s", r.RemoteAddr)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Client connected: %s", conn.RemoteAddr())
	if xffHeader := r.Header.Get("X-Forwarded-For"); xffHeader != "" {
		log.Printf("X-Forwarded-For: %s", xffHeader)
	}

	for {
		// Read message from client
		_, clientMessage, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return
			}
			log.Printf("Error reading message: %v", err)
			break
		}

		// Parse client message
		var msg Message
		if err := json.Unmarshal(clientMessage, &msg); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		// log.Printf("Received from client: %s (ID: %s)", msg.Content, msg.MessageID)

		// Create response message (echo back with original message)
		response := Message{
			Timestamp: msg.Timestamp,
			Content:   msg.Content,
			MessageID: msg.MessageID,
		}

		// Send response back to client
		responseJSON, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response: %v", err)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, responseJSON); err != nil {
			log.Printf("Error sending response: %v", err)
			break
		}
	}
}
