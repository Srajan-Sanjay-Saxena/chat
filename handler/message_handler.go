package handler

import (
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"encoding/json"
	"net/http"
	"github.com/google/uuid"
)

func MessageHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {
	if repo == nil {
		logger.Log.Error("MessageHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method!=http.MethodGet {
			logger.Log.Error("Invalid method for MessageHandler", "method", r.Method)
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
			logger.Log.Error("JWT verification failed in MessageHandler", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Unauthorized: " + err.Error(),
			})
			return
		}

		// Extract conversation ID from query parameters
		conversationIDStr := r.URL.Query().Get("conversation_id")
		if conversationIDStr == "" {
			logger.Log.Error("Missing conversation_id query parameter in MessageHandler")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Missing conversation_id query parameter",
			})
			return
		}

		conversationID, err := uuid.Parse(conversationIDStr)
		if err != nil {
			logger.Log.Error("Invalid conversation_id format in MessageHandler", "conversation_id", conversationIDStr, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid conversation_id format",
			})
			return
		}

		// Check if the user is a participant in the conversation
		isParticipant, err := repo.IsParticipant(r.Context(), conversationID, userID)
		if err != nil {
			logger.Log.Error("Failed to check participant status in MessageHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to check participant status",
			})
			return
		}

		if !isParticipant {
			logger.Log.Warn("User is not a participant in the conversation", "conversation_id", conversationID, "user_id", userID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "You are not a participant in this conversation",
			})
			return
		}

		// Fetch messages for the conversation

		var msgResp *repository.MessageResponse

		msgResp, err = repo.GetMessagesByConversationID(r.Context(), conversationID, nil, 50)
		if err != nil {
			logger.Log.Error("Failed to fetch messages in MessageHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to fetch messages",
			})
			return
		}

		// Pagination logic
		msgRespJSON, err := json.Marshal(msgResp)
		if err != nil {
			logger.Log.Error("Failed to marshal messages response in MessageHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to process messages",
			})
			return
		}

		logger.Log.Info("Message response prepared successfully", "conversation_id", conversationID, "user_id", userID, "message_count", len(msgResp.Messages))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(msgRespJSON)
	})
}