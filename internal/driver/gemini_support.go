package driver

import (
	"encoding/json"
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"time"
)

const geminiQuotaRefreshInterval = 10 * time.Minute

type GeminiState struct {
	ProjectID                      string  `json:"project_id"`
	LastLoadAt                     int64   `json:"last_load_at,omitempty"`
	DailyRequestsRemainingFraction float64 `json:"daily_requests_remaining_fraction,omitempty"`
	DailyRequestsResetAt           int64   `json:"daily_requests_reset_at,omitempty"`
	QuotaUpdatedAt                 int64   `json:"quota_updated_at,omitempty"`
}

func parseGeminiState(state json.RawMessage) GeminiState {
	var s GeminiState
	if len(state) == 0 {
		return s
	}
	_ = json.Unmarshal(state, &s)
	return s
}

func parseGeminiLoadResponse(body []byte) GeminiState {
	var payload struct {
		ProjectID string `json:"cloudaicompanionProject"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.ProjectID == "" {
		return GeminiState{}
	}
	return GeminiState{
		ProjectID:  payload.ProjectID,
		LastLoadAt: time.Now().Unix(),
	}
}

type geminiQuotaInfo struct {
	DailyRequestsRemainingFraction float64
	DailyRequestsResetAt           int64
}

func clampFraction(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func (s *GeminiState) applyQuota(info *geminiQuotaInfo, now time.Time) {
	if s == nil || info == nil {
		return
	}
	s.DailyRequestsRemainingFraction = clampFraction(info.DailyRequestsRemainingFraction)
	s.DailyRequestsResetAt = info.DailyRequestsResetAt
	s.QuotaUpdatedAt = now.Unix()
}

func needsGeminiProject(path string) bool {
	return strings.Contains(path, ":generateContent") ||
		strings.Contains(path, ":streamGenerateContent") ||
		strings.Contains(path, ":countTokens")
}

func injectGeminiProject(rawBody []byte, projectID string) []byte {
	var body map[string]interface{}
	if json.Unmarshal(rawBody, &body) != nil {
		return rawBody
	}
	body["project"] = projectID
	out, err := json.Marshal(body)
	if err != nil {
		return rawBody
	}
	return out
}

func parseGeminiRetryDelay(body []byte) time.Duration {
	if len(body) == 0 {
		return 0
	}

	var payload []struct {
		Error *struct {
			Details []struct {
				Type            string `json:"@type"`
				RetryDelay      string `json:"retryDelay"`
				QuotaResetDelay string `json:"quotaResetDelay"`
				QuotaResetTime  string `json:"quotaResetTimeStamp"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return 0
	}

	for _, item := range payload {
		if item.Error == nil {
			continue
		}
		for _, detail := range item.Error.Details {
			if detail.RetryDelay != "" {
				if d, err := time.ParseDuration(detail.RetryDelay); err == nil && d > 0 {
					return d
				}
			}
		}
	}
	for _, item := range payload {
		if item.Error == nil {
			continue
		}
		for _, detail := range item.Error.Details {
			if detail.QuotaResetDelay != "" {
				if d, err := time.ParseDuration(detail.QuotaResetDelay); err == nil && d > 0 {
					return d
				}
			}
		}
	}
	for _, item := range payload {
		if item.Error == nil {
			continue
		}
		for _, detail := range item.Error.Details {
			if detail.QuotaResetTime != "" {
				if ts, err := time.Parse(time.RFC3339, detail.QuotaResetTime); err == nil {
					if d := time.Until(ts); d > 0 {
						return d
					}
				}
			}
		}
	}
	return 0
}

type geminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
}

func parseGeminiUsageMetadata(body []byte) *geminiUsageMetadata {
	if len(body) == 0 {
		return nil
	}

	var payload struct {
		UsageMetadata *geminiUsageMetadata `json:"usageMetadata"`
		Response      *struct {
			UsageMetadata *geminiUsageMetadata `json:"usageMetadata"`
		} `json:"response"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}
	if payload.UsageMetadata != nil {
		return payload.UsageMetadata
	}
	if payload.Response != nil {
		return payload.Response.UsageMetadata
	}
	return nil
}

func geminiCLIUserAgent(model string) string {
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return fmt.Sprintf("GeminiCLI/0.32.1/%s (%s; %s) google-api-nodejs-client/10.6.1", model, runtime.GOOS, runtime.GOARCH)
}

func withDefaultQuery(rawQuery, key, value string) string {
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	if q.Get(key) == "" {
		q.Set(key, value)
	}
	return q.Encode()
}
