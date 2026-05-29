package middleware

import (
	"net/http"
)

// SecurityHeaders sets baseline HTTP security headers on every response.
//
// The service is JSON-only (/health, /send) and is served full-HTTPS behind a
// Cloudflare Tunnel, so there is no CSP to manage, but a few headers still
// apply:
//   - HSTS tells browsers to stick to HTTPS for a year, subdomains included.
//   - nosniff stops MIME sniffing of our JSON responses.
//   - DENY framing and no-referrer harden against clickjacking / referrer leaks
//     even though there is no HTML to embed — defence in depth and cheap.
//   - no-store ensures neither browsers nor intermediary caches (Cloudflare)
//     ever retain an API response, notably /send.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}
