// Package auth gates Sift's HTTP surface with a single shared password.
// There's exactly one legitimate user of a self-hosted Sift instance --
// the person who owns the inbox it reads -- so there's no user table,
// login form, or session store, just an operator-configured secret
// checked on every request.
package auth

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuthMiddleware wraps next behind HTTP Basic Auth. Username is
// ignored; only the password is checked, via constant-time comparison so
// response timing can't leak how much of a guessed password was correct.
func BasicAuthMiddleware(password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Sift"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
