package driver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (d *CodexDriver) Probe(ctx context.Context, acct *domain.Account, token string, client *http.Client) (ProbeResult, error) {
	if acct.Subject == "" {
		return ProbeResult{}, fmt.Errorf("codex account missing subject")
	}

	body := `{"model":"gpt-5.1-codex","messages":[{"role":"user","content":"Reply with OK only."}],"stream":true}`
	req, err := http.NewRequestWithContext(ctx, "POST", d.cfg.APIURL, strings.NewReader(body))
	if err != nil {
		return ProbeResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Host", "chatgpt.com")
	req.Header.Set("Chatgpt-Account-Id", acct.Subject)

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

	scanner := bufio.NewScanner(bytes.NewReader(bodyBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: response.output_text.delta") {
			result.ClearCooldown = true
			return result, nil
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"error":{`) {
			return result, fmt.Errorf("upstream error in stream")
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	return result, fmt.Errorf("stream ended without output")
}

func (d *CodexDriver) DescribeAccount(acct *domain.Account) []AccountField {
	if acct == nil || acct.Identity == nil {
		return nil
	}
	if orgTitle := acct.Identity["orgTitle"]; orgTitle != "" {
		return []AccountField{{Label: "organization", Value: orgTitle}}
	}
	return nil
}

func (d *CodexDriver) AutoPriority(state json.RawMessage) int {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return 50
	}
	primaryRemain := 100.0
	if s.PrimaryUtil > 0 {
		primaryRemain = (1.0 - s.PrimaryUtil) * 100
	}
	secondaryRemain := 100.0
	if s.SecondaryUtil > 0 {
		secondaryRemain = (1.0 - s.SecondaryUtil) * 100
	}
	pri := primaryRemain
	if secondaryRemain < pri {
		pri = secondaryRemain
	}
	return int(pri)
}

func (d *CodexDriver) IsStale(state json.RawMessage, now time.Time) bool {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return false
	}
	nowUnix := now.Unix()
	return (s.PrimaryReset > 0 && s.PrimaryReset < nowUnix) ||
		(s.SecondaryReset > 0 && s.SecondaryReset < nowUnix) ||
		(s.PrimaryUtil > 0 && s.PrimaryReset == 0) ||
		(s.SecondaryUtil > 0 && s.SecondaryReset == 0)
}

func (d *CodexDriver) ComputeExhaustedCooldown(state json.RawMessage, now time.Time) time.Time {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return time.Time{}
	}
	nowUnix := now.Unix()
	var cooldownUntil int64
	if s.PrimaryUtil >= 0.99 && s.PrimaryReset > nowUnix {
		cooldownUntil = s.PrimaryReset
	}
	if s.SecondaryUtil >= 0.99 && s.SecondaryReset > nowUnix && s.SecondaryReset > cooldownUntil {
		cooldownUntil = s.SecondaryReset
	}
	if cooldownUntil > 0 {
		return time.Unix(cooldownUntil, 0).UTC()
	}
	return time.Time{}
}

func (d *CodexDriver) CalcCost(model string, usage *Usage) float64 {
	if usage == nil {
		return 0
	}
	lower := strings.ToLower(model)
	var inPrice, outPrice, cacheReadPrice float64
	switch {
	case strings.Contains(lower, "o3"):
		inPrice, outPrice, cacheReadPrice = 2, 8, 0.50
	case strings.Contains(lower, "o4-mini"):
		inPrice, outPrice, cacheReadPrice = 1.10, 4.40, 0.275
	case strings.Contains(lower, "codex-mini"):
		inPrice, outPrice, cacheReadPrice = 1.50, 6, 0.375
	case strings.Contains(lower, "4.1-nano"):
		inPrice, outPrice, cacheReadPrice = 0.10, 0.40, 0.025
	case strings.Contains(lower, "4.1-mini"):
		inPrice, outPrice, cacheReadPrice = 0.40, 1.60, 0.10
	case strings.Contains(lower, "4.1"):
		inPrice, outPrice, cacheReadPrice = 2, 8, 0.50
	default:
		inPrice, outPrice, cacheReadPrice = 2, 8, 0.50
	}
	return (float64(usage.InputTokens)*inPrice + float64(usage.OutputTokens)*outPrice +
		float64(usage.CacheReadTokens)*cacheReadPrice) / 1_000_000
}

func (d *CodexDriver) GetUtilization(state json.RawMessage) []UtilWindow {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return nil
	}
	var windows []UtilWindow
	if s.PrimaryUtil > 0 || s.PrimaryReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "primary",
			Pct:   int(s.PrimaryUtil * 100),
			Reset: s.PrimaryReset,
		})
	}
	if s.SecondaryUtil > 0 || s.SecondaryReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "secondary",
			Pct:   int(s.SecondaryUtil * 100),
			Reset: s.SecondaryReset,
		})
	}
	return windows
}

func (d *CodexDriver) CanServe(_ json.RawMessage, _ string, _ time.Time) bool {
	return true
}
