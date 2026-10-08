package driver

import (
	"fmt"
	"net/http"
	"strings"
)

type claudeModelEntry struct {
	PublicID             string
	UpstreamID           string
	ContextWindow        int
	Advertise            bool
	CodeSystemEnvelope   bool
	CompatModernEnvelope bool
	Pricing              claudeModelPricing
}

type claudeModelPricing struct {
	Input       float64
	Output      float64
	CacheRead   float64
	CacheCreate float64
	PromptTiers []claudePromptPriceTier // Ordered by increasing AboveTokens.
}

type claudePromptPriceTier struct {
	AboveTokens int
	Multiplier  float64
}

var (
	claudeFablePricing    = claudeModelPricing{Input: 10, Output: 50, CacheRead: 1.00, CacheCreate: 12.50}
	claudeFable51Pricing  = claudeModelPricing{Input: 10, Output: 50, CacheRead: 0.25, CacheCreate: 12.50}
	claudeOpusPricing     = claudeModelPricing{Input: 5, Output: 25, CacheRead: 0.50, CacheCreate: 6.25}
	claudeOpus55Pricing   = claudeModelPricing{Input: 4, Output: 20, CacheRead: 0.20, CacheCreate: 5}
	claudeSonnetPricing   = claudeModelPricing{Input: 3, Output: 15, CacheRead: 0.30, CacheCreate: 3.75}
	claudeSonnet5Pricing  = claudeModelPricing{Input: 2, Output: 10, CacheRead: 0.20, CacheCreate: 2.50}
	claudeSonnet55Pricing = claudeModelPricing{Input: 2, Output: 10, CacheRead: 0.10, CacheCreate: 2.50}
	claudeHaikuPricing    = claudeModelPricing{Input: 1, Output: 5, CacheRead: 0.10, CacheCreate: 1.25}
	claudeHaiku55Pricing  = claudeModelPricing{
		Input: 0.10, Output: 0.50, CacheRead: 0.01, CacheCreate: 0.125,
		PromptTiers: []claudePromptPriceTier{{AboveTokens: 100000, Multiplier: 5}},
	}
)

