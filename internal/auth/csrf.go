package auth

import (
	"net/http"
	"net/url"
)

// OriginCheckMiddleware rejects state-changing requests (anything but
// GET/HEAD/OPTIONS) whose Origin or Referer header names a different host
// than the one serving the request.
//
// Sift authenticates with HTTP Basic Auth, and browsers auto-attach cached
// Basic Auth credentials to *any* request to an origin they're cached for
// -- including one a malicious page on another site triggers via a hidden
// form, with no way for the victim to notice. Basic Auth alone does not
// protect against this; it's the same weakness cookies have, and the
// mitigation is the same: verify where the request actually came from.
//
// A same-site browser request always carries an Origin or Referer naming
// this host. A request with neither (curl, a script, a direct API client)
// isn't a browser page triggering an action on the user's behalf, so
// there's nothing for a forged cross-site request to exploit -- it's let
// through rather than breaking direct API use.
func OriginCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		source := r.Header.Get("Origin")
		if source == "" {
			source = r.Header.Get("Referer")
		}
		if source == "" {
			next.ServeHTTP(w, r)
			return
		}

		u, err := url.Parse(source)
		if err != nil || !hostMatches(u.Host, r) {
			http.Error(w, "cross-origin request rejected", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hostMatches also accepts X-Forwarded-Host, so this doesn't break a
// deployment sitting behind the reverse proxy the README itself
// recommends for TLS termination -- r.Host there is the proxy's internal
// address, not the public one Origin/Referer will name.
func hostMatches(originHost string, r *http.Request) bool {
	if originHost == r.Host {
		return true
	}
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" && originHost == fwd {
		return true
	}
	return false
}
