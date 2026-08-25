package observability

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder captures the response status so the middleware can log
// and count it after the handler runs -- http.ResponseWriter doesn't
// expose what was already written.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// LoggingMiddleware logs each request's method, path, status, and
// duration, and increments HTTPRequestsByStatusClass. Wraps the whole
// mux, not individual handlers -- every route gets this for free.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK} // WriteHeader isn't called on an implicit 200
		next.ServeHTTP(rec, r)

		HTTPRequestsByStatusClass.Add(statusClass(rec.status), 1)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func statusClass(status int) string {
	if status < 100 || status > 599 {
		return "other"
	}
	return fmt.Sprintf("%dxx", status/100)
}
