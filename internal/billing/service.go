package billing

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/store"
)

type Service struct {
	store store.Store
	now   func() time.Time
}

func NewService(s store.Store) *Service {
	return &Service{store: s, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Balance(ctx context.Context, userID string) (int64, int64, error) {
	checkpoint, err := s.store.GetBillingBalanceCheckpoint(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	var baseBalance, baseSeq int64
	if checkpoint != nil {
		baseBalance = checkpoint.BalanceMicros
		baseSeq = checkpoint.LedgerSeq
	}
	delta, maxSeq, err := s.store.SumBillingLedgerAfter(ctx, userID, baseSeq)
	if err != nil {
		return 0, 0, err
	}
	return baseBalance + delta, maxSeq, nil
}

func (s *Service) WriteCheckpoint(ctx context.Context, userID string) error {
	balance, maxSeq, err := s.Balance(ctx, userID)
	if err != nil {
		return err
	}
	return s.store.UpsertBillingBalanceCheckpoint(ctx, &domain.BillingBalanceCheckpoint{
		UserID:        userID,
		LedgerSeq:     maxSeq,
		BalanceMicros: balance,
		CreatedAt:     s.now(),
	})
}

func (s *Service) ReserveRequest(ctx context.Context, br *domain.BillableRequest) error {
	if br == nil {
		return fmt.Errorf("missing billable request")
	}
	return s.store.CreateBillableRequest(ctx, br)
}

func (s *Service) MarkRequestStatus(ctx context.Context, requestID, status, errMsg string) error {
	return s.store.MarkBillableRequestStatus(ctx, requestID, status, errMsg)
}

func (s *Service) SettleUsage(ctx context.Context, requestID string, usage *driver.Usage) (*domain.BillingLedgerEntry, error) {
	br, err := s.store.GetBillableRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if br == nil {
		return nil, fmt.Errorf("billable request not found")
	}
	now := s.now()
	br.Status = "usage_observed"
	if usage != nil {
		br.InputTokens = usage.InputTokens
		br.OutputTokens = usage.OutputTokens
		br.CacheReadTokens = usage.CacheReadTokens
		br.CacheCreateTokens = usage.CacheCreateTokens
	}
	br.UsageObservedAt = &now
	entry, snapshot, err := s.DebitBillableRequest(ctx, br, usage)
	if err != nil {
		_ = s.store.MarkBillableRequestStatus(ctx, requestID, "settlement_failed", err.Error())
		return nil, err
	}
	br.PriceSnapshotJSON = snapshot
	if err := s.store.UpdateBillableRequestUsage(ctx, br); err != nil {
		return nil, err
	}
	if err := s.store.MarkBillableRequestSettled(ctx, requestID, entry.ID, s.now()); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *Service) Credit(ctx context.Context, userID, amountKind, sourceType, sourceID, idempotencyKey, description string, amountMicros int64) (*domain.BillingLedgerEntry, error) {
	if existing, err := s.store.GetBillingLedgerEntryByIdempotencyKey(ctx, idempotencyKey); err != nil || existing != nil {
		return existing, err
	}
	entry := &domain.BillingLedgerEntry{
		ID:             uuid.NewString(),
		UserID:         userID,
		AmountMicros:   amountMicros,
		Kind:           amountKind,
		SourceType:     sourceType,
		SourceID:       sourceID,
		IdempotencyKey: idempotencyKey,
		Description:    description,
		MetadataJSON:   "{}",
		CreatedAt:      s.now(),
	}
	if err := s.store.InsertBillingLedgerEntry(ctx, entry); err != nil {
		existing, getErr := s.store.GetBillingLedgerEntryByIdempotencyKey(ctx, idempotencyKey)
		if getErr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	return entry, nil
}

func (s *Service) AdminAdjust(ctx context.Context, userID string, amountMicros int64, reason string) (*domain.BillingLedgerEntry, error) {
	return s.Credit(ctx, userID, "admin_adjustment", "admin", uuid.NewString(), "admin_adjustment:"+uuid.NewString(), reason, amountMicros)
}

func (s *Service) DebitUsage(ctx context.Context, requestID, userID string, usage *driver.Usage) (*domain.BillingLedgerEntry, string, error) {
	existing, err := s.store.GetBillingLedgerEntryByIdempotencyKey(ctx, "usage:"+requestID)
	if err != nil || existing != nil {
		if existing == nil {
			return nil, "", err
		}
		return existing, existing.PriceSnapshotJSON, nil
	}
	price, err := s.store.GetModelPrice(ctx, usageModel(ctx, requestID))
	if err != nil {
		return nil, "", err
	}
	if price == nil {
		br, err := s.store.GetBillableRequest(ctx, requestID)
		if err != nil {
			return nil, "", err
		}
		if br != nil {
			price, err = s.store.GetModelPrice(ctx, br.Model)
			if err != nil {
				return nil, "", err
			}
		}
	}
	if price == nil {
		return nil, "", fmt.Errorf("missing model price")
	}
	charge := ChargeMicros(usage, price)
	snapshot := SnapshotJSON(Snapshot(price))
	entry := &domain.BillingLedgerEntry{
		ID:                uuid.NewString(),
		UserID:            userID,
		AmountMicros:      -charge,
		Kind:              "usage_debit",
		SourceType:        "request",
		SourceID:          requestID,
		IdempotencyKey:    "usage:" + requestID,
		Description:       "usage charge",
		PriceSnapshotJSON: snapshot,
		MetadataJSON:      "{}",
		CreatedAt:         s.now(),
	}
	if err := s.store.InsertBillingLedgerEntry(ctx, entry); err != nil {
		return nil, "", err
	}
	return entry, snapshot, nil
}

func (s *Service) DebitBillableRequest(ctx context.Context, br *domain.BillableRequest, usage *driver.Usage) (*domain.BillingLedgerEntry, string, error) {
	if br == nil {
		return nil, "", fmt.Errorf("missing billable request")
	}
	price, err := s.store.GetModelPrice(ctx, br.Model)
	if err != nil {
		return nil, "", err
	}
	if price == nil {
		return nil, "", fmt.Errorf("missing model price for %s", br.Model)
	}
	existing, err := s.store.GetBillingLedgerEntryByIdempotencyKey(ctx, "usage:"+br.RequestID)
	if err != nil || existing != nil {
		if existing == nil {
			return nil, "", err
		}
		return existing, existing.PriceSnapshotJSON, nil
	}
	charge := ChargeMicros(usage, price)
	snapshot := SnapshotJSON(Snapshot(price))
	entry := &domain.BillingLedgerEntry{
		ID:                uuid.NewString(),
		UserID:            br.UserID,
		AmountMicros:      -charge,
		Kind:              "usage_debit",
		SourceType:        "request",
		SourceID:          br.RequestID,
		IdempotencyKey:    "usage:" + br.RequestID,
		Description:       "usage charge",
		PriceSnapshotJSON: snapshot,
		MetadataJSON:      "{}",
		CreatedAt:         s.now(),
	}
	if err := s.store.InsertBillingLedgerEntry(ctx, entry); err != nil {
		return nil, "", err
	}
	return entry, snapshot, nil
}

func (s *Service) CreditPayment(ctx context.Context, order *domain.PaymentOrder) (*domain.BillingLedgerEntry, error) {
	if order == nil {
		return nil, fmt.Errorf("missing payment order")
	}
	return s.Credit(ctx, order.UserID, "payment_credit", "payment_order", order.OutTradeNo, "payment:"+order.OutTradeNo, "payment recharge", order.CreditMicros)
}

func (s *Service) FulfillPaymentOrder(ctx context.Context, order *domain.PaymentOrder, zpayTradeNo, paymentType string, paidAt time.Time) (*domain.BillingLedgerEntry, error) {
	if order == nil {
		return nil, fmt.Errorf("missing payment order")
	}
	idempotencyKey := "payment:" + order.OutTradeNo
	entry := &domain.BillingLedgerEntry{
		ID:             uuid.NewString(),
		UserID:         order.UserID,
		AmountMicros:   order.CreditMicros,
		Kind:           "payment_credit",
		SourceType:     "payment_order",
		SourceID:       order.OutTradeNo,
		IdempotencyKey: idempotencyKey,
		Description:    "payment recharge",
		MetadataJSON:   "{}",
		CreatedAt:      paidAt.UTC(),
	}
	if err := s.store.FulfillPaymentOrderWithCredit(ctx, order.OutTradeNo, zpayTradeNo, paymentType, paidAt.UTC(), entry); err != nil {
		return nil, err
	}
	credited, err := s.store.GetBillingLedgerEntryByIdempotencyKey(ctx, idempotencyKey)
	if err != nil || credited != nil {
		return credited, err
	}
	return entry, nil
}

func (s *Service) FulfillReferral(ctx context.Context, invitee *domain.User) error {
	if invitee == nil || invitee.ReferredByUserID == "" {
		return nil
	}
	existing, err := s.store.GetReferralByInvitee(ctx, invitee.ID)
	if err != nil || existing != nil {
		return err
	}
	newUserBonus, _ := settingMicros(ctx, s.store, "referral_new_user_bonus_micros")
	inviterBonus, _ := settingMicros(ctx, s.store, "referral_inviter_bonus_micros")
	now := s.now()
	ref := &domain.Referral{
		ID:            uuid.NewString(),
		InviterUserID: invitee.ReferredByUserID,
		InviteeUserID: invitee.ID,
		InviteCode:    invitee.ReferralCode,
		CreatedAt:     now,
		CreditedAt:    now,
	}
	var inviteeCredit, inviterCredit *domain.BillingLedgerEntry
	if newUserBonus != 0 {
		inviteeCredit = referralCredit(invitee.ID, newUserBonus, "referral_signup_credit", "referral:new_user:"+invitee.ID, now)
	}
	if inviterBonus != 0 {
		inviterCredit = referralCredit(invitee.ReferredByUserID, inviterBonus, "referral_signup_credit", "referral:inviter:"+invitee.ID, now)
	}
	return s.store.CreateReferralWithCredits(ctx, ref, inviteeCredit, inviterCredit)
}

func referralCredit(userID string, amount int64, kind, idempotency string, now time.Time) *domain.BillingLedgerEntry {
	return &domain.BillingLedgerEntry{
		ID:             uuid.NewString(),
		UserID:         userID,
		AmountMicros:   amount,
		Kind:           kind,
		SourceType:     "referral",
		SourceID:       idempotency,
		IdempotencyKey: idempotency,
		Description:    "referral reward",
		MetadataJSON:   "{}",
		CreatedAt:      now,
	}
}

func settingMicros(ctx context.Context, s store.Store, key string) (int64, error) {
	raw, err := s.GetBillingSetting(ctx, key)
	if err != nil || raw == "" {
		return 0, err
	}
	return strconv.ParseInt(raw, 10, 64)
}

type modelKey struct{}

func ContextWithModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, modelKey{}, model)
}

func usageModel(ctx context.Context, _ string) string {
	model, _ := ctx.Value(modelKey{}).(string)
	return model
}
