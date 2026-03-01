package identity

import (
	"testing"
)

func TestRewriteUserID_Valid(t *testing.T) {
	// A valid user_id: user_{64hex}_account__session_{uuid}
	hash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	uuid := "12345678-1234-1234-1234-123456789abc"
	original := "user_" + hash + "_account__session_" + uuid
	result := RewriteUserID(original, "acct-1", "org-uuid-1")
	if result == original {
		t.Error("rewritten user_id should differ from original")
	}
	if result == "" {
		t.Error("should return a non-empty user_id")
	}
	// Should contain session_ prefix
	if !sessionUUIDPattern.MatchString(result) {
		t.Error("result should contain a session UUID")
	}
}

func TestRewriteUserID_Invalid(t *testing.T) {
	result := RewriteUserID("invalid-format", "acct-1", "org-uuid-1")
	if result == "" {
		t.Error("should return fallback user_id")
	}
	// Should use "default" as session tail
	if !sessionUUIDPattern.MatchString(result) {
		t.Error("fallback result should still contain session UUID")
	}
}

func TestExtractSessionUUID(t *testing.T) {
	uuid := "12345678-1234-1234-1234-123456789abc"
	got := ExtractSessionUUID("user_xxx_account__session_" + uuid)
	if got != uuid {
		t.Errorf("expected %s, got %s", uuid, got)
	}

	got = ExtractSessionUUID("invalid-no-session")
	if got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

func TestDeterministicHash(t *testing.T) {
	r1 := RewriteUserID("invalid", "acct-1", "org-1")
	r2 := RewriteUserID("invalid", "acct-1", "org-1")
	if r1 != r2 {
		t.Error("same inputs should produce same output")
	}

	r3 := RewriteUserID("invalid", "acct-2", "org-1")
	if r1 == r3 {
		t.Error("different accountID should produce different output")
	}
}
