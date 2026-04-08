package driver

import (
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// User ID rewrite tests (migrated from identity/rewrite_test.go)
// ---------------------------------------------------------------------------

func TestRewriteUserID_Valid(t *testing.T) {
	hash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	uuid := "12345678-1234-1234-1234-123456789abc"
	original := "user_" + hash + "_account__session_" + uuid
	result := rewriteUserID(original, "acct-1", "org-uuid-1")
	if result == original {
		t.Error("rewritten user_id should differ from original")
	}
	if result == "" {
		t.Error("should return a non-empty user_id")
	}
	if !sessionUUIDPattern.MatchString(result) {
		t.Error("result should contain a session UUID")
	}
}

func TestRewriteUserID_Invalid(t *testing.T) {
	result := rewriteUserID("invalid-format", "acct-1", "org-uuid-1")
	if result == "" {
		t.Error("should return fallback user_id")
	}
	if !sessionUUIDPattern.MatchString(result) {
		t.Error("fallback result should still contain session UUID")
	}
}

func TestExtractSessionUUID(t *testing.T) {
	uuid := "12345678-1234-1234-1234-123456789abc"
	got := extractSessionUUID("user_xxx_account__session_" + uuid)
	if got != uuid {
		t.Errorf("expected %s, got %s", uuid, got)
	}

	got = extractSessionUUID("invalid-no-session")
	if got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

func TestDeterministicHash(t *testing.T) {
	r1 := rewriteUserID("invalid", "acct-1", "org-1")
	r2 := rewriteUserID("invalid", "acct-1", "org-1")
	if r1 != r2 {
		t.Error("same inputs should produce same output")
	}

	r3 := rewriteUserID("invalid", "acct-2", "org-1")
	if r1 == r3 {
		t.Error("different accountID should produce different output")
	}
}

// ---------------------------------------------------------------------------
// Warmup tests (migrated from identity/warmup_test.go)
// ---------------------------------------------------------------------------

func TestIsWarmup_WarmupString(t *testing.T) {
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Warmup"},
		},
	}
	if !isWarmupRequest(body) {
		t.Error("should detect 'Warmup' content")
	}
}

func TestIsWarmup_TitlePrompt(t *testing.T) {
	body := map[string]interface{}{
		"system": "Please write a 5-10 word title for this conversation.",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "test"},
		},
	}
	if !isWarmupRequest(body) {
		t.Error("should detect title prompt in system")
	}
}

func TestIsWarmup_NormalRequest(t *testing.T) {
	body := map[string]interface{}{
		"system": "You are a helpful assistant.",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello, how are you?"},
		},
	}
	if isWarmupRequest(body) {
		t.Error("normal request should not be warmup")
	}
}

func TestWarmupEvents(t *testing.T) {
	events := warmupEvents("claude-sonnet-4-20250514")
	if len(events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(events))
	}

	for i, ev := range events {
		if !strings.HasPrefix(ev, "event: ") {
			t.Errorf("event %d should start with 'event: '", i)
		}
		if !strings.HasSuffix(ev, "\n\n") {
			t.Errorf("event %d should end with double newline", i)
		}
		if !strings.Contains(ev, "data: ") {
			t.Errorf("event %d should contain 'data: '", i)
		}
	}

	if !strings.Contains(events[0], "claude-sonnet-4-20250514") {
		t.Error("first event should contain model name")
	}
}

// ---------------------------------------------------------------------------
// Fingerprint consistency tests (Phase 3)
// ---------------------------------------------------------------------------

func TestSetClaudeRequiredHeaders_UAFromStainless(t *testing.T) {
	h := make(http.Header)
	h.Set("x-stainless-package-version", "2.3.1")
	setClaudeRequiredHeaders(h, "tok", "2023-06-01", "")

	ua := h.Get("User-Agent")
	if ua != "claude-cli/2.3.1 (external, cli)" {
		t.Fatalf("User-Agent = %q, want derived from stainless version", ua)
	}
}

func TestSetClaudeRequiredHeaders_UAFallback(t *testing.T) {
	h := make(http.Header)
	setClaudeRequiredHeaders(h, "tok", "2023-06-01", "")

	ua := h.Get("User-Agent")
	if ua != "claude-cli/"+defaultClaudeVersion+" (external, cli)" {
		t.Fatalf("User-Agent = %q, want fallback version", ua)
	}
}
