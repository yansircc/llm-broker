package server

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func compatOpenAIChatToClaudeRequest(req *compatOpenAIChatRequest) (*compatClaudeRequest, string, error) {
	if req == nil {
		return nil, "", errCompat("request is required")
	}
	if compatHasTools(req.Tools) || compatHasToolChoice(req.ToolChoice) {
		return nil, "", errCompat("tools are not supported on the claude compat surface yet")
	}

	_, model, requestedModel, err := resolveCompatModel(req.Model)
	if err != nil {
		return nil, "", err
	}
	if !compatProviderMatches(domain.ProviderClaude, requestedModel) {
		return nil, "", errCompat("model must be a claude model, e.g. claude/claude-sonnet-4-5")
	}
	if len(req.Messages) == 0 {
		return nil, "", errCompat("messages is required")
	}

	stopSequences, err := parseCompatStop(req.Stop)
	if err != nil {
		return nil, "", err
	}
	responseFormat, err := parseCompatResponseFormat(req.ResponseFormat)
	if err != nil {
		return nil, "", err
	}

	claudeReq := &compatClaudeRequest{
		Model:         model,
		MaxTokens:     compatMaxTokens(req),
		Stream:        req.Stream,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: stopSequences,
	}

	var systemParts []string
	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		content, err := compatExtractTextContent(message.Content)
		if err != nil {
			return nil, "", err
		}
		switch role {
		case "system", "developer":
			if content != "" {
				systemParts = append(systemParts, content)
			}
		case "user", "assistant":
			claudeReq.Messages = append(claudeReq.Messages, compatClaudeMessage{
				Role:    role,
				Content: content,
			})
		default:
			return nil, "", errCompat("unsupported message role: " + strings.TrimSpace(message.Role))
		}
	}

	if len(claudeReq.Messages) == 0 {
		return nil, "", errCompat("at least one user or assistant message is required")
	}
	if instruction := compatClaudeResponseFormatInstruction(responseFormat); instruction != "" {
		systemParts = append(systemParts, instruction)
	}
	claudeReq.Messages = compatInjectClaudeSystemReminders(claudeReq.Messages, systemParts)

	return claudeReq, requestedModel, nil
}

func compatInjectClaudeSystemReminders(messages []compatClaudeMessage, systemParts []string) []compatClaudeMessage {
	reminder := compatClaudeSystemReminder(systemParts)
	if reminder == "" {
		return messages
	}

	for i := range messages {
		if messages[i].Role != "user" {
			continue
		}
		if messages[i].Content == "" {
			messages[i].Content = reminder
		} else {
			messages[i].Content = reminder + "\n\n" + messages[i].Content
		}
		return messages
	}

	return append([]compatClaudeMessage{{Role: "user", Content: reminder}}, messages...)
}

func compatClaudeSystemReminder(systemParts []string) string {
	blocks := make([]string, 0, len(systemParts))
	for _, part := range systemParts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		blocks = append(blocks, "<system-reminder>\n"+trimmed+"\n</system-reminder>")
	}
	return strings.Join(blocks, "\n\n")
}

func compatClaudeToOpenAIChatResponse(body []byte, requestedModel string) (*compatOpenAIChatResponse, error) {
	var resp compatClaudeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	content := make([]string, 0, len(resp.Content))
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			content = append(content, block.Text)
		}
	}

	openAIResp := &compatOpenAIChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []compatOpenAIChatChoice{
			{
				Index: 0,
				Message: compatOpenAIResponseMessage{
					Role:    "assistant",
					Content: strings.Join(content, "\n\n"),
				},
				FinishReason: compatClaudeFinishReason(resp.StopReason),
			},
		},
	}

	if resp.Usage != nil {
		openAIResp.Usage = &compatOpenAIChatUsageInfo{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return openAIResp, nil
}

func compatClaudeFinishReason(stopReason string) string {
	switch stopReason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

func compatClaudeResponseFormatInstruction(spec *compatResponseFormatSpec) string {
	if spec == nil {
		return ""
	}
	switch spec.Type {
	case "json_object":
		return "Return only a valid JSON object. Do not include markdown fences or extra commentary."
	case "json_schema":
		schema, err := json.Marshal(spec.JSONSchema.Schema)
		if err != nil {
			return "Return only valid JSON that matches the requested JSON Schema."
		}
		return "Return only valid JSON that matches this JSON Schema: " + string(schema)
	default:
		return ""
	}
}

func compatHasTools(raw json.RawMessage) bool {
	if !hasCompatValue(raw) {
		return false
	}

	var tools []json.RawMessage
	if err := json.Unmarshal(raw, &tools); err == nil {
		return len(tools) > 0
	}

	return true
}

func compatHasToolChoice(raw json.RawMessage) bool {
	if !hasCompatValue(raw) {
		return false
	}

	var choice string
	if err := json.Unmarshal(raw, &choice); err == nil {
		trimmed := strings.ToLower(strings.TrimSpace(choice))
		return trimmed != "" && trimmed != "none"
	}

	return true
}
