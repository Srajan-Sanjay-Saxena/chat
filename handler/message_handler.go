package handler

import (
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// MessageHandler handles paginated retrieval of messages for a conversation.
//
// Endpoint:
//   GET /messages?conversation_id=<uuid>&before=<cursor>&limit=<n>
//
// Requirements:
//   - Authenticated user must be present in the request context.
//   - User must be a participant in the conversation.
//
// Pagination:
//   - Messages are returned newest-first.
//   - The `before` cursor retrieves older messages.
//   - `limit` defaults to 30 and is capped by the repository layer.

func MessageHandler(repo *repository.Repository) http.Handler {
	if repo == nil {
		logger.Log.Error("MessageHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			logger.Log.Debug("Invalid method for MessageHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		userID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			logger.Log.Warn("unauthorized access to MessageHandler")
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: user not found in context")
			return
		}

		conversationIDStr := r.URL.Query().Get("conversation_id")
		if conversationIDStr == "" {
			logger.Log.Debug("Missing conversation_id query parameter in MessageHandler")
			writeJSONError(w, http.StatusBadRequest, "Missing conversation_id query parameter")
			return
		}

		conversationID, err := uuid.Parse(conversationIDStr)
		if err != nil {
			logger.Log.Debug("Invalid conversation_id format in MessageHandler", "conversation_id", conversationIDStr, "error", err)
			writeJSONError(w, http.StatusBadRequest, "Invalid conversation_id format")
			return
		}

		isParticipant, err := repo.IsParticipant(r.Context(), conversationID, userID)
		if err != nil {
			logger.Log.Error("Failed to check participant status in MessageHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to check participant status")
			return
		}

		if !isParticipant {
			logger.Log.Info("User is not a participant in the conversation", "conversation_id", conversationID, "user_id", userID)
			writeJSONError(w, http.StatusForbidden, "You are not a participant in this conversation")
			return
		}

		beforeStr := r.URL.Query().Get("before")
		var before *string
		if beforeStr == "" {
			before = nil
		} else {
			before = &beforeStr
		}

		limitStr := r.URL.Query().Get("limit")
		if limitStr == "" {
			limitStr = "30"
		}
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			logger.Log.Debug("Invalid 'limit' parameter format in MessageHandler", "limit", limitStr, "error", err)
			writeJSONError(w, http.StatusBadRequest, "Invalid 'limit' parameter format")
			return
		}

		var msgResp *repository.MessageResponse

		msgResp, err = repo.GetMessagesByConversationID(r.Context(), conversationID, before, limit)
		if err != nil {
			logger.Log.Error("Failed to fetch messages in MessageHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to fetch messages")
			return
		}

		msgRespJSON, err := json.Marshal(msgResp)
		if err != nil {
			logger.Log.Error("Failed to marshal messages response in MessageHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to process messages")
			return
		}

		logger.Log.Debug("Message response prepared successfully", "conversation_id", conversationID, "user_id", userID, "message_count", len(msgResp.Messages))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(msgRespJSON)
	})
}
