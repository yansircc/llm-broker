package compat

import (
	"encoding/json"
	"fmt"
	"time"
)

// ConvertResponse transforms an Anthropic Messages API response body
// into an OpenAI ChatCompletionResponse.
func ConvertResponse(anthropicBody []byte) ([]byte, *ChatUsage, error) {
	var msg struct {
		ID         string        `json:"id"`
		Model      string        `json:"model"`
		Content    []contentItem `json:"content"`
		StopReason string        `json:"stop_reason"`
		Usage      struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(anthropicBody, &msg); err != nil {
		return nil, nil, fmt.Errorf("parse anthropic response: %w", err)
	}

	// Build assistant message
	respMsg := &ChatRespMessage{Role: "assistant"}
	var toolCalls []ToolCall
	var textParts []string

	for _, item := range msg.Content {
		switch item.Type {
		case "text":
			textParts = append(textParts, item.Text)
		case "tool_use":
			args, _ := json.Marshal(item.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:   item.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      item.Name,
					Arguments: string(args),
				},
			})
		}
	}

	if len(textParts) > 0 {
		joined := textParts[0]
		for _, t := range textParts[1:] {
			joined += t
		}
		respMsg.Content = &joined
	}
	if len(toolCalls) > 0 {
		respMsg.ToolCalls = toolCalls
	}

	finishReason := mapStopReason(msg.StopReason)
	usage := &ChatUsage{
		PromptTokens:     msg.Usage.InputTokens,
		CompletionTokens: msg.Usage.OutputTokens,
		TotalTokens:      msg.Usage.InputTokens + msg.Usage.OutputTokens,
	}

	resp := ChatCompletionResponse{
		ID:      "chatcmpl-" + msg.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   msg.Model,
		Choices: []ChatChoice{{
			Index:        0,
			Message:      respMsg,
			FinishReason: &finishReason,
		}},
		Usage: usage,
	}

	data, err := json.Marshal(resp)
	return data, usage, err
}

// contentItem represents a block in Anthropic's content array.
type contentItem struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

// mapStopReason converts Anthropic stop_reason to OpenAI finish_reason.
func mapStopReason(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}
