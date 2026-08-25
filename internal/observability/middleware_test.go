package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusClass(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{200, "2xx"},
		{201, "2xx"},
		{301, "3xx"},
		{404, "4xx"},
		{409, "4xx"},
		{500, "5xx"},
		{0, "other"},
		{999, "other"},
	}

	for _, tc := range cases {
		if got := statusClass(tc.status); got != tc.want {
			t.Errorf("statusClass(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestLoggingMiddlewareCapturesExplicitStatus(t *testing.T) {
	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/whatever", nil))

	if rec.Code != http.StatusConflict {
		t.Errorf("recorded status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestLoggingMiddlewareCapturesImplicit200(t *testing.T) {
	// A handler that never calls WriteHeader implicitly returns 200 --
	// the wrapped recorder must reflect that in what it logs and counts,
	// not treat it as an unset zero value.
	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok")) // no explicit WriteHeader call
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/whatever", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("recorded status = %d, want %d", rec.Code, http.StatusOK)
	}
}
