package ws

import (
	"chat-v2/logger"
	"chat-v2/service"
	"context"
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
	Type           string    `json:"type"`
	ID             uuid.UUID `json:"id"`
	SenderID       uuid.UUID `json:"sender_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

const writeWait = 10 * time.Second
const pongWait = 60 * time.Second
const maxMsgSize = 1024 * 4 // 4KB maximum message size
const pingPeriod = (pongWait * 9) / 10

func (client *client) readPump(messageService *service.MessageService) {

	// Adding deadline to prevent hanging connections
	client.conn.SetReadLimit(maxMsgSize)
	client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error { client.conn.SetReadDeadline(time.Now().Add(pongWait)); client.touch(); return nil })

	for {
		// Read message from WebSocket connection
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			logger.Log.Error("error reading message from client", "error", err)
			break
		}
		// mark active on successful read
		client.touch()

		// Unmarshal incoming message
		var inMsg inMessage
		if err := json.Unmarshal(message, &inMsg); err != nil {
			logger.Log.Error("error unmarshalling incoming message", "error", err)
			continue
		}

		// Validate incoming message
		if inMsg.Type != "message" && inMsg.Type != "subscribe" && inMsg.Type != "unsubscribe" {
			logger.Log.Error("invalid message type received", "type", inMsg.Type)
			continue
		}

		if inMsg.ConversationID == uuid.Nil {
			logger.Log.Error("invalid conversation id received", "type", inMsg.Type)
			continue
		}

		if inMsg.Type == "message" && inMsg.Content == "" {
			logger.Log.Error("invalid message received", "conversation_id", inMsg.ConversationID, "content_length", len(inMsg.Content))
			continue
		}

		// Handle different message types
		switch inMsg.Type {
		case "message":
			// Create message in the database and publish to conversation channel using message service
			// Use a context with timeout for the message creation to avoid hanging if the database is slow
			msgctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			outMsg, err := messageService.CreateMessage(msgctx, client.userID, inMsg.ConversationID, inMsg.Content)
			if err != nil {
				if err == service.ErrNotParticipant {
					logger.Log.Warn("client is not a participant for message publish", "user_id", client.userID, "conversation_id", inMsg.ConversationID)
				} else {
					logger.Log.Error("error creating message", "error", err)
				}
				cancel()
				continue
			}

			logger.Log.Info("message received from client", "user_id", client.userID, "conversation_id", outMsg.ConversationID, "content_length", len(outMsg.Content))
			cancel() // Cancel the message creation context as soon as we're done with it
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

	}

	// Close the connection and unregister the client from the hub
	client.close()
}

func (client *client) writePump() {

	// Create a ticker for sending periodic pings to the client
	ticker := time.NewTicker(pingPeriod)
	// Ensure the WebSocket connection is closed when this function exits
	defer func() {
		ticker.Stop()
		client.close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				// Channel closed, close WebSocket connection
				logger.Log.Info("send channel closed, closing connection", "user_id", client.userID)
				client.close()
				return
			}
			// Write message to WebSocket connection
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logger.Log.Error("error writing message to client", "error", err)
				client.close()
				return
			}
			// mark active on successful write
			client.touch()
			// Log the message sent to the client
			logger.Log.Info("message sent to client", "user_id", client.userID, "message_length", len(message))

		case <-ticker.C:
			// Send a ping message to the client
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Log.Error("error sending ping to client", "error", err)
				client.close()
				return
			}
			client.touch()
			logger.Log.Info("ping sent to client", "user_id", client.userID)
		}
	}
}
