package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func TestFulfillPaymentOrderWithCreditCommitsOrderAndLedger(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "payments.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}
	st, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	order := &domain.PaymentOrder{
		ID:                 "order-1",
		OutTradeNo:         "out-1",
		UserID:             "user-1",
		Gateway:            "zpay",
		Status:             "pending",
		ProductName:        "credit",
		AmountCNYFen:       990,
		CreditMicros:       9_900_000,
		ExchangeRateMicros: 1_000_000,
		PaymentType:        "alipay",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := st.SavePaymentOrder(ctx, order); err != nil {
		t.Fatalf("SavePaymentOrder: %v", err)
	}

	entry := &domain.BillingLedgerEntry{
		ID:             "ledger-1",
		UserID:         "user-1",
		AmountMicros:   9_900_000,
		Kind:           "payment_credit",
		SourceType:     "payment_order",
		SourceID:       "out-1",
		IdempotencyKey: "payment:out-1",
		Description:    "payment recharge",
		MetadataJSON:   "{}",
		CreatedAt:      now,
	}
	if err := st.FulfillPaymentOrderWithCredit(ctx, "out-1", "zpay-1", "alipay", now, entry); err != nil {
		t.Fatalf("FulfillPaymentOrderWithCredit: %v", err)
	}
	if err := st.FulfillPaymentOrderWithCredit(ctx, "out-1", "zpay-1", "alipay", now, entry); err != nil {
		t.Fatalf("FulfillPaymentOrderWithCredit duplicate: %v", err)
	}

	paid, err := st.GetPaymentOrderByOutTradeNo(ctx, "out-1")
	if err != nil {
		t.Fatalf("GetPaymentOrderByOutTradeNo: %v", err)
	}
	if paid == nil || paid.Status != "paid" || paid.ZpayTradeNo != "zpay-1" {
		t.Fatalf("paid order = %#v", paid)
	}
	sum, _, err := st.SumBillingLedgerAfter(ctx, "user-1", 0)
	if err != nil {
		t.Fatalf("SumBillingLedgerAfter: %v", err)
	}
	if sum != 9_900_000 {
		t.Fatalf("ledger sum = %d, want 9900000", sum)
	}
}
