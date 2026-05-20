package ws

import (
	"chat-v2/helper"
	"chat-v2/logger"
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func NewWebSocketHandler(maker *helper.JWTMaker, hub *Hub, isParticipant func(context.Context, uuid.UUID, uuid.UUID) (bool, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get the JWT token from query parameters
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logger.Log.Error("Authorization header is missing in WebSocket request")
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		// Expected format: "Bearer <token
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix){
			logger.Log.Error("Invalid Authorization header format", "header", authHeader)
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		jwtToken := strings.TrimPrefix(authHeader, prefix)

		// Verify the JWT token
		claims, err := maker.VerifyToken(jwtToken)
		if err != nil {
			http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Upgrade the connection to WebSocket
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for simplicity
			},
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "Failed to upgrade to WebSocket: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Handle the WebSocket connection (e.g., read/write messages)
		go handleWebSocketConnection(conn, hub, claims.ID, isParticipant)

	})
}

func handleWebSocketConnection(conn *websocket.Conn, hub *Hub, userID uuid.UUID, isParticipant func(context.Context, uuid.UUID, uuid.UUID) (bool, error)) {
	// Create a new client and register it with the hub
	client := &client{
		conn: conn,
		send: make(chan []byte, 256),
		subscribedConversations: make(map[uuid.UUID]bool),
		userID: userID,
		hub: hub,
		isParticipant: isParticipant,
	}
	hub.register <- client

	// Start goroutines for reading and writing messages
	go client.readPump()
	go client.writePump()
}