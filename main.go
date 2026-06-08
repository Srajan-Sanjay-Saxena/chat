package main

import (
	"chat-v2/Middleware"
	"chat-v2/config"
	"chat-v2/db"
	"chat-v2/handler"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"chat-v2/db/redis"
	"chat-v2/ws"
	"context"
	"fmt"
	"github.com/google/uuid"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fmt.Println("Starting chat-v2 server...")

	// Initialize logger
	logger.Init()
	
	// Load configuration
	cfg, err := config.LoadConfig()

	if err != nil {
		logger.Log.Error("Failed to load configuration", "error", err)
		log.Fatalf("configuration loading failed: %v", err)
	}

	port := cfg.Port
	logger.Log.Info("Configuration loaded successfully")

	// Connect to the database
	if err := db.Connect(cfg.DBSource); err != nil {
		logger.Log.Error("Failed to connect to database", "error", err)
		log.Fatalf("database connection failed: %v", err)
	}
	logger.Log.Info("Database connection established")

	// Connect to Redis

	redisClient, err := redis.Connect(cfg.RedisAddr, cfg.RedisUsername, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		logger.Log.Error("Failed to connect to Redis", "error", err)
		log.Fatalf("Redis connection failed: %v", err)
	}
	defer redisClient.Close()
	logger.Log.Info("Connected to Redis")

	// Initialize presence store
	presenceStore := redis.NewPresenceStore(redisClient)
	logger.Log.Info("Presence store initialized")
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
	hub := ws.NewHub(presenceStore)
	go hub.Run()                                          // Start the hub in a separate goroutine
	go hub.StartIdleSweeper(5*time.Minute, 1*time.Minute) // Start sweeper with 5 min idle timeout and 1 min check interval
	logger.Log.Info("WebSocket hub started")

	// Start the server
	mux := http.NewServeMux()
	authMiddleware := Middleware.JWTMiddleware(maker)
	corsMiddleware := Middleware.NewCORSMiddleware(cfg.WSAllowedOrigins)
	mux.Handle("/health", handler.HealthCheckHandler())
	// Authentication routes
	mux.Handle("/api/signup", handler.SignUpHandler(repo)) 
	mux.Handle("/api/login", handler.LoginHandler(repo, maker))
	logger.Log.Info("Authentication handlers registered under /signup and /login")
	// Conversation routes
	mux.Handle("/api/me", authMiddleware(handler.MeHandler(repo)))
	mux.Handle("/api/conversation/join", authMiddleware(handler.ConversationJoinHandler(repo)))
	mux.Handle("/api/conversation/leave", authMiddleware(handler.ConversationLeaveHandler(repo)))
	mux.Handle("/api/conversation/create", authMiddleware(handler.ConvCreateHandler(repo))) 
	mux.Handle("/api/conversation/list", authMiddleware(handler.ConvListHandler(repo)))
	mux.Handle("/api/conversation/members", authMiddleware(handler.ConvMemberListHandler(repo)))
	mux.Handle("/api/conversation/messages", authMiddleware(handler.MessageHandler(repo)))
	mux.Handle("/api/users/search", authMiddleware(handler.UserSearchHandler(repo)))
	logger.Log.Info("Conversation handlers registered under /conversation/*")
	// WebSocket route
	mux.Handle("/api/past_messages", authMiddleware(handler.MessageHandler(repo)))
	logger.Log.Info("Message handler registered at /past_messages")
	mux.Handle("/ws", ws.NewWebSocketHandler(repo, hub, maker, cfg.WSAllowedOrigins,
		func(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
			return repo.IsParticipant(ctx, conversationID, userID)
		},
	))

	mux.Handle("/api/presence", authMiddleware(handler.PresenceHandler(repo, presenceStore)))
	
	// Temporary migration logic
	helper.Migrate()

	// Start the HTTP server
	// Wrap the mux with CORS middleware
	handlerWithCORS := corsMiddleware(mux)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: handlerWithCORS,
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
	hub.Stop()
	<-hub.Done()
	logger.Log.Info("WebSocket hub stopped")

	// Close database connection
	db.GetDB().Close()
	logger.Log.Info("Database connection closed")

	logger.Log.Info("Server stopped")
}
