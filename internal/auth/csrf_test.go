package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginCheckMiddleware(t *testing.T) {
	protected := OriginCheckMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name          string
		method        string
		host          string
		origin        string
		referer       string
		forwardedHost string
		wantStatus    int
	}{
		{"GET ignores origin entirely", http.MethodGet, "sift.example", "https://evil.example", "", "", http.StatusOK},
		{"POST no origin or referer (curl/API client)", http.MethodPost, "sift.example", "", "", "", http.StatusOK},
		{"POST matching origin", http.MethodPost, "sift.example", "https://sift.example", "", "", http.StatusOK},
		{"POST matching origin with port", http.MethodPost, "sift.example:8080", "http://sift.example:8080", "", "", http.StatusOK},
		{"POST cross-origin forged request", http.MethodPost, "sift.example", "https://evil.example", "", "", http.StatusForbidden},
		{"POST falls back to referer when origin absent", http.MethodPost, "sift.example", "", "https://sift.example/applications", "", http.StatusOK},
		{"POST cross-site referer", http.MethodPost, "sift.example", "", "https://evil.example/attack", "", http.StatusForbidden},
		{"POST malformed origin", http.MethodPost, "sift.example", "not a url \x7f", "", "", http.StatusForbidden},
		{"POST origin matches X-Forwarded-Host behind a proxy", http.MethodPost, "127.0.0.1:8080", "https://sift.example", "", "sift.example", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/applications", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}
			if tc.forwardedHost != "" {
				req.Header.Set("X-Forwarded-Host", tc.forwardedHost)
			}

			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}
