package driver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type geminiIdentity struct {
	Subject string
	Email   string
	Name    string
	Picture string
}

func fetchGeminiUserInfo(ctx context.Context, client *http.Client, accessToken string) (*geminiIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", geminiUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read userinfo: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned %d: %s", resp.StatusCode, truncateBytes(body, 200))
	}

	var info struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse userinfo: %w", err)
	}
	if info.Sub == "" {
		return nil, fmt.Errorf("userinfo missing sub")
	}

	return &geminiIdentity{
		Subject: info.Sub,
		Email:   info.Email,
		Name:    info.Name,
		Picture: info.Picture,
	}, nil
}

func parseGeminiIDToken(idToken string) *geminiIdentity {
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
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil
	}
	if claims.Sub == "" {
		return nil
	}

	return &geminiIdentity{
		Subject: claims.Sub,
		Email:   claims.Email,
		Name:    claims.Name,
		Picture: claims.Picture,
	}
}
