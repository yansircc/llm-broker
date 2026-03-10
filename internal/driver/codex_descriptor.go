package driver

import "github.com/yansircc/llm-broker/internal/domain"

// CodexConfig holds the configuration needed by the Codex driver.
type CodexConfig struct {
	APIURL string
	Pauses ErrorPauses
}

// CodexDriver implements Driver for Codex (OpenAI).
type CodexDriver struct {
	cfg CodexConfig
}

func NewCodexDriver(cfg CodexConfig) *CodexDriver {
	return &CodexDriver{cfg: cfg}
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
		{ID: "gpt-5.4", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 1050000},
		{ID: "gpt-5.3-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.2-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1-codex-max", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1-codex-mini", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "codex-1", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 192000},
	}
}
