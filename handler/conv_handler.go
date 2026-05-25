package handler

import (
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"strings"
)

func ConversationJoinHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {
	if repo == nil {
		logger.Log.Error("ConversationJoinHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	return conversationMembershipHandler(repo, maker, "join")
}

func ConversationLeaveHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {
	if repo == nil {
		logger.Log.Error("ConversationLeaveHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	return conversationMembershipHandler(repo, maker, "leave")
}

func conversationMembershipHandler(repo *repository.Repository, maker *helper.JWTMaker, operation string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodPost {
			logger.Log.Error("Invalid method for ConversationMembershipHandler", "method", r.Method, "operation", operation)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// verify JWT and extract user ID
		userID, err := helper.JWTVerifier(r, maker)
		if err != nil {
			logger.Log.Error("JWT verification failed in ConversationMembershipHandler", "error", err, "operation", operation)
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
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

func ConvListHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodGet {
			logger.Log.Error("Invalid method for convListHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// verify JWT and extract user ID
		userID, err := helper.JWTVerifier(r, maker)
		if err != nil {
			logger.Log.Error("JWT verification failed in convListHandler", "error", err)
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
			return
		}

		// Fetch conversations for the user
		conversations, err := repo.GetConversationsByUserID(r.Context(), userID)
		if err != nil {
			logger.Log.Error("Failed to fetch conversations for user in convListHandler", "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to fetch conversations")
			return
		}

		// Return conversations as JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
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
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// verify JWT and extract user ID
		userID, err := helper.JWTVerifier(r, maker)
		if err != nil {
			logger.Log.Error("JWT verification failed in convCreateHandler", "error", err)
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
			return
		}

		// Parse request body for conversation details (e.g., name, participant IDs)
		var req struct {
			Title          string      `json:"title"`
			ParticipantIDs []uuid.UUID `json:"participant_ids"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Log.Error("Failed to decode request body in convCreateHandler", "error", err)
			writeJSONError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}

		// title should not be empty
		if strings.TrimSpace(req.Title) == "" {
			logger.Log.Error("Conversation title cannot be empty in convCreateHandler", "user_id", userID)
			writeJSONError(w, http.StatusBadRequest, "Conversation title cannot be empty")
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
			writeJSONError(w, http.StatusInternalServerError, "Failed to create conversation")
			return
		}

		logger.Log.Info("Conversation created successfully", "user_id", userID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Conversation created successfully",
		})
	})
}

func ConvMemberListHandler(repo *repository.Repository, maker *helper.JWTMaker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method check
		if r.Method != http.MethodGet {
			logger.Log.Error("Invalid method for convMemberListHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// call JWTVerifier from helper to verify token and extract user ID
		userID, err := helper.JWTVerifier(r, maker)
		if err != nil {
			logger.Log.Error("JWT verification failed in convMemberListHandler", "error", err)
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
