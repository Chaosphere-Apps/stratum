package security

import (
	"testing"
	"time"
)

func TestRateLimiterResetsAndSeparatesKeys(t *testing.T) {
	limiter := NewRateLimiter()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	if allowed, _ := limiter.Allow("login:a", 2, time.Minute); !allowed {
		t.Fatal("first request should be allowed")
	}
	if allowed, _ := limiter.Allow("login:a", 2, time.Minute); !allowed {
		t.Fatal("second request should be allowed")
	}
	if allowed, retry := limiter.Allow("login:a", 2, time.Minute); allowed || retry <= 0 {
		t.Fatalf("third request allowed=%v retry=%s", allowed, retry)
	}
	if allowed, _ := limiter.Allow("login:b", 2, time.Minute); !allowed {
		t.Fatal("separate key should be allowed")
	}
	now = now.Add(time.Minute)
	if allowed, _ := limiter.Allow("login:a", 2, time.Minute); !allowed {
		t.Fatal("expired window should reset")
	}
}

func TestRateLimiterCapsUntrustedKeyCardinality(t *testing.T) {
	limiter := NewRateLimiter()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	for index := 0; index < 10000; index++ {
		limiter.buckets[string(rune(index+1))] = rateBucket{count: 1, resetAt: now.Add(time.Minute)}
	}
	if allowed, retry := limiter.Allow("overflow", 1, time.Minute); allowed || retry != time.Minute {
		t.Fatalf("overflow key allowed=%v retry=%s", allowed, retry)
	}
	now = now.Add(time.Minute)
	if allowed, _ := limiter.Allow("overflow", 1, time.Minute); !allowed {
		t.Fatal("expired buckets should be reclaimed")
	}
}
