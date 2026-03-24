package driver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCodexBuildRequestUsesSubjectForAccountHeader(t *testing.T) {
	d := NewCodexDriver(CodexConfig{APIURL: "https://chatgpt.com/backend-api/codex"})
	input := &RelayInput{
		RawBody: []byte(`{"model":"gpt-5.1-codex"}`),
		Headers: make(http.Header),
	}
	acct := &domain.Account{
		Provider: domain.ProviderCodex,
		Subject:  "acct-subject-123",
		Identity: map[string]string{"chatgptAccountId": "stale-identity"},
	}

	req, err := d.BuildRequest(context.Background(), input, acct, "tok")
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if got := req.Header.Get("Chatgpt-Account-Id"); got != acct.Subject {
		t.Fatalf("Chatgpt-Account-Id = %q, want %q", got, acct.Subject)
	}
}

func TestCodexProbeRequiresSubject(t *testing.T) {
	d := NewCodexDriver(CodexConfig{APIURL: "https://chatgpt.com/backend-api/codex"})
	acct := &domain.Account{Provider: domain.ProviderCodex}
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("event: response.output_text.delta\n")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := d.Probe(context.Background(), acct, "tok", client)
	if err == nil {
		t.Fatal("Probe() error = nil, want missing subject error")
	}
}

// --- Per-family rate limit tests ---

func multiFamilyHeaders() http.Header {
	h := make(http.Header)
	// Standard codex family
	h.Set("x-codex-primary-used-percent", "62")
	h.Set("x-codex-primary-reset-after-seconds", "7200")
	h.Set("x-codex-secondary-used-percent", "99")
	h.Set("x-codex-secondary-reset-after-seconds", "129000")
	// Spark (bengalfox) family
	h.Set("x-codex-bengalfox-primary-used-percent", "0")
	h.Set("x-codex-bengalfox-primary-reset-after-seconds", "18000")
	h.Set("x-codex-bengalfox-secondary-used-percent", "1")
	h.Set("x-codex-bengalfox-secondary-reset-after-seconds", "604000")
	h.Set("x-codex-bengalfox-limit-name", "GPT-5.3-Codex-Spark")
	return h
}

func TestCaptureHeaders_MultiFamilyParsing(t *testing.T) {
	d := NewCodexDriver(CodexConfig{})
	state := d.captureHeaders(multiFamilyHeaders(), nil)

	var s CodexState
	if err := json.Unmarshal(state, &s); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if len(s.Families) != 2 {
		t.Fatalf("Families count = %d, want 2", len(s.Families))
	}

	std := s.family("")
	if std.PrimaryUtil != 0.62 {
		t.Errorf("standard PrimaryUtil = %v, want 0.62", std.PrimaryUtil)
	}
	if std.SecondaryUtil != 0.99 {
		t.Errorf("standard SecondaryUtil = %v, want 0.99", std.SecondaryUtil)
	}

	spark := s.family("bengalfox")
	if spark.PrimaryUtil != 0 {
		t.Errorf("spark PrimaryUtil = %v, want 0", spark.PrimaryUtil)
	}
	if spark.SecondaryUtil != 0.01 {
		t.Errorf("spark SecondaryUtil = %v, want 0.01", spark.SecondaryUtil)
	}
	if spark.LimitName != "GPT-5.3-Codex-Spark" {
		t.Errorf("spark LimitName = %q, want GPT-5.3-Codex-Spark", spark.LimitName)
	}
}

