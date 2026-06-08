package handler
import (
	"net/http"
	"chat-v2/repository"
	"chat-v2/logger"
	"chat-v2/helper"
	"chat-v2/db/redis"
	"encoding/json"
)

func PresenceHandler(repo *repository.Repository, p *redis.PresenceStore) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			logger.Log.Debug("Invalid method for presenceHandler", "method", r.Method)
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		userID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			logger.Log.Debug("User ID not found in context for presenceHandler")
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		friends, err := repo.GetFriends(r.Context(), userID)
		if err != nil {
			logger.Log.Error("Failed to get friends for presenceHandler", "user_id", userID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to get friends")
			return
		}
		presenceMap , err := p.GetMassPresence(r.Context(), friends)
		if err != nil {
			logger.Log.Error("Failed to get presence information", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to get presence information")
			return
		}

		prsenceJson , err := json.Marshal(presenceMap)
		if err != nil {
			logger.Log.Error("Failed to marshal presence information", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to marshal presence information")
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(prsenceJson)
	})
}