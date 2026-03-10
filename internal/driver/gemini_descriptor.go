package driver

import (
	"encoding/json"

	"github.com/yansircc/llm-broker/internal/domain"
)

const geminiAPIClientHeader = "gl-node/25.8.0"

type GeminiConfig struct {
	APIURL            string
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRedirectURI  string
	Pauses            ErrorPauses
}

type GeminiDriver struct {
	cfg GeminiConfig
}

func NewGeminiDriver(cfg GeminiConfig) *GeminiDriver {
	return &GeminiDriver{cfg: cfg}
}

func (d *GeminiDriver) Provider() domain.Provider { return domain.ProviderGemini }

func (d *GeminiDriver) BucketKey(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	state := parseGeminiState(json.RawMessage(acct.ProviderStateJSON))
	if acct.Subject != "" && state.ProjectID != "" {
		return string(domain.ProviderGemini) + ":" + acct.Subject + ":" + state.ProjectID
	}
	if acct.Subject != "" {
		return string(domain.ProviderGemini) + ":" + acct.Subject
	}
	return string(domain.ProviderGemini) + ":" + acct.ID
}

func (d *GeminiDriver) Info() ProviderInfo {
	return ProviderInfo{
		Label:               "Gemini",
		RelayPaths:          []string{"/gemini/{path...}"},
		OAuthStateRequired:  true,
		CallbackPlaceholder: "https://codeassist.google.com/authcode?code=...",
		CallbackHint:        "project provisioning is completed during OAuth exchange.",
		ProbeLabel:          "loadCodeAssist",
	}
}

func (d *GeminiDriver) Models() []Model {
	return []Model{
		{ID: "gemini-2.5-flash", Object: "model", Created: 1709164800, OwnedBy: "google", ContextWindow: 1048576},
		{ID: "gemini-2.5-pro", Object: "model", Created: 1709164800, OwnedBy: "google", ContextWindow: 1048576},
	}
}