func TestCaptureHeaders_LegacyMigration(t *testing.T) {
	d := NewCodexDriver(CodexConfig{})
	legacy := `{"primary_util":0.5,"primary_reset":9999999999,"secondary_util":0.8,"secondary_reset":9999999999}`

	// New headers arrive (only standard family)
	h := make(http.Header)
	h.Set("x-codex-primary-used-percent", "60")
	h.Set("x-codex-primary-reset-after-seconds", "3600")
	h.Set("x-codex-secondary-used-percent", "85")
	h.Set("x-codex-secondary-reset-after-seconds", "86400")

	state := d.captureHeaders(h, json.RawMessage(legacy))
	var s CodexState
	json.Unmarshal(state, &s)

	// Legacy flat fields should be zeroed
	if s.PrimaryUtil != 0 || s.SecondaryUtil != 0 {
		t.Fatalf("legacy flat fields not zeroed: primary=%v secondary=%v", s.PrimaryUtil, s.SecondaryUtil)
	}

	// New values should be in Families[""]
	std := s.family("")
	if std.PrimaryUtil != 0.6 {
		t.Errorf("migrated PrimaryUtil = %v, want 0.6", std.PrimaryUtil)
	}
	if std.SecondaryUtil != 0.85 {
		t.Errorf("migrated SecondaryUtil = %v, want 0.85", std.SecondaryUtil)
	}
}

