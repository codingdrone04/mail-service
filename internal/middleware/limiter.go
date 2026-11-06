package middleware

import (
	"sync"
	"time"
)

// Limiter is a fixed-window counter keyed by any string (site + IP, site, …).
// It lives in memory: counters reset when the service restarts, which is fine
// for anti-abuse limits on a single instance.
type Limiter struct {
	mu      sync.Mutex
	windows map[string]*window
	now     func() time.Time
}

type window struct {
	start time.Time
	span  time.Duration
	count int
}

func NewLimiter() *Limiter {
	l := &Limiter{windows: map[string]*window{}, now: time.Now}
	go func() {
		for range time.Tick(5 * time.Minute) {
			l.sweep()
		}
	}()
	return l
}

// Allow counts one hit for key and reports whether it stays within max per span.
// max <= 0 means unlimited.
func (l *Limiter) Allow(key string, max int, span time.Duration) bool {
	if max <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w, ok := l.windows[key]
	if !ok || now.Sub(w.start) >= w.span {
		l.windows[key] = &window{start: now, span: span, count: 1}
		return true
	}
	if w.count >= max {
		return false
	}
	w.count++
	return true
}

func (l *Limiter) sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for k, w := range l.windows {
		if now.Sub(w.start) >= w.span {
			delete(l.windows, k)
		}
	}
}
