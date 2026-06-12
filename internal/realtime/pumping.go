package realtime

import (
	"chat-v2/logger"
	"github.com/gorilla/websocket"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"context"
)

type incomingMessage struct {
	Type           string `json:"type"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string `json:"content"`
}

func (client *client) writePump() {
	for {
		msg, ok := <-client.send
		if !ok {
			// Channel closed, close the connection
			logger.Log.Debug("send channel closed, closing connection", "user_id", client.userID)
			client.Close()
			return
		}
		if client.conn == nil {
			logger.Log.Warn("client connection is nil in writePump, skipping message send", "user_id", client.userID)
			continue
		}

		if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			logger.Log.Error("error writing message to client", "error", err)
			client.Close()
			return
		}
	}
}

func (client *client) readPump(r *RealtimeHandler) {

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
		switch inMsg.Type {
		case "message":
			// call message service and get output message struct, then publish to hub broadcast channel
			if r.messageService == nil {
				logger.Log.Warn("message service is not initialized, cannot process message request", "user_id", client.userID)
				continue
			}
			createdMsg, err := r.messageService.CreateMessage(context.Background(), client.userID, inMsg.ConversationID, inMsg.Content)
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
				done: doneCh,
			}
			<-doneCh
			logger.Log.Debug("Client subscribed to conversation", "user_id", client.userID, "conversation_id", inMsg.ConversationID)

		case "unsubscribe":
			doneCh := make(chan struct{})
			r.hub.unsubscribe <- subscriptionRequest{
				client:         client,
				conversationID: inMsg.ConversationID,
				done: doneCh,
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