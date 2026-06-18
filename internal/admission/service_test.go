package admission

import (
	"context"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/billing"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/store"
)

func TestAdmissionRequiresVerifiedEmailAndPositiveBalance(t *testing.T) {
	st := store.NewMockStore()
	b := billing.NewService(st)
	svc := NewService(st, b)
	ctx := context.Background()
	now := time.Now().UTC()
	_ = st.UpsertAdmissionLimit(ctx, &domain.AdmissionLimit{Scope: "global", MinBalanceMicros: 1, UpdatedAt: now})
	_ = st.InsertBillingLedgerEntry(ctx, &domain.BillingLedgerEntry{ID: "l1", UserID: "u1", AmountMicros: 1, IdempotencyKey: "credit", MetadataJSON: "{}", CreatedAt: now})

	if _, _, err := svc.Admit(ctx, Request{UserID: "u1", APIKeyID: "k1"}); err == nil {
		t.Fatal("unverified email admitted")
	}
	if _, _, err := svc.Admit(ctx, Request{UserID: "u2", APIKeyID: "k2", EmailVerified: true}); err == nil {
		t.Fatal("zero balance admitted")
	}
	if _, release, err := svc.Admit(ctx, Request{UserID: "u1", APIKeyID: "k1", EmailVerified: true}); err != nil {
		t.Fatalf("verified positive balance rejected: %v", err)
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

	_, release, err := svc.Admit(ctx, Request{UserID: "u1", APIKeyID: "k1", EmailVerified: true, RewardOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, _, err := svc.Admit(ctx, Request{UserID: "u1", APIKeyID: "k1", EmailVerified: true, RewardOnly: true}); err == nil {
		t.Fatal("second reward-only concurrent request admitted")
	}
}
