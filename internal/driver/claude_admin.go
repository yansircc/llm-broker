package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/identity"
)

func (d *ClaudeDriver) Probe(ctx context.Context, acct *domain.Account, token string, client *http.Client) (ProbeResult, error) {
	body := `{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequestWithContext(ctx, "POST", d.cfg.APIURL, strings.NewReader(body))
	if err != nil {
		return ProbeResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	identity.SetRequiredHeaders(req.Header, token, d.cfg.APIVersion, d.cfg.BetaHeader)
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
		Effect:  d.Interpret(resp.StatusCode, resp.Header, bodyBytes, "", json.RawMessage(acct.ProviderStateJSON)),
		Observe: true,
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return result, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	result.ClearCooldown = true
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
	fiveRemain := 100.0
	if s.FiveHourUtil > 0 {
		fiveRemain = (1.0 - s.FiveHourUtil) * 100
	}
	sevenRemain := 100.0
	if s.SevenDayUtil > 0 {
		sevenRemain = (1.0 - s.SevenDayUtil) * 100
	}
	pri := fiveRemain
	if sevenRemain < pri {
		pri = sevenRemain
	}
	return int(pri)
}

func (d *ClaudeDriver) IsStale(state json.RawMessage, now time.Time) bool {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return false
	}
	nowUnix := now.Unix()
	return (s.FiveHourReset > 0 && s.FiveHourReset < nowUnix) ||
		(s.SevenDayReset > 0 && s.SevenDayReset < nowUnix) ||
		(s.FiveHourUtil > 0 && s.FiveHourReset == 0) ||
		(s.SevenDayUtil > 0 && s.SevenDayReset == 0)
}

func (d *ClaudeDriver) ComputeExhaustedCooldown(state json.RawMessage, now time.Time) time.Time {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return time.Time{}
	}
	nowUnix := now.Unix()
	var cooldownUntil int64
	if s.FiveHourUtil >= 0.99 && s.FiveHourReset > nowUnix {
		cooldownUntil = s.FiveHourReset
	}
	if s.SevenDayUtil >= 0.99 && s.SevenDayReset > nowUnix && s.SevenDayReset > cooldownUntil {
		cooldownUntil = s.SevenDayReset
	}
	if cooldownUntil > 0 {
		return time.Unix(cooldownUntil, 0).UTC()
	}
	return time.Time{}
}

func (d *ClaudeDriver) CanServe(state json.RawMessage, model string, now time.Time) bool {
	if !isOpusModel(model) {
		return true
	}
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return true
	}
	return s.OpusCooldownUntil == 0 || now.Unix() >= s.OpusCooldownUntil
}

func (d *ClaudeDriver) CalcCost(model string, usage *Usage) float64 {
	if usage == nil {
		return 0
	}
	lower := strings.ToLower(model)
	var inPrice, outPrice, cacheReadPrice, cacheCreatePrice float64
	switch {
	case strings.Contains(lower, "opus"):
		inPrice, outPrice, cacheReadPrice, cacheCreatePrice = 5, 25, 0.50, 6.25
	case strings.Contains(lower, "haiku"):
		inPrice, outPrice, cacheReadPrice, cacheCreatePrice = 1, 5, 0.10, 1.25
	default:
		inPrice, outPrice, cacheReadPrice, cacheCreatePrice = 3, 15, 0.30, 3.75
	}
	return (float64(usage.InputTokens)*inPrice + float64(usage.OutputTokens)*outPrice +
		float64(usage.CacheReadTokens)*cacheReadPrice + float64(usage.CacheCreateTokens)*cacheCreatePrice) / 1_000_000
}

func (d *ClaudeDriver) GetUtilization(state json.RawMessage) []UtilWindow {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return nil
	}
	var windows []UtilWindow
	if s.FiveHourUtil > 0 || s.FiveHourReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "5h",
			Pct:   int(s.FiveHourUtil * 100),
			Reset: s.FiveHourReset,
		})
	}
	if s.SevenDayUtil > 0 || s.SevenDayReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "7d",
			Pct:   int(s.SevenDayUtil * 100),
			Reset: s.SevenDayReset,
		})
	}
	return windows
}
