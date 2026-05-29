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
	conn                    *websocket.Conn // connection to the client
	send                    chan []byte     // outbound messages to this client
	subscribedConversations map[uuid.UUID]bool
	userID                  uuid.UUID
	hub                     *Hub
	isParticipant           func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	closeOnce               sync.Once
	mu                      sync.Mutex
	lastActive              time.Time
}

type Hub struct {
	// On scaling add clients map for conversation ID
	// to list of clients subscribed to that conversation for more efficient broadcasting
	clients     map[*client]bool      // registered clients
	register    chan *client          // clients send themselves here to register
	unregister  chan *client          // clients send themselves here to unregister
	subscribe   chan subscription     // clients want to get messages from this conversation
	unsubscribe chan subscription     // clients want to stop getting messages from this conversation
	broadcast   chan broadcastMessage // messages to broadcast to clients, with conversation ID for routing
	stop        chan struct{}         // closed to signal hub to stop and clean up
	done        chan struct{}         // closed when hub has fully stopped and cleaned up
	once        sync.Once             // ensures Stop can only be called once
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

// StartIdleSweeper starts a background goroutine that periodically closes clients
// which have been idle for longer than idleTimeout. The goroutine stops when
// the Hub is stopped (when h.stop is closed).
func (h *Hub) StartIdleSweeper(idleTimeout, period time.Duration) {
	go func() {
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				for c := range h.clients {
					c.mu.Lock()
					la := c.lastActive
					c.mu.Unlock()
					if now.Sub(la) > idleTimeout {
						logger.Log.Info("sweeper closing idle client", "user_id", c.userID)
						c.close()
					}
				}
			case <-h.stop:
				return
			}
		}
	}()
}

func (c *client) touch() {
	c.mu.Lock()
	c.lastActive = time.Now()
	c.mu.Unlock()
}

// Idempotent close that can be called from multiple goroutines
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

// Idempotent close that can be called from multiple goroutines
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
