package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (d *ClaudeDriver) Probe(ctx context.Context, acct *domain.Account, token string, client *http.Client, model string) (ProbeResult, error) {
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	if _, err := normalizeClaudeModelID(model); err != nil {
		return ProbeResult{}, err
	}
	entry, ok := claudeModelEntryForID(model)
	if !ok {
		return ProbeResult{}, NewRequestValidationError(400, "unknown probe model")
	}
	model = entry.UpstreamID
	payload, _ := json.Marshal(map[string]any{"model": model, "max_tokens": 1, "messages": []map[string]string{{"role": "user", "content": "hi"}}})
	// Probes must use the same OAuth envelope and beta URL as real inference;
	// a simplified request can receive a generic rejection without quota headers.
	req, err := d.BuildRequest(ctx, &RelayInput{
		RawBody: payload,
		Headers: make(http.Header),
		Model:   model,
		UserID:  "admin-probe",
	}, acct, token)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("build probe request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ProbeResult{}, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ProbeResult{}, err
	}

	result := ProbeResult{
		Effect:  d.Interpret(resp.StatusCode, resp.Header, bodyBytes, model, json.RawMessage(acct.ProviderStateJSON)),
		Observe: true,
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return result, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	result.ClearCooldown = !isFableModel(model)
	return result, nil
}

func (d *ClaudeDriver) DescribeAccount(acct *domain.Account) []AccountField {
	if acct == nil || acct.Identity == nil {
		return nil
	}
	if orgName := acct.Identity["orgName"]; orgName != "" {
		return []AccountField{{Label: "organization", Value: orgName}}
	}
	return nil
}

func (d *ClaudeDriver) AutoPriority(state json.RawMessage) int {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return 50
	}
	return claudePriority(s, "", time.Now())
}

func claudePriority(s ClaudeState, model string, now time.Time) int {
	priority := 100
	keys := []string{"5h", "7d"}
	if isFableModel(model) {
		keys = append(keys, "7d_oi")
	}
	for _, key := range keys {
		q := s.quota(key)
		if q.Util == nil || (quotaReset(q) > 0 && quotaReset(q) <= now.Unix()) {
			continue
		}
		remain := int((1 - *q.Util) * 100)
		if remain < priority {
			priority = remain
		}
	}
	return max(0, priority)
}

func (d *ClaudeDriver) IsStale(state json.RawMessage, now time.Time) bool {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return false
	}
	// The default health probe cannot refresh a model-specific quota. Fable
	// expiration is checked at scheduling time, avoiding perpetual Haiku probes.
	for _, key := range []string{"5h", "7d"} {
		q := s.quota(key)
		if (quotaReset(q) > 0 && quotaReset(q) <= now.Unix()) || (q.Util != nil && *q.Util > 0 && quotaReset(q) == 0) {
			return true
		}
	}
	return false
}

func (d *ClaudeDriver) ComputeExhaustedCooldown(state json.RawMessage, now time.Time) time.Time {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return time.Time{}
	}
	var until int64
	for _, key := range []string{"5h", "7d"} {
		q := s.quota(key)
		if quotaBlocked(q, overageAllowed(s), now) && quotaReset(q) > until {
			until = quotaReset(q)
		}
	}
	if until > now.Unix() {
		return time.Unix(until, 0).UTC()
	}
	return time.Time{}
}

func (d *ClaudeDriver) CanServe(state json.RawMessage, model string, now time.Time) bool {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return true
	}
	if isFableModel(model) {
		return !quotaBlocked(s.quota("7d_oi"), overageAllowed(s), now)
	}
	return !isOpusModel(model) || s.OpusCooldownUntil <= now.Unix()
}

func (d *ClaudeDriver) AssessCapacity(state json.RawMessage, model string, now time.Time) CapacityAssessment {
	class := "default"
	if isOpusModel(model) {
		class = "opus"
	}
	if isFableModel(model) {
		class = "fable"
	}
	var s ClaudeState
	_ = json.Unmarshal(state, &s)
	return CapacityAssessment{Eligible: d.CanServe(state, model, now), Priority: claudePriority(s, model, now), Class: class}
}

func (d *ClaudeDriver) CalcCost(model string, usage *Usage) float64 {
	if usage == nil {
		return 0
	}
	pricing := claudeSonnetPricing
	if entry, ok := claudeModelEntryForID(model); ok {
		pricing = entry.Pricing
	}
	return pricing.cost(usage)
}

func (d *ClaudeDriver) GetUtilization(state json.RawMessage) []UtilWindow {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return nil
	}
	now := time.Now()
	windows := make([]UtilWindow, 0, 4)
	for _, entry := range []struct{ key, label, scope string }{{"5h", "5h", "account"}, {"7d", "7d", "account"}, {"7d_oi", "Fable 7d", "model"}, {"overage", "usage credits", "billing"}} {
		q := s.quota(entry.key)
		if q.Util == nil && quotaReset(q) == 0 && q.Status == "" && q.ObservedAt == 0 {
			continue
		}
		status := q.Status
		if status == "" {
			status = "unknown"
		}
		if (quotaReset(q) > 0 && quotaReset(q) <= now.Unix()) || (q.RetryUntil > 0 && q.RetryUntil <= now.Unix()) {
			status = "stale"
		}
		reason := q.Reason
		if entry.key == "overage" && reason == "out_of_credits" {
			reason = "usage credits exhausted"
		}
		if entry.key == "7d_oi" && quotaBlocked(q, overageAllowed(s), now) {
			status = "rejected"
			reason = "Fable only: weekly quota exhausted"
		}
		if entry.key == "7d_oi" && q.Status == "rejected" && !quotaBlocked(q, overageAllowed(s), now) && overageAllowed(s) && status != "stale" {
			status = "usage_credits"
			reason = "subscription limit reached; usage credits available"
		}
		w := UtilWindow{Key: entry.key, Label: entry.label, Scope: entry.scope, Reset: quotaReset(q), Status: status, Reason: reason, ObservedAt: q.ObservedAt / int64(time.Second), Unknown: q.Util == nil || status == "stale"}
		if q.Util != nil {
			raw := *q.Util * 100
			w.RawPct = &raw
			w.Pct = int(math.Max(0, math.Min(100, raw)))
		}
		windows = append(windows, w)
	}
	return windows
}
