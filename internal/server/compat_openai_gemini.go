package server

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func compatOpenAIChatToGeminiRequest(req *compatOpenAIChatRequest) (*compatGeminiRequest, error) {
	if req == nil {
		return nil, errCompat("request is required")
	}
	if compatHasTools(req.Tools) || compatHasToolChoice(req.ToolChoice) {
		return nil, errCompat("tools are not supported on the gemini compat surface yet")
	}
	_, model, requestedModel, err := resolveCompatModel(req.Model)
	if err != nil {
		return nil, err
	}
	if !compatProviderMatches(domain.ProviderGemini, requestedModel) {
		return nil, errCompat("model must be a gemini model, e.g. gemini/gemini-2.5-flash")
	}
	if len(req.Messages) == 0 {
		return nil, errCompat("messages is required")
	}

	stopSequences, err := parseCompatStop(req.Stop)
	if err != nil {
		return nil, err
	}
	responseFormat, err := parseCompatResponseFormat(req.ResponseFormat)
	if err != nil {
		return nil, err
	}

	geminiReq := &compatGeminiRequest{
		Model: model,
		GenerationConfig: &compatGeminiGenerationConfig{
			MaxOutputTokens: compatMaxTokens(req),
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			StopSequences:   stopSequences,
		},
	}
	if geminiReq.GenerationConfig.MaxOutputTokens <= 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = compatClaudeDefaultMaxTokens
	}

	if err := applyCompatGeminiResponseFormat(geminiReq.GenerationConfig, responseFormat); err != nil {
		return nil, err
	}

	var systemParts []string
	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		content, err := compatExtractTextContent(message.Content)
		if err != nil {
			return nil, err
		}
		switch role {
		case "system", "developer":
			if content != "" {
				systemParts = append(systemParts, content)
			}
		case "user":
			geminiReq.Contents = append(geminiReq.Contents, compatGeminiContent{
				Role:  "user",
				Parts: []compatGeminiPart{{Text: content}},
			})
		case "assistant":
			geminiReq.Contents = append(geminiReq.Contents, compatGeminiContent{
				Role:  "model",
				Parts: []compatGeminiPart{{Text: content}},
			})
		default:
			return nil, errCompat("unsupported message role: " + strings.TrimSpace(message.Role))
		}
	}
	if len(geminiReq.Contents) == 0 {
		return nil, errCompat("at least one user or assistant message is required")
	}
	if len(systemParts) > 0 {
		geminiReq.SystemInstruction = &compatGeminiContent{
			Parts: []compatGeminiPart{{Text: strings.Join(systemParts, "\n\n")}},
		}
	}
	return geminiReq, nil
}

func compatGeminiToOpenAIChatResponse(body []byte, requestedModel string) (*compatOpenAIChatResponse, error) {
	resp, err := compatParseGeminiResponse(body)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errCompat("empty gemini response")
	}

	content := ""
	finishReason := "stop"
	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		finishReason = compatGeminiFinishReason(candidate.FinishReason)
		if candidate.Content != nil {
			var builder strings.Builder
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					builder.WriteString(part.Text)
				}
			}
			content = builder.String()
		}
	}

	openAIResp := &compatOpenAIChatResponse{
		ID:      compatGeminiResponseID(resp),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []compatOpenAIChatChoice{
			{
				Index: 0,
				Message: compatOpenAIResponseMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
	}
	if resp.Usage != nil {
		openAIResp.Usage = &compatOpenAIChatUsageInfo{
			PromptTokens:     resp.Usage.PromptTokenCount,
			CompletionTokens: resp.Usage.CandidatesTokenCount,
			TotalTokens:      resp.Usage.PromptTokenCount + resp.Usage.CandidatesTokenCount,
		}
	}
	return openAIResp, nil
}

func compatGeminiFinishReason(reason string) string {
	switch strings.ToUpper(strings.TrimSpace(reason)) {
	case "MAX_TOKENS":
		return "length"
	case "UNEXPECTED_TOOL_CALL":
		return "tool_calls"
	case "SAFETY", "RECITATION", "LANGUAGE", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "IMAGE_SAFETY":
		return "content_filter"
	default:
		return "stop"
	}
}

func compatParseGeminiResponse(body []byte) (*compatGeminiResponse, error) {
	var resp compatGeminiResponse
	if json.Unmarshal(body, &resp) == nil {
		if resp.ResponseID != "" || resp.ModelVersion != "" || len(resp.Candidates) > 0 || resp.Usage != nil {
			return &resp, nil
		}
	}

	var envelope compatGeminiResponseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Response == nil {
		return nil, errCompat("invalid gemini response")
	}
	if envelope.Response.ResponseID == "" && envelope.Response.ModelVersion == "" && len(envelope.Response.Candidates) == 0 && envelope.Response.Usage == nil {
		return nil, errCompat("invalid gemini response")
	}
	return envelope.Response, nil
}

func compatGeminiResponseID(resp *compatGeminiResponse) string {
	if resp == nil || strings.TrimSpace(resp.ResponseID) == "" {
		return "chatcmpl-compat"
	}
	return resp.ResponseID
}

func applyCompatGeminiResponseFormat(cfg *compatGeminiGenerationConfig, spec *compatResponseFormatSpec) error {
	if cfg == nil || spec == nil {
		return nil
	}
	switch spec.Type {
	case "", "text":
		return nil
	case "json_object":
		cfg.ResponseMIMEType = "application/json"
		return nil
	case "json_schema":
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseJSONSchema = spec.JSONSchema.Schema
		return nil
	default:
		return errCompat("unsupported response_format type: " + spec.Type)
	}
}
