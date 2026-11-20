package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func productionFableHeaders(t *testing.T) http.Header {
	t.Helper()
	raw, err := os.ReadFile("testdata/claude-fable-quota-429.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Headers map[string]string }
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	for k, v := range fixture.Headers {
		h.Set(k, v)
	}
	// Keep recorded ratios/statuses; shift resets so this regression never ages out.
	reset := fmt.Sprint(time.Now().Add(6 * 24 * time.Hour).Unix())
	for _, key := range []string{"7d-reset", "7d_oi-reset", "reset"} {
		h.Set("anthropic-ratelimit-unified-"+key, reset)
	}
	return h
}

func TestClaudeQuotaProductionRejectionIsModelScoped(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)
	h := productionFableHeaders(t)
	e := d.Interpret(429, h, []byte(`{"error":{"type":"rate_limit_error"}}`), "claude-fable-5-2", nil)
	if e.Kind != EffectCooldown || !e.CooldownUntil.IsZero() || e.QuotaScope != "model" || e.QuotaKey != "7d_oi" {
		t.Fatalf("wrong scope: %+v", e)
	}
	if delay := time.Until(e.RetryAt); delay < 483565*time.Second || delay > 483567*time.Second {
		t.Fatalf("retry delay %s", delay)
	}
	state := d.MergeState(json.RawMessage(`{}`), e.UpdatedState)
	now := time.Now()
	for _, model := range []string{"claude-fable-5", "claude-fable-5-1", "claude-fable-5-2"} {
		if d.CanServe(state, model, now) {
			t.Errorf("Fable allowed: %s", model)
		}
	}
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"} {
		if !d.CanServe(state, model, now) {
			t.Errorf("ordinary model blocked: %s", model)
		}
	}
	if !d.ComputeExhaustedCooldown(state, now).IsZero() {
		t.Fatal("Fable enforced a shared cooldown")
	}
	if !d.CanServe(state, "claude-fable-5-2", e.RetryAt.Add(time.Second)) {
		t.Fatal("expired Fable rejection still blocks")
	}
	if d.AssessCapacity(state, "claude-opus-5-5", now).Priority < 33 {
		t.Fatal("Fable depleted ordinary model priority")
	}
	windows := d.GetUtilization(state)
	if len(windows) != 3 || windows[0].Pct != 66 || windows[1].Pct != 100 || windows[1].RawPct == nil || *windows[1].RawPct != 101 || !windows[2].Unknown {
		t.Fatalf("wrong projections: %+v", windows)
	}
}

func TestClaudeQuotaScopeEvidence(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)
	for _, tc := range []struct{ name, key, value string }{
		{"shared five hour", "5h-status", "rejected"},
		{"shared weekly", "7d-status", "rejected"},
		{"unknown representative", "representative-claim", "new_unknown_limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := productionFableHeaders(t)
			h.Set("anthropic-ratelimit-unified-"+tc.key, tc.value)
			e := d.Interpret(429, h, nil, "claude-fable-5-2", nil)
			if e.CooldownUntil.IsZero() || e.QuotaScope != "account" {
				t.Fatalf("lost account rejection: %+v", e)
			}
		})
	}
}

func TestClaudeQuotaSparseMergingAndOrdering(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)
	t0 := time.Now()
	h := productionFableHeaders(t)
	original := captureClaudeObservation(h, t0)
	state := d.MergeState(json.RawMessage(`{"five_hour_util":0.4,"five_hour_reset":9999999999}`), mustMarshalJSON(original))
	empty := captureClaudeObservation(nil, t0.Add(time.Second))
	state = d.MergeState(state, mustMarshalJSON(empty))
	var s ClaudeState
	_ = json.Unmarshal(state, &s)
	if s.FiveHourUtil == nil || *s.FiveHourUtil != 0.4 || s.Fable == nil || *s.Fable.Util != 1.01 {
		t.Fatalf("missing field erased state: %s", state)
	}
	zero := http.Header{}
	zero.Set("anthropic-ratelimit-unified-7d_oi-utilization", "0")
	zero.Set("anthropic-ratelimit-unified-7d_oi-status", "allowed")
	fresh := captureClaudeObservation(zero, t0.Add(2*time.Second))
	state = d.MergeState(state, mustMarshalJSON(fresh))
	state = d.MergeState(state, mustMarshalJSON(original)) // late old observation must not roll back Fable
	_ = json.Unmarshal(state, &s)
	if *s.Fable.Util != 0 || s.Fable.Status != "allowed" || *s.FiveHourUtil != 0.4 {
		t.Fatalf("wrong merge: %s", state)
	}
	invalid := http.Header{}
	invalid.Set("anthropic-ratelimit-unified-7d-utilization", "NaN")
	invalid.Set("anthropic-ratelimit-unified-7d-reset", "nonsense")
	state = d.MergeState(state, mustMarshalJSON(captureClaudeObservation(invalid, t0.Add(3*time.Second))))
	_ = json.Unmarshal(state, &s)
	if s.SevenDayUtil == nil || *s.SevenDayUtil != 0.66 {
		t.Fatal("invalid data erased known value")
	}
}

