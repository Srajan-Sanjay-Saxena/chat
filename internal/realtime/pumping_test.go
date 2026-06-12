package realtime

import(
	"testing"
	"sync"
	"github.com/google/uuid"

)

func TestWritePump(t *testing.T) {

	// Create a test client
	testUserID := uuid.New()
	testClient := &client{
		send: make(chan []byte, 1), // Buffered channel to prevent blocking
		userID: testUserID,
		closeOnce: sync.Once{},
	}
	// Simulate the writePump in a separate goroutine
	go testClient.writePump()
	// Send a test message to the client
	testMessage := []byte("test message")
	testClient.send <- testMessage
	// close send channel to simulate client disconnection
	close(testClient.send)
}

