package middleware

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := &Limiter{windows: map[string]*window{}, now: func() time.Time { return now }}
	for i := 0; i < 2; i++ {
		if !l.Allow("k", 2, time.Minute) {
			t.Fatal("should allow")
		}
	}
	if l.Allow("k", 2, time.Minute) {
		t.Fatal("third hit must be refused")
	}
	if !l.Allow("other", 2, time.Minute) || !l.Allow("k", 0, time.Minute) {
		t.Fatal("keys are independent and 0 means unlimited")
	}
	now = now.Add(time.Minute)
	if !l.Allow("k", 2, time.Minute) {
		t.Fatal("window should reset")
	}
}
