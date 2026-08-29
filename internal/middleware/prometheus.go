package middleware

import (
	"net/http"
	"strconv"
	"time"

	"chat-v2/internal/metrics"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

// Metrics is a middleware that records Prometheus metrics for HTTP requests.
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// path extraction and filteration for metrics
			if r.URL.Path == "/metrics" || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			recorder := newStatusRecorder(w)
			next.ServeHTTP(recorder, r)

			duration := time.Since(start)
			status := strconv.Itoa(recorder.status)

			// Record metrics
			metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			metrics.HTTPRequestsDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())

		})
	}
}
