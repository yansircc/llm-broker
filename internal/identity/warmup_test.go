package identity

import (
	"strings"
	"testing"
)

func TestIsWarmup_WarmupString(t *testing.T) {
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Warmup"},
		},
	}
	if !IsWarmupRequest(body) {
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
	if !IsWarmupRequest(body) {
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
	if IsWarmupRequest(body) {
		t.Error("normal request should not be warmup")
	}
}

func TestWarmupEvents(t *testing.T) {
	events := WarmupEvents("claude-sonnet-4-20250514")
	if len(events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(events))
	}

	// Each event should be a valid SSE event
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

	// First event should contain the model
	if !strings.Contains(events[0], "claude-sonnet-4-20250514") {
		t.Error("first event should contain model name")
	}
}
