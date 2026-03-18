package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func TestLogAnalyticsIgnoreFailedAttempts(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "logs.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Add(-2 * time.Second)
	for _, entry := range []*domain.RequestLog{
		{
			UserID:          "user-1",
			AccountID:       "acct-1",
			Model:           "claude-sonnet-4-6",
			InputTokens:     120,
			OutputTokens:    34,
			CacheReadTokens: 8,
			CostUSD:         1.25,
			Status:          "ok",
			CreatedAt:       now,
		},
		{
			UserID:     "user-1",
			AccountID:  "acct-1",
			Model:      "claude-sonnet-4-6",
			Status:     "upstream_529",
			CreatedAt:  now,
			DurationMs: 900,
		},
	} {
		if err := store.InsertRequestLog(context.Background(), entry); err != nil {
			t.Fatalf("InsertRequestLog(%s): %v", entry.Status, err)
		}
	}

	usage, err := store.QueryUsagePeriods(context.Background(), "user-1", time.UTC)
	if err != nil {
		t.Fatalf("QueryUsagePeriods: %v", err)
	}

	var today domain.UsagePeriod
	for _, period := range usage {
		if period.Label == "today" {
			today = period
			break
		}
	}
	if today.Requests != 1 {
		t.Fatalf("today.Requests = %d, want 1", today.Requests)
	}
	if today.InputTokens != 120 || today.OutputTokens != 34 || today.CacheReadTokens != 8 {
		t.Fatalf("today usage = %+v, want only successful request usage", today)
	}

	modelUsage, err := store.QueryModelUsage(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("QueryModelUsage: %v", err)
	}
	if len(modelUsage) != 1 {
		t.Fatalf("len(modelUsage) = %d, want 1", len(modelUsage))
	}
	if modelUsage[0].Requests != 1 {
		t.Fatalf("modelUsage[0].Requests = %d, want 1", modelUsage[0].Requests)
	}
	if modelUsage[0].InputTokens != 120 || modelUsage[0].OutputTokens != 34 || modelUsage[0].CacheReadTokens != 8 {
		t.Fatalf("modelUsage[0] = %+v, want only successful request usage", modelUsage[0])
	}

	totalCosts, err := store.QueryUserTotalCosts(context.Background())
	if err != nil {
		t.Fatalf("QueryUserTotalCosts: %v", err)
	}
	if totalCosts["user-1"] != 1.25 {
		t.Fatalf("totalCosts[user-1] = %v, want 1.25", totalCosts["user-1"])
	}
}

func TestRequestLogObservabilityQueries(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "observability.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	entries := []*domain.RequestLog{
		{
			UserID:            "user-1",
			AccountID:         "acct-1",
			Provider:          "claude",
			Surface:           "compat",
			Model:             "claude-sonnet-4-6",
			Path:              "/compat/v1/chat/completions",
			CellID:            "cell-compat-1",
			BucketKey:         "claude:bucket-1",
			Status:            "upstream_400",
			EffectKind:        "cooldown",
			UpstreamStatus:    400,
			UpstreamRequestID: "req_400",
			RequestBytes:      2048,
			AttemptCount:      1,
			DurationMs:        1200,
			CreatedAt:         now,
		},
		{
			UserID:            "user-2",
			AccountID:         "acct-2",
			Provider:          "claude",
			Surface:           "native",
			Model:             "claude-sonnet-4-6",
			Path:              "/v1/messages",
			CellID:            "cell-native-1",
			BucketKey:         "claude:bucket-2",
			Status:            "upstream_403",
			EffectKind:        "block",
			UpstreamStatus:    403,
			UpstreamRequestID: "req_403",
			RequestBytes:      1024,
			AttemptCount:      2,
			DurationMs:        800,
			CreatedAt:         now.Add(2 * time.Second),
		},
		{
			UserID:       "user-1",
			AccountID:    "acct-1",
			Provider:     "claude",
			Surface:      "compat",
			Model:        "claude-sonnet-4-6",
			Path:         "/compat/v1/chat/completions",
			CellID:       "cell-compat-1",
			BucketKey:    "claude:bucket-1",
			Status:       "ok",
			EffectKind:   "success",
			RequestBytes: 4096,
			AttemptCount: 1,
			DurationMs:   1500,
			CreatedAt:    now.Add(4 * time.Second),
		},
	}
	for _, entry := range entries {
		if err := store.InsertRequestLog(context.Background(), entry); err != nil {
			t.Fatalf("InsertRequestLog(%s): %v", entry.Status, err)
		}
	}

	failures, total, err := store.QueryRequestLogs(context.Background(), domain.RequestLogQuery{
		FailuresOnly: true,
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("QueryRequestLogs(FailuresOnly): %v", err)
	}
	if total != 2 || len(failures) != 2 {
		t.Fatalf("failures total=%d len=%d, want 2", total, len(failures))
	}
	if failures[0].UpstreamRequestID != "req_403" {
		t.Fatalf("failures[0].UpstreamRequestID = %q, want req_403", failures[0].UpstreamRequestID)
	}
	if failures[1].Surface != "compat" {
		t.Fatalf("failures[1].Surface = %q, want compat", failures[1].Surface)
	}

	outcomes, err := store.QueryRelayOutcomeStats(context.Background(), now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("QueryRelayOutcomeStats: %v", err)
	}
	if len(outcomes) != 3 {
		t.Fatalf("len(outcomes) = %d, want 3", len(outcomes))
	}

	cellRisk, err := store.QueryCellRiskStats(context.Background(), now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("QueryCellRiskStats: %v", err)
	}
	if len(cellRisk) != 2 {
		t.Fatalf("len(cellRisk) = %d, want 2", len(cellRisk))
	}

	var compatStat *domain.CellRiskStat
	for i := range cellRisk {
		if cellRisk[i].CellID == "cell-compat-1" {
			compatStat = &cellRisk[i]
			break
		}
	}
	if compatStat == nil {
		t.Fatal("missing compat cell risk stat")
	}
	if compatStat.Requests != 2 || compatStat.Successes != 1 || compatStat.Status400 != 1 {
		t.Fatalf("compatStat = %+v, want requests=2 successes=1 status400=1", *compatStat)
	}
}
