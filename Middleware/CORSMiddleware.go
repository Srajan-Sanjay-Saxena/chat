package middleware

import (
	"net/http"

	"github.com/rs/cors"
)

// CORSMiddleware is a middleware function that sets CORS headers for incoming requests.
func NewCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	// Configure CORS options
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		// AllowedOrigins:   []string{"*"}, // Allow all origins for development; restrict in production
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"Accept",
			"Origin",
			"X-Requested-With",
		},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	})

	return c.Handler
}
