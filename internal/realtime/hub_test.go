package realtime

import (
	"testing"
	"chat-v2/logger"
	"time"
	"sync"
	"github.com/google/uuid"
)

// This file is for testing the Hub logic without involving WebSocket connections


func TestHubFunctionality(t *testing.T) {
	testHub := NewHub()
	go testHub.Run()
	t.Cleanup(func() {
		testHub.Stop()
		<-testHub.Done()
	})
	testUserID := uuid.New()
	testClient := &client{userID: testUserID, send: make(chan []byte, 1), closeOnce: sync.Once{}}
	testConversationID := uuid.New()
	subReq := subscriptionRequest{client: testClient, conversationID: testConversationID, done: make(chan struct{})}
	testHub.subscribe <- subReq 
	select {
		case <-subReq.done:
		// Subscription completed
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Expected subscription to complete, but it did not")
	}
	logger.Log.Info("Client subscribed to conversation successfully")

	// Broadcast a message to the conversation and check if the client receives it
	testHub.broadcast <- broadcastRequest{message: []byte("test message"), conversationID: testConversationID}
	select {
		case msg := <-testClient.send:
			if string(msg) != "test message" {
				t.Fatalf("Expected client to receive 'test message', but got '%s'", string(msg))
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Expected client to receive a message, but did not receive anything")
	}
	logger.Log.Info("Client received broadcast message successfully")

	unsubReq := subscriptionRequest{client: testClient, conversationID: testConversationID, done: make(chan struct{})}
	testHub.unsubscribe <- unsubReq
	select {
		case <-unsubReq.done:
		// Unsubscription completed
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Expected unsubscription to complete, but it did not")
	}
	logger.Log.Info("Client unsubscribed from conversation successfully")

	// Sending a broadcast after unsubscribing should not deliver the message to the client
	testHub.broadcast <- broadcastRequest{message: []byte("test message after unregister"), conversationID: testConversationID}
	select {
		case msg, ok := <-testClient.send:
			if ok {
				t.Fatalf("Expected client send channel to be closed after unregistering, but it is still open and received message: '%s'", string(msg))
			}
		case <-time.After(100 * time.Millisecond):
			// Expected outcome, no message should be received
	}

	testHub.unregister <- testClient
	if _, ok := <-testClient.send; ok {
		t.Fatalf("Expected client send channel to be closed after unregistering, but it is still open")
	}
	

	// Try unregistering the same client again to ensure it does not cause any issues
	testHub.unregister <- testClient
	if _, ok := <-testClient.send; ok {
		t.Fatalf("Expected client send channel to be closed after unregistering, but it is still open on second unregister")
	}

	// try sending a message request after unregistering to ensure it does not cause any issues
	testHub.broadcast <- broadcastRequest{message: []byte("test message after unregister"), conversationID: testConversationID}
	select {
		case msg, ok := <-testClient.send:
			if ok {
				t.Fatalf("Expected client send channel to be closed after unregistering, but it is still open and received message: '%s'", string(msg))
			}
		case <-time.After(100 * time.Millisecond):
			// Expected outcome, no message should be received
	}
}


func TestHubStop(t *testing.T) {
	testHub := NewHub()
	go testHub.Run()
	testHub.Stop()
	<-testHub.Done()
	// Check if the hub has stopped and all clients are closed
	if len(testHub.clients) != 0 {
		t.Fatalf("Expected all clients to be closed when hub is stopped, but some clients are still registered")
	}

	// Try stopping the hub again to ensure Stop can be called multiple times without panicking
	testHub.Stop()
	<-testHub.Done()

}

func TestClientClose_Idempotent(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		hub.Stop()
		<-hub.Done()
	})
	testUserID := uuid.New()
	client := &client{userID: testUserID, send: make(chan []byte, 1), closeOnce: sync.Once{}}
	// Call Close multiple times and ensure it does not panic and the send channel is closed
	client.Close()
	client.Close()
	client.Close()
}