package middleware

import (
	"encoding/json"
	"net"
	"net/http"
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

func getClientIP(r *http.Request) string {
	// Check reverse proxy headers first
	if xff := r.Header.Get("X-Real-IP"); xff != "" {
		return xff
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
