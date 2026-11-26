package accounts

import (
	"sync"
	"time"
)

// RateLimiter enforces a sliding-window request limit per key.
type RateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string][]time.Time
}

// NewRateLimiter creates a sliding-window limiter. A non-positive limit disables limiting.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		now:    time.Now,
		hits:   make(map[string][]time.Time),
	}
}

// Allow records a hit for key when within quota; otherwise returns false and retry-after duration.
func (r *RateLimiter) Allow(key string) (bool, time.Duration) {
	if r == nil || r.limit <= 0 || r.window <= 0 {
		return true, 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	cutoff := now.Add(-r.window)

	existing := r.hits[key]
	pruned := existing[:0]
	for _, ts := range existing {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}

	if len(pruned) >= r.limit {
		r.hits[key] = pruned
		retryAfter := pruned[0].Add(r.window).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	pruned = append(pruned, now)
	r.hits[key] = pruned
	return true, 0
}
