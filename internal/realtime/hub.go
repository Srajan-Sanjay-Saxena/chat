package realtime

import (
	"chat-v2/internal/message"
	"chat-v2/internal/pkg/logger"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Hub struct {
	clients                 map[*Client]struct{}
	conversationSubscribers map[uuid.UUID]map[*Client]struct{}
	clientSubscriptions     map[*Client]map[uuid.UUID]struct{}

	register    chan *Client
	unregister  chan *Client
	subscribe   chan subscriptionRequest
	unsubscribe chan subscriptionRequest
	broadcast   chan broadcastRequest

	stop chan struct{}
	done chan struct{}
	once sync.Once
}

type subscriptionRequest struct {
	client         *Client
	conversationID uuid.UUID
	done           chan struct{}
}

type broadcastRequest struct {
	message        []byte
	conversationID uuid.UUID
}

func NewHub() *Hub {
	return &Hub{
		clients:                 make(map[*Client]struct{}),
		conversationSubscribers: make(map[uuid.UUID]map[*Client]struct{}),
		clientSubscriptions:     make(map[*Client]map[uuid.UUID]struct{}),
		register:                make(chan *Client),
		unregister:              make(chan *Client),
		subscribe:               make(chan subscriptionRequest),
		unsubscribe:             make(chan subscriptionRequest),
		broadcast:               make(chan broadcastRequest),
		stop:                    make(chan struct{}),
		done:                    make(chan struct{}),
	}
}

func (h *Hub) Run() {
	defer close(h.done)

	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}

		case client := <-h.unregister:
			h.handleUnregister(client)

		case req := <-h.subscribe:
			h.handleSubscribe(req)

		case req := <-h.unsubscribe:
			h.handleUnsubscribe(req)

		case req := <-h.broadcast:
			h.handleBroadcast(req)

		case <-h.stop:
			for client := range h.clients {
				h.handleUnregister(client)
			}
			return
		}
	}
}

func (h *Hub) handleUnregister(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}

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
	if _, ok := h.clients[req.client]; !ok {
		h.clients[req.client] = struct{}{}
	}

	if _, ok := h.conversationSubscribers[req.conversationID]; !ok {
		h.conversationSubscribers[req.conversationID] = make(map[*Client]struct{})
	}
	h.conversationSubscribers[req.conversationID][req.client] = struct{}{}

	if _, ok := h.clientSubscriptions[req.client]; !ok {
		h.clientSubscriptions[req.client] = make(map[uuid.UUID]struct{})
	}
	h.clientSubscriptions[req.client][req.conversationID] = struct{}{}

	req.done <- struct{}{}

	ack := map[string]string{
		"type":            "subscribe_ack",
		"conversation_id": req.conversationID.String(),
	}
	if b, err := json.Marshal(ack); err == nil {
		req.client.SendMessage(b)
	}
}

func (h *Hub) handleUnsubscribe(req subscriptionRequest) {
	if clients, ok := h.conversationSubscribers[req.conversationID]; ok {
		delete(clients, req.client)
		if len(clients) == 0 {
			delete(h.conversationSubscribers, req.conversationID)
		}
	}

	if subs, ok := h.clientSubscriptions[req.client]; ok {
		delete(subs, req.conversationID)
		if len(subs) == 0 {
			delete(h.clientSubscriptions, req.client)
		}
	}

	req.done <- struct{}{}

	ack := map[string]string{
		"type":            "unsubscribe_ack",
		"conversation_id": req.conversationID.String(),
	}
	if b, err := json.Marshal(ack); err == nil {
		req.client.SendMessage(b)
	}
}

func (h *Hub) handleBroadcast(req broadcastRequest) {
	clients, ok := h.conversationSubscribers[req.conversationID]
	if !ok {
		return
	}

	for client := range clients {
		select {
		case client.send <- req.message:
		default:
			logger.Warn("Client send buffer full", "user_id", client.userID)
			h.handleUnregister(client)
		}
	}
}

func (h *Hub) Broadcast(msg *message.OutMessage) {
	if h == nil || msg == nil {
		return
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		logger.Error("Failed to marshal message", "error", err)
		return
	}

	h.broadcast <- broadcastRequest{
		message:        payload,
		conversationID: msg.ConversationID,
	}
}

func (h *Hub) Subscribe(client *Client, conversationID uuid.UUID) {
	done := make(chan struct{})
	h.subscribe <- subscriptionRequest{client: client, conversationID: conversationID, done: done}
	<-done
}

func (h *Hub) Unsubscribe(client *Client, conversationID uuid.UUID) {
	done := make(chan struct{})
	h.unsubscribe <- subscriptionRequest{client: client, conversationID: conversationID, done: done}
	<-done
}

func (h *Hub) Register(client *Client)   { h.register <- client }
func (h *Hub) Unregister(client *Client) { h.unregister <- client }

func (h *Hub) Stop() {
	h.once.Do(func() { close(h.stop) })
}

func (h *Hub) Done() <-chan struct{} { return h.done }

func (h *Hub) StartIdleChecker(idleTimeout, checkInterval time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			for client := range h.clients {
				if now.Sub(client.LastActive()) > idleTimeout {
					logger.Info("Closing idle client", "user_id", client.userID)
					h.unregister <- client
				}
			}
		case <-h.stop:
			return
		}
	}
}
