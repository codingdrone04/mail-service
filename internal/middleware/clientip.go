package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP resolves the real client IP for rate-limiting.
//
// The service runs behind a Cloudflare Tunnel, so the TCP RemoteAddr is always
// the tunnel/localhost and would make every visitor share a single limiter
// bucket. Cloudflare sets CF-Connecting-IP to the genuine visitor IP, so we
// trust it first, then fall back to the first hop of X-Forwarded-For, and only
// use RemoteAddr if neither header is present.
//
// WARNING: these headers are only trustworthy because Cloudflare strips and
// rewrites them at the edge. If this service is ever exposed directly (not
// behind Cloudflare), they become client-spoofable and must not be trusted.
func ClientIP(r *http.Request) string {
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
		return cf
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
