package realtime

import (
	"chat-v2/logger"
	"net/http"
	"chat-v2/helper"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"sync"
	"strings"
)

type WSHandler struct {
	realtimeHandler *RealtimeHandler
	maker *helper.JWTMaker
	upgrader websocket.Upgrader
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// method check
	if r.Method != http.MethodGet {
		logger.Log.Warn("WebSocket connection attempt with invalid method", "method", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// token check and userID extraction
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

	// upgrade connection to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log.Error("Failed to upgrade to WebSocket", "error", err)
		return
	}
	username := r.URL.Query().Get("username")

	h.HandleWsConnection(conn, userID, username)
}

func NewWSHandler(realtimeHandler *RealtimeHandler, maker *helper.JWTMaker, allowedOrigins []string) http.Handler {
	return &WSHandler{
		realtimeHandler: realtimeHandler,
		maker: maker,
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

func (h *WSHandler) HandleWsConnection(conn *websocket.Conn, userID uuid.UUID, username string) {
	client := &client{
		conn: conn,
		send: make(chan []byte, 256),
		userID: userID,
		username: username,
		closeOnce: sync.Once{},
	}

	go client.writePump()
	go client.readPump(h.realtimeHandler)
}
	