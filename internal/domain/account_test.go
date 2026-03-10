package domain

import (
	"testing"
)

func TestHydratePersist_RoundTrip(t *testing.T) {
	a := &Account{
		ID:       "test-1",
		Email:    "test@example.com",
		Proxy:    &ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 1080},
		Identity: map[string]string{"orgName": "Test Org", "orgUUID": "uuid-123"},
	}

	// Persist → serialise transient fields
	a.PersistRuntime()
	if a.ProxyJSON == "" {
		t.Fatal("ProxyJSON should be set after PersistRuntime")
	}
	if a.IdentityJSON == "" {
		t.Fatal("IdentityJSON should be set after PersistRuntime")
	}

	// Clear transient fields
	a.Proxy = nil
	a.Identity = nil

	// Hydrate → restore transient fields
	a.HydrateRuntime()
	if a.Proxy == nil || a.Proxy.Host != "127.0.0.1" || a.Proxy.Port != 1080 {
		t.Fatal("Proxy should be restored after HydrateRuntime")
	}
	if a.Identity == nil || a.Identity["orgName"] != "Test Org" {
		t.Fatal("Identity should be restored after HydrateRuntime")
	}
}

func TestIdentityString(t *testing.T) {
	a := &Account{
		Identity: map[string]string{"account_uuid": "uuid-abc-123"},
	}
	got := a.IdentityString("account_uuid")
	if got != "uuid-abc-123" {
		t.Errorf("expected uuid-abc-123, got %s", got)
	}

	a2 := &Account{}
	got = a2.IdentityString("account_uuid")
	if got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}
