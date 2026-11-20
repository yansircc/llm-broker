package driver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yansircc/llm-broker/internal/domain"
)

func TestRetiredModelsRejectBeforeSchedulingBuildingOrProbing(t *testing.T) {
	codex := NewCodexDriver(CodexConfig{})
	claude := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)
	tests := []struct {
		driver Driver
		model  string
		use    string
	}{
		{codex, "gpt-5.5", "gpt-6.1-sol"},
		{codex, "gpt-5.5-2026-04-23", "gpt-6.1-sol"},
		{codex, " GPT-5.4 ", "gpt-6-sol"},
		{codex, "gpt-5.4-2026-03-05", "gpt-6-sol"},
		{codex, "gpt-5.4-mini", "gpt-6-luna"},
		{codex, "gpt-5.4-mini-2026-03-17", "gpt-6-luna"},
		{codex, "gpt-5.3-codex", "gpt-6.1-sol"},
		{codex, "gpt-5.3-codex-spark", "gpt-6-luna"},
		{codex, "gpt-5.2", "gpt-6.1-sol"},
		{codex, "gpt-5.2-2025-12-11", "gpt-6.1-sol"},
		{claude, "claude-opus-4", "claude-opus-4-8"},
		{claude, "claude-opus-4-20250514", "claude-opus-4-8"},
		{claude, "claude-opus-4.0", "claude-opus-4-8"},
		{claude, "claude-opus-4-0", "claude-opus-4-8"},
		{claude, "claude-opus-4-1", "claude-opus-4-8"},
		{claude, "claude-opus-4-1-20250805", "claude-opus-4-8"},
		{claude, "claude-opus-4.1", "claude-opus-4-8"},
		{claude, "claude-sonnet-4", "claude-sonnet-5-5"},
		{claude, "claude-sonnet-4-20250514", "claude-sonnet-5-5"},
		{claude, "claude-sonnet-4.0", "claude-sonnet-5-5"},
		{claude, "claude-sonnet-4-0", "claude-sonnet-5-5"},
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("retired model probe reached upstream")
		return nil, nil
	})}
	acct := &domain.Account{Subject: "test-subject"}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			body := map[string]interface{}{"model": tt.model}
			raw, _ := json.Marshal(body)
			w := httptest.NewRecorder()
			if !tt.driver.InterceptRequest(w, body, tt.model) || w.Code != http.StatusBadRequest {
				t.Fatalf("pre-scheduling rejection: status=%d body=%s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.use) {
				t.Fatalf("missing replacement %s: %s", tt.use, w.Body.String())
			}
			_, err := tt.driver.BuildRequest(context.Background(), &RelayInput{RawBody: raw, Headers: make(http.Header)}, acct, "unused")
			assertRetiredModelError(t, err, tt.use)
			_, err = tt.driver.Probe(context.Background(), acct, "unused", client, tt.model)
			assertRetiredModelError(t, err, tt.use)
		})
	}
}

func assertRetiredModelError(t *testing.T, err error, replacement string) {
	t.Helper()
	var validation *RequestValidationError
	if !errors.As(err, &validation) || validation.StatusCode != http.StatusBadRequest || !strings.Contains(validation.Message, replacement) {
		t.Fatalf("error = %v, want 400 with replacement %s", err, replacement)
	}
}

func TestModelRetirementPreservesActiveSelectionsAndHistoricalCost(t *testing.T) {
	codex := NewCodexDriver(CodexConfig{})
	claude := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)
	for _, d := range []Driver{codex, claude} {
		for _, model := range d.Models() {
			w := httptest.NewRecorder()
			if d.InterceptRequest(w, map[string]interface{}{"model": model.ID}, model.ID) {
				t.Errorf("active model %s was intercepted: %s", model.ID, w.Body.String())
			}
		}
	}
	// Older does not mean retired; these retain their established selection semantics.
	for _, id := range []string{"claude-sonnet-4-5", "claude-opus-4-5", "claude-haiku-4-5"} {
		if _, err := normalizeClaudeModelID(id); err != nil {
			t.Errorf("still-supported %s: %v", id, err)
		}
	}
	if got := codex.probeModel(); got != "gpt-5.6-sol" {
		t.Fatalf("default probe changed to %s", got)
	}
	if got := codex.CalcCost("gpt-5.5", &Usage{InputTokens: 1000000, OutputTokens: 1000000}); got != 87.5 {
		t.Fatalf("historical GPT-5.5 cost = %v", got)
	}
	if got := claude.CalcCost("claude-sonnet-4", &Usage{InputTokens: 1000000, OutputTokens: 1000000}); got != 18 {
		t.Fatalf("historical Sonnet 4 cost = %v", got)
	}
	// Uncatalogued Codex slugs keep their existing passthrough behavior.
	raw := []byte(`{"model":"custom-model"}`)
	if _, err := codex.BuildRequest(context.Background(), &RelayInput{RawBody: raw}, &domain.Account{Subject: "subject"}, "unused"); err != nil {
		t.Fatalf("unrelated model passthrough changed: %v", err)
	}
}
