package security

import (
	"strings"
	"sync"
	"time"
)

type Principal struct {
	ID     string   `json:"id"`
	Tenant string   `json:"tenant,omitempty"`
	Roles  []string `json:"roles,omitempty"`
}

type Policy struct {
	Roles map[string][]string `json:"roles"`
}

func (p Policy) Can(principal Principal, permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	for _, role := range principal.Roles {
		for _, allowed := range p.Roles[role] {
			if allowed == permission || allowed == "*" {
				return true
			}
		}
	}
	return false
}

type RateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string][]time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{limit: limit, window: window, requests: map[string][]time.Time{}}
}

func (r *RateLimiter) Allow(key string, now time.Time) bool {
	if key == "" {
		key = "anonymous"
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := now.Add(-r.window)
	kept := r.requests[key][:0]
	for _, ts := range r.requests[key] {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= r.limit {
		r.requests[key] = kept
		return false
	}
	kept = append(kept, now)
	r.requests[key] = kept
	return true
}
