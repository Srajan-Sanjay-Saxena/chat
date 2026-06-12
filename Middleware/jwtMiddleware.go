package middleware

import (
	"chat-v2/helper"
	"net/http"
)

func JWTMiddleware(maker *helper.JWTMaker) func(http.Handler) http.Handler {
	// Middleware to authenticate requests using JWT tokens
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract the Bearer token
			token, err := helper.ExtractBearerToken(r)
			if err != nil {
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Verify the token and extract user claims
			claims, err := maker.VerifyToken(token)
			if err != nil {
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Set the token in the request context for downstream handlers to use
			ctx := helper.SetUserContext(r.Context(), claims.ID)
			r = r.WithContext(ctx)

			// Call the next handler in the chain
			next.ServeHTTP(w, r)
		})
	}
}