func TestCanServe_PerFamily(t *testing.T) {
	d := NewCodexDriver(CodexConfig{})
	now := time.Now()
	futureReset := now.Add(1 * time.Hour).Unix()

	state, _ := json.Marshal(CodexState{
		Families: map[string]CodexFamilyLimits{
			"":          {SecondaryUtil: 0.99, SecondaryReset: futureReset}, // standard exhausted
			"bengalfox": {SecondaryUtil: 0.01, SecondaryReset: futureReset}, // spark available
		},
	})

	tests := []struct {
		model string
		want  bool
	}{
		{"gpt-5.3-codex", false},       // standard family exhausted
		{"gpt-5.4", false},             // standard family
		{"gpt-5.3-codex-spark", true},  // spark family has capacity
		{"codex-1", false},             // standard family
	}
	for _, tt := range tests {
		if got := d.CanServe(state, tt.model, now); got != tt.want {
			t.Errorf("CanServe(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestCanServe_ExpiredResetAllows(t *testing.T) {
	d := NewCodexDriver(CodexConfig{})
	now := time.Now()
	pastReset := now.Add(-1 * time.Hour).Unix()

	state, _ := json.Marshal(CodexState{
		Families: map[string]CodexFamilyLimits{
			"": {SecondaryUtil: 0.99, SecondaryReset: pastReset},
		},
	})

	if !d.CanServe(state, "gpt-5.3-codex", now) {
		t.Error("CanServe should return true when reset time has passed")
	}
}

func TestComputeExhaustedCooldown_PartialExhaustion(t *testing.T) {
	d := NewCodexDriver(CodexConfig{})
	now := time.Now()
	futureReset := now.Add(2 * time.Hour).Unix()

	// Standard exhausted, spark has capacity — no bucket cooldown
	state, _ := json.Marshal(CodexState{
		Families: map[string]CodexFamilyLimits{
			"":          {SecondaryUtil: 0.99, SecondaryReset: futureReset},
			"bengalfox": {SecondaryUtil: 0.01, SecondaryReset: futureReset},
		},
	})
	cd := d.ComputeExhaustedCooldown(state, now)
	if !cd.IsZero() {
		t.Errorf("expected no cooldown when spark has capacity, got %v", cd)
	}
}

func TestComputeExhaustedCooldown_AllExhausted(t *testing.T) {
	d := NewCodexDriver(CodexConfig{})
	now := time.Now()
	stdReset := now.Add(2 * time.Hour).Unix()
	sparkReset := now.Add(6 * time.Hour).Unix()

	state, _ := json.Marshal(CodexState{
		Families: map[string]CodexFamilyLimits{
			"":          {SecondaryUtil: 0.99, SecondaryReset: stdReset},
			"bengalfox": {SecondaryUtil: 0.99, SecondaryReset: sparkReset},
		},
	})
	cd := d.ComputeExhaustedCooldown(state, now)
	if cd.IsZero() {
		t.Fatal("expected cooldown when all families exhausted")
	}
	// Should use earliest reset (standard @ 2h)
	if cd.Unix() != stdReset {
		t.Errorf("cooldown = %v, want earliest reset %v", cd.Unix(), stdReset)
	}
}

func TestAutoPriority_BestFamily(t *testing.T) {
	d := NewCodexDriver(CodexConfig{})

	state, _ := json.Marshal(CodexState{
		Families: map[string]CodexFamilyLimits{
			"":          {SecondaryUtil: 0.99}, // 1% remaining
			"bengalfox": {SecondaryUtil: 0.01}, // 99% remaining
		},
	})
	pri := d.AutoPriority(state)
	if pri != 99 {
		t.Errorf("AutoPriority = %d, want 99 (best family capacity)", pri)
	}
}

func TestIsStale_PerFamily(t *testing.T) {
	d := NewCodexDriver(CodexConfig{})
	now := time.Now()

	// Standard has expired reset, spark is fine
	state, _ := json.Marshal(CodexState{
		Families: map[string]CodexFamilyLimits{
			"":          {PrimaryUtil: 0.5, PrimaryReset: now.Add(-1 * time.Hour).Unix()},
			"bengalfox": {PrimaryUtil: 0.1, PrimaryReset: now.Add(1 * time.Hour).Unix()},
		},
	})
	if !d.IsStale(state, now) {
		t.Error("IsStale should return true when any family has expired reset")
	}
}

func TestGetUtilization_PerFamily(t *testing.T) {
	d := NewCodexDriver(CodexConfig{})
	futureReset := time.Now().Add(1 * time.Hour).Unix()

	state, _ := json.Marshal(CodexState{
		Families: map[string]CodexFamilyLimits{
			"": {
				PrimaryUtil: 0.42, PrimaryReset: futureReset,
				SecondaryUtil: 0.99, SecondaryReset: futureReset,
			},
			"bengalfox": {
				PrimaryUtil: 0, PrimaryReset: futureReset,
				SecondaryUtil: 0.01, SecondaryReset: futureReset,
				LimitName: "GPT-5.3-Codex-Spark",
			},
		},
	})
	windows := d.GetUtilization(state)
	if len(windows) != 2 {
		t.Fatalf("GetUtilization returned %d windows, want 2 (merged)", len(windows))
	}
	// Primary window: standard 42%, spark 0%
	if windows[0].Label != "primary" {
		t.Errorf("windows[0].Label = %q, want 'primary'", windows[0].Label)
	}
	if windows[0].Pct != 42 {
		t.Errorf("windows[0].Pct = %d, want 42", windows[0].Pct)
	}
	if windows[0].SubPct != 0 {
		t.Errorf("windows[0].SubPct = %d, want 0", windows[0].SubPct)
	}
	if windows[0].SubLabel != "GPT-5.3-Codex-Spark" {
		t.Errorf("windows[0].SubLabel = %q, want 'GPT-5.3-Codex-Spark'", windows[0].SubLabel)
	}
	// Secondary window: standard 99%, spark 1%
	if windows[1].Pct != 99 {
		t.Errorf("windows[1].Pct = %d, want 99", windows[1].Pct)
	}
	if windows[1].SubPct != 1 {
		t.Errorf("windows[1].SubPct = %d, want 1", windows[1].SubPct)
	}
}

func TestCodexModelFamily(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"gpt-5.3-codex", ""},
		{"gpt-5.4", ""},
		{"gpt-5.3-codex-spark", "bengalfox"},
		{"GPT-5.3-Codex-Spark", "bengalfox"},
		{"codex-1", ""},
		{"gpt-5.1-codex-mini", ""},
	}
	for _, tt := range tests {
		if got := codexModelFamily(tt.model); got != tt.want {
			t.Errorf("codexModelFamily(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestDiscoverCodexFamilyPrefixes(t *testing.T) {
	h := multiFamilyHeaders()
	prefixes := discoverCodexFamilyPrefixes(h)
	if len(prefixes) != 2 {
		t.Fatalf("found %d prefixes, want 2", len(prefixes))
	}
	// Should contain "" and "bengalfox"
	foundStd, foundSpark := false, false
	for _, p := range prefixes {
		if p == "" {
			foundStd = true
		}
		if p == "bengalfox" {
			foundSpark = true
		}
	}
	if !foundStd || !foundSpark {
		t.Errorf("prefixes = %v, want [\"\", \"bengalfox\"]", prefixes)
	}
}
