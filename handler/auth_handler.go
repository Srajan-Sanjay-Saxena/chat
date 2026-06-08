package handler

import (
	"chat-v2/db"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"time"
)

type SignUpRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func SignUpHandler(repo *repository.Repository) http.Handler {

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

		// Validate input
		if req.Username == "" || req.Password == "" || req.Email == "" {
			logger.Log.Debug("Missing fields in SignUpRequest", "username", req.Username, "email", req.Email)
			writeJSONError(w, http.StatusBadRequest, "Username, password and email are required")
			return
		}

		if helper.ValidateEmail(req.Email) == false {
			logger.Log.Debug("Invalid email format in SignUpRequest", "email", req.Email)
			writeJSONError(w, http.StatusBadRequest, "Invalid email format")
			return
		}

		if helper.ValidatePassword(req.Password) == false {
			logger.Log.Debug("Weak password in SignUpRequest", "username", req.Username)
			writeJSONError(w, http.StatusBadRequest, "Password does not meet strength requirements")
			return
		}

		if helper.ValidateUsername(req.Username) == false {
			logger.Log.Debug("Invalid username in SignUpRequest", "username", req.Username)
			writeJSONError(w, http.StatusBadRequest, "Username does not meet requirements")
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
		if err := repo.CreateUser(r.Context(), user); err != nil {
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
			"status":  "User created successfully",
			"user_id": user.ID.String(),
		})
	})
}

func LoginHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {

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

		if req.Username == "" || req.Password == "" {
			logger.Log.Error("Missing fields in LoginRequest", "username", req.Username)
			writeJSONError(w, http.StatusBadRequest, "Username and password are required")
			return
		}

		// Get user by username
		user, err := repo.GetUserByUsername(r.Context(), req.Username)
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
		token, err := maker.CreateToken(user.ID, time.Hour*24)
		if err != nil {
			logger.Log.Error("Failed to create token for user", "username", req.Username, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to create token")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":     "Login successful",
			"user_id":    user.ID.String(),
			"username":   user.Username,
			"email":      user.Email,
			"created_at": user.CreatedAt.Format(time.RFC3339),
			"token":      token,
			"expires_at": now.Add(time.Hour * 24).Format(time.RFC3339),
		})
	})
}

func MeHandler(repo *repository.Repository) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodGet {
			logger.Log.Debug("Invalid method for MeHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		userID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			logger.Log.Debug("Failed to get user from context in convCreateHandler")
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: user not found in context")
			return
		}
		
		user,err := repo.GetUserByID(r.Context(), userID)
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