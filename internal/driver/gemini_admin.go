package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (d *GeminiDriver) Probe(ctx context.Context, acct *domain.Account, token string, client *http.Client) (ProbeResult, error) {
	state := parseGeminiState(json.RawMessage(acct.ProviderStateJSON))
	now := time.Now()

	if state.ProjectID == "" {
		info, err := loadGeminiCodeAssist(ctx, client, d.cfg.APIURL, token)
		if err != nil {
			return ProbeResult{}, err
		}
		state.ProjectID = info.ProjectID
		state.LastLoadAt = now.Unix()
	}

	if quota, err := retrieveGeminiUserQuota(ctx, client, d.cfg.APIURL, token); err == nil {
		state.applyQuota(quota, now)
	}

	result := ProbeResult{
		Effect: Effect{
			Kind:         EffectSuccess,
			Scope:        EffectScopeBucket,
			UpdatedState: mustMarshalJSON(state),
		},
		Observe:       true,
		ClearCooldown: true,
	}
	if state.ProjectID == "" {
		return result, fmt.Errorf("loadCodeAssist returned empty project id")
	}
	return result, nil
}

func (d *GeminiDriver) DescribeAccount(acct *domain.Account) []AccountField {
	state := parseGeminiState(json.RawMessage(acct.ProviderStateJSON))
	if state.ProjectID == "" {
		return nil
	}
	return []AccountField{{Label: "project", Value: state.ProjectID}}
}

func (d *GeminiDriver) AutoPriority(state json.RawMessage) int {
	s := parseGeminiState(state)
	if s.QuotaUpdatedAt > 0 || s.DailyRequestsResetAt > 0 {
		return int(clampFraction(s.DailyRequestsRemainingFraction) * 100)
	}
	return 50
}

func (d *GeminiDriver) IsStale(state json.RawMessage, now time.Time) bool {
	s := parseGeminiState(state)
	if s.ProjectID == "" {
		return true
	}
	if s.QuotaUpdatedAt == 0 || s.DailyRequestsResetAt == 0 {
		return true
	}
	nowUnix := now.Unix()
	return nowUnix >= s.QuotaUpdatedAt+int64(geminiQuotaRefreshInterval/time.Second) ||
		(s.DailyRequestsResetAt > 0 && s.DailyRequestsResetAt <= nowUnix)
}

func (d *GeminiDriver) ComputeExhaustedCooldown(state json.RawMessage, now time.Time) time.Time {
	s := parseGeminiState(state)
	if s.DailyRequestsResetAt > now.Unix() && clampFraction(s.DailyRequestsRemainingFraction) <= 0 {
		return time.Unix(s.DailyRequestsResetAt, 0).UTC()
	}
	return time.Time{}
}

func (d *GeminiDriver) CanServe(state json.RawMessage, _ string, now time.Time) bool {
	s := parseGeminiState(state)
	if s.ProjectID == "" {
		return false
	}
	return s.DailyRequestsResetAt == 0 ||
		s.DailyRequestsResetAt <= now.Unix() ||
		clampFraction(s.DailyRequestsRemainingFraction) > 0
}

func (d *GeminiDriver) CalcCost(_ string, _ *Usage) float64 { return 0 }

func (d *GeminiDriver) GetUtilization(state json.RawMessage) []UtilWindow {
	s := parseGeminiState(state)
	if s.DailyRequestsResetAt == 0 && s.DailyRequestsRemainingFraction == 0 {
		return nil
	}
	return []UtilWindow{{
		Label: "daily",
		Pct:   int((1 - clampFraction(s.DailyRequestsRemainingFraction)) * 100),
		Reset: s.DailyRequestsResetAt,
	}}
}
