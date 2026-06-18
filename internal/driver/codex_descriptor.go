package driver

import (
	"sync/atomic"

	"github.com/yansircc/llm-broker/internal/domain"
)

// CodexConfig holds the configuration needed by the Codex driver.
type CodexConfig struct {
	APIURL string
	Pauses ErrorPauses
}

// CodexDriver implements Driver for Codex (OpenAI).
type CodexDriver struct {
	cfg CodexConfig

	// lastGoodModel is the most recent standard-family model that a real relay
	// request succeeded with, observed live in Interpret. Probe prefers it so
	// health checks track whatever model clients currently use, instead of a
	// hardcoded name that silently 400s once the provider drops it. Shared
	// across all accounts (the driver is a singleton); empty until the first
	// success after startup, where probeModel falls back to the catalog.
	lastGoodModel atomic.Pointer[string]
}

func NewCodexDriver(cfg CodexConfig) *CodexDriver {
	return &CodexDriver{cfg: cfg}
}

// probeModel returns the model to use for health-check probes: the live
// observed model if known, else the catalog's primary entry.
func (d *CodexDriver) probeModel() string {
	if m := d.lastGoodModel.Load(); m != nil && *m != "" {
		return *m
	}
	if models := d.Models(); len(models) > 0 {
		return models[0].ID
	}
	return "gpt-5.5"
}

// noteGoodModel records a model that a real relay request succeeded with,
// limited to the standard family so probes always elicit full rate-limit
// headers (spark-family models can route differently).
func (d *CodexDriver) noteGoodModel(model string) {
	if model == "" || codexModelFamily(model) != "" {
		return
	}
	d.lastGoodModel.Store(&model)
}

func (d *CodexDriver) Provider() domain.Provider { return domain.ProviderCodex }

func (d *CodexDriver) BucketKey(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	if acct.Subject != "" {
		return string(domain.ProviderCodex) + ":" + acct.Subject
	}
	return string(domain.ProviderCodex) + ":" + acct.ID
}

func (d *CodexDriver) Info() ProviderInfo {
	return ProviderInfo{
		Label:               "Codex",
		RelayPaths:          []string{"/openai/responses"},
		OAuthStateRequired:  false,
		CallbackPlaceholder: "http://localhost:1455/auth/callback?code=...",
		CallbackHint:        "account metadata is extracted from the id_token.",
		ProbeLabel:          "codex",
	}
}

func (d *CodexDriver) Models() []Model {
	return []Model{
		{ID: "gpt-5.5", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.4", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 1050000},
		{ID: "gpt-5.4-mini", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 1050000},
		{ID: "gpt-5.3-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.3-codex-spark", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 128000},
		{ID: "gpt-5.2-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.2", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1-codex-max", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1-codex-mini", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 192000},
		{ID: "gpt-5-codex-mini", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 192000},
		{ID: "gpt-5", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 192000},
		{ID: "codex-1", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 192000},
	}
}
