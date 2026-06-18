package billing

import (
	"context"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/store"
)

func TestBalanceUsesCheckpointPlusLedgerDelta(t *testing.T) {
	st := store.NewMockStore()
	svc := NewService(st)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = st.InsertBillingLedgerEntry(ctx, &domain.BillingLedgerEntry{ID: "l1", UserID: "u1", AmountMicros: 100, IdempotencyKey: "1", MetadataJSON: "{}", CreatedAt: now})
	_ = st.InsertBillingLedgerEntry(ctx, &domain.BillingLedgerEntry{ID: "l2", UserID: "u1", AmountMicros: 50, IdempotencyKey: "2", MetadataJSON: "{}", CreatedAt: now})
	if err := st.UpsertBillingBalanceCheckpoint(ctx, &domain.BillingBalanceCheckpoint{UserID: "u1", LedgerSeq: 1, BalanceMicros: 100, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}

	balance, seq, err := svc.Balance(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if balance != 150 || seq != 2 {
		t.Fatalf("balance=%d seq=%d, want 150/2", balance, seq)
	}
}

func TestUsageSettlementIsIdempotentAndSnapshotsPrice(t *testing.T) {
	st := store.NewMockStore()
	svc := NewService(st)
	ctx := context.Background()
	now := time.Now().UTC()
	price := &domain.ModelPrice{Model: "gpt-5", InputMicrosPerMillion: 1_000_000, OutputMicrosPerMillion: 2_000_000, UpdatedAt: now}
	_ = st.UpsertModelPrice(ctx, price)
	br := &domain.BillableRequest{RequestID: "req-1", UserID: "u1", APIKeyID: "k1", Model: "gpt-5", Surface: domain.SurfaceNative, Status: "usage_observed", CreatedAt: now}
	_ = st.CreateBillableRequest(ctx, br)

	entry, snapshot, err := svc.DebitBillableRequest(ctx, br, &driver.Usage{InputTokens: 1000, OutputTokens: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if entry.AmountMicros != -3000 {
		t.Fatalf("amount = %d, want -3000", entry.AmountMicros)
	}
	price.InputMicrosPerMillion = 9_000_000
	_ = st.UpsertModelPrice(ctx, price)
	again, againSnapshot, err := svc.DebitBillableRequest(ctx, br, &driver.Usage{InputTokens: 1000, OutputTokens: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != entry.ID {
		t.Fatalf("idempotent debit returned %q, want %q", again.ID, entry.ID)
	}
	if againSnapshot != snapshot {
		t.Fatalf("snapshot changed after price update: %s != %s", againSnapshot, snapshot)
	}
}
