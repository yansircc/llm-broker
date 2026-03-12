package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	geminiOAuthAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	geminiOAuthTokenURL     = "https://oauth2.googleapis.com/token"
	geminiUserInfoURL       = "https://openidconnect.googleapis.com/v1/userinfo"
	geminiOAuthScope        = "openid email profile https://www.googleapis.com/auth/cloud-platform"
)

type geminiExchangeResult struct {
	TokenResponse
	Identity *geminiIdentity
}

func generateGeminiAuthURL(cfg GeminiConfig) (string, OAuthSession, error) {
	if cfg.OAuthClientID == "" || cfg.OAuthClientSecret == "" || cfg.OAuthRedirectURI == "" {
		return "", OAuthSession{}, fmt.Errorf("gemini oauth is not configured")
	}

	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", OAuthSession{}, fmt.Errorf("generate PKCE: %w", err)
	}
	state := generateState()

	params := url.Values{
		"client_id":             {cfg.OAuthClientID},
		"response_type":         {"code"},
		"redirect_uri":          {cfg.OAuthRedirectURI},
		"scope":                 {geminiOAuthScope},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"access_type":           {"offline"},
		"prompt":                {"consent select_account"},
	}

	return geminiOAuthAuthorizeURL + "?" + params.Encode(), OAuthSession{
		CodeVerifier: verifier,
		State:        state,
	}, nil
}

func exchangeGeminiCode(ctx context.Context, client *http.Client, cfg GeminiConfig, code, verifier string) (*geminiExchangeResult, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {cfg.OAuthClientID},
		"client_secret": {cfg.OAuthClientSecret},
		"code":          {code},
		"redirect_uri":  {cfg.OAuthRedirectURI},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", geminiOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client = httpClientOrDefault(client, 30*time.Second)
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
		return nil, fmt.Errorf("google token API returned %d: %s", resp.StatusCode, truncateBytes(body, 200))
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
	if tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("empty refresh_token in response")
	}

	result := &geminiExchangeResult{
		TokenResponse: TokenResponse{
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			ExpiresIn:    tokenResp.ExpiresIn,
		},
	}
	if tokenResp.IDToken != "" {
		result.Identity = parseGeminiIDToken(tokenResp.IDToken)
	}
	return result, nil
}

func refreshGeminiToken(ctx context.Context, cfg GeminiConfig, client *http.Client, refreshToken string) (*TokenResponse, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {cfg.OAuthClientID},
		"client_secret": {cfg.OAuthClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", geminiOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
		return nil, fmt.Errorf("google oauth returned %d: %s", resp.StatusCode, truncateBytes(body, 200))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("empty access_token in response")
	}

	return &TokenResponse{
		AccessToken: tokenResp.AccessToken,
		ExpiresIn:   tokenResp.ExpiresIn,
	}, nil
}
