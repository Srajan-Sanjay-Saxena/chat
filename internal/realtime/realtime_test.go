package realtime

import (
	"testing"
	"net/http"
	"net/http/httptest"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"sync"
	"strings"
	"time"
	"go.uber.org/goleak"
)

func TestBehavior(t *testing.T) {
	// Create a hub
	testHub := NewHub()
	go testHub.Run()
	t.Cleanup(func() {
		testHub.Stop()
		select {
		case <-testHub.Done():
			// Hub stopped successfully
		case <-time.After(1 * time.Second):
			t.Fatalf("Expected hub to stop within 1 second, but it did not")
		}
		defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"))
	})

	clientCh := make(chan *client, 1)

	// create a test server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}

		c := &client{
			conn:                    conn,
			send:                    make(chan []byte, 4),
			userID:                  uuid.New(),
			username:                "testuser",
			closeOnce:              sync.Once{},
		}
		
		clientCh <- c

		// start pumps
		realtimeHandler := NewRealtimeHandler(testHub, nil, nil)
		c.readPump(realtimeHandler)
		c.writePump()
	}))
	defer srv.Close()

	// Connect a client to the test server
	dialer := websocket.Dialer{}
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Get the client from the channel
	var testClient *client
	select {
	case testClient = <-clientCh:
		// Got the client, continue with the test
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Expected to receive a client from the channel, but timed out")
	}

	// create a conversation and subscribe the client to it
	testConversationID := uuid.New()
	doneCh := make(chan struct{})
	testHub.subscribe <- subscriptionRequest{client: testClient, conversationID: testConversationID, done: doneCh}
	select {
	case <-doneCh:
		// Subscription completed
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Expected subscription to complete, but it did not")
	}

	testClient.Close()
	testHub.unregister <- testClient
	testClient.Close() // Call Close again to ensure it does not cause any issues
	
	// ensure send channel is closed after unregistering
	select {
	case _, ok := <-testClient.send:
		if ok {
			t.Fatalf("Expected client send channel to be closed after unregistering, but it is still open")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Expected client send channel to be closed after unregistering, but it is still open (timed out)")
	}

}