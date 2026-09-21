package security

import (
	"sync"
	"time"
)

type rateBucket struct {
	count   int
	resetAt time.Time
}

// RateLimiter is a bounded, process-local fixed-window limiter. It protects
// authentication endpoints even in stateless mode. Multi-replica deployments
// should additionally enforce a distributed limit at the ingress layer.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
	now     func() time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{buckets: make(map[string]rateBucket), now: time.Now}
}

func (l *RateLimiter) Allow(key string, limit int, window time.Duration) (bool, time.Duration) {
	if limit <= 0 || window <= 0 {
		return true, 0
	}
	now := l.now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, exists := l.buckets[key]
	if !exists && len(l.buckets) >= 10000 {
		for bucketKey, candidate := range l.buckets {
			if !now.Before(candidate.resetAt) {
				delete(l.buckets, bucketKey)
			}
		}
		if len(l.buckets) >= 10000 {
			return false, window
		}
	}
	if bucket.resetAt.IsZero() || !now.Before(bucket.resetAt) {
		bucket = rateBucket{resetAt: now.Add(window)}
	}
	if bucket.count >= limit {
		return false, bucket.resetAt.Sub(now).Round(time.Second)
	}
	bucket.count++
	l.buckets[key] = bucket
	return true, 0
}
