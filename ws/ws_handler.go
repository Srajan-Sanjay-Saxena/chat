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
	messageService *service.MessageService
	upgrader       websocket.Upgrader
	hub            *Hub
	maker          *helper.JWTMaker
	isParticipant  func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

// wsHandler expects requests like this:
// GET /ws
// It does not take any query parameters, but it expects the user ID to be available in the request context (set by authentication middleware).
// And using this handler client side can connect to the WebSocket server and receive real-time updates for the specified conversation.
// After the connection is established, the client can send messages in the following format:
// {
//		"type": "message",
//     "conversation_id": "uuid-of-conversation",
//     "content": "message content"
// }
// The server will validate the message, check if the user is a participant in the conversation, and then broadcast the message to all connected clients subscribed to that conversation.
// Client can request for the subscription to a conversation by sending a message like this:
// {
//     "type": "subscribe",
//     "conversation_id": "uuid-of-conversation"
// }
// And to unsubscribe:
// {
//     "type": "unsubscribe",
//     "conversation_id": "uuid-of-conversation"
// }

func (h *wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Method check
	if r.Method != http.MethodGet {
		logger.Log.Warn("WebSocket connection attempt with invalid method", "method", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		logger.Log.Warn("WebSocket connection attempt without token")
		http.Error(w, "Unauthorized: token is required", http.StatusUnauthorized)
		return
	}

	claims, err := h.maker.VerifyToken(token)
	if err != nil {
		logger.Log.Warn("WebSocket connection attempt with invalid token", "error", err)
		http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
		return
	}

	userID := claims.ID

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
	maker *helper.JWTMaker,
	allowedOrigins []string,
	isParticipant func(context.Context, uuid.UUID, uuid.UUID) (bool, error),
) http.Handler {

	return &wsHandler{
		hub:           hub,
		maker:         maker,
		isParticipant: isParticipant,
		messageService: service.NewMessageService(
			repo,
			NewLocalPublisher(hub),
			isParticipant,
		),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")

				allowed := isAllowedOrigin(origin, allowedOrigins)

				logger.Log.Info(
					"ws origin check",
					"origin", origin,
					"allowed", allowed,
				)

				return allowed
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
