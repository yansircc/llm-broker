package driver

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func (d *GeminiDriver) GenerateAuthURL() (string, OAuthSession, error) {
	return generateGeminiAuthURL(d.cfg)
}

func (d *GeminiDriver) ExchangeCode(ctx context.Context, code, verifier, _ string) (*ExchangeResult, error) {
	result, err := exchangeGeminiCode(ctx, d.cfg, code, verifier)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	identity := result.Identity
	if identity == nil || identity.Subject == "" {
		identity, err = fetchGeminiUserInfo(ctx, client, result.AccessToken)
		if err != nil {
			return nil, err
		}
	}
	if identity.Subject == "" {
		return nil, fmt.Errorf("could not obtain google subject")
	}

	loadInfo, err := loadGeminiCodeAssist(ctx, client, d.cfg.APIURL, result.AccessToken)
	if err != nil {
		return nil, err
	}
	projectID := loadInfo.ProjectID
	if projectID == "" {
		projectID, err = onboardGeminiUser(ctx, client, d.cfg.APIURL, result.AccessToken)
		if err != nil {
			return nil, err
		}
	}
	if projectID == "" {
		return nil, fmt.Errorf("gemini project provisioning returned empty project id")
	}

	state := GeminiState{
		ProjectID:  projectID,
		LastLoadAt: time.Now().Unix(),
	}
	if quota, err := retrieveGeminiUserQuota(ctx, client, d.cfg.APIURL, result.AccessToken); err == nil {
		state.applyQuota(quota, time.Now())
	}

	email := identity.Email
	if email == "" {
		email = "gemini-" + time.Now().Format("0102-1504")
	}

	return &ExchangeResult{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		Subject:      identity.Subject,
		Email:        email,
		Identity: map[string]string{
			"sub":     identity.Subject,
			"email":   identity.Email,
			"name":    identity.Name,
			"picture": identity.Picture,
		},
		ProviderState: mustMarshalJSON(state),
	}, nil
}

func (d *GeminiDriver) RefreshToken(ctx context.Context, client *http.Client, refreshToken string) (*TokenResponse, error) {
	return refreshGeminiToken(ctx, d.cfg, client, refreshToken)
}
