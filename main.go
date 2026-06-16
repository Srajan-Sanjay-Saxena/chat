package main

import (
	"chat-v2/handler"
	"chat-v2/config"
	"chat-v2/db"
	"chat-v2/db/redis"
	"chat-v2/helper"
	"chat-v2/internal/realtime"
	"chat-v2/logger"
	"chat-v2/middleware"
	"chat-v2/repository"
	"chat-v2/service"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// "github.com/google/uuid"
)

func main() {
	fmt.Println("Starting chat-v2 server...")

	// Initialize logger
	logger.Init()

	// Load configuration
	cfg, err := config.LoadConfig(".env")

	if err != nil {
		logger.Log.Error("Failed to load configuration", "error", err)
	}

	port := cfg.Port
	logger.Log.Info("Configuration loaded successfully")

	// Connect to the database
	DB, err := db.Connect(cfg.DBSource)
	if err != nil {
		logger.Log.Error("Failed to connect to database", "error", err)
		log.Fatalf("database connection failed: %v", err)
	}
	defer DB.Close()
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

	// Initialize repository
	repo, err := repository.NewRepository(DB, "public")
	if err != nil {
		logger.Log.Error("Failed to initialize repository", "error", err)
		log.Fatalf("repository initialization failed: %v", err)
	}
	logger.Log.Info("Repository initialized")

	// Initiaize JWT maker
	maker, err := helper.NewJWTMaker(cfg.JWTSecret)
	if err != nil {
		logger.Log.Error("Failed to initialize JWT maker", "error", err)
		log.Fatalf("JWT maker initialization failed: %v", err)
	}
	logger.Log.Info("JWT maker initialized")

	// Hub for managing WebSocket clients and broadcasting messages
	// hub := ws.NewHub(presenceStore)
	// go hub.Run()                                          // Start the hub in a separate goroutine
	// go hub.StartIdleSweeper(5*time.Minute, 1*time.Minute) // Start sweeper with 5 min idle timeout and 1 min check interval

	hub := realtime.NewHub()
	go hub.Run()
	go hub.StartIdleTimeoutChecker(5*time.Minute, 1*time.Minute)
	publisher := realtime.NewLocalBus(hub.Broadcast)
	subscriptionService := service.NewSubscriptionService(repo)
	messageService := service.NewMessageService(repo, publisher)
	realtimeHandler := realtime.NewRealtimeHandler(hub, subscriptionService, messageService)

	logger.Log.Info("WebSocket hub started")

	h := &handler.Handler{
		Repo:  repo,
		Maker: maker,
	}

	// Start the server
	mux := http.NewServeMux()
	authMiddleware := middleware.JWTMiddleware(maker)
	corsMiddleware := middleware.NewCORSMiddleware(cfg.WSAllowedOrigins)
	mux.Handle("/health", h.HealthCheckHandler())
	// Authentication routes
	mux.Handle("/api/signup", h.SignUpHandler())
	mux.Handle("/api/login", h.LoginHandler())
	mux.Handle("/api/logout", authMiddleware(h.LogoutHandler()))
	logger.Log.Info("Authentication handlers registered under /signup and /login")
	// Conversation routes
	mux.Handle("/api/me", authMiddleware(h.MeHandler()))
	mux.Handle("/api/conversation/join", authMiddleware(h.ConversationJoinHandler()))
	mux.Handle("/api/conversation/leave", authMiddleware(h.ConversationLeaveHandler()))
	mux.Handle("/api/conversation/create", authMiddleware(h.ConvCreateHandler()))
	mux.Handle("/api/conversation/list", authMiddleware(h.ConvListHandler()))
	mux.Handle("/api/conversation/members", authMiddleware(h.ConvMemberListHandler()))
	mux.Handle("/api/conversation/messages", authMiddleware(h.MessageHandler()))
	mux.Handle("/api/users/search", authMiddleware(handler.UserSearchHandler(repo)))
	logger.Log.Info("Conversation handlers registered under /conversation/*")
	// WebSocket route
	mux.Handle("/api/past_messages", authMiddleware(h.MessageHandler()))
	logger.Log.Info("Message handler registered at /past_messages")
	// mux.Handle("/ws", ws.NewWebSocketHandler(repo, hub, maker, cfg.WSAllowedOrigins,
	// 	func(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	// 		return repo.IsParticipant(ctx, conversationID, userID)
	// 	},
	// ))

	mux.Handle("/api/ws", authMiddleware(realtime.NewWSHandler(realtimeHandler, maker, cfg.WSAllowedOrigins)))
	mux.Handle("/api/presence", authMiddleware(h.PresenceHandler(presenceStore)))

	// Temporary migration logic
	if err := helper.Migrate(DB, "public"); err != nil {
		logger.Log.Error("Database migration failed", "error", err)
		log.Fatalf("database migration failed: %v", err)
	}
	logger.Log.Info("Database migration completed successfully")

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

	logger.Log.Info("Server stopped")
}
