package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	codexOAuthClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexOAuthAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	codexOAuthTokenURL     = "https://auth.openai.com/oauth/token"
	codexOAuthRedirectURI  = "http://localhost:1455/auth/callback"
	codexOAuthScope        = "openid profile email offline_access"
)

// GenerateCodexAuthURL creates a PKCE-secured authorization URL for Codex OAuth.
func GenerateCodexAuthURL() (authURL string, session OAuthSession, err error) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", OAuthSession{}, fmt.Errorf("generate PKCE: %w", err)
	}
	state := generateState()

	params := url.Values{
		"response_type":              {"code"},
		"client_id":                  {codexOAuthClientID},
		"redirect_uri":              {codexOAuthRedirectURI},
		"scope":                     {codexOAuthScope},
		"state":                     {state},
		"code_challenge":            {challenge},
		"code_challenge_method":     {"S256"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
	}

	return codexOAuthAuthorizeURL + "?" + params.Encode(), OAuthSession{
		CodeVerifier: verifier,
		State:        state,
	}, nil
}

// ExchangeCodexCode exchanges a Codex authorization code for tokens.
func ExchangeCodexCode(ctx context.Context, code, verifier string) (*ExchangeCodeResult, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {codexOAuthClientID},
		"code":          {code},
		"redirect_uri":  {codexOAuthRedirectURI},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", codexOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codex token API returned %d: %s", resp.StatusCode, truncateBytes(body, 200))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("empty access_token in response")
	}

	result := &ExchangeCodeResult{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	}

	if tokenResp.IDToken != "" {
		if info := ParseCodexIDToken(tokenResp.IDToken); info != nil {
			result.CodexInfo = info
		}
	}

	return result, nil
}

// CallCodexRefresh performs a Codex OAuth token refresh.
func CallCodexRefresh(ctx context.Context, client *http.Client, refreshToken string) (*TokenResponse, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {codexOAuthClientID},
		"scope":         {"openid profile email"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", codexOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
		return nil, fmt.Errorf("codex oauth returned %d: %s", resp.StatusCode, string(respBody))
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

// CodexIDInfo holds extracted info from the Codex ID token.
type CodexIDInfo struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	Email            string `json:"email"`
	OrgTitle         string `json:"org_title"`
}

// ParseCodexIDToken extracts account info from a JWT id_token payload.
func ParseCodexIDToken(idToken string) *CodexIDInfo {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return nil
	}

	payload := parts[1]
	if m := len(payload) % 4; m != 0 {
		payload += strings.Repeat("=", 4-m)
	}
	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil
	}

	var claims struct {
		Email string `json:"email"`
		Auth  struct {
			ChatGPTAccountID string `json:"chatgpt_account_id"`
			Organizations    []struct {
				Title string `json:"title"`
			} `json:"organizations"`
		} `json:"https://api.openai.com/auth"`
	}
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil
	}

	info := &CodexIDInfo{
		ChatGPTAccountID: claims.Auth.ChatGPTAccountID,
		Email:            claims.Email,
	}
	if len(claims.Auth.Organizations) > 0 {
		info.OrgTitle = claims.Auth.Organizations[0].Title
	}
	return info
}
