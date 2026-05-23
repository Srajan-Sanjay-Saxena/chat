package ws

import (
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"chat-v2/service"
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func NewWebSocketHandler(repo *repository.Repository, maker *helper.JWTMaker, hub *Hub, allowedOrigins []string, isParticipant func(context.Context, uuid.UUID, uuid.UUID) (bool, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodGet {
			logger.Log.Warn("WebSocket connection attempt with invalid method", "method", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get User ID from context (set by JWT middleware
		userID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			var err error
			userID, err = helper.JWTVerifier(r, maker)
			if err != nil {
				logger.Log.Warn("WebSocket connection attempt without user ID in context")
				http.Error(w, "Unauthorized: user ID not found in context", http.StatusUnauthorized)
				return
			}
		}

		// Upgrade the connection to WebSocket
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return isAllowedOrigin(r.Header.Get("Origin"), allowedOrigins)
			},
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Log.Error("Failed to upgrade to WebSocket", "error", err)
			http.Error(w, "Failed to upgrade to WebSocket: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Handle the WebSocket connection (e.g., read/write messages)
		go handleWebSocketConnection(repo, conn, hub, userID, isParticipant)

	})
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	origin = strings.TrimSpace(strings.TrimSuffix(origin, "/"))
	if len(allowedOrigins) == 0 {
		return origin != "" && (strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1"))
	}

	if origin == "" {
		return false
	}

	for _, allowed := range allowedOrigins {
		allowed = strings.TrimSpace(strings.TrimSuffix(allowed, "/"))
		if allowed == "*" {
			return true
		}
		if allowed != "" && strings.EqualFold(origin, allowed) {
			return true
		}
	}

	return false
}

func handleWebSocketConnection(repo *repository.Repository, conn *websocket.Conn, hub *Hub, userID uuid.UUID, isParticipant func(context.Context, uuid.UUID, uuid.UUID) (bool, error)) {
	// Create a new client and register it with the hub
	client := &client{
		conn:                    conn,
		send:                    make(chan []byte, 256),
		subscribedConversations: make(map[uuid.UUID]bool),
		userID:                  userID,
		hub:                     hub,
		isParticipant:           isParticipant,
	}
	hub.register <- client
	messageService := service.NewMessageService(repo, isParticipant)
	ctx, cancel := context.WithCancel(context.Background())

	// Start goroutines for reading and writing messages
	go func() {
		defer cancel()
		client.readPump(ctx, messageService)
	}()
	go client.writePump()
}
