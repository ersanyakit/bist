package security

import (
	"testing"
	"time"
)

func TestPolicyCanChecksRolePermissions(t *testing.T) {
	policy := Policy{Roles: map[string][]string{"analyst": {"analysis:read"}, "admin": {"*"}}}
	if !policy.Can(Principal{Roles: []string{"analyst"}}, "analysis:read") {
		t.Fatal("expected analyst to read analysis")
	}
	if policy.Can(Principal{Roles: []string{"analyst"}}, "admin:write") {
		t.Fatal("analyst must not write admin")
	}
	if !policy.Can(Principal{Roles: []string{"admin"}}, "admin:write") {
		t.Fatal("admin wildcard should allow")
	}
}

func TestRateLimiterEnforcesWindow(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !limiter.Allow("u", now) || !limiter.Allow("u", now.Add(time.Second)) {
		t.Fatal("first two requests should pass")
	}
	if limiter.Allow("u", now.Add(2*time.Second)) {
		t.Fatal("third request should be rejected")
	}
	if !limiter.Allow("u", now.Add(2*time.Minute)) {
		t.Fatal("request after window should pass")
	}
}
