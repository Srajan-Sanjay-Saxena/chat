package main

import (
	"chat-v2/config"
	"chat-v2/db"
	"chat-v2/db/redis"
	"chat-v2/handler"
	"chat-v2/helper"
	"chat-v2/internal/cache"
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

	// Load configuration
	cfg, err := config.LoadConfig(".env")

	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	env := cfg.Env

	// Initialize logger
	logger.Init(env)

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

	// Connect to Redis if configured; otherwise continue without Redis-backed presence
	redisClient, err := redis.Connect(cfg.RedisAddr, cfg.RedisUsername, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		logger.Log.Warn("Redis unavailable; continuing without Redis-backed presence", "error", err)
		redisClient = nil
	} else if redisClient != nil {
		defer redisClient.Close()
		logger.Log.Info("Connected to Redis")
	} else {
		logger.Log.Info("Redis not configured; continuing without Redis-backed presence")
	}

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

	var publisher service.EventBus
	if redisClient != nil {
		redisBus := service.NewRedisBus(redisClient)
		if err := redisBus.Subscribe(context.Background(), hub.Broadcast); err != nil {
			logger.Log.Warn("Failed to subscribe RedisBus, falling back to LocalBus", "error", err)
			publisher = realtime.NewLocalBus(hub.Broadcast)
		} else {
			publisher = redisBus
			logger.Log.Info("Using Redis Pub/Sub event bus for real-time message broadcasting")
		}
	} else {
		publisher = realtime.NewLocalBus(hub.Broadcast)
		logger.Log.Info("Using LocalBus in-memory event bus for real-time message broadcasting")
	}

	var msgCache cache.Cache
	if redisClient != nil {
		msgCache = cache.NewRedisCache(redisClient, 24*time.Hour)
		logger.Log.Info("Using RedisCache for message history caching")
	} else {
		msgCache = cache.NewMemoryCache(10 * time.Minute)
		logger.Log.Info("Using MemoryCache fallback for message history caching")
	}

	subscriptionService := service.NewSubscriptionService(repo)
	rawMessageService := service.NewMessageService(repo, publisher)
	cachedMessageService := service.NewCachedMessageService(rawMessageService, repo, msgCache)
	realtimeHandler := realtime.NewRealtimeHandler(hub, subscriptionService, cachedMessageService)

	logger.Log.Info("WebSocket hub started")

	h := &handler.Handler{
		Repo:         repo,
		Maker:        maker,
		CacheService: cachedMessageService,
	}

	// Start the server
	mux := http.NewServeMux()
	authMiddleware := middleware.JWTMiddleware(maker)
	corsMiddleware := middleware.NewCORSMiddleware(cfg.WSAllowedOrigins)
	// Initialize Rate Limiters
	loginLimiter := middleware.LimitMiddleware(redisClient, "login", 5, 1*time.Minute)
	signupLimiter := middleware.LimitMiddleware(redisClient, "signup", 3, 1*time.Hour)
	apiLimiter := middleware.LimitMiddleware(redisClient, "api", 60, 1*time.Minute)
	wsLimiter := middleware.LimitMiddleware(redisClient, "ws_conn", 10, 1*time.Minute)

	mux.Handle("/health", h.HealthCheckHandler())
	// Authentication routes
	mux.Handle("/api/signup", signupLimiter(h.SignUpHandler()))
	mux.Handle("/api/login", loginLimiter(h.LoginHandler()))
	mux.Handle("/api/logout", authMiddleware(h.LogoutHandler()))
	logger.Log.Info("Authentication handlers registered under /signup and /login")
	// Conversation routes
	mux.Handle("/api/me", apiLimiter(authMiddleware(h.MeHandler())))
	mux.Handle("/api/conversation/join", apiLimiter(authMiddleware(h.ConversationJoinHandler())))
	mux.Handle("/api/conversation/leave", apiLimiter(authMiddleware(h.ConversationLeaveHandler())))
	mux.Handle("/api/conversation/create", apiLimiter(authMiddleware(h.ConvCreateHandler())))
	mux.Handle("/api/conversation/list", apiLimiter(authMiddleware(h.ConvListHandler())))
	mux.Handle("/api/conversation/members", apiLimiter(authMiddleware(h.ConvMemberListHandler())))
	mux.Handle("/api/conversation/messages", apiLimiter(authMiddleware(h.MessageHandler())))
	mux.Handle("/api/users/search", apiLimiter(authMiddleware(handler.UserSearchHandler(repo))))
	logger.Log.Info("Conversation handlers registered under /conversation/*")
	// WebSocket route
	mux.Handle("/api/past_messages", apiLimiter(authMiddleware(h.MessageHandler())))
	logger.Log.Info("Message handler registered at /past_messages")

	mux.Handle("/api/ws", wsLimiter(authMiddleware(realtime.NewWSHandler(realtimeHandler, maker, cfg.WSAllowedOrigins))))
	mux.Handle("/api/presence", apiLimiter(authMiddleware(h.PresenceHandler(presenceStore))))

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
