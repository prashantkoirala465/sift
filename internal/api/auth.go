package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

const oauthStateCookie = "sift_oauth_state"

func registerAuthRoutes(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /auth/google/login", handleGoogleLogin(deps))
	mux.HandleFunc("GET /auth/google/callback", handleGoogleCallback(deps))
}

func handleGoogleLogin(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.OAuthConfig == nil {
			http.Error(w, "Google OAuth is not configured", http.StatusServiceUnavailable)
			return
		}

		state, err := randomState()
		if err != nil {
			slog.Error("generate oauth state", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookie,
			Value:    state,
			Path:     "/auth/google",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   600,
		})

		// AccessTypeOffline + prompt=consent: without both, Google only
		// returns a refresh_token on a mailbox's very first consent grant.
		// Reconnecting after a revoke would silently get an access-only
		// token that stops working in an hour with no way to renew it.
		url := deps.OAuthConfig.AuthCodeURL(state,
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("prompt", "consent"),
		)
		http.Redirect(w, r, url, http.StatusFound)
	}
}

func handleGoogleCallback(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.OAuthConfig == nil {
			http.Error(w, "Google OAuth is not configured", http.StatusServiceUnavailable)
			return
		}

		cookie, err := r.Cookie(oauthStateCookie)
		if err != nil || cookie.Value == "" || r.URL.Query().Get("state") != cookie.Value {
			http.Error(w, "invalid or missing oauth state", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/auth/google", MaxAge: -1})

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing authorization code", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		token, err := deps.OAuthConfig.Exchange(ctx, code)
		if err != nil {
			slog.Error("exchange oauth code", "error", err)
			http.Error(w, "failed to exchange authorization code", http.StatusBadGateway)
			return
		}
		if token.RefreshToken == "" {
			http.Error(w, "Google did not return a refresh token; revoke Sift's access at "+
				"https://myaccount.google.com/permissions and try connecting again", http.StatusBadGateway)
			return
		}

		if err := deps.TokenStore.SaveToken(ctx, token); err != nil {
			slog.Error("save oauth token", "error", err)
			http.Error(w, "failed to store token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<p>Gmail connected. You can close this tab.</p>"))
	}
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
