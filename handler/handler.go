package handler

import (
	"net/http"
	"chat-v2/repository"
	"chat-v2/helper"
)

type Handler struct {
	// Add any dependencies or fields you need here
	Repo *repository.Repository
	Maker *helper.JWTMaker
}

func (h *Handler)HealthCheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Health-Check", "OK")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}
