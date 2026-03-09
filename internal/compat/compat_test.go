package compat

import (
	"encoding/json"
	"testing"
)

func TestConvertRequest_BasicTextMessage(t *testing.T) {
	req := &ChatCompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []ChatMessage{
			{Role: "system", Content: json.RawMessage(`"You are helpful."`)},
			{Role: "user", Content: json.RawMessage(`"Hello!"`)},
		},
		MaxTokens: intPtr(1024),
	}

	body, err := ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	if body["model"] != "claude-sonnet-4-6" {
		t.Errorf("model = %v, want claude-sonnet-4-6", body["model"])
	}
	if body["system"] != "You are helpful." {
		t.Errorf("system = %v, want 'You are helpful.'", body["system"])
	}
	if body["max_tokens"] != 1024 {
		t.Errorf("max_tokens = %v, want 1024", body["max_tokens"])
	}

	msgs, ok := body["messages"].([]map[string]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages = %v, want 1 user message", body["messages"])
	}
	if msgs[0]["role"] != "user" {
		t.Errorf("messages[0].role = %v, want user", msgs[0]["role"])
	}
}

func TestConvertRequest_MultipleSystemMessages(t *testing.T) {
	req := &ChatCompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []ChatMessage{
			{Role: "system", Content: json.RawMessage(`"Part 1"`)},
			{Role: "developer", Content: json.RawMessage(`"Part 2"`)},
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
		},
	}

	body, err := ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	sys, _ := body["system"].(string)
	if sys != "Part 1\n\nPart 2" {
		t.Errorf("system = %q, want 'Part 1\\n\\nPart 2'", sys)
	}
}

func TestConvertRequest_ToolConversion(t *testing.T) {
	req := &ChatCompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"What is the weather?"`)},
		},
		Tools: []ChatTool{{
			Type: "function",
			Function: ChatFunction{
				Name:        "get_weather",
				Description: "Get current weather",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
			},
		}},
	}

	body, err := ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	tools, ok := body["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %v, want 1 tool", body["tools"])
	}

	tool, _ := tools[0].(map[string]any)
	if tool["name"] != "get_weather" {
		t.Errorf("tool.name = %v, want get_weather", tool["name"])
	}
	if tool["input_schema"] == nil {
		t.Error("tool.input_schema is nil")
	}
	if tool["parameters"] != nil {
		t.Error("tool should not have 'parameters' key (OpenAI field)")
	}
}

func TestConvertRequest_ToolCallsInAssistant(t *testing.T) {
	req := &ChatCompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Weather?"`)},
			{
				Role:    "assistant",
				Content: json.RawMessage(`null`),
				ToolCalls: []ToolCall{{
					ID:   "call_abc",
					Type: "function",
					Function: ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"city":"Berlin"}`,
					},
				}},
			},
			{
				Role:       "tool",
				ToolCallID: "call_abc",
				Content:    json.RawMessage(`"Sunny, 25°C"`),
			},
		},
	}

	body, err := ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	msgs, _ := body["messages"].([]map[string]any)
	if len(msgs) != 3 {
		t.Fatalf("messages count = %d, want 3", len(msgs))
	}

	// Assistant should have tool_use content block
	assistantContent, ok := msgs[1]["content"].([]any)
	if !ok {
		t.Fatalf("assistant content should be array, got %T", msgs[1]["content"])
	}
	toolUse, _ := assistantContent[0].(map[string]any)
	if toolUse["type"] != "tool_use" {
		t.Errorf("tool_use block type = %v", toolUse["type"])
	}
	if toolUse["id"] != "call_abc" {
		t.Errorf("tool_use id = %v, want call_abc", toolUse["id"])
	}

	// Tool result should be user message with tool_result block
	if msgs[2]["role"] != "user" {
		t.Errorf("tool result role = %v, want user", msgs[2]["role"])
	}
}

func TestConvertRequest_ConsecutiveToolResultsMerge(t *testing.T) {
	req := &ChatCompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Do two things"`)},
			{
				Role:    "assistant",
				Content: json.RawMessage(`null`),
				ToolCalls: []ToolCall{
					{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "fn1", Arguments: "{}"}},
					{ID: "call_2", Type: "function", Function: ToolCallFunction{Name: "fn2", Arguments: "{}"}},
				},
			},
			{Role: "tool", ToolCallID: "call_1", Content: json.RawMessage(`"result1"`)},
			{Role: "tool", ToolCallID: "call_2", Content: json.RawMessage(`"result2"`)},
		},
	}

	body, err := ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	msgs, _ := body["messages"].([]map[string]any)
	// Two consecutive tool messages (both become user) should merge into one
	if len(msgs) != 3 {
		t.Fatalf("messages count = %d, want 3 (user, assistant, merged user/tool_results)", len(msgs))
	}

	// Merged user message should have 2 tool_result blocks
	toolResults, ok := msgs[2]["content"].([]any)
	if !ok {
		t.Fatalf("merged content should be array, got %T", msgs[2]["content"])
	}
	if len(toolResults) != 2 {
		t.Errorf("merged tool_results count = %d, want 2", len(toolResults))
	}
}

func TestConvertRequest_ResponseFormatJSON(t *testing.T) {
	req := &ChatCompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Give me JSON"`)},
		},
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}

	body, err := ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	sys, _ := body["system"].(string)
	if sys == "" {
		t.Error("expected system prompt for json_object response_format")
	}
}

