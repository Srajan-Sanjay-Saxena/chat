package realtime

import (
	"chat-v2/logger"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const writeWait = 10 * time.Second
const pongWait = 60 * time.Second
const maxMsgSize = 1024 * 4 // 4KB maximum message size
const pingPeriod = (pongWait * 9) / 10

type incomingMessage struct {
	Type           string    `json:"type"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
	Username       string    `json:"username,omitempty"`
}

func (client *client) writePump() {

	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				// The hub closed the channel, so we should close the connection
				client.Close()
				return
			}
			if client.conn == nil {
				logger.Log.Warn("attempted to send message to client with nil connection", "user_id", client.userID)
				continue
			}
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logger.Log.Error("error writing message to client", "error", err)
				client.Close()
				return
			}
			logger.Log.Debug("message sent to client", "user_id", client.userID, "message_length", len(message))
			client.UpdateLastActive() // mark client as active on successful write
		case <-ticker.C:
			if client.conn == nil {
				logger.Log.Warn("attempted to send ping to client with nil connection", "user_id", client.userID)
				continue
			}
			// Send ping to client to keep connection alive and check if it's still responsive
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Log.Warn("ping to client failed, closing connection", "user_id", client.userID, "error", err)
				client.Close()
				return
			}
			client.UpdateLastActive() // mark client as active on successful ping
			logger.Log.Debug("ping sent to client", "user_id", client.userID)
		}
	}
}

func (client *client) readPump(r *RealtimeHandler) {

	client.conn.SetReadLimit(maxMsgSize)
	client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(pongWait))
		client.UpdateLastActive()
		return nil
	})

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			logger.Log.Error("error reading message from client", "error", err)
			break
		}

		inMsg, err := parseAndValidateIncomingMessage(message)
		if err != nil {
			logger.Log.Warn("invalid incoming message from client", "user_id", client.userID, "error", err)
			continue
		}
		// mark client as active on successful read
		client.UpdateLastActive()
		// Process message based on type
		switch inMsg.Type {
		case "message":
			// call message service and get output message struct, then publish to hub broadcast channel
			if r.messageService == nil {
				logger.Log.Warn("message service is not initialized, cannot process message request", "user_id", client.userID)
				continue
			}
			createdMsg, err := r.messageService.CreateMessage(context.Background(), client.userID, inMsg.ConversationID, inMsg.Content, inMsg.Username)
			if err != nil {
				logger.Log.Error("error creating message in message service for incoming message request", "user_id", client.userID, "conversation_id", inMsg.ConversationID, "error", err)
				continue
			}
			logger.Log.Debug("Message created successfully in message service", "message_id", createdMsg.ID, "conversation_id", inMsg.ConversationID, "user_id", client.userID)

		case "subscribe":
			if r.subscriptionService == nil {
				logger.Log.Warn("subscription service is not initialized, cannot process subscribe request", "user_id", client.userID)
				continue
			}
			isParticipant, err := r.subscriptionService.IsParticipant(context.Background(), inMsg.ConversationID, client.userID)
			if err != nil {
				logger.Log.Error("error checking if user is participant in conversation for subscribe request", "user_id", client.userID, "conversation_id", inMsg.ConversationID, "error", err)
				continue
			}
			if !isParticipant {
				logger.Log.Warn("user is not a participant in conversation for subscribe request", "user_id", client.userID, "conversation_id", inMsg.ConversationID)
				continue
			}
			doneCh := make(chan struct{})
			r.hub.subscribe <- subscriptionRequest{
				client:         client,
				conversationID: inMsg.ConversationID,
				done:           doneCh,
			}
			<-doneCh
			logger.Log.Debug("Client subscribed to conversation", "user_id", client.userID, "conversation_id", inMsg.ConversationID)

		case "unsubscribe":
			doneCh := make(chan struct{})
			r.hub.unsubscribe <- subscriptionRequest{
				client:         client,
				conversationID: inMsg.ConversationID,
				done:           doneCh,
			}
			<-doneCh
			logger.Log.Debug("Client unsubscribed from conversation", "user_id", client.userID, "conversation_id", inMsg.ConversationID)
		}
	}
	client.Close()
}

func parseAndValidateIncomingMessage(message []byte) (*incomingMessage, error) {
	var inMsg incomingMessage
	if err := json.Unmarshal(message, &inMsg); err != nil {
		logger.Log.Error("error parsing incoming message", "error", err)
		return nil, err
	}

	// Validate incoming message
	if inMsg.Type != "message" && inMsg.Type != "subscribe" && inMsg.Type != "unsubscribe" {
		return nil, fmt.Errorf("invalid message type received: %s", inMsg.Type)
	}

	if inMsg.ConversationID == uuid.Nil {
		return nil, fmt.Errorf("conversation_id is required in incoming message")
	}

	if inMsg.Type == "message" && inMsg.Content == "" {
		return nil, fmt.Errorf("message content cannot be empty for message type 'message'")
	}
	return &inMsg, nil
}
