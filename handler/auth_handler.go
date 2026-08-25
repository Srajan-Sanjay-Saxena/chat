package handler

import (
	"chat-v2/db"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type SignUpRequest struct {
	Username string `json:"username" validate:"required,min=3,max=20,alphanum_underscore"`
	Password string `json:"password" validate:"required,min=8"`
	Email    string `json:"email" validate:"required,email"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

func (h *Handler) SignUpHandler() http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodPost {
			logger.Log.Debug("Invalid method for SignUpHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Parse request body
		var req SignUpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Log.Debug("Failed to decode SignUpRequest", "error", err)
			writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		// Validate input using struct tags
		if err := helper.ValidateRequest(&req); err != nil {
			logger.Log.Debug("Validation failed for SignUpRequest", "error", err)
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Create user in the database
		hashedPassword, err := helper.HashPassword(req.Password)
		if err != nil {
			logger.Log.Error("Failed to hash password for user", "username", req.Username, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to create user")
			return
		}

		user := &db.User{
			ID:           uuid.New(),
			Username:     req.Username,
			PasswordHash: hashedPassword,
			Email:        req.Email,
			CreatedAt:    time.Now(),
		}

		// Create user
		if err := h.Repo.CreateUser(r.Context(), user); err != nil {
			// Check if the error is due to duplicate username

			if errors.Is(err, repository.ErrUserExists) {
				logger.Log.Debug("Username already exists", "username", req.Username)
				writeJSONError(w, http.StatusConflict, "Username or email already exists")
				return
			}

			// Log the error and return a generic error message

			logger.Log.Error("Failed to create user", "username", req.Username, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to create user")
			return
		}

		logger.Log.Info("User created successfully", "username", req.Username)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "User created successfully",
			"user_id": user.ID.String(),
		})
	})
}

func (h *Handler) LoginHandler() http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodPost {
			logger.Log.Error("Invalid method for LoginHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Parse request body
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Log.Error("Failed to decode LoginRequest", "error", err)
			writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		// Validate input using struct tags
		if err := helper.ValidateRequest(&req); err != nil {
			logger.Log.Debug("Validation failed for LoginRequest", "error", err)
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Get user by username
		user, err := h.Repo.GetUserByUsername(r.Context(), req.Username)
		if err != nil || user == nil {
			logger.Log.Error("User not found", "username", req.Username, "error", err)
			writeJSONError(w, http.StatusUnauthorized, "Invalid username or password")
			return
		}

		// compare the hashed password
		if !helper.CheckPasswordHash(req.Password, user.PasswordHash) {
			logger.Log.Error("Invalid password for username", "username", req.Username)
			writeJSONError(w, http.StatusUnauthorized, "Invalid username or password")
			return
		}

		now := time.Now()
		token, err := h.Maker.CreateToken(user.ID, time.Hour*24)
		if err != nil {
			logger.Log.Error("Failed to create token for user", "username", req.Username, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to create token")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "access_token",
			Value:    token,
			Expires:  now.Add(time.Hour * 2),
			Path:     "/api/",
			HttpOnly: true,
			Secure:   true,
			// SameSite: http.SameSiteLaxMode,
			SameSite: http.SameSiteNoneMode,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(map[string]string{
			"status":     "Login successful",
			"user_id":    user.ID.String(),
			"username":   user.Username,
			"email":      user.Email,
			"created_at": user.CreatedAt.Format(time.RFC3339),
			"expires_at": now.Add(time.Hour * 24).Format(time.RFC3339),
		})

	})
}

func (h *Handler) MeHandler() http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodGet {
			logger.Log.Debug("Invalid method for MeHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		userID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			logger.Log.Debug("Failed to get user from context in MeHandler")
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: user not found in context")
			return
		}

		user, err := h.Repo.GetUserByID(r.Context(), userID)
		if err != nil {
			logger.Log.Error("Failed to get user by ID in MeHandler", "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to get user information")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"user_id":    user.ID.String(),
			"username":   user.Username,
			"email":      user.Email,
			"created_at": user.CreatedAt.Format(time.RFC3339),
		})

	})
}

func (h *Handler) LogoutHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodPost {
			logger.Log.Debug("Invalid method for LogoutHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "access_token",
			Value:    "",
			Expires:  time.Unix(0, 0),
			Path:     "/api/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "Logout successful",
		})

	})
}