func TestConvertRequest_StopSequences(t *testing.T) {
	// String stop
	req := &ChatCompletionRequest{
		Model:    "claude-sonnet-4-6",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
		Stop:     json.RawMessage(`"STOP"`),
	}

	body, err := ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	seqs, _ := body["stop_sequences"].([]string)
	if len(seqs) != 1 || seqs[0] != "STOP" {
		t.Errorf("stop_sequences = %v, want [STOP]", seqs)
	}

	// Array stop
	req.Stop = json.RawMessage(`["A","B"]`)
	body, err = ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	seqs, _ = body["stop_sequences"].([]string)
	if len(seqs) != 2 {
		t.Errorf("stop_sequences = %v, want [A,B]", seqs)
	}
}

func TestConvertRequest_DefaultMaxTokens(t *testing.T) {
	req := &ChatCompletionRequest{
		Model:    "claude-sonnet-4-6",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
	}

	body, err := ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	if body["max_tokens"] != defaultMaxTokens {
		t.Errorf("max_tokens = %v, want %d", body["max_tokens"], defaultMaxTokens)
	}
}

func TestConvertRequest_ToolChoiceMapping(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantNil  bool
	}{
		{`"auto"`, "auto", false},
		{`"none"`, "", true},
		{`"required"`, "any", false},
		{`{"type":"function","function":{"name":"foo"}}`, "tool", false},
	}

	for _, tt := range tests {
		req := &ChatCompletionRequest{
			Model:      "claude-sonnet-4-6",
			Messages:   []ChatMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
			ToolChoice: json.RawMessage(tt.input),
			Tools:      []ChatTool{{Type: "function", Function: ChatFunction{Name: "foo"}}},
		}

		body, err := ConvertRequest(req)
		if err != nil {
			t.Errorf("input=%s: %v", tt.input, err)
			continue
		}

		tc := body["tool_choice"]
		if tt.wantNil {
			if tc != nil {
				t.Errorf("input=%s: tool_choice = %v, want nil", tt.input, tc)
			}
			continue
		}

		tcMap, ok := tc.(map[string]any)
		if !ok {
			t.Errorf("input=%s: tool_choice type = %T", tt.input, tc)
			continue
		}
		if tcMap["type"] != tt.wantType {
			t.Errorf("input=%s: tool_choice.type = %v, want %s", tt.input, tcMap["type"], tt.wantType)
		}
	}
}

func TestConvertResponse_BasicText(t *testing.T) {
	anthropicResp := `{
		"id": "msg_123",
		"model": "claude-sonnet-4-6",
		"content": [{"type": "text", "text": "Hello!"}],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`

	data, usage, err := ConvertResponse([]byte(anthropicResp))
	if err != nil {
		t.Fatal(err)
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Object != "chat.completion" {
		t.Errorf("object = %s, want chat.completion", resp.Object)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices count = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message == nil {
		t.Fatal("message is nil")
	}
	if resp.Choices[0].Message.Content == nil || *resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("content = %v, want 'Hello!'", resp.Choices[0].Message.Content)
	}
	if *resp.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %v, want stop", *resp.Choices[0].FinishReason)
	}

	if usage.PromptTokens != 10 || usage.CompletionTokens != 5 || usage.TotalTokens != 15 {
		t.Errorf("usage = %+v", usage)
	}
}

func TestConvertResponse_ToolUse(t *testing.T) {
	anthropicResp := `{
		"id": "msg_456",
		"model": "claude-sonnet-4-6",
		"content": [
			{"type": "text", "text": "Let me check."},
			{"type": "tool_use", "id": "toolu_abc", "name": "get_weather", "input": {"city": "Berlin"}}
		],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 20, "output_tokens": 15}
	}`

	data, _, err := ConvertResponse([]byte(anthropicResp))
	if err != nil {
		t.Fatal(err)
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}

	msg := resp.Choices[0].Message
	if msg.Content == nil || *msg.Content != "Let me check." {
		t.Errorf("content = %v, want 'Let me check.'", msg.Content)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool_calls count = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "toolu_abc" {
		t.Errorf("tool_call id = %s, want toolu_abc", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("tool_call name = %s, want get_weather", msg.ToolCalls[0].Function.Name)
	}
	if *resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish_reason = %v, want tool_calls", *resp.Choices[0].FinishReason)
	}
}

func TestConvertResponse_StopReasonMapping(t *testing.T) {
	tests := []struct {
		anthropic string
		openai    string
	}{
		{"end_turn", "stop"},
		{"stop_sequence", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
	}

	for _, tt := range tests {
		got := mapStopReason(tt.anthropic)
		if got != tt.openai {
			t.Errorf("mapStopReason(%s) = %s, want %s", tt.anthropic, got, tt.openai)
		}
	}
}

func TestConvertRequest_ContentParts(t *testing.T) {
	req := &ChatCompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`[{"type":"text","text":"Look at this"},{"type":"image_url","image_url":{"url":"https://example.com/img.png"}}]`),
		}},
	}

	body, err := ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	msgs, _ := body["messages"].([]map[string]any)
	content, ok := msgs[0]["content"].([]any)
	if !ok {
		t.Fatalf("content should be array, got %T", msgs[0]["content"])
	}
	if len(content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(content))
	}

	imgBlock, _ := content[1].(map[string]any)
	if imgBlock["type"] != "image" {
		t.Errorf("image block type = %v, want image", imgBlock["type"])
	}
}

func intPtr(n int) *int { return &n }
