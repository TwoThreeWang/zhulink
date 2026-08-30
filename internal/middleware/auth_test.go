package middleware

import (
	"testing"
	"time"
)

func TestValidSessionTimes(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	issuedAt := now.Add(-48 * time.Hour).Unix()
	renewedAt := now.Add(-12 * time.Hour).Unix()
	expiresAt := now.Add(90 * 24 * time.Hour).Unix()

	tests := []struct {
		name     string
		issued   int64
		renewed  int64
		expires  int64
		expected bool
	}{
		{name: "valid", issued: issuedAt, renewed: renewedAt, expires: expiresAt, expected: true},
		{name: "idle expired", issued: issuedAt, renewed: now.Add(-30 * 24 * time.Hour).Unix(), expires: expiresAt},
		{name: "absolute expired", issued: issuedAt, renewed: renewedAt, expires: now.Unix()},
		{name: "renewed before issued", issued: issuedAt, renewed: issuedAt - 1, expires: expiresAt},
		{name: "future issued", issued: now.Add(time.Second).Unix(), renewed: now.Add(time.Second).Unix(), expires: expiresAt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validSessionTimes(now, tt.issued, tt.renewed, tt.expires); got != tt.expected {
				t.Fatalf("validSessionTimes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRenewalMaxAge(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)

	if maxAge, renew := renewalMaxAge(now, now.Add(-23*time.Hour).Unix(), now.Add(90*24*time.Hour).Unix()); renew || maxAge != SessionIdleMaxAge {
		t.Fatalf("session renewed before threshold: maxAge=%d renew=%v", maxAge, renew)
	}

	maxAge, renew := renewalMaxAge(now, now.Add(-24*time.Hour).Unix(), now.Add(90*24*time.Hour).Unix())
	if !renew || maxAge != SessionIdleMaxAge {
		t.Fatalf("session not renewed at threshold: maxAge=%d renew=%v", maxAge, renew)
	}

	remaining := 12 * time.Hour
	maxAge, renew = renewalMaxAge(now, now.Add(-24*time.Hour).Unix(), now.Add(remaining).Unix())
	if !renew || maxAge != int(remaining/time.Second) {
		t.Fatalf("renewal exceeded absolute expiry: maxAge=%d renew=%v", maxAge, renew)
	}
}
