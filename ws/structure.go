package ws

import (
	"chat-v2/logger"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type client struct {
	conn                    *websocket.Conn
	send                    chan []byte
	subscribedConversations map[uuid.UUID]bool
	userID                  uuid.UUID
	hub                     *Hub
	isParticipant           func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	closeOnce               sync.Once
}

type Hub struct {
	clients     map[*client]bool
	register    chan *client
	unregister  chan *client
	subscribe   chan subscription
	unsubscribe chan subscription
	broadcast   chan broadcastMessage
	stop        chan struct{}
	done        chan struct{}
	once        sync.Once
}

type broadcastMessage struct {
	message        []byte
	conversationID uuid.UUID
}

type subscription struct {
	client         *client
	conversationID uuid.UUID
}

const participantCheckTimeout = 2 * time.Second

func NewHub() *Hub {
	return &Hub{
		clients:     make(map[*client]bool),
		register:    make(chan *client),
		unregister:  make(chan *client),
		subscribe:   make(chan subscription),
		unsubscribe: make(chan subscription),
		broadcast:   make(chan broadcastMessage),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
}

func (c *client) close() {
	c.closeOnce.Do(func() {
		// Try a non-blocking send first to avoid deadlock if called from hub goroutine.
		select {
		case c.hub.unregister <- c:
			// delivered synchronously
		default:
			// hub not ready; deliver asynchronously so caller won't block
			go func(cl *client) {
				c.hub.unregister <- cl
			}(c)
		}

		// Always close the network connection here; hub will close client.send.
		_ = c.conn.Close()
	})
}

func (h *Hub) Stop() {
	h.once.Do(func() {
		close(h.stop)
	})
}

func (h *Hub) Done() <-chan struct{} {
	return h.done
}

func (h *Hub) Run() {
	defer close(h.done)

	for {
		select {
		// Handle registration of new clients
		case client := <-h.register:
			h.clients[client] = true
			logger.Log.Info("New client registered", "user_id", client.userID)

		// Handle unregistration of clients
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			logger.Log.Info("Client unregistered", "user_id", client.userID)

		// Handle broadcasting messages to subscribed clients
		case message := <-h.broadcast:
			for client := range h.clients {
				if client.subscribedConversations[message.conversationID] {
					select {
					case client.send <- message.message:
					default:
						logger.Log.Warn("client send buffer full, disconnecting slow client", "user_id", client.userID, "conversation_id", message.conversationID)
						client.close()
					}
				}
			}
			logger.Log.Info("Message broadcasted", "conversation_id", message.conversationID, "message_length", len(message.message))

		// Handle subscription to conversation channels
		case sub := <-h.subscribe:
			// If an isParticipant checker is available, verify membership first
			allowed := true
			if sub.client.isParticipant != nil {
				var err error
				ctx, cancel := context.WithTimeout(context.Background(), participantCheckTimeout)
				allowed, err = sub.client.isParticipant(ctx, sub.conversationID, sub.client.userID)
				cancel()
				if err != nil {
					logger.Log.Error("participant check failed", "error", err, "user_id", sub.client.userID, "conversation_id", sub.conversationID)
					ack := map[string]string{"type": "error", "action": "subscribe", "conversation_id": sub.conversationID.String(), "reason": "check_failed"}
					if b, err := json.Marshal(ack); err == nil {
						select {
						case sub.client.send <- b:
						default:
						}
					}
					continue
				}
			}

			if !allowed {
				ack := map[string]string{"type": "error", "action": "subscribe", "conversation_id": sub.conversationID.String(), "reason": "not_participant"}
				if b, err := json.Marshal(ack); err == nil {
					select {
					case sub.client.send <- b:
					default:
					}
				}
				logger.Log.Warn("Client not participant for subscribe", "user_id", sub.client.userID, "conversation_id", sub.conversationID)
				continue
			}

			sub.client.subscribedConversations[sub.conversationID] = true
			ack := map[string]string{"type": "subscribed", "conversation_id": sub.conversationID.String()}
			if b, err := json.Marshal(ack); err == nil {
				select {
				case sub.client.send <- b:
				default:
				}
			}
			logger.Log.Info("Client subscribed to conversation", "user_id", sub.client.userID, "conversation_id", sub.conversationID)

		// Handle unsubscription from conversation channels
		case sub := <-h.unsubscribe:
			delete(sub.client.subscribedConversations, sub.conversationID)
			ack := map[string]string{"type": "unsubscribed", "conversation_id": sub.conversationID.String()}
			if b, err := json.Marshal(ack); err == nil {
				select {
				case sub.client.send <- b:
				default:
				}
			}
			logger.Log.Info("Client unsubscribed from conversation", "user_id", sub.client.userID, "conversation_id", sub.conversationID)

		case <-h.stop:
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			logger.Log.Info("hub stopped and clients closed")
			return

		}
	}
}
