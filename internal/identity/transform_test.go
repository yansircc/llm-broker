package identity

import (
	"testing"
)

func TestStripThinkingBlocks_RemovesThinking(t *testing.T) {
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{"type": "thinking", "thinking": "let me think..."},
					map[string]interface{}{"type": "text", "text": "Hello"},
				},
			},
		},
	}

	stripThinkingBlocks(body)

	msgs := body["messages"].([]interface{})
	content := msgs[0].(map[string]interface{})["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(content))
	}
	block := content[0].(map[string]interface{})
	if block["type"] != "text" || block["text"] != "Hello" {
		t.Fatalf("expected text block with 'Hello', got %v", block)
	}
}

func TestStripThinkingBlocks_RemovesRedactedThinking(t *testing.T) {
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{"type": "redacted_thinking", "data": "abc"},
					map[string]interface{}{"type": "text", "text": "World"},
				},
			},
		},
	}

	stripThinkingBlocks(body)

	msgs := body["messages"].([]interface{})
	content := msgs[0].(map[string]interface{})["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(content))
	}
	block := content[0].(map[string]interface{})
	if block["type"] != "text" || block["text"] != "World" {
		t.Fatalf("expected text block with 'World', got %v", block)
	}
}

func TestStripThinkingBlocks_PreservesUserMessages(t *testing.T) {
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "Hi"},
				},
			},
		},
	}

	stripThinkingBlocks(body)

	msgs := body["messages"].([]interface{})
	content := msgs[0].(map[string]interface{})["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(content))
	}
	block := content[0].(map[string]interface{})
	if block["type"] != "text" || block["text"] != "Hi" {
		t.Fatalf("user message should be untouched, got %v", block)
	}
}

func TestStripThinkingBlocks_AllThinkingBecomesEmptyText(t *testing.T) {
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{"type": "thinking", "thinking": "hmm"},
					map[string]interface{}{"type": "redacted_thinking", "data": "xyz"},
				},
			},
		},
	}

	stripThinkingBlocks(body)

	msgs := body["messages"].([]interface{})
	content := msgs[0].(map[string]interface{})["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("expected 1 placeholder block, got %d", len(content))
	}
	block := content[0].(map[string]interface{})
	if block["type"] != "text" || block["text"] != "" {
		t.Fatalf("expected empty text placeholder, got %v", block)
	}
}

func TestStripThinkingBlocks_NoMessages(t *testing.T) {
	body := map[string]interface{}{
		"model": "claude-sonnet-4-20250514",
	}

	// Should not panic
	stripThinkingBlocks(body)

	if _, ok := body["messages"]; ok {
		t.Fatal("should not have created messages field")
	}
}
