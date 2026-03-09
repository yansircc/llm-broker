package compat

import (
	"encoding/json"
	"fmt"
	"strings"
)

const defaultMaxTokens = 4096

// ConvertRequest transforms an OpenAI ChatCompletionRequest into an Anthropic
// Messages API body (map[string]any) suitable for the relay pipeline.
func ConvertRequest(req *ChatCompletionRequest) (map[string]any, error) {
	body := map[string]any{
		"model": req.Model,
	}

	// Extract system messages and convert the rest
	var systemParts []string
	var anthropicMsgs []map[string]any

	for _, msg := range req.Messages {
		switch msg.Role {
		case "system", "developer":
			text, err := extractTextContent(msg.Content)
			if err != nil {
				return nil, fmt.Errorf("system message content: %w", err)
			}
			if text != "" {
				systemParts = append(systemParts, text)
			}

		case "user":
			m, err := convertUserMessage(msg)
			if err != nil {
				return nil, fmt.Errorf("user message: %w", err)
			}
			anthropicMsgs = append(anthropicMsgs, m)

		case "assistant":
			m, err := convertAssistantMessage(msg)
			if err != nil {
				return nil, fmt.Errorf("assistant message: %w", err)
			}
			anthropicMsgs = append(anthropicMsgs, m)

		case "tool":
			m, err := convertToolResultMessage(msg)
			if err != nil {
				return nil, fmt.Errorf("tool message: %w", err)
			}
			anthropicMsgs = append(anthropicMsgs, m)
		}
	}

	// Hint JSON mode via system prompt
	if req.ResponseFormat != nil && req.ResponseFormat.Type == "json_object" {
		systemParts = append(systemParts, "Respond with valid JSON only. Do not include any text outside the JSON object.")
	}

	if len(systemParts) > 0 {
		body["system"] = strings.Join(systemParts, "\n\n")
	}

	// Merge consecutive same-role messages (required by Anthropic)
	anthropicMsgs = mergeConsecutiveMessages(anthropicMsgs)
	body["messages"] = anthropicMsgs

	// max_tokens (required for Anthropic)
	switch {
	case req.MaxCompletionTokens != nil:
		body["max_tokens"] = *req.MaxCompletionTokens
	case req.MaxTokens != nil:
		body["max_tokens"] = *req.MaxTokens
	default:
		body["max_tokens"] = defaultMaxTokens
	}

	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if req.Stream {
		body["stream"] = true
	}

	// Stop sequences
	if req.Stop != nil {
		seqs, err := normalizeStop(req.Stop)
		if err != nil {
			return nil, fmt.Errorf("stop: %w", err)
		}
		if len(seqs) > 0 {
			body["stop_sequences"] = seqs
		}
	}

	// Tool choice — resolve first so "none" can suppress tools
	var toolChoiceNone bool
	if req.ToolChoice != nil {
		tc, none, err := convertToolChoice(req.ToolChoice)
		if err != nil {
			return nil, fmt.Errorf("tool_choice: %w", err)
		}
		toolChoiceNone = none
		if tc != nil {
			body["tool_choice"] = tc
		}
	}

	// Tools — skip entirely when tool_choice is "none"
	if len(req.Tools) > 0 && !toolChoiceNone {
		tools, err := convertTools(req.Tools)
		if err != nil {
			return nil, fmt.Errorf("tools: %w", err)
		}
		body["tools"] = tools
	}

	// User → metadata.user_id
	if req.User != "" {
		body["metadata"] = map[string]any{
			"user_id": req.User,
		}
	}

	return body, nil
}

// ---------------------------------------------------------------------------
// Message conversion
// ---------------------------------------------------------------------------

// extractTextContent returns the text from a content field that may be
// a JSON string, an array of content parts, or null.
func extractTextContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	// Try as plain string first
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}

	// Try as array of content parts
	var parts []ContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("cannot parse content: %s", string(raw))
	}
	var texts []string
	for _, p := range parts {
		if p.Type == "text" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
}

func convertUserMessage(msg ChatMessage) (map[string]any, error) {
	content, err := convertContent(msg.Content)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"role":    "user",
		"content": content,
	}, nil
}

func convertAssistantMessage(msg ChatMessage) (map[string]any, error) {
	var blocks []any

	// Text content
	text, err := extractTextContent(msg.Content)
	if err != nil {
		return nil, err
	}
	if text != "" {
		blocks = append(blocks, map[string]any{
			"type": "text",
			"text": text,
		})
	}

	// Tool calls → tool_use blocks
	for _, tc := range msg.ToolCalls {
		var input any
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				// If arguments is not valid JSON, wrap as string
				input = map[string]any{"raw": tc.Function.Arguments}
			}
		} else {
			input = map[string]any{}
		}
		blocks = append(blocks, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Function.Name,
			"input": input,
		})
	}

	m := map[string]any{"role": "assistant"}
	if len(blocks) > 0 {
		m["content"] = blocks
	} else {
		// Empty assistant message — Anthropic needs at least something
		m["content"] = text
	}
	return m, nil
}

