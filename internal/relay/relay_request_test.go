package relay

import (
	"testing"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

func TestRouteAffinityKeyIsStableNamespacedAndOpaque(t *testing.T) {
	affinity := driver.RouteAffinity{RawKey: "raw-session-secret", Kind: "session-id"}
	base := routeAffinityKey("user-a", domain.ProviderCodex, domain.SurfaceNative, affinity)
	if base == "" {
		t.Fatal("routeAffinityKey returned empty key")
	}
	if got := routeAffinityKey("user-a", domain.ProviderCodex, domain.SurfaceNative, affinity); got != base {
		t.Fatalf("same namespace produced %q, want stable %q", got, base)
	}

	tests := []struct {
		name     string
		userID   string
		provider domain.Provider
		surface  domain.Surface
		affinity driver.RouteAffinity
	}{
		{name: "user", userID: "user-b", provider: domain.ProviderCodex, surface: domain.SurfaceNative, affinity: affinity},
		{name: "provider", userID: "user-a", provider: domain.ProviderClaude, surface: domain.SurfaceNative, affinity: affinity},
		{name: "surface", userID: "user-a", provider: domain.ProviderCodex, surface: domain.SurfaceCompat, affinity: affinity},
		{name: "kind", userID: "user-a", provider: domain.ProviderCodex, surface: domain.SurfaceNative, affinity: driver.RouteAffinity{RawKey: affinity.RawKey, Kind: "conversation"}},
		{name: "raw key", userID: "user-a", provider: domain.ProviderCodex, surface: domain.SurfaceNative, affinity: driver.RouteAffinity{RawKey: "another-session", Kind: affinity.Kind}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := routeAffinityKey(tt.userID, tt.provider, tt.surface, tt.affinity); got == base {
				t.Fatalf("changed namespace produced unchanged key %q", got)
			}
		})
	}
	if got := routeAffinityKey("user-a", domain.ProviderCodex, domain.SurfaceNative, driver.RouteAffinity{}); got != "" {
		t.Fatalf("empty raw key produced %q, want empty", got)
	}
}
