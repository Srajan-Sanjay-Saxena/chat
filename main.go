package main

import (
	"chat-v2/config"
	"chat-v2/db"
	"chat-v2/handler"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"chat-v2/ws"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/joho/godotenv"
	"github.com/google/uuid"
)

func main() {
	// Load .env file
	godotenv.Load()

	// Initialize logger
	logger.Init()

	// Load configuration
	cfg := config.LoadConfig()
	port := cfg.Port
	if port == "" {
		logger.Log.Warn("PORT not set in environment, defaulting to 8080")
		port = "8080"
	}
	logger.Log.Info("Configuration loaded", "port", port)

	// Connect to the database
	if err := db.Connect(cfg.DbSource); err != nil {
		logger.Log.Error("Failed to connect to database", "error", err)
		log.Fatalf("database connection failed: %v", err)
	}
	logger.Log.Info("Database connection established")

	
	// Initialize repositories and handlers

	repo := repository.NewRepository(db.GetDB())
	logger.Log.Info("Repository initialized")

	// Initiaize JWT maker
	maker, err := helper.NewJWTMaker(cfg.JWTSecret)
	if err != nil {
		logger.Log.Error("Failed to initialize JWT maker", "error", err)
		log.Fatalf("JWT maker initialization failed: %v", err)
	}
	logger.Log.Info("JWT maker initialized")

	// Hub for managing WebSocket clients and broadcasting messages
	hub := ws.NewHub()
	go hub.Run()
	logger.Log.Info("WebSocket hub started")

	// Start the server
	mux := http.NewServeMux()
	logger.Log.Info("Registering handlers")
	mux.Handle("/health", handler.HealthCheckHandler())
	logger.Log.Info("Health check handler registered at /health")
	mux.Handle("/signup", handler.SignUpHandler(repo))
	logger.Log.Info("Sign-up handler registered at /signup")
	mux.Handle("/login", handler.LoginHandler(repo, maker))
	logger.Log.Info("Login handler registered at /login")
	mux.Handle("/ws", ws.NewWebSocketHandler(maker, hub, 
		func(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
			return repo.IsParticipant(ctx, conversationID, userID)
		},
	))

	// Start the HTTP server
	server := &http.Server{
		Addr: fmt.Sprintf(":%s", port),
		Handler: mux,
	}

	go func() {
		logger.Log.Info("Starting server", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("Server failed to start", "error", err)
			log.Fatalf("server failed to start: %v", err)
		}
	}()

	// Graceful shutdown handling

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logger.Log.Info("Shutdown signal received, shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	// Close database connection
	db.GetDB().Close()
	logger.Log.Info("Database connection closed")

	logger.Log.Info("Server stopped")
}
