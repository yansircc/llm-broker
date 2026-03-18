package driver

import (
	"fmt"
	"net/http"
	"strings"
)

type claudeModelEntry struct {
	PublicID      string
	UpstreamID    string
	ContextWindow int
	Advertise     bool
}

var claudeModelEntries = []claudeModelEntry{
	{PublicID: "claude-opus-4-6", UpstreamID: "claude-opus-4-6", ContextWindow: 200000, Advertise: true},
	{PublicID: "claude-opus-4-5", UpstreamID: "claude-opus-4-5", ContextWindow: 200000, Advertise: true},
	{PublicID: "claude-opus-4-1", UpstreamID: "claude-opus-4-1", ContextWindow: 200000, Advertise: true},
	{PublicID: "claude-opus-4", UpstreamID: "claude-opus-4", ContextWindow: 200000, Advertise: true},
	{PublicID: "claude-sonnet-4-6", UpstreamID: "claude-sonnet-4-6", ContextWindow: 200000, Advertise: true},
	{PublicID: "claude-sonnet-4-5", UpstreamID: "claude-sonnet-4-5", ContextWindow: 200000, Advertise: true},
	{PublicID: "claude-sonnet-4", UpstreamID: "claude-sonnet-4", ContextWindow: 200000, Advertise: true},
	{PublicID: "claude-haiku-4-5-20251001", UpstreamID: "claude-haiku-4-5-20251001", ContextWindow: 200000, Advertise: true},
	{PublicID: "claude-haiku-4-5", UpstreamID: "claude-haiku-4-5-20251001", ContextWindow: 200000, Advertise: true},
	{PublicID: "claude-haiku-4-6", UpstreamID: "claude-haiku-4-5-20251001", ContextWindow: 200000, Advertise: false},
}

func claudeSupportedModels() []Model {
	models := make([]Model, 0, len(claudeModelEntries))
	for _, entry := range claudeModelEntries {
		if !entry.Advertise {
			continue
		}
		models = append(models, Model{
			ID:            entry.PublicID,
			Object:        "model",
			Created:       1709164800,
			OwnedBy:       "anthropic",
			ContextWindow: entry.ContextWindow,
		})
	}
	return models
}

func normalizeClaudeModelID(model string) (string, error) {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "", NewRequestValidationError(http.StatusBadRequest, "model is required")
	}

	lower := strings.ToLower(trimmed)
	for _, entry := range claudeModelEntries {
		if lower == entry.PublicID {
			return entry.UpstreamID, nil
		}
	}

	switch {
	case strings.HasPrefix(lower, "gpt-"), strings.HasPrefix(lower, "codex-"), strings.HasPrefix(lower, "o1"), strings.HasPrefix(lower, "o3"), strings.HasPrefix(lower, "o4"):
		return "", NewRequestValidationError(http.StatusBadRequest, fmt.Sprintf("model %q does not belong to Claude; use the OpenAI/Codex relay instead", trimmed))
	case strings.HasPrefix(lower, "gemini-"):
		return "", NewRequestValidationError(http.StatusBadRequest, fmt.Sprintf("model %q does not belong to Claude; use the Gemini relay instead", trimmed))
	case strings.HasPrefix(lower, "claude-"):
		return "", NewRequestValidationError(http.StatusBadRequest, fmt.Sprintf("unsupported Claude model %q", trimmed))
	default:
		return "", NewRequestValidationError(http.StatusBadRequest, fmt.Sprintf("unsupported Claude model %q", trimmed))
	}
}

func normalizeClaudeModelField(body map[string]interface{}) error {
	if body == nil {
		return NewRequestValidationError(http.StatusBadRequest, "model is required")
	}
	rawModel, _ := body["model"].(string)
	model, err := normalizeClaudeModelID(rawModel)
	if err != nil {
		return err
	}
	body["model"] = model
	return nil
}
