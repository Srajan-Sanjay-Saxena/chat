package handler

import (
	"chat-v2/db"
	"chat-v2/helper"
	"chat-v2/repository"
	"encoding/json"
	"net/http"
	"time"
	"chat-v2/logger"
	"github.com/google/uuid"
	"errors"
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

	if repo == nil {
		logger.Log.Error("SignUpHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodPost {
			logger.Log.Error("Invalid method for SignUpHandler", "method", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Method not allowed",
			})
			return
		}

		// Parse request body
		var req SignUpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Log.Error("Failed to decode SignUpRequest", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request payload",
			})
			return
		}

		// Validate input
		if req.Username == "" || req.Password == "" || req.Email == "" {
			logger.Log.Error("Missing fields in SignUpRequest", "username", req.Username, "email", req.Email)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Username, password and email are required",
			})
			return
		}

		if helper.ValidateEmail(req.Email) == false {
			logger.Log.Error("Invalid email format in SignUpRequest", "email", req.Email)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email format",
			})
			return
		}

		if helper.ValidatePassword(req.Password) == false {
			logger.Log.Error("Weak password in SignUpRequest", "username", req.Username)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Password does not meet strength requirements",
			})
			return
		}

		if helper.ValidateUsername(req.Username) == false {
			logger.Log.Error("Invalid username in SignUpRequest", "username", req.Username)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Username does not meet requirements",
			})
			return
		}

		// Create user in the database
		hashedPassword, err := helper.HashPassword(req.Password)
		if err != nil {
			logger.Log.Error("Failed to hash password for user", "username", req.Username, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to create user",
			})
			return
		}

		user := &db.User{
			ID: uuid.New(),
			Username: req.Username,
			PasswordHash: hashedPassword,
			Email: req.Email,
			CreatedAt: time.Now(),
		}

		// Create user
		if err := repo.CreateUser(r.Context(), user); err != nil {
			// Check if the error is due to duplicate username 
			
			if errors.Is(err, repository.ErrUserExists) {
				logger.Log.Error("Username already exists", "username", req.Username)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Username or email already exists",
				})
				return
			}

			// Log the error and return a generic error message

			logger.Log.Error("Failed to create user", "username", req.Username, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to create user",
			})
			return
		}

		logger.Log.Info("User created successfully", "username", req.Username)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "User created successfully",
			"user_id": user.ID.String(),
		})
	})
}

func LoginHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {

	if repo == nil {
		logger.Log.Error("LoginHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	if maker == nil {
		logger.Log.Error("LoginHandler initialization failed: JWT maker is nil")
		panic("JWT maker cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodPost {
			logger.Log.Error("Invalid method for LoginHandler", "method", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Method not allowed",
			})
			return
		}

		// Parse request body
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Log.Error("Failed to decode LoginRequest", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request payload",
			})
			return
		}

		if req.Username == ""  || req.Password == "" {
			logger.Log.Error("Missing fields in LoginRequest", "username", req.Username)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Username and password are required",
			})
			return
		}

		// Get user by username
		user, err := repo.GetUserByUsername(r.Context(), req.Username)
		if err != nil || user == nil {
			logger.Log.Error("User not found", "username", req.Username, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid username or password",
			})
			return
		}

		// compare the hashed password
		if !helper.CheckPasswordHash(req.Password, user.PasswordHash) {
			logger.Log.Error("Invalid password for username", "username", req.Username)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid username or password",
			})
			return
		}

		now := time.Now()
		token , err := maker.CreateToken(user.ID, time.Hour * 24)
		if err != nil {
			logger.Log.Error("Failed to create token for user", "username", req.Username, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to create token",
			})
			return
		}

		logger.Log.Info("User logged in successfully", "username", req.Username)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "Login successful",
			"user_id": user.ID.String(),
			"token": token,
			"expires_at" : now.Add(time.Hour * 24).Format(time.RFC3339),
		})
	})
}

