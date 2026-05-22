package handler

import (
	"chat-v2/logger"
	"net/http"
	"chat-v2/repository"
	"github.com/google/uuid"
	"chat-v2/helper"
	"encoding/json"
	"strings"
)

// Small Info *.com/conversation/join?conversation_id=123e4567-e89b-12d3-a456-426614174000 for joining a conversation
// Small Info *.com/conversation/leave?conversation_id=123e4567-e89b-12d3-a456-426614174000 for leaving a conversation

func ConversationHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {
	if repo == nil {
		logger.Log.Error("ConversationJoinHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodPost {
			logger.Log.Error("Invalid method for ConversationJoinHandler", "method", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Method not allowed",
			})
			return
		}

		// call JWTVerifier from helper to verify token and extract user ID
		userID, err := helper.JWTVerifier(r, maker)
		if err != nil {
			logger.Log.Error("JWT verification failed in ConversationJoinHandler", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Unauthorized: " + err.Error(),
			})
			return
		}
		// Extract operation (join/leave) from URL path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 2 {
			logger.Log.Error("Invalid URL path for ConversationJoinHandler", "path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid URL path",
			})
			return
		}

		operation := pathParts[len(pathParts)-1]
		if operation != "join" && operation != "leave" {
			logger.Log.Error("Invalid operation in URL path for ConversationJoinHandler", "operation", operation)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid operation in URL path",
			})
			return
		}

		// Extract conversation ID from URL query parameters
		conversationIDStr := r.URL.Query().Get("conversation_id")
		if conversationIDStr == "" {
			logger.Log.Error("Missing conversation_id query parameter in ConversationJoinHandler")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Missing conversation_id query parameter",
			})
			return
		}
		conversationID, err := uuid.Parse(conversationIDStr)

		if err != nil {
			logger.Log.Error("Invalid conversation_id format in ConversationJoinHandler", "conversation_id", conversationIDStr, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid conversation_id format",
			})
			return
		}

		if operation == "join" {
			// Add user to conversation participants
			err = repo.AddParticipant(r.Context(), conversationID, userID)
			if err != nil {
				logger.Log.Error("Failed to add participant to conversation in ConversationJoinHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to join conversation",
				})
				return
			}
			logger.Log.Info("User joined conversation", "conversation_id", conversationID, "user_id", userID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Successfully joined conversation",
			})
			return
		} else if operation == "leave" {
			// Remove user from conversation participants
			err = repo.RemoveParticipant(r.Context(), conversationID, userID)
			if err != nil {
				logger.Log.Error("Failed to remove participant from conversation in ConversationJoinHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to leave conversation",
				})
				return
			}
			logger.Log.Info("User left conversation", "conversation_id", conversationID, "user_id", userID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Successfully left conversation",
			})
		}
	})
}


func ConvListHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodGet {
			logger.Log.Error("Invalid method for convListHandler", "method", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Method not allowed",
			})
			return
		}

		// call JWTVerifier from helper to verify token and extract user ID
		userID, err := helper.JWTVerifier(r, maker)
		if err != nil {
			logger.Log.Error("JWT verification failed in convListHandler", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Unauthorized: " + err.Error(),
			})
			return
		}

		// Fetch conversations for the user
		conversations, err := repo.GetConversationsByUserID(r.Context(), userID)
		if err != nil {
			logger.Log.Error("Failed to fetch conversations for user in convListHandler", "user_id", userID, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to fetch conversations",
			})
			return
		}

		// Return conversations as JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"conversations": conversations,
		})
	})
}

func ConvCreateHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check	
		if r.Method != http.MethodPost {
			logger.Log.Error("Invalid method for convCreateHandler", "method", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Method not allowed",
			})
			return
		}

		// call JWTVerifier from helper to verify token and extract user ID
		userID, err := helper.JWTVerifier(r, maker)
		if err != nil {
			logger.Log.Error("JWT verification failed in convCreateHandler", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Unauthorized: " + err.Error(),
			})
			return
		}
		
		// Parse request body for conversation details (e.g., name, participant IDs)
		var req struct {
			Title          string   `json:"title"`
			ParticipantIDs []uuid.UUID `json:"participant_ids"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Log.Error("Failed to decode request body in convCreateHandler", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// title should not be empty
		if strings.TrimSpace(req.Title) == "" {
			logger.Log.Error("Conversation title cannot be empty in convCreateHandler", "user_id", userID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Conversation title cannot be empty",
			})
			return
		}

		// Create conversation in the database
		// Ensure the creator is included in the participant list
		foundCreator := false
		for _, id := range req.ParticipantIDs {
			if id == userID {
				foundCreator = true
				break
			}
		}
		if !foundCreator {
			req.ParticipantIDs = append(req.ParticipantIDs, userID)
		}

		err = repo.CreateConversationWithParticipants(r.Context(), req.Title, req.ParticipantIDs)
		if err != nil {
			logger.Log.Error("Failed to create conversation in convCreateHandler", "user_id", userID, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to create conversation",
			})
			return
		}

		logger.Log.Info("Conversation created successfully", "user_id", userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Conversation created successfully",
		})
	})	
}