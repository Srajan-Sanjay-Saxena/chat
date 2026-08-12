package handler

import (
	"chat-v2/helper"
	"chat-v2/repository"
	"chat-v2/service"
	"net/http"
)

type Handler struct {
	// Add any dependencies or fields you need here
	Repo         *repository.Repository
	Maker        *helper.JWTMaker
	CacheService *service.CachedMessageService
}

func (h *Handler)HealthCheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Health-Check", "OK")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}
