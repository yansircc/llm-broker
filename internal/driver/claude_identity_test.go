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

// ---------------------------------------------------------------------------
// Prompt env masking tests (Phase 4)
// ---------------------------------------------------------------------------

func TestPromptEnvMasker_SystemString(t *testing.T) {
	m := newPromptEnvMasker("/Users/user")
	body := map[string]interface{}{
		"system": "# Environment\n - Platform: linux\n - Shell: bash\n - OS Version: Linux 6.5.0-44\n - Primary working directory: /home/alice/myproject\n - Git user: Alice Smith\n",
	}
	m.maskSystem(body)
	s := body["system"].(string)

	checks := []struct{ pattern, want string }{
		{"Platform: ", " - Platform: darwin"},
		{"Shell: ", " - Shell: /bin/zsh"},
		{"OS Version: ", " - OS Version: Darwin 25.4.0"},
		{"Primary working directory: ", " - Primary working directory: /Users/user/project"},
		{"Git user: ", " - Git user: User"},
	}
	for _, c := range checks {
		if !strings.Contains(s, c.want) {
			t.Errorf("system should contain %q, got:\n%s", c.want, s)
		}
	}
}

func TestPromptEnvMasker_SystemDoesNotRewriteUserContent(t *testing.T) {
	m := newPromptEnvMasker("/Users/user")
	// Compat user system prompt without " - " prefix must NOT be rewritten
	body := map[string]interface{}{
		"system": "You are an assistant.\nPlatform: describe the platform.\nShell: explain shell commands.\nOS Version: return the OS.\nGit user: show the user.",
	}
	m.maskSystem(body)
	s := body["system"].(string)

	if strings.Contains(s, "darwin") || strings.Contains(s, "/bin/zsh") || strings.Contains(s, "Darwin 25.4.0") {
		t.Errorf("user-authored system text should not be rewritten, got:\n%s", s)
	}
}

func TestPromptEnvMasker_SystemArray(t *testing.T) {
	m := newPromptEnvMasker("/Users/user")
	body := map[string]interface{}{
		"system": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": " - Platform: linux\n - Shell: bash",
			},
		},
	}
	m.maskSystem(body)
	blocks := body["system"].([]interface{})
	text := blocks[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(text, " - Platform: darwin") {
		t.Errorf("array block should be masked, got: %s", text)
	}
}

func TestPromptEnvMasker_HomePathInWorkDir(t *testing.T) {
	m := newPromptEnvMasker("/Users/canonical")
	// Home paths in " - working directory:" lines are rewritten
	body := map[string]interface{}{
		"system": " - Primary working directory: /home/alice/myproject\n",
	}
	m.maskSystem(body)
	s := body["system"].(string)
	if strings.Contains(s, "/home/alice/") {
		t.Errorf("home path in workdir line should be masked, got: %s", s)
	}
	if !strings.Contains(s, "/Users/canonical/") {
		t.Errorf("should use canonical home, got: %s", s)
	}
}

func TestPromptEnvMasker_BareHomePathNotRewritten(t *testing.T) {
	m := newPromptEnvMasker("/Users/canonical")
	// Bare home paths outside " - working directory:" lines are NOT rewritten
	// in system field (could be user content)
	body := map[string]interface{}{
		"system": "path is /Users/alice/code and /home/bob/work",
	}
	m.maskSystem(body)
	s := body["system"].(string)
	if strings.Contains(s, "/Users/canonical/") {
		t.Error("bare home paths in system should not be rewritten")
	}
}

func TestPromptEnvMasker_MessageReminders(t *testing.T) {
	m := newPromptEnvMasker("/Users/user")
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "Look at /Users/alice/file.go <system-reminder>Platform: linux\nShell: bash</system-reminder> rest",
			},
		},
	}
	m.maskMessageReminders(body)
	msgs := body["messages"].([]interface{})
	text := msgs[0].(map[string]interface{})["content"].(string)

	// User text outside <system-reminder> must be preserved
	if !strings.Contains(text, "/Users/alice/file.go") {
		t.Error("user text outside system-reminder should be preserved")
	}
	// Inside system-reminder should be masked
	if strings.Contains(text, "Platform: linux") {
		t.Error("env inside system-reminder should be masked")
	}
	if !strings.Contains(text, "Platform: darwin") {
		t.Errorf("should contain masked platform, got: %s", text)
	}
}

func TestPromptEnvMasker_Disabled(t *testing.T) {
	// When PromptEnvHome is empty, masker is nil → no masking
	d := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)
	if d.transformer.envMasker != nil {
		t.Error("masker should be nil when PromptEnvHome is empty")
	}
}

func TestPromptEnvMasker_Enabled(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{PromptEnvHome: "/Users/x"}, NoopStainlessStore{}, 4)
	if d.transformer.envMasker == nil {
		t.Error("masker should be non-nil when PromptEnvHome is set")
	}
}

func TestPromptEnvMasker_OverrideStainless(t *testing.T) {
	m := newPromptEnvMasker("/Users/user")
	h := make(http.Header)
	h.Set("x-stainless-os", "Linux")
	h.Set("x-stainless-arch", "x64")

	m.overrideStainless(h)

	if got := h.Get("x-stainless-os"); got != "Darwin" {
		t.Errorf("x-stainless-os = %q, want Darwin", got)
	}
	if got := h.Get("x-stainless-arch"); got != "arm64" {
		t.Errorf("x-stainless-arch = %q, want arm64", got)
	}
}
