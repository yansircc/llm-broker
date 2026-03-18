package driver

import (
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/identity"
)

// ClaudeConfig holds the configuration needed by the Claude driver.
type ClaudeConfig struct {
	APIURL     string
	APIVersion string
	BetaHeader string
	Pauses     ErrorPauses
}

// ClaudeDriver implements Driver for Claude.
type ClaudeDriver struct {
	cfg         ClaudeConfig
	transformer *identity.Transformer
}

func NewClaudeDriver(cfg ClaudeConfig, transformer *identity.Transformer) *ClaudeDriver {
	return &ClaudeDriver{cfg: cfg, transformer: transformer}
}

func (d *ClaudeDriver) Provider() domain.Provider { return domain.ProviderClaude }

func (d *ClaudeDriver) BucketKey(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	if acct.Subject != "" {
		return string(domain.ProviderClaude) + ":" + acct.Subject
	}
	return string(domain.ProviderClaude) + ":" + acct.ID
}

func (d *ClaudeDriver) Info() ProviderInfo {
	return ProviderInfo{
		Label:               "Claude",
		RelayPaths:          []string{"/v1/messages", "/v1/messages/count_tokens"},
		OAuthStateRequired:  true,
		CallbackPlaceholder: "https://platform.claude.com/oauth/code/callback?code=...",
		CallbackHint:        "email and organization metadata are fetched after token exchange.",
		ProbeLabel:          "haiku",
	}
}

func (d *ClaudeDriver) Models() []Model {
	return claudeSupportedModels()
}
