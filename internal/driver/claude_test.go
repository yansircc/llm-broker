package driver

import (
	"math"
	"testing"
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
