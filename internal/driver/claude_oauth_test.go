package driver

import (
	"net/url"
	"testing"
)

func TestClaudeOAuthURLsUsePlatformHost(t *testing.T) {
	for name, rawURL := range map[string]string{
		"authorize": claudeOAuthAuthorizeURL,
		"org_api":   claudeAIBaseURL,
		"token":     claudeOAuthTokenURL,
		"redirect":  claudeOAuthRedirectURI,
	} {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			t.Fatalf("%s URL parse failed: %v", name, err)
		}
		if parsed.Host != "platform.claude.com" {
			t.Fatalf("%s host = %q, want platform.claude.com", name, parsed.Host)
		}
	}
}
