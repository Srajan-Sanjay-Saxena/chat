package Middleware

import (
	"github.com/rs/cors"
	"net/http"
)

// CORSMiddleware is a middleware function that sets CORS headers for incoming requests.
func NewCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	// Configure CORS options
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		// AllowedOrigins:   []string{"*"}, // Allow all origins for development; restrict in production
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	})

	return c.Handler
}
