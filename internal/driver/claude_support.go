package driver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ClaudeState holds the provider-specific rate-limit state for Claude accounts.
type ClaudeState struct {
	FiveHourUtil      float64 `json:"five_hour_util"`
	FiveHourReset     int64   `json:"five_hour_reset"`
	SevenDayUtil      float64 `json:"seven_day_util"`
	SevenDayReset     int64   `json:"seven_day_reset"`
	OpusCooldownUntil int64   `json:"opus_cooldown_until,omitempty"`
}

func (d *ClaudeDriver) captureState(headers http.Header, prevState json.RawMessage) ClaudeState {
	var prev ClaudeState
	if len(prevState) > 0 {
		_ = json.Unmarshal(prevState, &prev)
	}
	s := ClaudeState{
		OpusCooldownUntil: prev.OpusCooldownUntil,
	}
	if headers == nil {
		return s
	}
	if v := headers.Get("anthropic-ratelimit-unified-5h-utilization"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			s.FiveHourUtil = f
		}
	}
	if v := headers.Get("anthropic-ratelimit-unified-5h-reset"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			s.FiveHourReset = secs
		}
	}
	if v := headers.Get("anthropic-ratelimit-unified-7d-utilization"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			s.SevenDayUtil = f
		}
	}
	if v := headers.Get("anthropic-ratelimit-unified-7d-reset"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			s.SevenDayReset = secs
		}
	}
	return s
}

func parseClaudeUsage(data string) *Usage {
	var wrapper struct {
		Usage *struct {
			InputTokens       int `json:"input_tokens"`
			OutputTokens      int `json:"output_tokens"`
			CacheReadTokens   int `json:"cache_read_input_tokens"`
			CacheCreateTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(data), &wrapper) == nil && wrapper.Usage != nil {
		return &Usage{
			InputTokens:       wrapper.Usage.InputTokens,
			OutputTokens:      wrapper.Usage.OutputTokens,
			CacheReadTokens:   wrapper.Usage.CacheReadTokens,
			CacheCreateTokens: wrapper.Usage.CacheCreateTokens,
		}
	}
	return nil
}

func sanitizeClaudeError(statusCode int, body []byte) (int, []byte) {
	var parsed struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error.Type != "" {
		return statusCode, buildClaudeErrorJSON(parsed.Error.Type, parsed.Error.Message)
	}
	return statusCode, buildClaudeErrorJSON("api_error", "unexpected upstream error")
}

func sanitizeClaudeErrorJSON(statusCode int, body []byte) []byte {
	_, sanitized := sanitizeClaudeError(statusCode, body)
	return sanitized
}

func buildClaudeErrorJSON(errType, msg string) []byte {
	resp := struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}{
		Type: "error",
		Error: struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}{Type: errType, Message: msg},
	}
	data, _ := json.Marshal(resp)
	return data
}

func isOpusModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "opus")
}
