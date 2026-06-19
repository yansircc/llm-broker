package admission

import (
	"context"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/billing"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/store"
)

func TestAdmissionRequiresPositiveBalance(t *testing.T) {
	st := store.NewMockStore()
	b := billing.NewService(st)
	svc := NewService(st, b)
	ctx := context.Background()
	now := time.Now().UTC()
	_ = st.UpsertAdmissionLimit(ctx, &domain.AdmissionLimit{Scope: "global", MinBalanceMicros: 1, UpdatedAt: now})
	_ = st.InsertBillingLedgerEntry(ctx, &domain.BillingLedgerEntry{ID: "l1", UserID: "u1", AmountMicros: 1, IdempotencyKey: "credit", MetadataJSON: "{}", CreatedAt: now})

	if _, _, err := svc.Admit(ctx, Request{UserID: "u2", APIKeyID: "k2"}); err == nil {
		t.Fatal("zero balance admitted")
	}
	if _, release, err := svc.Admit(ctx, Request{UserID: "u1", APIKeyID: "k1"}); err != nil {
		t.Fatalf("positive balance rejected: %v", err)
	} else {
		release()
	}
}

func TestRewardOnlyConcurrencyIsOne(t *testing.T) {
	st := store.NewMockStore()
	b := billing.NewService(st)
	svc := NewService(st, b)
	ctx := context.Background()
	now := time.Now().UTC()
	_ = st.UpsertAdmissionLimit(ctx, &domain.AdmissionLimit{Scope: "reward_only", MaxConcurrent: 1, MinBalanceMicros: 1, UpdatedAt: now})
	_ = st.InsertBillingLedgerEntry(ctx, &domain.BillingLedgerEntry{ID: "l1", UserID: "u1", AmountMicros: 10, IdempotencyKey: "credit", MetadataJSON: "{}", CreatedAt: now})

	_, release, err := svc.Admit(ctx, Request{UserID: "u1", APIKeyID: "k1", RewardOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, _, err := svc.Admit(ctx, Request{UserID: "u1", APIKeyID: "k1", RewardOnly: true}); err == nil {
		t.Fatal("second reward-only concurrent request admitted")
	}
}

func TestAdmissionRejectsExceededAPIKeyBudget(t *testing.T) {
	st := store.NewMockStore()
	b := billing.NewService(st)
	svc := NewService(st, b)
	ctx := context.Background()
	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	if err := st.CreateAPIKey(ctx, &domain.APIKey{
		ID:                  "k-budget",
		UserID:              "u-budget",
		Name:                "budget",
		Status:              "active",
		TokenHash:           "hash",
		TokenPrefix:         "sk_",
		AllowedSurface:      domain.SurfaceAll,
		DailyBudgetMicros:   1_000_000,
		MonthlyBudgetMicros: 10_000_000,
		CreatedAt:           now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertBillingLedgerEntry(ctx, &domain.BillingLedgerEntry{ID: "credit", UserID: "u-budget", AmountMicros: 10_000_000, IdempotencyKey: "credit", MetadataJSON: "{}", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateBillableRequest(ctx, &domain.BillableRequest{
		RequestID: "req-budget",
		UserID:    "u-budget",
		APIKeyID:  "k-budget",
		Model:     "gpt-5",
		Status:    "settled",
		CreatedAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertBillingLedgerEntry(ctx, &domain.BillingLedgerEntry{
		ID:             "usage",
		UserID:         "u-budget",
		AmountMicros:   -1_000_000,
		Kind:           "usage_debit",
		SourceType:     "request",
		SourceID:       "req-budget",
		IdempotencyKey: "usage:req-budget",
		MetadataJSON:   "{}",
		CreatedAt:      now.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	decision, _, err := svc.Admit(ctx, Request{UserID: "u-budget", APIKeyID: "k-budget"})
	if err == nil {
		t.Fatal("budget-exceeded api key admitted")
	}
	if decision.Reason != "api_key_daily_budget_exceeded" {
		t.Fatalf("reason = %q, want api_key_daily_budget_exceeded", decision.Reason)
	}
}

func TestAPIKeyBudgetUsesRequestWindowNotSettlementWindow(t *testing.T) {
	st := store.NewMockStore()
	b := billing.NewService(st)
	svc := NewService(st, b)
	ctx := context.Background()
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	if err := st.CreateAPIKey(ctx, &domain.APIKey{
		ID:                "k-window",
		UserID:            "u-window",
		Name:              "window",
		Status:            "active",
		TokenHash:         "hash-window",
		TokenPrefix:       "sk_",
		AllowedSurface:    domain.SurfaceAll,
		DailyBudgetMicros: 1_000_000,
		CreatedAt:         now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertBillingLedgerEntry(ctx, &domain.BillingLedgerEntry{ID: "credit-window", UserID: "u-window", AmountMicros: 10_000_000, IdempotencyKey: "credit-window", MetadataJSON: "{}", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateBillableRequest(ctx, &domain.BillableRequest{
		RequestID: "req-yesterday",
		UserID:    "u-window",
		APIKeyID:  "k-window",
		Model:     "gpt-5",
		Status:    "settled",
		CreatedAt: now.Add(-24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertBillingLedgerEntry(ctx, &domain.BillingLedgerEntry{
		ID:             "usage-window",
		UserID:         "u-window",
		AmountMicros:   -1_000_000,
		Kind:           "usage_debit",
		SourceType:     "request",
		SourceID:       "req-yesterday",
		IdempotencyKey: "usage:req-yesterday",
		MetadataJSON:   "{}",
		CreatedAt:      now,
	}); err != nil {
		t.Fatal(err)
	}

	if _, release, err := svc.Admit(ctx, Request{UserID: "u-window", APIKeyID: "k-window"}); err != nil {
		t.Fatalf("late-settled prior-day usage blocked today's budget: %v", err)
	} else {
		release()
	}
}