func convertToolResultMessage(msg ChatMessage) (map[string]any, error) {
	text, err := extractTextContent(msg.Content)
	if err != nil {
		return nil, err
	}

	block := map[string]any{
		"type":        "tool_result",
		"tool_use_id": msg.ToolCallID,
	}
	if text != "" {
		block["content"] = text
	}

	return map[string]any{
		"role":    "user",
		"content": []any{block},
	}, nil
}

// convertContent handles the polymorphic content field: string | []ContentPart.
// Returns either a string or []map for Anthropic.
func convertContent(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	// Try plain string
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}

	// Try array of content parts
	var parts []ContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("cannot parse content: %s", string(raw))
	}

	var blocks []any
	for _, p := range parts {
		switch p.Type {
		case "text":
			blocks = append(blocks, map[string]any{
				"type": "text",
				"text": p.Text,
			})
		case "image_url":
			if p.ImageURL != nil {
				blocks = append(blocks, map[string]any{
					"type": "image",
					"source": map[string]any{
						"type": "url",
						"url":  p.ImageURL.URL,
					},
				})
			}
		}
	}
	if len(blocks) == 0 {
		return "", nil
	}
	return blocks, nil
}

// ---------------------------------------------------------------------------
// Tools conversion
// ---------------------------------------------------------------------------

func convertTools(tools []ChatTool) ([]any, error) {
	var out []any
	for _, t := range tools {
		if t.Type != "function" {
			continue
		}
		tool := map[string]any{
			"name": t.Function.Name,
		}
		if t.Function.Description != "" {
			tool["description"] = t.Function.Description
		}
		if len(t.Function.Parameters) > 0 {
			var schema any
			if err := json.Unmarshal(t.Function.Parameters, &schema); err != nil {
				return nil, fmt.Errorf("tool %s parameters: %w", t.Function.Name, err)
			}
			tool["input_schema"] = schema
		}
		out = append(out, tool)
	}
	return out, nil
}

func convertToolChoice(raw json.RawMessage) (any, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false, nil
	}

	// Try as string first: "auto", "none", "required"
	var s string
	if json.Unmarshal(raw, &s) == nil {
		switch s {
		case "auto":
			return map[string]any{"type": "auto"}, false, nil
		case "none":
			return nil, true, nil // strip tools entirely
		case "required":
			return map[string]any{"type": "any"}, false, nil
		default:
			return map[string]any{"type": "auto"}, false, nil
		}
	}

	// Try as object: {"type":"function","function":{"name":"X"}}
	var obj struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return map[string]any{"type": "auto"}, false, nil
	}
	if obj.Type == "function" && obj.Function.Name != "" {
		return map[string]any{
			"type": "tool",
			"name": obj.Function.Name,
		}, false, nil
	}
	return map[string]any{"type": "auto"}, false, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// normalizeStop converts stop (string | []string) to []string.
func normalizeStop(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var s string
	if json.Unmarshal(raw, &s) == nil {
		return []string{s}, nil
	}

	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("stop must be string or string array")
	}
	return arr, nil
}

// mergeConsecutiveMessages merges adjacent messages with the same role,
// which Anthropic requires (strict user/assistant alternation).
func mergeConsecutiveMessages(msgs []map[string]any) []map[string]any {
	if len(msgs) <= 1 {
		return msgs
	}

	var merged []map[string]any
	for _, msg := range msgs {
		role, _ := msg["role"].(string)
		if len(merged) == 0 {
			merged = append(merged, msg)
			continue
		}

		last := merged[len(merged)-1]
		lastRole, _ := last["role"].(string)
		if lastRole != role {
			merged = append(merged, msg)
			continue
		}

		// Same role — merge content
		last["content"] = mergeContent(last["content"], msg["content"])
	}
	return merged
}

// mergeContent combines two Anthropic content values.
// Each may be a string or []any of content blocks.
func mergeContent(a, b any) any {
	blocksA := toContentBlocks(a)
	blocksB := toContentBlocks(b)
	return append(blocksA, blocksB...)
}

func toContentBlocks(v any) []any {
	switch c := v.(type) {
	case []any:
		return c
	case string:
		if c == "" {
			return nil
		}
		return []any{map[string]any{"type": "text", "text": c}}
	default:
		return nil
	}
}
