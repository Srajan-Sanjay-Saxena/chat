package handler

import (
	"net/http"
)

func HealthCheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Health-Check", "OK")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

