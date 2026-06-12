package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"chat-v2/service"
)

// TestClientClose_IdempotentAndHubCleanup verifies that calling client.close()
// multiple times is safe (idempotent) and that the hub removes the client and
// closes the send channel.
func TestClientClose_IdempotentAndHubCleanup(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	defer func() {
		hub.Stop()
		<-hub.Done()
	}()

	clientCh := make(chan *client, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}

		c := &client{
			conn:                    conn,
			send:                    make(chan []byte, 4),
			subscribedConversations: make(map[uuid.UUID]bool),
			userID:                  uuid.New(),
			hub:                     hub,
		}
		// register and hand client back to test
		hub.register <- c
		clientCh <- c

		// start pumps
		go func() {
			c.readPump(service.NewMessageService(nil, NewLocalPublisher(hub)))
		}()
		go c.writePump()
	}))
	defer srv.Close()

	dialer := websocket.Dialer{}
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// receive client pointer from handler
	var c *client
	select {
	case c = <-clientCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client from handler")
	}

	// wait for hub to contain the client
	waitUntil := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(waitUntil) {
		if _, ok := hub.clients[c]; ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, ok := hub.clients[c]; !ok {
		t.Fatal("client was not registered in hub")
	}

	// call close twice (idempotency)
	c.close()
	c.close()

	// give hub loop a moment to process unregister
	waitUntil = time.Now().Add(1 * time.Second)
	for time.Now().Before(waitUntil) {
		if _, ok := hub.clients[c]; !ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, ok := hub.clients[c]; ok {
		t.Fatal("client still present in hub after close")
	}

	// ensure send channel is closed
	closed := false
	waitUntil = time.Now().Add(1 * time.Second)
	for time.Now().Before(waitUntil) {
		select {
		case _, ok := <-c.send:
			if !ok {
				closed = true
			}
		default:
		}
		if closed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !closed {
		t.Fatal("client.send channel was not closed by hub unregister")
	}
}
