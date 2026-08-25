// Package api wires up Sift's HTTP surface.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/prashantkoirala465/sift/internal/gmail"
)

// Deps are the dependencies handlers need. OAuthConfig is nil when Google
// OAuth hasn't been configured -- the auth routes still register, they just
// respond 503 until it is.
type Deps struct {
	OAuthConfig *oauth2.Config
	TokenStore  gmail.TokenStore
}

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealthz)
	registerAuthRoutes(mux, deps)

	return mux
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeJSON writes v as a JSON response. Encode failures here are almost
// always a client that disconnected mid-write, not something the caller can
// act on, so we log rather than propagate.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write json response failed", "error", err)
	}
}
