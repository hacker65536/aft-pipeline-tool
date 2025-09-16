package models

import (
	"testing"
	"time"
)

func TestAccountsCache_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		cache    AccountsCache
		expected bool
	}{
		{
			name: "not expired",
			cache: AccountsCache{
				CachedAt: time.Now().Add(-30 * time.Minute),
				TTL:      3600, // 1 hour
			},
			expected: false,
		},
		{
			name: "expired",
			cache: AccountsCache{
				CachedAt: time.Now().Add(-2 * time.Hour),
				TTL:      3600, // 1 hour
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expired := time.Since(tt.cache.CachedAt).Seconds() > float64(tt.cache.TTL)
			if expired != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, expired)
			}
		})
	}
}
