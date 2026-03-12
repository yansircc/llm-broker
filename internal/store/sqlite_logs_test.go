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
