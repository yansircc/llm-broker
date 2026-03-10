package driver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/identity"
)

func (d *ClaudeDriver) GenerateAuthURL() (string, OAuthSession, error) {
	return generateClaudeAuthURL()
}

func (d *ClaudeDriver) ExchangeCode(ctx context.Context, code, verifier, state string) (*ExchangeResult, error) {
	result, err := exchangeClaudeCode(ctx, code, verifier, state)
	if err != nil {
		return nil, err
	}

	orgUUID, email, orgName, err := fetchClaudeOrgWithToken(ctx, result.AccessToken)
	if err != nil {
		orgUUID = fetchOrgUUIDFromAPIHeader(ctx, d.cfg.APIURL, result.AccessToken, d.cfg.APIVersion, d.cfg.BetaHeader)
		email = "account-" + time.Now().Format("0102-1504")
	}

	if orgUUID == "" {
		return nil, fmt.Errorf("could not obtain organization UUID (subject)")
	}

	return &ExchangeResult{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		Subject:      orgUUID,
		Email:        email,
		Identity: map[string]string{
			"orgUUID": orgUUID,
			"orgName": orgName,
			"email":   email,
		},
	}, nil
}

func (d *ClaudeDriver) RefreshToken(ctx context.Context, client *http.Client, refreshToken string) (*TokenResponse, error) {
	return refreshClaudeToken(ctx, client, refreshToken)
}

func fetchOrgUUIDFromAPIHeader(ctx context.Context, apiURL, accessToken, apiVersion, betaHeader string) string {
	body := `{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	identity.SetRequiredHeaders(req.Header, accessToken, apiVersion, betaHeader)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	return resp.Header.Get("Anthropic-Organization-Id")
}
