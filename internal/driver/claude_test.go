package driver

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClaudeCalcCost(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)

	tests := []struct {
		name  string
		model string
		usage *Usage
		want  float64
	}{
		{
			name:  "haiku",
			model: "claude-haiku-4-5-20251001",
			usage: &Usage{InputTokens: 1923, OutputTokens: 214},
			want:  0.002993,
		},
		{
			name:  "sonnet",
			model: "claude-sonnet-4-6",
			usage: &Usage{InputTokens: 1488, OutputTokens: 101},
			want:  0.005979,
		},
		{
			name:  "opus",
			model: "claude-opus-4-6",
			usage: &Usage{InputTokens: 1460, OutputTokens: 59},
			want:  0.008775,
		},
		{
			name:  "haiku cache",
			model: "claude-haiku-4-5-20251001",
			usage: &Usage{InputTokens: 3, OutputTokens: 13, CacheCreateTokens: 75436},
			want:  0.094363,
		},
		{
			name:  "sonnet cache",
			model: "claude-sonnet-4-6",
			usage: &Usage{InputTokens: 3, OutputTokens: 760, CacheReadTokens: 147856, CacheCreateTokens: 299},
			want:  0.05688705,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := d.CalcCost(tc.model, tc.usage); math.Abs(got-tc.want) > 1e-12 {
				t.Fatalf("CalcCost(%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}

func TestClaudeInterpret_400DisabledOrganizationBlocks(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{
		Pauses: ErrorPauses{Pause401: time.Minute},
	}, NoopStainlessStore{}, 4)

	before := time.Now()
	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"This organization has been disabled."}}`)
	effect := d.Interpret(http.StatusBadRequest, make(http.Header), body, "claude-sonnet-4-6", json.RawMessage(`{}`))

	if effect.Kind != EffectBlock {
		t.Fatalf("Kind = %v, want block", effect.Kind)
	}
	if effect.Scope != EffectScopeBucket {
		t.Fatalf("Scope = %v, want bucket", effect.Scope)
	}
	if effect.UpstreamStatus != http.StatusBadRequest {
		t.Fatalf("UpstreamStatus = %d, want %d", effect.UpstreamStatus, http.StatusBadRequest)
	}
	if effect.CooldownUntil.Before(before.Add(50 * time.Second)) {
		t.Fatalf("CooldownUntil = %s, want around now+1m", effect.CooldownUntil.Format(time.RFC3339))
	}
	if !strings.Contains(effect.ErrorMessage, "organization has been disabled") {
		t.Fatalf("ErrorMessage = %q, want disabled signal", effect.ErrorMessage)
	}
	if !json.Valid(effect.UpdatedState) {
		t.Fatalf("UpdatedState = %q, want valid JSON", string(effect.UpdatedState))
	}
}

func TestClaudeInterpret_403NonBanReturnsReject(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{
		Pauses: ErrorPauses{Pause403: 10 * time.Minute},
	}, nil)

	effect := d.Interpret(http.StatusForbidden, make(http.Header), []byte(`{"error":{"message":"request rejected"}}`), "claude-sonnet-4-6", json.RawMessage(`{}`))

	if effect.Kind != EffectReject {
		t.Fatalf("Kind = %v, want reject", effect.Kind)
	}
	if effect.UpstreamStatus != http.StatusForbidden {
		t.Fatalf("UpstreamStatus = %d, want %d", effect.UpstreamStatus, http.StatusForbidden)
	}
	if !effect.CooldownUntil.IsZero() {
		t.Fatalf("CooldownUntil = %v, want zero", effect.CooldownUntil)
	}
}

func TestClaudeInterpret_400NonBanReturnsReject(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{
		Pauses: ErrorPauses{Pause403: 10 * time.Minute},
	}, nil)

	effect := d.Interpret(http.StatusBadRequest, make(http.Header), []byte(`{"error":{"message":"request rejected"}}`), "claude-sonnet-4-6", json.RawMessage(`{}`))

	if effect.Kind != EffectReject {
		t.Fatalf("Kind = %v, want reject", effect.Kind)
	}
	if effect.UpstreamStatus != http.StatusBadRequest {
		t.Fatalf("UpstreamStatus = %d, want %d", effect.UpstreamStatus, http.StatusBadRequest)
	}
	if !effect.CooldownUntil.IsZero() {
		t.Fatalf("CooldownUntil = %v, want zero", effect.CooldownUntil)
	}
}

func TestClaudeInterpret_404ReturnsReject(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{}, nil)

	effect := d.Interpret(http.StatusNotFound, make(http.Header), []byte(`{"error":{"type":"not_found_error","message":"model: claude-haiku-4-6"}}`), "claude-haiku-4-6", json.RawMessage(`{}`))

	if effect.Kind != EffectReject {
		t.Fatalf("Kind = %v, want reject", effect.Kind)
	}
	if effect.UpstreamStatus != http.StatusNotFound {
		t.Fatalf("UpstreamStatus = %d, want %d", effect.UpstreamStatus, http.StatusNotFound)
	}
	if effect.UpstreamErrorType != "not_found_error" {
		t.Fatalf("UpstreamErrorType = %q, want not_found_error", effect.UpstreamErrorType)
	}
}

func TestClaudeInterpret_502ReturnsServerError(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{}, nil)

	effect := d.Interpret(http.StatusBadGateway, make(http.Header), []byte(`{"error":{"type":"api_error","message":"upstream bad gateway"}}`), "claude-sonnet-4-6", json.RawMessage(`{}`))

	if effect.Kind != EffectServerError {
		t.Fatalf("Kind = %v, want server_error", effect.Kind)
	}
	if effect.UpstreamStatus != http.StatusBadGateway {
		t.Fatalf("UpstreamStatus = %d, want %d", effect.UpstreamStatus, http.StatusBadGateway)
	}
}

func TestClaudeRequiresFreshSession(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
		want bool
	}{
		{
			name: "one-shot user text stays portable",
			body: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role": "user",
						"content": []interface{}{
							map[string]interface{}{"type": "text", "text": "hello"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "tools require same session",
			body: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "hello"},
				},
				"tools": []interface{}{
					map[string]interface{}{"name": "run"},
				},
			},
			want: true,
		},
		{
			name: "assistant turn requires same session",
			body: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{"role": "assistant", "content": "hello"},
				},
			},
			want: true,
		},
		{
			name: "tool result requires same session",
			body: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role": "user",
						"content": []interface{}{
							map[string]interface{}{"type": "tool_result", "tool_use_id": "tool-1", "content": "done"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "multi-turn requires same session",
			body: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "hello"},
					map[string]interface{}{"role": "user", "content": "follow up"},
				},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := claudeRequiresFreshSession(tc.body); got != tc.want {
				t.Fatalf("claudeRequiresFreshSession() = %v, want %v", got, tc.want)
			}
		})
	}
}