func TestClaudeQuotaPaidAndExpiredProjection(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)
	h := productionFableHeaders(t)
	h.Set("anthropic-ratelimit-unified-overage-status", "allowed")
	h.Del("anthropic-ratelimit-unified-overage-disabled-reason")
	e := d.Interpret(200, h, nil, "claude-fable-5-2", nil)
	state := d.MergeState(json.RawMessage(`{}`), e.UpdatedState)
	if !d.CanServe(state, "claude-fable-5-2", time.Now()) {
		t.Fatal("successful paid usage blocked")
	}
	// A real rejection still overrides a contradictory paid-allowed header.
	e = d.Interpret(429, h, nil, "claude-fable-5-2", nil)
	state = d.MergeState(state, e.UpdatedState)
	if d.CanServe(state, "claude-fable-5-2", time.Now()) {
		t.Fatal("actual rejection ignored")
	}
	past := time.Now().Add(-time.Hour).Unix()
	raw := json.RawMessage(fmt.Sprintf(`{"seven_day_util":0.66,"seven_day_reset":%d}`, past))
	windows := d.GetUtilization(raw)
	if len(windows) != 1 || !windows[0].Unknown || windows[0].Status != "stale" {
		t.Fatalf("expired data presented as live: %+v", windows)
	}
}

func TestClaudeRetryTimeFormats(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)
	now := time.Now().UTC().Truncate(time.Second)
	want := now.Add(2 * time.Hour)
	for _, tc := range []struct{ key, value string }{
		{"Retry-After", "7200"}, {"Retry-After", want.Format(http.TimeFormat)},
		{"anthropic-ratelimit-unified-reset", fmt.Sprint(want.Unix())},
		{"anthropic-ratelimit-unified-reset", want.Format(time.RFC3339)},
	} {
		h := http.Header{}
		h.Set(tc.key, tc.value)
		if got := d.retryUntil(h, now); !got.Equal(want) {
			t.Errorf("%s=%s => %s", tc.key, tc.value, got)
		}
	}
	for _, v := range []string{"bad", "-5", "99999999999999999"} {
		h := http.Header{}
		h.Set("Retry-After", v)
		if got := d.retryUntil(h, now); !got.Equal(now.Add(time.Minute)) {
			t.Errorf("invalid retry %s => %s", v, got)
		}
	}
}

func TestClaudeProbeSelectedModelAndScope(t *testing.T) {
	var requested string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct{ Model string }
		_ = json.NewDecoder(r.Body).Decode(&input)
		requested = input.Model
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"content":[{"type":"text","text":"hi"}]}`)
	}))
	defer up.Close()
	d := NewClaudeDriver(ClaudeConfig{APIURL: up.URL}, NoopStainlessStore{}, 4)
	for _, model := range []string{"", "claude-opus-5-5", "claude-fable-5-2"} {
		result, err := d.Probe(context.Background(), &domain.Account{ProviderStateJSON: `{}`}, "test", up.Client(), model)
		if err != nil {
			t.Fatal(err)
		}
		if model != "" && requested != model {
			t.Fatalf("requested %s", requested)
		}
		if result.ClearCooldown == strings.Contains(model, "fable") {
			t.Fatal("wrong probe cooldown scope")
		}
	}
	if _, err := d.Probe(context.Background(), &domain.Account{}, "test", up.Client(), "unknown"); err == nil {
		t.Fatal("unknown model accepted")
	}
}

func TestClaudeQuotaRolloverDoesNotInventRestoredAllowance(t *testing.T) {
	d := NewClaudeDriver(ClaudeConfig{}, NoopStainlessStore{}, 4)
	now := time.Now()
	oldReset := now.Add(-time.Hour).Unix()
	old := json.RawMessage(fmt.Sprintf(`{"seven_day_util":1,"seven_day_reset":%d,"seven_day_meta":{"status":"rejected","observed_at":%d},"overage":{"status":"rejected","reason":"out_of_credits"}}`, oldReset, now.Add(-2*time.Hour).UnixNano()))
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-7d-reset", fmt.Sprint(now.Add(7*24*time.Hour).Unix()))
	h.Set("anthropic-ratelimit-unified-overage-status", "rejected")
	merged := d.MergeState(old, mustMarshalJSON(captureClaudeObservation(h, now)))
	var s ClaudeState
	_ = json.Unmarshal(merged, &s)
	if s.SevenDayUtil != nil || s.SevenDayMeta.Status != "" {
		t.Fatalf("new cycle inherited stale allowance: %s", merged)
	}
	if s.Overage.Reason != "out_of_credits" {
		t.Fatal("missing reason cleared a known reason")
	}
	if !d.GetUtilization(merged)[0].Unknown {
		t.Fatal("new cycle without usage presented as full")
	}
}
