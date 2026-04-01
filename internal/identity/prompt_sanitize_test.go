package identity

import (
	"strings"
	"testing"
)

func TestSanitizePromptEnv_EnvBlock(t *testing.T) {
	profile := CanonicalProfile{
		Platform:   "linux",
		Shell:      "bash",
		OSVersion:  "Linux 6.5.0-44-generic",
		WorkingDir: "/home/user/project",
		HomePrefix: "/home/user/",
	}

	input := `You are Claude Code.
<env>
 - Platform: darwin
 - Shell: zsh
 - OS Version: Darwin 24.4.0
 - Working directory: /Users/jack/my-project
 - Primary working directory: /Users/jack/my-project
</env>`

	got := SanitizePromptEnv(input, profile)

	if !strings.Contains(got, "Platform: linux") {
		t.Errorf("expected canonical platform in env block: %s", got)
	}
	if !strings.Contains(got, "Shell: bash") {
		t.Errorf("expected canonical shell in env block: %s", got)
	}
	if !strings.Contains(got, "OS Version: Linux 6.5.0-44-generic") {
		t.Errorf("expected canonical OS version in env block: %s", got)
	}
	if !strings.Contains(got, "Working directory: /home/user/project") {
		t.Errorf("expected canonical working dir in env block: %s", got)
	}
	if strings.Contains(got, "/Users/jack/") {
		t.Errorf("home path should be replaced in env block: %s", got)
	}
	// Prefix outside env block should be untouched
	if !strings.Contains(got, "You are Claude Code.") {
		t.Errorf("text outside env block should be preserved: %s", got)
	}
}

func TestSanitizePromptEnv_NoEnvBlock(t *testing.T) {
	profile := CanonicalProfile{
		Platform:   "linux",
		Shell:      "bash",
		OSVersion:  "Linux 6.5.0",
		WorkingDir: "/home/user/project",
		HomePrefix: "/home/user/",
	}

	// User content with paths and Platform: should NOT be sanitized
	input := "Why does /Users/alice/project/main.go fail? Platform: darwin is required."
	got := SanitizePromptEnv(input, profile)
	if got != input {
		t.Errorf("text without <env> block should be unchanged, got %q", got)
	}
}

func TestSanitizePromptEnv_MixedContent(t *testing.T) {
	profile := CanonicalProfile{
		Platform:   "linux",
		Shell:      "bash",
		OSVersion:  "Linux 6.5.0",
		WorkingDir: "/home/user/project",
		HomePrefix: "/home/user/",
	}

	// Env block + user content with paths
	input := `<env>
Platform: darwin
Working directory: /Users/jack/code
</env>
The file at /Users/alice/docs/readme.md has an error.`

	got := SanitizePromptEnv(input, profile)

	// Env block should be sanitized
	if strings.Contains(got, "Platform: darwin") {
		t.Errorf("env block platform should be replaced: %s", got)
	}
	// User content outside env block should be preserved
	if !strings.Contains(got, "/Users/alice/docs/readme.md") {
		t.Errorf("path outside env block should be preserved: %s", got)
	}
}

func TestDeriveCanonicalProfile_Deterministic(t *testing.T) {
	p1 := DeriveCanonicalProfile("acct-1", "seed")
	p2 := DeriveCanonicalProfile("acct-1", "seed")
	if p1 != p2 {
		t.Error("same inputs should produce same profile")
	}
}

func TestDeriveCanonicalProfile_ValidFields(t *testing.T) {
	p := DeriveCanonicalProfile("test-account", "test-seed")
	if p.Platform == "" || p.Shell == "" || p.OSVersion == "" || p.WorkingDir == "" || p.HomePrefix == "" {
		t.Errorf("profile has empty fields: %+v", p)
	}
	if p.StainlessOS == "" || p.StainlessArch == "" {
		t.Errorf("stainless fields empty: %+v", p)
	}
}
