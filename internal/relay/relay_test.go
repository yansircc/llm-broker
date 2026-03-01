package relay

import (
	"testing"
)

func TestShouldRetry(t *testing.T) {
	retryable := []int{529, 429, 401, 403}
	for _, code := range retryable {
		if !shouldRetry(code) {
			t.Errorf("shouldRetry(%d) = false, want true", code)
		}
	}

	nonRetryable := []int{200, 500, 502, 503, 400, 404}
	for _, code := range nonRetryable {
		if shouldRetry(code) {
			t.Errorf("shouldRetry(%d) = true, want false", code)
		}
	}
}

func TestIsOldSession(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
		want bool
	}{
		{
			name: "multiple_messages",
			body: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "hi"},
					map[string]interface{}{"role": "assistant", "content": "hello"},
				},
				"tools": []interface{}{map[string]interface{}{"name": "bash"}},
			},
			want: true,
		},
		{
			name: "multi_text_blocks",
			body: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role": "user",
						"content": []interface{}{
							map[string]interface{}{"type": "text", "text": "one"},
							map[string]interface{}{"type": "text", "text": "two"},
						},
					},
				},
				"tools": []interface{}{map[string]interface{}{"name": "bash"}},
			},
			want: true,
		},
		{
			name: "no_tools",
			body: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "hi"},
				},
			},
			want: true,
		},
		{
			name: "new_session",
			body: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "hi"},
				},
				"tools": []interface{}{map[string]interface{}{"name": "bash"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOldSession(tt.body)
			if got != tt.want {
				t.Errorf("isOldSession() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalcCost(t *testing.T) {
	tests := []struct {
		name                                     string
		model                                    string
		input, output, cacheRead, cacheCreate     int
		want                                     float64
	}{
		{"opus", "claude-opus-4-20250514", 1000, 500, 100, 50, (1000*15 + 500*75 + 100*1.5 + 50*18.75) / 1e6},
		{"haiku", "claude-haiku-4-5-20251001", 1000, 500, 100, 50, (1000*0.8 + 500*4 + 100*0.08 + 50*1.0) / 1e6},
		{"sonnet", "claude-sonnet-4-20250514", 1000, 500, 100, 50, (1000*3 + 500*15 + 100*0.3 + 50*3.75) / 1e6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcCost(tt.model, tt.input, tt.output, tt.cacheRead, tt.cacheCreate)
			if got != tt.want {
				t.Errorf("calcCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalcCodexCost(t *testing.T) {
	tests := []struct {
		name                          string
		model                         string
		input, output, cacheRead      int
		want                          float64
	}{
		{"o3", "o3", 1000, 500, 100, (1000*2 + 500*8 + 100*0.5) / 1e6},
		{"o4-mini", "o4-mini", 1000, 500, 100, (1000*1.1 + 500*4.4 + 100*0.275) / 1e6},
		{"codex-mini", "codex-mini-latest", 1000, 500, 100, (1000*1.5 + 500*6 + 100*0.375) / 1e6},
		{"4.1-nano", "gpt-4.1-nano", 1000, 500, 100, (1000*0.1 + 500*0.4 + 100*0.025) / 1e6},
		{"4.1-mini", "gpt-4.1-mini", 1000, 500, 100, (1000*0.4 + 500*1.6 + 100*0.1) / 1e6},
		{"4.1", "gpt-4.1", 1000, 500, 100, (1000*2 + 500*8 + 100*0.5) / 1e6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcCodexCost(tt.model, tt.input, tt.output, tt.cacheRead)
			if got != tt.want {
				t.Errorf("calcCodexCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOpusModel(t *testing.T) {
	if !isOpusModel("claude-opus-4-20250514") {
		t.Error("opus model should return true")
	}
	if !isOpusModel("claude-Opus-4") {
		t.Error("Opus (uppercase) should return true")
	}
	if isOpusModel("claude-sonnet-4-20250514") {
		t.Error("sonnet model should return false")
	}
	if isOpusModel("haiku") {
		t.Error("haiku model should return false")
	}
}
