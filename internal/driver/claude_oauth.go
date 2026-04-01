package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	claudeOAuthClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	claudeOAuthTokenURL     = "https://console.anthropic.com/v1/oauth/token"
	claudeOAuthRedirectURI  = "https://platform.claude.com/oauth/code/callback"
	claudeOAuthScope        = "org:create_api_key user:profile user:inference user:sessions:claude_code"
	claudeOAuthAuthorizeURL = "https://claude.ai/oauth/authorize"
	claudeAIBaseURL         = "https://claude.ai"
	defaultOAuthUA          = "claude-cli/2.2.0 (external, cli)"
)

// OAuthCLIVersion is set by main to match the configured CLI version.
// OAuth/refresh requests use this UA. Defaults to defaultOAuthUA.
var OAuthCLIVersion string

func oauthUA() string {
	if OAuthCLIVersion != "" {
		return "claude-cli/" + OAuthCLIVersion + " (external, cli)"
	}
	return defaultOAuthUA
}

type claudeOrgResponse struct {
	UUID         string   `json:"uuid"`
	Name         string   `json:"name"`
	EmailAddress string   `json:"email_address"`
	Capabilities []string `json:"capabilities"`
}

func generateClaudeAuthURL() (string, OAuthSession, error) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", OAuthSession{}, fmt.Errorf("generate PKCE: %w", err)
	}
	state := generateState()

	params := url.Values{
		"code":                  {"true"},
		"client_id":             {claudeOAuthClientID},
		"response_type":         {"code"},
		"redirect_uri":          {claudeOAuthRedirectURI},
		"scope":                 {claudeOAuthScope},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}

	return claudeOAuthAuthorizeURL + "?" + params.Encode(), OAuthSession{
		CodeVerifier: verifier,
		State:        state,
	}, nil
}

func exchangeClaudeCode(ctx context.Context, client *http.Client, code, verifier, state string) (*TokenResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     claudeOAuthClientID,
		"code":          code,
		"redirect_uri":  claudeOAuthRedirectURI,
		"code_verifier": verifier,
		"state":         state,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", claudeOAuthTokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", oauthUA())
	req.Header.Set("Referer", "https://claude.ai/")
	req.Header.Set("Origin", "https://claude.ai")

	client = httpClientOrDefault(client, 30*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token API returned %d: %s", resp.StatusCode, truncateBytes(respBody, 200))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("empty access_token in response")
	}
	return &tokenResp, nil
}

func fetchClaudeOrgWithToken(ctx context.Context, client *http.Client, accessToken string) (uuid, email, name string, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", claudeAIBaseURL+"/api/organizations", nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	client = httpClientOrDefault(client, 15*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("organizations API returned %d: %s", resp.StatusCode, truncateBytes(body, 200))
	}

	var orgs []claudeOrgResponse
	if err := json.Unmarshal(body, &orgs); err != nil {
		return "", "", "", fmt.Errorf("parse organizations: %w", err)
	}
	if len(orgs) == 0 {
		return "", "", "", fmt.Errorf("no organizations found")
	}

	best := -1
	bestCaps := -1
	for i, org := range orgs {
		hasChat := false
		for _, cap := range org.Capabilities {
			if cap == "chat" {
				hasChat = true
				break
			}
		}
		if hasChat && len(org.Capabilities) > bestCaps {
			best = i
			bestCaps = len(org.Capabilities)
		}
	}
	if best == -1 {
		best = 0
	}

	return orgs[best].UUID, orgs[best].EmailAddress, orgs[best].Name, nil
}

func refreshClaudeToken(ctx context.Context, client *http.Client, refreshToken string) (*TokenResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     claudeOAuthClientID,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", claudeOAuthTokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", oauthUA())
	req.Header.Set("Referer", "https://claude.ai/")
	req.Header.Set("Origin", "https://claude.ai")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth returned %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("empty access_token in response")
	}
	return &tokenResp, nil
}
