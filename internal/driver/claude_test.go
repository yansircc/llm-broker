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
	d := NewClaudeDriver(ClaudeConfig{}, nil)

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
	}, nil)

	before := time.Now()
	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"This organization has been disabled."}}`)
	effect := d.Interpret(http.StatusBadRequest, make(http.Header), body, "claude-sonnet-4-6", json.RawMessage(`{}`))

	if effect.Kind != EffectBlock {
		t.Fatalf("Kind = %v, want block", effect.Kind)
	}
	if effect.Scope != EffectScopeBucket {
		t.Fatalf("Scope = %v, want bucket", effect.Scope)
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
