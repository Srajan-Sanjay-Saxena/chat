package realtime

import (
	"chat-v2/logger"
	"chat-v2/service"
	"sync"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"	
	// "time"
	"encoding/json"
)

type client struct {
	conn 				  *websocket.Conn
	send 				  chan []byte
	userID				  uuid.UUID
	username			  string
	closeOnce			  sync.Once
	// mu					  sync.Mutex
	// lastActive			  time.Time
}

type subscriptionRequest struct {
	client 				  *client
	conversationID		  uuid.UUID
	done 				  chan struct{}
}

type broadcastRequest struct {
	message 				  []byte
	conversationID		  uuid.UUID
}

type Hub struct {
	clients map[*client]struct{}
	unregister chan *client
	subscribe chan subscriptionRequest
	unsubscribe chan subscriptionRequest
	broadcast chan broadcastRequest
	stop chan struct{}
	done chan struct{}
	once sync.Once
	conversationSubscribers map[uuid.UUID]map[*client]struct{}
	clientSubscriptions map[*client]map[uuid.UUID]struct{}
}

type RealtimeHandler struct {
	hub *Hub
	subscriptionService *service.SubscriptionService
	messageService *service.MessageService
}

func NewRealtimeHandler(hub *Hub, subscriptionService *service.SubscriptionService, messageService *service.MessageService) *RealtimeHandler {
	return &RealtimeHandler{
		hub: hub,
		subscriptionService: subscriptionService,
		messageService: messageService,
	}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		unregister: make(chan *client),
		subscribe: make(chan subscriptionRequest),
		unsubscribe: make(chan subscriptionRequest),
		broadcast: make(chan broadcastRequest),
		stop: make(chan struct{}),
		done: make(chan struct{}),
		conversationSubscribers: make(map[uuid.UUID]map[*client]struct{}),
		clientSubscriptions: make(map[*client]map[uuid.UUID]struct{}),
	}
}

func (c *client) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		if c.conn == nil {
			logger.Log.Warn("client connection is nil on close, skipping close", "user_id", c.userID)
			return
		}
		if err := c.conn.Close(); err != nil {
			logger.Log.Error("error closing client connection", "user_id", c.userID, "error", err)
		}
	})
	
}

func (h *Hub) handleUnregister(client *client) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	// Remove client from all conversation subscriber lists
	if subs, ok := h.clientSubscriptions[client]; ok {
		for convID := range subs {
			if clients, ok := h.conversationSubscribers[convID]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.conversationSubscribers, convID)
				}
			}
		}
		delete(h.clientSubscriptions, client)
	}
	delete(h.clients, client)
	close(client.send)
	client.Close()
}

func (h *Hub) handleSubscribe(req subscriptionRequest) {
	// Ensure client is registered in the hub
	if _, ok := h.clients[req.client]; !ok {
		h.clients[req.client] = struct{}{}
	}

	// Add client to conversation subscriber list
	if _, ok := h.conversationSubscribers[req.conversationID]; !ok {
		h.conversationSubscribers[req.conversationID] = make(map[*client]struct{})
	}
	h.conversationSubscribers[req.conversationID][req.client] = struct{}{}

	// Add conversation to client's subscription list
	if _, ok := h.clientSubscriptions[req.client]; !ok {
		h.clientSubscriptions[req.client] = make(map[uuid.UUID]struct{})
	}
	h.clientSubscriptions[req.client][req.conversationID] = struct{}{}
	req.done <- struct{}{}

	if req.client.conn == nil {
		logger.Log.Warn("client connection is nil in handleSubscribe, cannot send acknowledgment", "user_id", req.client.userID, "conversation_id", req.conversationID)
		return
	}
	// send acknowledgment message back to client
	ack := map[string]string{
		"type": "subscribe_ack",
		"conversation_id": req.conversationID.String(),
	}
	if b, err := json.Marshal(ack); err == nil {
		if err := req.client.conn.WriteMessage(websocket.TextMessage, b); err != nil {
			logger.Log.Error("error sending subscribe acknowledgment to client", "error", err, "user_id", req.client.userID, "conversation_id", req.conversationID)
		}
	}
}

func (h *Hub) handleUnsubscribe(req subscriptionRequest) {
	// Remove client from conversation subscriber list
	if clients, ok := h.conversationSubscribers[req.conversationID]; ok {
		delete(clients, req.client)
		if len(clients) == 0 {
			delete(h.conversationSubscribers, req.conversationID)
		}
	}
	// Remove conversation from client's subscription list
	if subs, ok := h.clientSubscriptions[req.client]; ok {
		delete(subs, req.conversationID)
		if len(subs) == 0 {
			delete(h.clientSubscriptions, req.client)
		}
	}
	req.done <- struct{}{}

	if req.client.conn == nil {
		logger.Log.Warn("client connection is nil in handleUnsubscribe, cannot send acknowledgment", "user_id", req.client.userID, "conversation_id", req.conversationID)
		return
	}
	// send acknowledgment message back to client
	ack := map[string]string{
		"type": "unsubscribe_ack",
		"conversation_id": req.conversationID.String(),
	}
	if b, err := json.Marshal(ack); err == nil {
		if err := req.client.conn.WriteMessage(websocket.TextMessage, b); err != nil {
			logger.Log.Error("error sending unsubscribe acknowledgment to client", "error", err, "user_id", req.client.userID, "conversation_id", req.conversationID)
		}
	}
}

func (h *Hub) handleBroadcast(req broadcastRequest) {
	// Broadcast message to all clients subscribed to the conversation, except the sender
	logger.Log.Debug("Broadcasting message", "conversation_id", req.conversationID, "message_length", len(req.message))
	if clients, ok := h.conversationSubscribers[req.conversationID]; ok {
		for client := range clients {
			select {
			case client.send <- req.message:
			default:
				logger.Log.Warn("client send buffer full, disconnecting slow client", "user_id", client.userID, "conversation_id", req.conversationID)
				h.handleUnregister(client)
			}
		}
	}
}

func (h *Hub) Run() {
	// Ensure done channel is closed when Run exits
	defer close(h.done)

	for {
		select {
			case client := <-h.unregister:
				h.handleUnregister(client)
			case subReq := <-h.subscribe:
				h.handleSubscribe(subReq)
			case unsubReq := <-h.unsubscribe:
				h.handleUnsubscribe(unsubReq)
			case broadcastReq := <-h.broadcast:
				h.handleBroadcast(broadcastReq)
			case <-h.stop:
				for client := range h.clients {
					h.handleUnregister(client)
				}
				logger.Log.Info("Hub stopping")
				return
		}
	}
}

func (h *Hub) Stop() {
	h.once.Do(func() {
		close(h.stop)
	})
}

func (h *Hub) Done() <-chan struct{} {
	return h.done
}