package store

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/requestlog"
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
		if _, err := store.InsertRequestLog(context.Background(), entry); err != nil {
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

func TestLogAnalyticsPreferSettledLedgerCost(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ledger-costs.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Add(-2 * time.Second)
	entry := &domain.RequestLog{
		UserID:          "user-1",
		RequestID:       "req-ledger",
		Model:           "gpt-5.5",
		InputTokens:     1000,
		OutputTokens:    20,
		CacheReadTokens: 500,
		CostUSD:         0.12,
		Status:          "ok",
		CreatedAt:       now,
	}
	if _, err := store.InsertRequestLog(context.Background(), entry); err != nil {
		t.Fatalf("InsertRequestLog: %v", err)
	}
	if err := store.InsertBillingLedgerEntry(context.Background(), &domain.BillingLedgerEntry{
		ID:             "ledger-req-ledger",
		UserID:         "user-1",
		AmountMicros:   -60_000,
		Kind:           "usage_debit",
		SourceType:     "request",
		SourceID:       "req-ledger",
		IdempotencyKey: "usage:req-ledger",
		Description:    "usage charge",
		MetadataJSON:   "{}",
		CreatedAt:      now,
	}); err != nil {
		t.Fatalf("InsertBillingLedgerEntry: %v", err)
	}

	logs, total, err := store.QueryRequestLogs(context.Background(), domain.RequestLogQuery{
		UserID: "user-1",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("QueryRequestLogs: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("request logs total=%d len=%d, want 1", total, len(logs))
	}
	assertFloat64Near(t, logs[0].CostUSD, 0.06, "request log cost")

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
	assertFloat64Near(t, today.CostUSD, 0.06, "today cost")

	modelUsage, err := store.QueryModelUsage(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("QueryModelUsage: %v", err)
	}
	if len(modelUsage) != 1 {
		t.Fatalf("len(modelUsage) = %d, want 1", len(modelUsage))
	}
	assertFloat64Near(t, modelUsage[0].CostUSD, 0.06, "model cost")

	totalCosts, err := store.QueryUserTotalCostsByIDs(context.Background(), []string{"user-1"})
	if err != nil {
		t.Fatalf("QueryUserTotalCostsByIDs: %v", err)
	}
	assertFloat64Near(t, totalCosts["user-1"], 0.06, "user total cost")
}

func assertFloat64Near(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 0.0000001 {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func TestQueryUserTotalCostsByIDs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "user-costs.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	for _, entry := range []*domain.RequestLog{
		{UserID: "user-1", Status: "ok", CostUSD: 1.25, CreatedAt: now},
		{UserID: "user-1", Status: "ok", CostUSD: 0.75, CreatedAt: now.Add(time.Second)},
		{UserID: "user-2", Status: "ok", CostUSD: 2.50, CreatedAt: now.Add(2 * time.Second)},
		{UserID: "user-3", Status: "upstream_403", CostUSD: 99, CreatedAt: now.Add(3 * time.Second)},
	} {
		if _, err := store.InsertRequestLog(context.Background(), entry); err != nil {
			t.Fatalf("InsertRequestLog(%s): %v", entry.UserID, err)
		}
	}

	totalCosts, err := store.QueryUserTotalCostsByIDs(context.Background(), []string{"user-1", "user-3"})
	if err != nil {
		t.Fatalf("QueryUserTotalCostsByIDs: %v", err)
	}
	if totalCosts["user-1"] != 2 {
		t.Fatalf("totalCosts[user-1] = %v, want 2", totalCosts["user-1"])
	}
	if totalCosts["user-3"] != 0 {
		t.Fatalf("totalCosts[user-3] = %v, want 0", totalCosts["user-3"])
	}
	if _, ok := totalCosts["user-2"]; ok {
		t.Fatalf("unexpected total for user-2 = %v", totalCosts["user-2"])
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
			CellID:            "cell-compat-1",
			Status:            "upstream_400",
			EffectKind:        "cooldown",
			UpstreamStatus:    400,
			UpstreamErrorType: "invalid_request_error",
			DurationMs:        1200,
			CreatedAt:         now,
		},
		{
			UserID:         "user-2",
			AccountID:      "acct-2",
			Provider:       "claude",
			Surface:        "native",
			Model:          "claude-sonnet-4-6",
			CellID:         "cell-native-1",
			Status:         "upstream_403",
			EffectKind:     "block",
			UpstreamStatus: 403,
			DurationMs:     800,
			CreatedAt:      now.Add(2 * time.Second),
		},
		{
			UserID:     "user-1",
			AccountID:  "acct-1",
			Provider:   "claude",
			Surface:    "compat",
			Model:      "claude-sonnet-4-6",
			CellID:     "cell-compat-1",
			Status:     "ok",
			EffectKind: "success",
			DurationMs: 1500,
			CreatedAt:  now.Add(4 * time.Second),
		},
	}
	for _, entry := range entries {
		if _, err := store.InsertRequestLog(context.Background(), entry); err != nil {
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
	// Most-recent failure comes first (created_at DESC). Last one inserted at offset 2s is the 403.
	if failures[0].UpstreamStatus != 403 {
		t.Fatalf("failures[0].UpstreamStatus = %d, want 403", failures[0].UpstreamStatus)
	}
	if failures[1].Surface != "compat" {
		t.Fatalf("failures[1].Surface = %q, want compat", failures[1].Surface)
	}
	if failures[1].UpstreamErrorType != "invalid_request_error" {
		t.Fatalf("failures[1].UpstreamErrorType = %q, want invalid_request_error", failures[1].UpstreamErrorType)
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

// TestRequestLogFileWriteAndPurge exercises the full opt-in disk pipeline:
// callers write a per-request JSON file using the row's id+created_at, and
// PurgeOldLogs reaps both the SQL row and its day directory.
func TestRequestLogFileWriteAndPurge(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "logs.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer store.Close()

	blobDir := requestlog.ResolveBlobDir(dbPath, requestlog.BlobModeAll)
	store.SetLogBlobDir(blobDir)

	old := time.Now().UTC().Add(-72 * time.Hour)
	recent := time.Now().UTC()

	oldEntry := &domain.RequestLog{
		UserID:    "user-old",
		AccountID: "acct-1",
		Model:     "claude-sonnet-4-6",
		Status:    "ok",
		CreatedAt: old,
	}
	id, err := store.InsertRequestLog(context.Background(), oldEntry)
	if err != nil {
		t.Fatalf("InsertRequestLog(old): %v", err)
	}
	if id != oldEntry.ID || id == 0 {
		t.Fatalf("InsertRequestLog returned id=%d but entry.ID=%d", id, oldEntry.ID)
	}
	oldObs := &requestlog.LogObservation{
		Path:              "/v1/messages",
		ClientBody:        []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
		ClientBodyExcerpt: "hi",
	}
	if err := requestlog.WriteLogFile(blobDir, oldEntry, oldObs); err != nil {
		t.Fatalf("WriteLogFile(old): %v", err)
	}
	oldFile := filepath.Join(blobDir,
		oldEntry.CreatedAt.UTC().Format("2006/01/02"),
		strconv.FormatInt(oldEntry.ID, 10)+".json")
	if _, err := os.Stat(oldFile); err != nil {
		t.Fatalf("old log file missing at %s: %v", oldFile, err)
	}

	recentEntry := &domain.RequestLog{
		UserID:    "user-new",
		AccountID: "acct-1",
		Model:     "claude-sonnet-4-6",
		Status:    "ok",
		CreatedAt: recent,
	}
	if _, err := store.InsertRequestLog(context.Background(), recentEntry); err != nil {
		t.Fatalf("InsertRequestLog(recent): %v", err)
	}
	if err := requestlog.WriteLogFile(blobDir, recentEntry, &requestlog.LogObservation{}); err != nil {
		t.Fatalf("WriteLogFile(recent): %v", err)
	}
	recentFile := filepath.Join(blobDir,
		recentEntry.CreatedAt.UTC().Format("2006/01/02"),
		strconv.FormatInt(recentEntry.ID, 10)+".json")
	if _, err := os.Stat(recentFile); err != nil {
		t.Fatalf("recent log file missing at %s: %v", recentFile, err)
	}

	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	deleted, err := store.PurgeOldLogs(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("PurgeOldLogs: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("PurgeOldLogs deleted=%d, want 1 row", deleted)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old log file purged, stat err=%v", err)
	}
	if _, err := os.Stat(recentFile); err != nil {
		t.Fatalf("recent log file should remain: %v", err)
	}
}

// TestRequestLogBlobDirDisabled confirms that when LOG_BLOBS is off,
// ResolveBlobDir returns "" and WriteLogFile is a no-op.
func TestRequestLogBlobDirDisabled(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "logs.db")

	if blob := requestlog.ResolveBlobDir(dbPath, requestlog.BlobModeOff); blob != "" {
		t.Fatalf("ResolveBlobDir(disabled) = %q, want \"\"", blob)
	}
	if blob := requestlog.ResolveBlobDir(":memory:", requestlog.BlobModeAll); blob != "" {
		t.Fatalf("ResolveBlobDir(:memory:) = %q, want \"\"", blob)
	}

	entry := &domain.RequestLog{ID: 42, CreatedAt: time.Now().UTC()}
	if err := requestlog.WriteLogFile("", entry, &requestlog.LogObservation{}); err != nil {
		t.Fatalf("WriteLogFile(\"\") should be a no-op, got %v", err)
	}
}
