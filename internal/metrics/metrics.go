package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)


// HTTP layer
var (
	// Total Http Requests labels: method, path, status_code
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "relay",
			Name: "http_total_requests",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	// Request Duration labels: method, path
	HTTPRequestsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "relay",
			Name: "http_request_duration_seconds",
			Help: "HTTP request latency distributions in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~16s
		},
		[]string{"method", "path"},
	)
)


// Websocket layer
var (
	// Active WebSocket Connections
	WSConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "relay",
			Name: "ws_connections_active",
			Help: "Number of active WebSocket connections",
		},
	)

	// Total WebSocket Messages labels: direction (inbound, outbound)
	WSMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "relay",
			Name: "ws_messages_total",
			Help: "Total websocket messages sent and received",
		},
		[]string{"direction"}, // inbound or outbound
	)
)

// Cache layer

var (	
	// Cache Hits and Misses labels: cache (message, IsParticipant, etc.)
	CacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "relay",
			Name: "cache_hits_total",
			Help: "Total cache hits",
		},
		[]string{"cache"}, // message, IsParticipant etc. (for more granular metrics)
	)

	// Cache Misses labels: cache (message, IsParticipant, etc.)
	CacheMissesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "relay",
			Name: "cache_misses_total",
			Help: "Total cache misses",
		},
		[]string{"cache"},
	)

	CacheOperationsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "relay",
			Name: "cache_operations_duration_seconds",
			Help: "Cache operation latency distributions in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 15), // 0.1ms to ~3s
		},
		[]string{"cache", "operation"}, // get, set, invalidation etc.
	)
)

// Database layer

var (
	DBQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
			Namespace: "relay",
			Name:      "db_query_duration_seconds",
			Help:      "Database query latency",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.5, 1},
		},
		[]string{"operation"}, // "get_messages", "create_message", "list_conversations", etc.
	)

	DBQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "relay",
			Name:      "db_queries_total",
			Help:      "Total database queries",
		},
		[]string{"operation", "status"}, // status: "success", "error"
	)
)

// Rate Limiter layer

var (
	RateLimitHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "relay",
			Name:      "rate_limit_hits_total",
			Help:      "Total requests rejected by rate limiter",
		},
		[]string{"limiter"}, // "login", "signup", "api", "ws"
	)
)

// Registration

// func init() {
// 	prometheus.MustRegister(
// 		HttpRequestsTotal,
// 		HttpRequestDuration,
// 		WSConnectionsActive,
// 		WSMessagesTotal,
// 		CacheHitsTotal,
// 		CacheMissesTotal,
// 		CacheOperationsDuration,
// 		DBQueryDuration,
// 		DBQueriesTotal,
// 		RateLimitHitsTotal,
// 	)
// }

func Register(reg prometheus.Registerer) {
	reg.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestsDuration,
		WSConnectionsActive,
		WSMessagesTotal,
		CacheHitsTotal,
		CacheMissesTotal,
		CacheOperationsDuration,
		DBQueryDuration,
		DBQueriesTotal,
		RateLimitHitsTotal,
	)
}

