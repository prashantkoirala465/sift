package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuthMiddleware(t *testing.T) {
	protected := BasicAuthMiddleware("correct-horse-battery-staple", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name       string
		setAuth    bool
		user, pass string
		wantStatus int
	}{
		{"no credentials", false, "", "", http.StatusUnauthorized},
		{"wrong password", true, "sift", "wrong", http.StatusUnauthorized},
		{"correct password, any username", true, "anyone", "correct-horse-battery-staple", http.StatusOK},
		{"correct password, empty username", true, "", "correct-horse-battery-staple", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.setAuth {
				req.SetBasicAuth(tc.user, tc.pass)
			}
			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusUnauthorized {
				if got := rec.Header().Get("WWW-Authenticate"); got == "" {
					t.Error("expected a WWW-Authenticate header on a 401")
				}
			}
		})
	}
}
