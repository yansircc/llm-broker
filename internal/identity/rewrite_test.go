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

func TestExtractSessionUUID_JSONFormat(t *testing.T) {
	jsonUserID := `{"device_id":"abcdef0123456789","account_uuid":"org-123","session_id":"aabbccdd-1122-3344-5566-778899aabbcc"}`
	got := ExtractSessionUUID(jsonUserID)
	if got != "aabbccdd-1122-3344-5566-778899aabbcc" {
		t.Errorf("expected session_id from JSON, got %q", got)
	}

	// JSON without device_id should NOT match
	partialJSON := `{"session_id":"aabbccdd-1122-3344-5566-778899aabbcc"}`
	got = ExtractSessionUUID(partialJSON)
	if got != "" {
		t.Errorf("JSON without device_id should not match, got %q", got)
	}
}

func TestRewriteUserID_JSONFormat(t *testing.T) {
	jsonUserID := `{"device_id":"abcdef0123456789","account_uuid":"org-123","session_id":"aabbccdd-1122-3344-5566-778899aabbcc"}`
	result := RewriteUserID(jsonUserID, "acct-1", "org-uuid-1")
	if result == "" {
		t.Error("should return non-empty user_id")
	}
	if result == jsonUserID {
		t.Error("should rewrite JSON user_id, not pass through")
	}
	if !sessionUUIDPattern.MatchString(result) {
		t.Error("result should contain a derived session UUID")
	}
	// Should use session_id from JSON as tail, not "default"
	defaultResult := RewriteUserID("totally-invalid", "acct-1", "org-uuid-1")
	if result == defaultResult {
		t.Error("JSON session_id should produce different result than default fallback")
	}

	// JSON without device_id should fall through to default
	partialJSON := `{"session_id":"aabbccdd-1122-3344-5566-778899aabbcc"}`
	partialResult := RewriteUserID(partialJSON, "acct-1", "org-uuid-1")
	if partialResult != defaultResult {
		t.Error("JSON without device_id should use default fallback")
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
