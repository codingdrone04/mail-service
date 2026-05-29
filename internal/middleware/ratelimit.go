package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type visitor struct {
	count    int
	lastSeen time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	max      int
	once     sync.Once
}

func RateLimit(maxPerMinute int, next http.Handler) http.Handler {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		max:      maxPerMinute,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rl.once.Do(func() {
			go rl.cleanup()
		})

		ip := getClientIP(r)

		rl.mu.Lock()
		v, exists := rl.visitors[ip]
		if !exists {
			rl.visitors[ip] = &visitor{count: 1, lastSeen: time.Now()}
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		if time.Since(v.lastSeen) > time.Minute {
			v.count = 1
			v.lastSeen = time.Now()
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		if v.count >= rl.max {
			rl.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"message": "Too many requests, please try again later",
			})
			return
		}

		v.count++
		v.lastSeen = time.Now()
		rl.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func (rl *rateLimiter) cleanup() {
	for {
		time.Sleep(3 * time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// getClientIP resolves the real client IP for rate-limiting.
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
func getClientIP(r *http.Request) string {
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
		return cf
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// XFF is a comma-separated list; the first entry is the original client.
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
