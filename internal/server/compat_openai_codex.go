package server

import (
	"encoding/json"
	"strings"
	"time"
)

func compatOpenAIChatToCodexResponsesRequest(req *compatOpenAIChatRequest) (map[string]any, error) {
	if req == nil {
		return nil, errCompat("request is required")
	}
	input := make([]map[string]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "user"
		}
		text, err := compatExtractTextContent(msg.Content)
		if err != nil {
			return nil, err
		}
		input = append(input, map[string]string{
			"role":    role,
			"content": text,
		})
	}
	out := map[string]any{
		"model": req.Model,
		"input": input,
	}
	if _, _, baseModel, err := resolveCompatModel(req.Model); err == nil {
		out["model"] = baseModel
	}
	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		out["max_output_tokens"] = *req.MaxCompletionTokens
	} else if req.MaxTokens != nil && *req.MaxTokens > 0 {
		out["max_output_tokens"] = *req.MaxTokens
	}
	if req.Temperature != nil {
		out["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		out["top_p"] = *req.TopP
	}
	if req.Stream {
		out["stream"] = true
	}
	return out, nil
}

func compatCodexResponsesToOpenAIChatResponse(body []byte, requestedModel string) (*compatOpenAIChatResponse, error) {
	var resp struct {
		ID         string `json:"id"`
		Model      string `json:"model"`
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	text := resp.OutputText
	if text == "" {
		var parts []string
		for _, item := range resp.Output {
			for _, content := range item.Content {
				if content.Text != "" {
					parts = append(parts, content.Text)
				}
			}
		}
		text = strings.Join(parts, "")
	}
	id := resp.ID
	if id == "" {
		id = "chatcmpl-compat"
	}
	model := requestedModel
	if resp.Model != "" {
		model = resp.Model
	}
	out := &compatOpenAIChatResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []compatOpenAIChatChoice{
			{
				Index: 0,
				Message: compatOpenAIResponseMessage{
					Role:    "assistant",
					Content: text,
				},
				FinishReason: "stop",
			},
		},
	}
	if resp.Usage != nil {
		total := resp.Usage.TotalTokens
		if total == 0 {
			total = resp.Usage.InputTokens + resp.Usage.OutputTokens
		}
		out.Usage = &compatOpenAIChatUsageInfo{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      total,
		}
	}
	return out, nil
}
