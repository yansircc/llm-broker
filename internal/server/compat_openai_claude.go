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
	modernEnvelope := compatClaudeUsesModernEnvelope(model)

	claudeReq := &compatClaudeRequest{
		Model:         model,
		MaxTokens:     compatMaxTokens(req),
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: stopSequences,
	}
	upstreamStream := req.Stream
	claudeReq.Stream = &upstreamStream
	if modernEnvelope {
		claudeReq.Temperature = nil
		claudeReq.OutputConfig = &compatClaudeOutputConfig{Effort: "medium"}
		claudeReq.Thinking = &compatClaudeThinking{Type: "adaptive"}
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
	claudeReq.System = compatClaudeSystemValue(modernEnvelope, systemParts)

	return claudeReq, requestedModel, nil
}

func compatClaudeUsesModernEnvelope(model string) bool {
	switch strings.TrimSpace(model) {
	case "claude-sonnet-4-6", "claude-opus-4-6":
		return true
	default:
		return false
	}
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

func compatClaudeSystemValue(modernEnvelope bool, parts []string) any {
	text := strings.Join(parts, "\n\n")
	if !modernEnvelope {
		return text
	}

	blocks := []compatClaudeSystemBlock{
		{
			Type: "text",
			Text: "You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK.",
			CacheControl: &compatClaudeCacheControlPolicy{
				Type: "ephemeral",
			},
		},
	}
	if strings.TrimSpace(text) != "" {
		blocks = append(blocks, compatClaudeSystemBlock{
			Type: "text",
			Text: text,
			CacheControl: &compatClaudeCacheControlPolicy{
				Type: "ephemeral",
			},
		})
	}
	return blocks
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