var claudeModelEntries = []claudeModelEntry{
	{PublicID: "claude-fable-5-1", UpstreamID: "claude-fable-5-1", ContextWindow: 1000000, Advertise: true, CodeSystemEnvelope: true, CompatModernEnvelope: true, Pricing: claudeFable51Pricing},
	{PublicID: "claude-fable-5", UpstreamID: "claude-fable-5", ContextWindow: 1000000, Advertise: true, CodeSystemEnvelope: true, Pricing: claudeFablePricing},
	{PublicID: "claude-opus-5-5", UpstreamID: "claude-opus-5-5", ContextWindow: 1000000, Advertise: true, CodeSystemEnvelope: true, CompatModernEnvelope: true, Pricing: claudeOpus55Pricing},
	{PublicID: "claude-opus-5", UpstreamID: "claude-opus-5", ContextWindow: 1000000, Advertise: true, CodeSystemEnvelope: true, CompatModernEnvelope: true, Pricing: claudeOpusPricing},
	{PublicID: "claude-opus-4-8", UpstreamID: "claude-opus-4-8", ContextWindow: 1000000, Advertise: true, CodeSystemEnvelope: true, Pricing: claudeOpusPricing},
	{PublicID: "claude-opus-4-7", UpstreamID: "claude-opus-4-7", ContextWindow: 200000, Advertise: true, CodeSystemEnvelope: true, Pricing: claudeOpusPricing},
	{PublicID: "claude-opus-4-6", UpstreamID: "claude-opus-4-6", ContextWindow: 200000, Advertise: true, CodeSystemEnvelope: true, CompatModernEnvelope: true, Pricing: claudeOpusPricing},
	{PublicID: "claude-opus-4-5", UpstreamID: "claude-opus-4-5", ContextWindow: 200000, Advertise: true, CodeSystemEnvelope: true, Pricing: claudeOpusPricing},
	{PublicID: "claude-opus-4-1", UpstreamID: "claude-opus-4-1", ContextWindow: 200000, Advertise: true, CodeSystemEnvelope: true, Pricing: claudeOpusPricing},
	{PublicID: "claude-opus-4", UpstreamID: "claude-opus-4", ContextWindow: 200000, Advertise: true, CodeSystemEnvelope: true, Pricing: claudeOpusPricing},
	{PublicID: "claude-sonnet-5-5", UpstreamID: "claude-sonnet-5-5", ContextWindow: 1000000, Advertise: true, CodeSystemEnvelope: true, CompatModernEnvelope: true, Pricing: claudeSonnet55Pricing},
	{PublicID: "claude-sonnet-5", UpstreamID: "claude-sonnet-5", ContextWindow: 1000000, Advertise: true, CodeSystemEnvelope: true, CompatModernEnvelope: true, Pricing: claudeSonnet5Pricing},
	{PublicID: "claude-sonnet-4-6", UpstreamID: "claude-sonnet-4-6", ContextWindow: 200000, Advertise: true, CodeSystemEnvelope: true, CompatModernEnvelope: true, Pricing: claudeSonnetPricing},
	{PublicID: "claude-sonnet-4-5", UpstreamID: "claude-sonnet-4-5", ContextWindow: 200000, Advertise: true, CodeSystemEnvelope: true, Pricing: claudeSonnetPricing},
	{PublicID: "claude-sonnet-4", UpstreamID: "claude-sonnet-4", ContextWindow: 200000, Advertise: true, CodeSystemEnvelope: true, Pricing: claudeSonnetPricing},
	{PublicID: "claude-haiku-5-5", UpstreamID: "claude-haiku-5-5", ContextWindow: 1000000, Advertise: true, CodeSystemEnvelope: true, CompatModernEnvelope: true, Pricing: claudeHaiku55Pricing},
	{PublicID: "claude-haiku-4-5-20251001", UpstreamID: "claude-haiku-4-5-20251001", ContextWindow: 200000, Advertise: true, Pricing: claudeHaikuPricing},
	{PublicID: "claude-haiku-4-5", UpstreamID: "claude-haiku-4-5-20251001", ContextWindow: 200000, Advertise: true, Pricing: claudeHaikuPricing},
	{PublicID: "claude-haiku-4-6", UpstreamID: "claude-haiku-4-5-20251001", ContextWindow: 200000, Advertise: false, Pricing: claudeHaikuPricing},
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

func ClaudeModelUsesCodeSystemEnvelope(model string) bool {
	entry, ok := claudeModelEntryForID(model)
	return ok && entry.CodeSystemEnvelope
}

func ClaudeModelUsesCompatModernEnvelope(model string) bool {
	entry, ok := claudeModelEntryForID(model)
	return ok && entry.CompatModernEnvelope
}

func normalizeClaudeModelID(model string) (string, error) {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "", NewRequestValidationError(http.StatusBadRequest, "model is required")
	}

	lower := strings.ToLower(trimmed)
	if entry, ok := claudeModelEntryForID(lower); ok {
		return entry.UpstreamID, nil
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

func claudeModelEntryForID(model string) (claudeModelEntry, bool) {
	normalized := strings.ToLower(strings.TrimSpace(model))
	for _, entry := range claudeModelEntries {
		if normalized == entry.PublicID || normalized == entry.UpstreamID {
			return entry, true
		}
	}
	return claudeModelEntry{}, false
}

func (p claudeModelPricing) cost(usage *Usage) float64 {
	promptTokens := usage.InputTokens + usage.CacheReadTokens + usage.CacheCreateTokens
	multiplier := 1.0
	for _, tier := range p.PromptTiers {
		if promptTokens > tier.AboveTokens {
			multiplier = tier.Multiplier
		}
	}
	return multiplier * (float64(usage.InputTokens)*p.Input + float64(usage.OutputTokens)*p.Output +
		float64(usage.CacheReadTokens)*p.CacheRead + float64(usage.CacheCreateTokens)*p.CacheCreate) / 1_000_000
}

var claudeCompatModelAliases = map[string]string{
	"claude-fable-5.1":           "claude-fable-5-1",
	"claude-haiku-5.5":           "claude-haiku-5-5",
	"claude-haiku-4.5":           "claude-haiku-4-5",
	"claude-opus-4.0":            "claude-opus-4",
	"claude-opus-4-0":            "claude-opus-4",
	"claude-opus-4.1":            "claude-opus-4-1",
	"claude-opus-4-1-20250805":   "claude-opus-4-1",
	"claude-opus-4.5":            "claude-opus-4-5",
	"claude-opus-4-5-20251101":   "claude-opus-4-5",
	"claude-opus-4.6":            "claude-opus-4-6",
	"claude-opus-4-20250514":     "claude-opus-4",
	"claude-opus-5.5":            "claude-opus-5-5",
	"claude-sonnet-4.0":          "claude-sonnet-4",
	"claude-sonnet-4-0":          "claude-sonnet-4",
	"claude-sonnet-4.5":          "claude-sonnet-4-5",
	"claude-sonnet-4-5-20250929": "claude-sonnet-4-5",
	"claude-sonnet-4.6":          "claude-sonnet-4-6",
	"claude-sonnet-4-20250514":   "claude-sonnet-4-6",
	"claude-sonnet-5.0":          "claude-sonnet-5",
	"claude-sonnet-5-0":          "claude-sonnet-5",
	"claude-sonnet-5.5":          "claude-sonnet-5-5",
}

// ClaudeCompatCanonicalModel normalizes aliases accepted by the OpenAI-compatible surface.
func ClaudeCompatCanonicalModel(model string) string {
	trimmed := strings.ToLower(strings.TrimSpace(model))
	if canonical, ok := claudeCompatModelAliases[trimmed]; ok {
		return canonical
	}
	return trimmed
}
