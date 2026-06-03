package ws

import (
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"chat-v2/service"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type wsHandler struct {
	messageService   *service.MessageService
	upgrader       	websocket.Upgrader
	hub             *Hub
	isParticipant   func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}


func (h *wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Method check
	if r.Method != http.MethodGet {
		logger.Log.Warn("WebSocket connection attempt with invalid method", "method", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := helper.GetUserFromContext(r.Context())
	if !ok {
		logger.Log.Warn("WebSocket connection attempt without user ID in context")
		http.Error(w, "Unauthorized: user ID not found in context", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log.Error("Failed to upgrade to WebSocket", "error", err)
		http.Error(w, "Failed to upgrade to WebSocket: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.handleWsConnection(conn, userID)
}

func NewWebSocketHandler(
    repo *repository.Repository,
    hub *Hub,
    allowedOrigins []string,
    isParticipant func(context.Context, uuid.UUID, uuid.UUID) (bool, error),
) http.Handler {

    return &wsHandler{
        hub:            hub,
        isParticipant:  isParticipant,
        messageService: service.NewMessageService(
            repo,
            NewLocalPublisher(hub),
            isParticipant,
        ),
        upgrader: websocket.Upgrader{
            CheckOrigin: func(r *http.Request) bool {
                return isAllowedOrigin(
                    r.Header.Get("Origin"),
                    allowedOrigins,
                )
            },
        },
    }
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

func (h *wsHandler) handleWsConnection(conn *websocket.Conn, userID uuid.UUID) {
	// Create a new client and register it with the hub
	client := &client{
		conn:                    conn,
		send:                    make(chan []byte, 256),
		subscribedConversations: make(map[uuid.UUID]bool),
		userID:                  userID,
		hub:                     h.hub,
		isParticipant:           h.isParticipant,
		lastActive:              time.Now(),
	}
	h.hub.register <- client
	// Start goroutines for reading and writing messages
	go func() {
		client.readPump(h.messageService)
	}()
	go client.writePump()
}