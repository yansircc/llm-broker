package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
)

func parseCompatResponseFormat(raw json.RawMessage) (*compatResponseFormatSpec, error) {
	if !hasCompatValue(raw) {
		return nil, nil
	}
	var spec compatResponseFormatSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, errCompat("response_format must be an object")
	}
	spec.Type = strings.ToLower(strings.TrimSpace(spec.Type))
	switch spec.Type {
	case "", "text":
		return &spec, nil
	case "json_object":
		return &spec, nil
	case "json_schema":
		if spec.JSONSchema == nil || spec.JSONSchema.Schema == nil {
			return nil, errCompat("response_format json_schema requires json_schema.schema")
		}
		return &spec, nil
	default:
		return nil, errCompat("unsupported response_format type: " + spec.Type)
	}
}

func parseCompatStop(raw json.RawMessage) ([]string, error) {
	if !hasCompatValue(raw) {
		return nil, nil
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if strings.TrimSpace(single) == "" {
			return nil, nil
		}
		return []string{single}, nil
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		out := make([]string, 0, len(many))
		for _, item := range many {
			if strings.TrimSpace(item) != "" {
				out = append(out, item)
			}
		}
		return out, nil
	}

	return nil, errCompat("stop must be a string or string array")
}

func compatExtractTextContent(raw json.RawMessage) (string, error) {
	if !hasCompatValue(raw) {
		return "", errCompat("message content is required")
	}

	var content string
	if err := json.Unmarshal(raw, &content); err == nil {
		return content, nil
	}

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		texts := make([]string, 0, len(parts))
		for _, part := range parts {
			switch part.Type {
			case "text", "input_text":
				texts = append(texts, part.Text)
			default:
				return "", errCompat("only text content parts are supported on the compat surface")
			}
		}
		return strings.Join(texts, "\n\n"), nil
	}

	return "", errCompat("message content must be a string or text-only content array")
}

func compatMaxTokens(req *compatOpenAIChatRequest) int {
	if req == nil {
		return compatClaudeDefaultMaxTokens
	}
	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		return *req.MaxCompletionTokens
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		return *req.MaxTokens
	}
	return compatClaudeDefaultMaxTokens
}

func writeCompatOpenAIUpstreamError(w http.ResponseWriter, status int, body []byte) {
	message := "unexpected upstream error"
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && strings.TrimSpace(parsed.Error.Message) != "" {
		message = parsed.Error.Message
	}
	writeCompatOpenAIError(w, status, "server_error", message)
}

func compatExtractErrorBody(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return trimmed
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return trimmed
	}

	lines := strings.Split(string(trimmed), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if strings.HasPrefix(payload, "{") || strings.HasPrefix(payload, "[") {
				return []byte(payload)
			}
		}
	}
	return trimmed
}

func writeCompatOpenAIError(w http.ResponseWriter, status int, errType, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errType,
		},
	})
}

func hasCompatValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}
