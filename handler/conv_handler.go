package handler

import (
	"chat-v2/db"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

func ConversationJoinHandler(repo *repository.Repository) http.Handler {
	if repo == nil {
		logger.Log.Error("ConversationJoinHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	return conversationMembershipHandler(repo, "join")
}

func ConversationLeaveHandler(repo *repository.Repository) http.Handler {
	if repo == nil {
		logger.Log.Error("ConversationLeaveHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	return conversationMembershipHandler(repo, "leave")
}

func conversationMembershipHandler(repo *repository.Repository, operation string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodPost {
			logger.Log.Error("Invalid method for ConversationMembershipHandler", "method", r.Method, "operation", operation)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		userID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			logger.Log.Error("Failed to get user from context in ConversationMembershipHandler", "operation", operation)
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: user not found in context")
			return
		}

		// Extract conversation ID from query parameters
		conversationID, err := ConvIdExtract(r)
		if err != nil {
			logger.Log.Error("Failed to extract conversation ID in ConversationMembershipHandler", "error", err, "operation", operation)
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Perform the requested operation (join or leave)

		// disallow join/leave for private conversations
		convObj, err := repo.GetConversationByID(r.Context(), conversationID)
		if err == nil && convObj != nil && convObj.Type == "private" {
			writeJSONError(w, http.StatusForbidden, "cannot join or leave private conversations")
			return
		}

		if operation == "join" {
			err = repo.AddParticipant(r.Context(), conversationID, userID)
			if err != nil {
				logger.Log.Error("Failed to add participant to conversation in ConversationJoinHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
				writeJSONError(w, http.StatusInternalServerError, "Failed to join conversation")
				return
			}
			logger.Log.Info("User joined conversation", "conversation_id", conversationID, "user_id", userID)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Successfully joined conversation"})
			return
		}

		err = repo.RemoveParticipant(r.Context(), conversationID, userID)
		if err != nil {
			logger.Log.Error("Failed to remove participant from conversation in ConversationLeaveHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to leave conversation")
			return
		}

		logger.Log.Info("User left conversation", "conversation_id", conversationID, "user_id", userID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Successfully left conversation"})
	})
}

func ConvListHandler(repo *repository.Repository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodGet {
			logger.Log.Error("Invalid method for convListHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		userID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			logger.Log.Error("Failed to get user from context in convListHandler")
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: user not found in context")
			return
		}

		// Fetch conversations for the user, prefilling the other participant's username for private chats to avoid N+1 queries.
		conversations, err := repo.GetConversationsWithOtherUsernameByUserID(r.Context(), userID)
		if err != nil {
			logger.Log.Error("Failed to fetch conversations for user in convListHandler", "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to fetch conversations")
			return
		}

		// Return conversations as JSON response (includes canonical_name)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"conversations": conversations,
		})
	})
}

func ConvCreateHandler(repo *repository.Repository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodPost {
			logger.Log.Error("Invalid method for convCreateHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		userID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			logger.Log.Error("Failed to get user from context in convCreateHandler")
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: user not found in context")
			return
		}

		// Parse request body for conversation details (e.g., name, participant IDs, type)
		var req struct {
			Type                 string   `json:"type"` // "group" or "private"; defaults to "group"
			Title                string   `json:"title"`
			DisplayName          string   `json:"display_name"`
			ParticipantUsernames []string `json:"participant_usernames"`
		}

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			logger.Log.Error("Failed to decode request body in convCreateHandler", "error", err)
			writeJSONError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}

		// default type
		if req.Type == "" {
			req.Type = "group"
		}

		// title required only for group conversations
		if req.Type == "group" && strings.TrimSpace(req.Title) == "" {
			logger.Log.Error("Conversation title cannot be empty in convCreateHandler", "user_id", userID)
			writeJSONError(w, http.StatusBadRequest, "Conversation title cannot be empty for group conversations")
			return
		}

		currentUser, err := repo.GetUserByID(r.Context(), userID)
		if err != nil {
			logger.Log.Error("Failed to fetch current user in convCreateHandler", "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to create conversation")
			return
		}

		usernameOrder := make([]string, 0, len(req.ParticipantUsernames)+1)
		usernameLookup := make(map[string]string, len(req.ParticipantUsernames)+1)

		addUsername := func(username string) {
			clean := strings.TrimSpace(username)
			if clean == "" {
				return
			}
			key := strings.ToLower(clean)
			if _, exists := usernameLookup[key]; exists {
				return
			}
			usernameLookup[key] = clean
			usernameOrder = append(usernameOrder, clean)
		}

		for _, username := range req.ParticipantUsernames {
			addUsername(username)
		}
		addUsername(currentUser.Username)

		// Private conversation rules: must end up with exactly 2 participants.
		if req.Type == "private" && len(usernameOrder) != 2 {
			writeJSONError(w, http.StatusBadRequest, "private conversations must have exactly 2 participants")
			return
		}

		resolvedUsernames := make([]string, 0, len(usernameOrder))
		for _, username := range usernameOrder {
			resolvedUsernames = append(resolvedUsernames, strings.ToLower(strings.TrimSpace(username)))
		}

		conv := &db.Conversation{
			Type:        req.Type,
			Title:       req.Title,
			DisplayName: req.DisplayName,
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
		}

		// For private chats, derive canonical_name and a display name.
		if conv.Type == "private" {
			if len(resolvedUsernames) == 2 && resolvedUsernames[0] != "" && resolvedUsernames[1] != "" {
				if resolvedUsernames[0] > resolvedUsernames[1] {
					resolvedUsernames[0], resolvedUsernames[1] = resolvedUsernames[1], resolvedUsernames[0]
				}
				conv.CanonicalName = resolvedUsernames[0] + ":" + resolvedUsernames[1]
			}

			if strings.TrimSpace(conv.DisplayName) == "" {
				for _, username := range usernameOrder {
					if strings.EqualFold(username, currentUser.Username) {
						continue
					}
					conv.DisplayName = username
					break
				}
			}

			if conv.CanonicalName != "" {
				if existing, err := repo.GetConversationByCanonicalName(r.Context(), conv.CanonicalName); err == nil && existing != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]any{"conversation": existing, "created": false})
					return
				}
			}
		}

		err = repo.CreateConversationWithParticipantsByUsernames(r.Context(), conv, usernameOrder)
		if err != nil {
			if errors.Is(err, repository.ErrConversationExists) && conv.CanonicalName != "" {
				if existing, err2 := repo.GetConversationByCanonicalName(r.Context(), conv.CanonicalName); err2 == nil && existing != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]any{"conversation": existing, "created": false})
					return
				}
			}
			logger.Log.Error("Failed to create conversation in convCreateHandler", "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to create conversation")
			return
		}

		logger.Log.Info("Conversation created successfully", "user_id", userID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"conversation": conv, "created": true})
	})
}

