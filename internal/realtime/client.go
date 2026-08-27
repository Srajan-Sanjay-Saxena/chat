package realtime

import (
	"chat-v2/internal/pkg/logger"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

type Client struct {
	conn       *websocket.Conn
	send       chan []byte
	userID     uuid.UUID
	username   string
	closeOnce  sync.Once
	mu         sync.Mutex
	lastActive time.Time
	limiter    *rate.Limiter
}

func NewClient(conn *websocket.Conn, userID uuid.UUID, username string) *Client {
	return &Client{
		conn:       conn,
		send:       make(chan []byte, 256),
		userID:     userID,
		username:   username,
		lastActive: time.Now(),
		limiter:    rate.NewLimiter(10, 20),
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		if c.conn != nil {
			c.conn.Close()
		}
	})
}

func (c *Client) SendMessage(msg []byte) {
	select {
	case c.send <- msg:
	default:
		logger.Warn("Client send buffer full", "user_id", c.userID)
	}
}

func (c *Client) UpdateLastActive() {
	c.mu.Lock()
	c.lastActive = time.Now()
	c.mu.Unlock()
}

func (c *Client) LastActive() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastActive
}

func (c *Client) UserID() uuid.UUID   { return c.userID }
func (c *Client) Username() string    { return c.username }
func (c *Client) Send() <-chan []byte { return c.send }
func (c *Client) Conn() *websocket.Conn { return c.conn }

func (c *Client) AllowMessage() bool {
	return c.limiter == nil || c.limiter.Allow()
}
