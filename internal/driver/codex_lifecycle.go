package driver

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func (d *CodexDriver) GenerateAuthURL() (string, OAuthSession, error) {
	return generateCodexAuthURL()
}

func (d *CodexDriver) ExchangeCode(ctx context.Context, code, verifier, _ string) (*ExchangeResult, error) {
	result, err := exchangeCodexCode(ctx, code, verifier)
	if err != nil {
		return nil, err
	}

	email := "codex-" + time.Now().Format("0102-1504")
	identity := make(map[string]string)
	var subject string

	if result.IDInfo != nil {
		if result.IDInfo.Email != "" {
			email = result.IDInfo.Email
		}
		subject = result.IDInfo.ChatGPTAccountID
		identity["chatgptAccountId"] = result.IDInfo.ChatGPTAccountID
		identity["email"] = result.IDInfo.Email
		identity["orgTitle"] = result.IDInfo.OrgTitle
	}

	if subject == "" {
		return nil, fmt.Errorf("could not obtain chatgptAccountId (subject) from ID token")
	}

	return &ExchangeResult{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		Subject:      subject,
		Email:        email,
		Identity:     identity,
	}, nil
}

func (d *CodexDriver) RefreshToken(ctx context.Context, client *http.Client, refreshToken string) (*TokenResponse, error) {
	return refreshCodexToken(ctx, client, refreshToken)
}
