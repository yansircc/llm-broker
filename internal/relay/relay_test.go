package relay

import (
	"testing"
)

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
