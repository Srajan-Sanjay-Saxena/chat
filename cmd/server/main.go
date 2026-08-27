package main

import (
	"chat-v2/internal/auth"
	"chat-v2/internal/config"
	"chat-v2/internal/conversation"
	"chat-v2/internal/message"
	"chat-v2/internal/middleware"
	"chat-v2/internal/pkg/logger"
	"chat-v2/internal/realtime"
	"chat-v2/internal/storage/postgres"
	"chat-v2/internal/storage/redis"
	"chat-v2/internal/user"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Load config
	cfg, err := config.Load(".env")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Init logger
	logger.Init(cfg.Env)
	logger.Info("Starting Relay server...")

	// Connect to Postgres
	db, err := postgres.NewClient(cfg.DBSource)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()
	logger.Info("Database connected")

	// Connect to Redis
	redisClient, err := redis.NewClientOrBoot(cfg.RedisAddr, cfg.RedisUsername, cfg.RedisPassword, cfg.RedisDB, cfg.Env)
	if err != nil || redisClient == nil {
		if cfg.Env == "production" {
			log.Fatalf("Redis required in production: %v", err)
		}
		logger.Warn("Redis unavailable, using in-memory fallbacks")
	} else {
		defer redisClient.Close()
		logger.Info("Redis connected")
	}

	// Init stores
	presenceStore := redis.NewPresenceStore(redisClient)

	// Init repositories
	userRepo := user.NewRepository(db)
	convRepo := conversation.NewRepository(db)
	msgRepo := message.NewRepository(db)

	// Init JWT
	jwtMaker, err := auth.NewJWTMaker()
	if err != nil {
		log.Fatalf("Failed to create JWT maker: %v", err)
	}

	// Init caches
	partCache := conversation.NewParticipantCache(redisClient, convRepo)

	var msgCache message.Cache
	if redisClient != nil {
		msgCache = message.NewRedisCache(redisClient, 24*time.Hour)
	} else {
		msgCache = message.NewMemoryCache(10 * time.Minute)
	}

	// Init hub
	hub := realtime.NewHub()
	go hub.Run()
	go hub.StartIdleChecker(5*time.Minute, 1*time.Minute)

	// Init publisher
	var publisher message.EventPublisher
	if redisClient != nil {
		redisPub := realtime.NewRedisPublisher(redisClient)
		if err := redisPub.Subscribe(context.Background(), hub); err != nil {
			logger.Warn("Redis pub/sub failed, using local", "error", err)
			publisher = realtime.NewLocalPublisher(hub)
		} else {
			publisher = redisPub
			logger.Info("Using Redis Pub/Sub")
		}
	} else {
		publisher = realtime.NewLocalPublisher(hub)
		logger.Info("Using local publisher")
	}

	// Init services
	msgService := message.NewService(msgRepo, convRepo, partCache, publisher, redisClient)
	cachedMsgService := message.NewCachedService(msgService, msgCache)

	// Init handlers
	userHandler := user.NewHandler(userRepo, jwtMaker)
	convHandler := conversation.NewHandler(convRepo, userRepo, presenceStore, partCache)
	msgHandler := message.NewHandler(msgRepo, convRepo, msgCache)
	wsHandler := realtime.NewHandler(hub, convRepo, cachedMsgService, presenceStore, cfg.WSAllowedOrigins)

	// Setup routes
	mux := http.NewServeMux()
	authMW := auth.Middleware(jwtMaker)

	// Rate limiters
	loginRL := middleware.RateLimit(redisClient, "login", 5, time.Minute)
	signupRL := middleware.RateLimit(redisClient, "signup", 3, time.Hour)
	apiRL := middleware.RateLimit(redisClient, "api", 60, time.Minute)
	wsRL := middleware.RateLimit(redisClient, "ws", 10, time.Minute)

	// Health
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Auth routes
	mux.Handle("/api/signup", signupRL(userHandler.SignUp()))
	mux.Handle("/api/login", loginRL(userHandler.Login()))
	mux.Handle("/api/logout", authMW(userHandler.Logout()))
	mux.Handle("/api/me", apiRL(authMW(userHandler.Me())))
	mux.Handle("/api/users/search", apiRL(authMW(userHandler.Search())))

	// Conversation routes
	mux.Handle("/api/conversation/create", apiRL(authMW(convHandler.Create())))
	mux.Handle("/api/conversation/list", apiRL(authMW(convHandler.List())))
	mux.Handle("/api/conversation/join", apiRL(authMW(convHandler.Join())))
	mux.Handle("/api/conversation/leave", apiRL(authMW(convHandler.Leave())))
	mux.Handle("/api/conversation/members", apiRL(authMW(convHandler.Members())))
	mux.Handle("/api/conversation/messages", apiRL(authMW(msgHandler.List())))

	// Presence
	mux.Handle("/api/presence", apiRL(authMW(convHandler.Presence())))

	// WebSocket
	mux.Handle("/api/ws", wsRL(authMW(wsHandler)))

	// Apply CORS
	handler := middleware.CORS(cfg.WSAllowedOrigins)(mux)

	// Start server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: handler,
	}

	go func() {
		logger.Info("Server starting", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server.Shutdown(ctx)
	hub.Stop()
	<-hub.Done()

	logger.Info("Server stopped")
}
