package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type geminiCodeAssistInfo struct {
	ProjectID string
}

type geminiQuotaResponse struct {
	Buckets []struct {
		ResetTime         string  `json:"resetTime"`
		TokenType         string  `json:"tokenType"`
		ModelID           string  `json:"modelId"`
		RemainingFraction float64 `json:"remainingFraction"`
	} `json:"buckets"`
}

func loadGeminiCodeAssist(ctx context.Context, client *http.Client, apiURL, accessToken string) (*geminiCodeAssistInfo, error) {
	body := `{"metadata":{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL+"/v1internal:loadCodeAssist", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("loadCodeAssist request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read loadCodeAssist: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loadCodeAssist returned %d: %s", resp.StatusCode, truncateBytes(respBody, 200))
	}

	var result struct {
		ProjectID string `json:"cloudaicompanionProject"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse loadCodeAssist: %w", err)
	}
	return &geminiCodeAssistInfo{ProjectID: result.ProjectID}, nil
}

func retrieveGeminiUserQuota(ctx context.Context, client *http.Client, apiURL, accessToken string) (*geminiQuotaInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL+"/v1internal:retrieveUserQuota", strings.NewReader(`{}`))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("retrieveUserQuota request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read retrieveUserQuota: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("retrieveUserQuota returned %d: %s", resp.StatusCode, truncateBytes(respBody, 200))
	}

	var payload geminiQuotaResponse
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("parse retrieveUserQuota: %w", err)
	}

	var info geminiQuotaInfo
	found := false
	for _, bucket := range payload.Buckets {
		if bucket.TokenType != "REQUESTS" {
			continue
		}
		remaining := clampFraction(bucket.RemainingFraction)
		resetAt := int64(0)
		if bucket.ResetTime != "" {
			ts, err := time.Parse(time.RFC3339, bucket.ResetTime)
			if err != nil {
				return nil, fmt.Errorf("parse quota reset time: %w", err)
			}
			resetAt = ts.Unix()
		}

		if !found || remaining < info.DailyRequestsRemainingFraction {
			info.DailyRequestsRemainingFraction = remaining
		}
		if resetAt > 0 && (info.DailyRequestsResetAt == 0 || resetAt < info.DailyRequestsResetAt) {
			info.DailyRequestsResetAt = resetAt
		}
		found = true
	}
	if !found {
		return nil, fmt.Errorf("retrieveUserQuota returned no request buckets")
	}
	return &info, nil
}

func onboardGeminiUser(ctx context.Context, client *http.Client, apiURL, accessToken string) (string, error) {
	body := `{"tierId":"standard-tier","metadata":{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL+"/v1internal:onboardUser", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("onboardUser request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read onboardUser: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("onboardUser returned %d: %s", resp.StatusCode, truncateBytes(respBody, 200))
	}

	var op struct {
		Name     string `json:"name"`
		Done     bool   `json:"done"`
		Response struct {
			Project struct {
				ID string `json:"id"`
			} `json:"cloudaicompanionProject"`
		} `json:"response"`
	}
	if err := json.Unmarshal(respBody, &op); err != nil {
		return "", fmt.Errorf("parse onboardUser: %w", err)
	}
	if op.Done {
		if op.Response.Project.ID == "" {
			return "", fmt.Errorf("onboardUser completed without project id")
		}
		return op.Response.Project.ID, nil
	}
	if op.Name == "" {
		return "", fmt.Errorf("onboardUser returned no operation name")
	}

	opURL := apiURL + "/v1internal/" + op.Name
	for range 12 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
		}

		req, err := http.NewRequestWithContext(ctx, "GET", opURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("getOperation failed: %w", err)
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("read getOperation: %w", err)
		}
		if err := json.Unmarshal(respBody, &op); err != nil {
			return "", fmt.Errorf("parse getOperation: %w", err)
		}
		if op.Done {
			if op.Response.Project.ID == "" {
				return "", fmt.Errorf("onboardUser completed without project id")
			}
			return op.Response.Project.ID, nil
		}
	}

	return "", fmt.Errorf("onboardUser timed out")
}
