package driver

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ClaudeQuota is an observed window, not a second schedulability flag.
// Nil values distinguish missing fields from an explicit zero.
type ClaudeQuota struct {
	Util       *float64 `json:"util,omitempty"`
	Reset      *int64   `json:"reset,omitempty"`
	Status     string   `json:"status,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	ObservedAt int64    `json:"observed_at,omitempty"` // local response observation, Unix nanoseconds
	RetryUntil int64    `json:"retry_until,omitempty"` // explicit rejection delay, Unix seconds
}

type ClaudeQuotaMeta struct {
	Status     string `json:"status,omitempty"`
	ObservedAt int64  `json:"observed_at,omitempty"`
}

// Public utilization fields retain their existing storage names. Metadata and
// model-specific windows are additive; no startup schema/data rewrite is needed.
type ClaudeState struct {
	ObservedAt int64 `json:"observed_at,omitempty"`

	FiveHourUtil      *float64        `json:"five_hour_util,omitempty"`
	FiveHourReset     int64           `json:"five_hour_reset,omitempty"`
	SevenDayUtil      *float64        `json:"seven_day_util,omitempty"`
	SevenDayReset     int64           `json:"seven_day_reset,omitempty"`
	FiveHourMeta      ClaudeQuotaMeta `json:"five_hour_meta,omitempty"`
	SevenDayMeta      ClaudeQuotaMeta `json:"seven_day_meta,omitempty"`
	Fable             *ClaudeQuota    `json:"fable,omitempty"`
	Overage           *ClaudeQuota    `json:"overage,omitempty"`
	OpusCooldownUntil int64           `json:"opus_cooldown_until,omitempty"`
}

type claudeObservation struct {
	ObservedAt int64                  `json:"observed_at"`
	Windows    map[string]ClaudeQuota `json:"windows"`
}

func captureClaudeObservation(headers http.Header, now time.Time) claudeObservation {
	o := claudeObservation{ObservedAt: now.UnixNano(), Windows: map[string]ClaudeQuota{}}
	for _, key := range []string{"5h", "7d", "7d_oi", "overage"} {
		prefix := "anthropic-ratelimit-unified-" + key + "-"
		q := ClaudeQuota{}
		if v := headers.Get(prefix + "utilization"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) && f >= 0 && f <= math.MaxFloat64/100 {
				q.Util = &f
			}
		}
		if t := parseClaudeReset(headers.Get(prefix + "reset")); !t.IsZero() {
			n := t.Unix()
			q.Reset = &n
		}
		switch v := strings.ToLower(headers.Get(prefix + "status")); v {
		case "allowed", "allowed_warning", "rejected":
			q.Status = v
		}
		q.Reason = truncate(headers.Get(prefix+"disabled-reason"), 120)
		if q.Util != nil || q.Reset != nil || q.Status != "" || q.Reason != "" {
			q.ObservedAt = now.UnixNano()
			o.Windows[key] = q
		}
	}
	return o
}

func parseClaudeReset(value string) time.Time {
	value = strings.TrimSpace(value)
	if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
		return time.Unix(n, 0).UTC()
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Time{}
}

func (d *ClaudeDriver) retryUntil(headers http.Header, now time.Time) time.Time {
	if delay := parseRetryAfterAt(headers.Get("Retry-After"), now); delay > 0 {
		return now.Add(delay)
	}
	if t := parseClaudeReset(headers.Get("anthropic-ratelimit-unified-reset")); t.After(now) {
		return t
	}
	pause := d.cfg.Pauses.Pause429
	if pause <= 0 {
		pause = time.Minute
	}
	return now.Add(pause)
}

func (s ClaudeState) quota(key string) ClaudeQuota {
	switch key {
	case "5h":
		return ClaudeQuota{Util: s.FiveHourUtil, Reset: &s.FiveHourReset, Status: s.FiveHourMeta.Status, ObservedAt: s.FiveHourMeta.ObservedAt}
	case "7d":
		return ClaudeQuota{Util: s.SevenDayUtil, Reset: &s.SevenDayReset, Status: s.SevenDayMeta.Status, ObservedAt: s.SevenDayMeta.ObservedAt}
	case "7d_oi":
		if s.Fable != nil {
			return *s.Fable
		}
	case "overage":
		if s.Overage != nil {
			return *s.Overage
		}
	}
	return ClaudeQuota{}
}

// MergeState runs against the latest bucket state in Pool.Observe. Interpret
// emits only response observations, never a stale copied account snapshot.
func (d *ClaudeDriver) MergeState(current, observation json.RawMessage) json.RawMessage {
	var s ClaudeState
	var o claudeObservation
	if err := json.Unmarshal(current, &s); err != nil {
		slog.Error("Claude quota state merge failed", "op", "decode_current", "error", err)
		return current
	}
	if err := json.Unmarshal(observation, &o); err != nil {
		slog.Error("Claude quota state merge failed", "op", "decode_observation", "error", err)
		return current
	}
	if o.ObservedAt > s.ObservedAt {
		s.ObservedAt = o.ObservedAt
	}
	for key, update := range o.Windows {
		q := s.quota(key)
		if update.ObservedAt <= q.ObservedAt {
			continue
		}
		// A new cycle cannot inherit utilization/status from an expired cycle.
		if reset := quotaReset(q); reset > 0 && reset <= update.ObservedAt/int64(time.Second) && update.Reset != nil && *update.Reset > reset {
			q = ClaudeQuota{RetryUntil: q.RetryUntil}
		}
		if update.Util != nil {
			q.Util = update.Util
		}
		if update.Reset != nil {
			q.Reset = update.Reset
		}
		if update.Status != "" {
			q.Status = update.Status
			if update.Status == "allowed" || update.Status == "allowed_warning" {
				q.Reason = ""
			}
		}
		if update.Reason != "" {
			q.Reason = update.Reason
		}
		if update.RetryUntil > 0 {
			q.RetryUntil = update.RetryUntil
		} else if update.Status == "allowed" || update.Status == "allowed_warning" {
			q.RetryUntil = 0
		}
		q.ObservedAt = update.ObservedAt
		switch key {
		case "5h":
			s.FiveHourUtil = q.Util
			if q.Reset != nil {
				s.FiveHourReset = *q.Reset
			}
			s.FiveHourMeta = ClaudeQuotaMeta{Status: q.Status, ObservedAt: q.ObservedAt}
		case "7d":
			s.SevenDayUtil = q.Util
			if q.Reset != nil {
				s.SevenDayReset = *q.Reset
			}
			s.SevenDayMeta = ClaudeQuotaMeta{Status: q.Status, ObservedAt: q.ObservedAt}
		case "7d_oi":
			s.Fable = &q
		case "overage":
			s.Overage = &q
		}
	}
	return mustMarshalJSON(s)
}

func quotaReset(q ClaudeQuota) int64 {
	if q.Reset != nil {
		return *q.Reset
	}
	return 0
}
func overageAllowed(s ClaudeState) bool {
	q := s.quota("overage")
	return q.Status == "allowed" || q.Status == "allowed_warning"
}
func quotaBlocked(q ClaudeQuota, paid bool, now time.Time) bool {
	if q.RetryUntil > 0 {
		return q.RetryUntil > now.Unix()
	}
	return q.Status == "rejected" && quotaReset(q) > now.Unix() && !paid
}
func isFableModel(model string) bool {
	entry, ok := claudeModelEntryForID(model)
	return ok && strings.HasPrefix(entry.UpstreamID, "claude-fable-")
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
	errType, message := parseClaudeErrorInfo(body)
	if errType != "" {
		return statusCode, buildClaudeErrorJSON(errType, message)
	}
	return statusCode, buildClaudeErrorJSON("api_error", "unexpected upstream error")
}

func parseClaudeErrorInfo(body []byte) (string, string) {
	var parsed struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return "", ""
	}
	return parsed.Error.Type, parsed.Error.Message
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
