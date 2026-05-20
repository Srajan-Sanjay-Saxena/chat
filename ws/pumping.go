package ws

import (
	"chat-v2/logger"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type inMessage struct {
	Type           string    `json:"type"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
}

type outMessage struct {
	ID             uuid.UUID `json:"id"`
	SenderID       uuid.UUID `json:"sender_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
	CreatedAt      int64     `json:"created_at"`
}

const writeWait = 10 * time.Second
const pongWait = 60 * time.Second
const maxMsgSize = 512
const pingPeriod = (pongWait * 9) / 10

func (client *client) readPump() {
	for {
		// Read message from WebSocket connection
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			logger.Log.Error("error reading message from client", "error", err)
			break
		}

		// Unmarshal incoming message
		var inMsg inMessage
		if err := json.Unmarshal(message, &inMsg); err != nil {
			logger.Log.Error("error unmarshalling incoming message", "error", err)
			continue
		}

		// Validate incoming message
		if inMsg.ConversationID == uuid.Nil || inMsg.Content == "" {
			logger.Log.Error("invalid message received", "conversation_id", inMsg.ConversationID, "content_length", len(inMsg.Content))
			continue
		}

		if inMsg.Type != "message" && inMsg.Type != "subscribe" && inMsg.Type != "unsubscribe" {
			logger.Log.Error("invalid message type received", "type", inMsg.Type)
			continue
		}

		// Handle different message types
		switch inMsg.Type {
		case "message":
			// Create outgoing message with additional metadata
			outMsg := outMessage{
				ID:             uuid.New(),
				SenderID:       client.userID,
				ConversationID: inMsg.ConversationID,
				Content:        inMsg.Content,
				CreatedAt:      time.Now().Unix(),
			}
			outMsgBytes, err := json.Marshal(outMsg)
			if err != nil {
				logger.Log.Error("error marshalling outgoing message", "error", err)
				continue
			}
			client.hub.broadcast <- broadcastMessage{
				sender:         client,
				message:        outMsgBytes,
				conversationID: inMsg.ConversationID,
			}
			logger.Log.Info("message received from client", "user_id", client.userID, "conversation_id", inMsg.ConversationID, "content_length", len(inMsg.Content))

		case "subscribe":
			client.hub.subscribe <- subscription{
				client:         client,
				conversationID: inMsg.ConversationID,
			}
			logger.Log.Info("Client subscribed to conversation", "user_id", client.userID, "conversation_id", inMsg.ConversationID)

		case "unsubscribe":
			client.hub.unsubscribe <- subscription{
				client:         client,
				conversationID: inMsg.ConversationID,
			}
			logger.Log.Info("Client unsubscribed from conversation", "user_id", client.userID, "conversation_id", inMsg.ConversationID)
		}

		// Adding deadline to prevent hanging connections
		client.conn.SetReadLimit(maxMsgSize)
		client.conn.SetReadDeadline(time.Now().Add(pongWait))
		client.conn.SetPongHandler(func(string) error { client.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	}

	// Unregister client from hub when done
	client.hub.unregister <- client
	// close the WebSocket connection
	client.conn.Close()
}

func (client *client) writePump() {

	// Create a ticker for sending periodic pings to the client
	ticker := time.NewTicker(pingPeriod)
	// Ensure the WebSocket connection is closed when this function exits
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				// Channel closed, close WebSocket connection
				client.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			// Write message to WebSocket connection
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logger.Log.Error("error writing message to client", "error", err)
				return
			}
			// Log the message sent to the client
			logger.Log.Info("message sent to client", "user_id", client.userID, "message_length", len(message))

		case <-ticker.C:
			// Send a ping message to the client
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Log.Error("error sending ping to client", "error", err)
				return
			}
			logger.Log.Info("ping sent to client", "user_id", client.userID)
		}
	}
}
