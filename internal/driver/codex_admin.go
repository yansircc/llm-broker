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

	body := `{"model":"gpt-5.1-codex","instructions":"Reply with OK only.","input":[{"role":"user","content":"test"}],"stream":true,"store":false}`
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
	// Return the best remaining capacity across all families.
	best := 0
	for _, prefix := range s.allFamilies() {
		f := s.family(prefix)
		primaryRemain := 100.0
		if f.PrimaryUtil > 0 {
			primaryRemain = (1.0 - f.PrimaryUtil) * 100
		}
		secondaryRemain := 100.0
		if f.SecondaryUtil > 0 {
			secondaryRemain = (1.0 - f.SecondaryUtil) * 100
		}
		worst := primaryRemain
		if secondaryRemain < worst {
			worst = secondaryRemain
		}
		if int(worst) > best {
			best = int(worst)
		}
	}
	if best == 0 && len(s.allFamilies()) == 0 {
		return 50
	}
	return best
}

func (d *CodexDriver) IsStale(state json.RawMessage, now time.Time) bool {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return false
	}
	nowUnix := now.Unix()
	for _, prefix := range s.allFamilies() {
		f := s.family(prefix)
		if (f.PrimaryReset > 0 && f.PrimaryReset < nowUnix) ||
			(f.SecondaryReset > 0 && f.SecondaryReset < nowUnix) ||
			(f.PrimaryUtil > 0 && f.PrimaryReset == 0) ||
			(f.SecondaryUtil > 0 && f.SecondaryReset == 0) {
			return true
		}
	}
	return false
}

// computeFamilyCooldown returns a cooldown time for a single family,
// or zero if the family is not exhausted.
func computeFamilyCooldown(f CodexFamilyLimits, nowUnix int64) int64 {
	var cd int64
	if f.PrimaryUtil >= 0.99 && f.PrimaryReset > nowUnix {
		cd = f.PrimaryReset
	}
	if f.SecondaryUtil >= 0.99 && f.SecondaryReset > nowUnix && f.SecondaryReset > cd {
		cd = f.SecondaryReset
	}
	return cd
}

func (d *CodexDriver) ComputeExhaustedCooldown(state json.RawMessage, now time.Time) time.Time {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return time.Time{}
	}
	families := s.allFamilies()
	if len(families) == 0 {
		return time.Time{}
	}
	nowUnix := now.Unix()

	// Only set bucket-level cooldown if ALL families are exhausted.
	var earliest int64
	for _, prefix := range families {
		f := s.family(prefix)
		cd := computeFamilyCooldown(f, nowUnix)
		if cd == 0 {
			return time.Time{} // this family still has capacity
		}
		if earliest == 0 || cd < earliest {
			earliest = cd
		}
	}
	if earliest > 0 {
		return time.Unix(earliest, 0).UTC()
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
	families := s.allFamilies()
	if len(families) == 0 {
		return nil
	}

	// Single family: return flat windows as before.
	if len(families) == 1 {
		f := s.family(families[0])
		var windows []UtilWindow
		if f.PrimaryUtil > 0 || f.PrimaryReset > 0 {
			windows = append(windows, UtilWindow{
				Label: "primary", Pct: int(f.PrimaryUtil * 100), Reset: f.PrimaryReset, SubPct: -1,
			})
		}
		if f.SecondaryUtil > 0 || f.SecondaryReset > 0 {
			windows = append(windows, UtilWindow{
				Label: "secondary", Pct: int(f.SecondaryUtil * 100), Reset: f.SecondaryReset, SubPct: -1,
			})
		}
		return windows
	}

	// Multiple families: merge into primary/secondary with sub-family data.
	// Standard ("") is the main value; first non-standard family is the sub.
	std := s.family("")
	var subPrefix string
	for _, p := range families {
		if p != "" {
			subPrefix = p
			break
		}
	}
	sub := s.family(subPrefix)
	subName := codexFamilyDisplayName(subPrefix, sub)
	if subName == "" {
		subName = "spark"
	}

	var windows []UtilWindow
	if std.PrimaryUtil > 0 || std.PrimaryReset > 0 || sub.PrimaryUtil > 0 || sub.PrimaryReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "primary", Pct: int(std.PrimaryUtil * 100), Reset: std.PrimaryReset,
			SubLabel: subName, SubPct: int(sub.PrimaryUtil * 100), SubReset: sub.PrimaryReset,
		})
	}
	if std.SecondaryUtil > 0 || std.SecondaryReset > 0 || sub.SecondaryUtil > 0 || sub.SecondaryReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "secondary", Pct: int(std.SecondaryUtil * 100), Reset: std.SecondaryReset,
			SubLabel: subName, SubPct: int(sub.SecondaryUtil * 100), SubReset: sub.SecondaryReset,
		})
	}
	return windows
}

func (d *CodexDriver) CanServe(state json.RawMessage, model string, now time.Time) bool {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return true
	}
	family := codexModelFamily(model)
	f := s.family(family)
	nowUnix := now.Unix()
	if f.PrimaryUtil >= 0.99 && f.PrimaryReset > nowUnix {
		return false
	}
	if f.SecondaryUtil >= 0.99 && f.SecondaryReset > nowUnix {
		return false
	}
	return true
}
