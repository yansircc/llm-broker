package driver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// CodexState holds the provider-specific rate-limit state for Codex accounts.
type CodexState struct {
	PrimaryUtil    float64 `json:"primary_util"`
	PrimaryReset   int64   `json:"primary_reset"`
	SecondaryUtil  float64 `json:"secondary_util"`
	SecondaryReset int64   `json:"secondary_reset"`
}

func (d *CodexDriver) captureHeaders(headers http.Header) json.RawMessage {
	if headers == nil {
		return nil
	}
	var s CodexState
	if v := headers.Get("x-codex-primary-used-percent"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			s.PrimaryUtil = pct / 100
		}
	}
	if v := headers.Get("x-codex-primary-reset-after-seconds"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			s.PrimaryReset = time.Now().Unix() + int64(secs)
		}
	}
	if v := headers.Get("x-codex-secondary-used-percent"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			s.SecondaryUtil = pct / 100
		}
	}
	if v := headers.Get("x-codex-secondary-reset-after-seconds"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			s.SecondaryReset = time.Now().Unix() + int64(secs)
		}
	}
	data, _ := json.Marshal(s)
	return data
}

type codexUsageFields struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Details      *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func codexUsageToUsage(u *codexUsageFields) *Usage {
	if u == nil {
		return nil
	}
	result := &Usage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
	}
	if u.Details != nil {
		result.CacheReadTokens = u.Details.CachedTokens
	}
	return result
}

func parseCodexUsage(data string) *Usage {
	var wrapper struct {
		Type     string `json:"type"`
		Response struct {
			Usage *codexUsageFields `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal([]byte(data), &wrapper) != nil {
		return nil
	}
	return codexUsageToUsage(wrapper.Response.Usage)
}

func parseCodexResetsIn(body []byte) time.Duration {
	var envelope struct {
		Error struct {
			ResetsInSeconds int `json:"resets_in_seconds"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.ResetsInSeconds > 0 {
		return time.Duration(envelope.Error.ResetsInSeconds) * time.Second
	}
	return 0
}

func extractCodexErrorMessage(body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		return envelope.Error.Message
	}
	return ""
}
