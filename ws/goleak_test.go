package ws

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
	"context"
    "github.com/gorilla/websocket"
    "go.uber.org/goleak"
    "github.com/google/uuid"

    "chat-v2/service"
)

// Quick integration test that connects a client, then disconnects and verifies
// there are no goroutine leaks using goleak.
func TestNoGoroutineLeaksOnConnectClose(t *testing.T) {
    defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"))

    hub := NewHub()
    go hub.Run()
    defer func() {
        hub.Stop()
        <-hub.Done()
    }()

    // start sweeper with short timeouts for test
    hub.StartIdleSweeper(200*time.Millisecond, 100*time.Millisecond)

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
            lastActive:              time.Now(),
        }
        hub.register <- c

        ctx, cancel := context.WithCancel(context.Background())
        go func() {
            defer cancel()
            c.readPump(ctx, service.NewMessageService(nil, NewLocalPublisher(hub), nil))
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

    // close the connection to trigger cleanup
    _ = conn.Close()

    // allow sweeper/hub time to run and goroutines to exit
    time.Sleep(500 * time.Millisecond)
}