func ConvMemberListHandler(repo *repository.Repository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodGet {
			logger.Log.Error("Invalid method for convMemberListHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		userID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			logger.Log.Error("Failed to get user from context in convMemberListHandler")
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: user not found in context")
			return
		}

		// Extract conversation ID from query parameters
		conversationID, err := ConvIdExtract(r)
		if err != nil {
			logger.Log.Error("Failed to extract conversation ID in convMemberListHandler", "error", err)
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Check if the user is a participant in the conversation
		statusCode, err := participantCheck(r, repo, conversationID, userID)
		if err != nil {
			writeJSONError(w, statusCode, err.Error())
			return
		}

		// Fetch participants for the conversation
		participants, err := repo.GetParticipantsByConversationID(r.Context(), conversationID)
		if err != nil {
			logger.Log.Error("Failed to fetch participants for conversation in convMemberListHandler", "conversation_id", conversationID, "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to fetch participants")
			return
		}

		// Return participants as JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"participants": participants,
		})
	})
}

func ConvIdExtract(r *http.Request) (uuid.UUID, error) {
	conversationIDStr := r.URL.Query().Get("conversation_id")
	if conversationIDStr == "" {
		return uuid.Nil, fmt.Errorf("missing conversation_id query parameter")
	}

	conversationID, err := uuid.Parse(conversationIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid conversation_id format: %w", err)
	}

	return conversationID, nil
}

func participantCheck(r *http.Request, repo *repository.Repository, conversationID, userID uuid.UUID) (int, error) {
	isParticipant, err := repo.IsParticipant(r.Context(), conversationID, userID)
	if err != nil {
		logger.Log.Error("Failed to check participant status", "conversation_id", conversationID, "user_id", userID, "error", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to check participant status: %w", err)
	}
	if !isParticipant {
		logger.Log.Warn("User is not a participant in conversation", "conversation_id", conversationID, "user_id", userID)
		return http.StatusForbidden, fmt.Errorf("you are not a participant in this conversation")
	}
	return 200, nil
}
