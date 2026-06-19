package store

import (
	"context"
	"errors"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

var ErrNotFound = errors.New("not found")

// Store is the persistence interface for broker.
// Account operations use typed structs instead of map[string]string.
// Session, stainless, OAuth, and refresh-lock coordination state are durable.
type Store interface {
	Ping(ctx context.Context) error
	Close() error

	// Accounts — typed structs
	GetAccount(ctx context.Context, id string) (*domain.Account, error)
	ListAccounts(ctx context.Context) ([]*domain.Account, error)
	SaveAccount(ctx context.Context, acct *domain.Account) error // UPSERT
	DeleteAccount(ctx context.Context, id string) error
	GetEgressCell(ctx context.Context, id string) (*domain.EgressCell, error)
	ListEgressCells(ctx context.Context) ([]*domain.EgressCell, error)
	SaveEgressCell(ctx context.Context, cell *domain.EgressCell) error
	DeleteEgressCell(ctx context.Context, id string) error
	GetQuotaBucket(ctx context.Context, bucketKey string) (*domain.QuotaBucket, error)
	ListQuotaBuckets(ctx context.Context) ([]*domain.QuotaBucket, error)
	SaveQuotaBucket(ctx context.Context, bucket *domain.QuotaBucket) error
	DeleteQuotaBucket(ctx context.Context, bucketKey string) error
	GetSessionBinding(ctx context.Context, sessionUUID string) (*domain.SessionBinding, error)
	ListSessionBindingsByAccount(ctx context.Context, accountID string) ([]domain.SessionBinding, error)
	SaveSessionBinding(ctx context.Context, binding *domain.SessionBinding) error
	DeleteSessionBinding(ctx context.Context, sessionUUID string) error
	PurgeExpiredSessionBindings(ctx context.Context, before time.Time) (int64, error)
	GetUserRouteBinding(ctx context.Context, userID string, provider domain.Provider, surface domain.Surface) (*domain.UserRouteBinding, error)
	SaveUserRouteBinding(ctx context.Context, binding *domain.UserRouteBinding) error
	DeleteUserRouteBindingsByUser(ctx context.Context, userID string) error
	GetStainlessBinding(ctx context.Context, accountID string) (*domain.StainlessBinding, error)
	SetStainlessBindingNX(ctx context.Context, binding *domain.StainlessBinding) (bool, error)
	DeleteStainlessBinding(ctx context.Context, accountID string) error
	PurgeExpiredStainlessBindings(ctx context.Context, before time.Time) (int64, error)
	SaveOAuthSession(ctx context.Context, session *domain.OAuthSessionState) error
	GetOAuthSession(ctx context.Context, sessionID string) (*domain.OAuthSessionState, error)
	DeleteOAuthSession(ctx context.Context, sessionID string) error
	GetAndDeleteOAuthSession(ctx context.Context, sessionID string) (*domain.OAuthSessionState, error)
	PurgeExpiredOAuthSessions(ctx context.Context, before time.Time) (int64, error)
	AcquireRefreshLock(ctx context.Context, lock *domain.RefreshLock) (bool, error)
	ReleaseRefreshLock(ctx context.Context, accountID, lockID string) error
	PurgeExpiredRefreshLocks(ctx context.Context, before time.Time) (int64, error)

	// Customers
	CreateUser(ctx context.Context, u *domain.User) error
	GetUser(ctx context.Context, id string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByReferralCode(ctx context.Context, code string) (*domain.User, error)
	ListUsers(ctx context.Context) ([]*domain.User, error)
	DeleteUser(ctx context.Context, id string) error
	UpdateUserStatus(ctx context.Context, id, status string) error
	UpdateUserPolicy(ctx context.Context, id string, allowedSurface domain.Surface, boundAccountID string) error
	UpdateUserLastLogin(ctx context.Context, id string) error
	MarkUserEmailVerified(ctx context.Context, id string, verifiedAt time.Time) error

	// API keys
	CreateAPIKey(ctx context.Context, key *domain.APIKey) error
	GetAPIKey(ctx context.Context, id string) (*domain.APIKey, error)
	GetAPIKeyByTokenHash(ctx context.Context, tokenHash string) (*domain.APIKey, *domain.User, error)
	ListAPIKeysByUser(ctx context.Context, userID string) ([]*domain.APIKey, error)
	DeleteAPIKey(ctx context.Context, id string) error
	UpdateAPIKey(ctx context.Context, key *domain.APIKey) error
	UpdateAPIKeyStatus(ctx context.Context, id, status string) error
	UpdateAPIKeyLastUsed(ctx context.Context, id string) error

	// Customer browser sessions and email verification
	CreateWebSession(ctx context.Context, session *domain.WebSession) error
	GetWebSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.WebSession, *domain.User, error)
	DeleteWebSessionByTokenHash(ctx context.Context, tokenHash string) error
	TouchWebSession(ctx context.Context, id string, now time.Time) error
	CreateEmailVerification(ctx context.Context, ev *domain.EmailVerification) error
	GetEmailVerificationByTokenHash(ctx context.Context, tokenHash string) (*domain.EmailVerification, error)
	ConsumeEmailVerification(ctx context.Context, id string, consumedAt time.Time) error
	DeletePendingEmailVerifications(ctx context.Context, userID, purpose string) error
	CountEmailVerificationsSince(ctx context.Context, userID, purpose string, since time.Time) (int, error)
	LastEmailVerification(ctx context.Context, userID, purpose string) (*domain.EmailVerification, error)
	SaveSecurityEvent(ctx context.Context, event *domain.SecurityEvent) error
	CountSecurityEvents(ctx context.Context, q domain.SecurityEventQuery) (int, error)

	// Billing, payments, and admission
	UpsertBillingSetting(ctx context.Context, key, value string, updatedAt time.Time) error
	GetBillingSetting(ctx context.Context, key string) (string, error)
	UpsertRuntimeSetting(ctx context.Context, setting *domain.RuntimeSetting) error
	GetRuntimeSetting(ctx context.Context, key string) (*domain.RuntimeSetting, error)
	ListRuntimeSettings(ctx context.Context) ([]*domain.RuntimeSetting, error)
	SaveIntegration(ctx context.Context, integration *domain.Integration) error
	GetIntegration(ctx context.Context, id string) (*domain.Integration, error)
	ListIntegrations(ctx context.Context, kind string) ([]*domain.Integration, error)
	ListEnabledIntegrations(ctx context.Context, kind, provider string) ([]*domain.Integration, error)
	SaveIntegrationEvent(ctx context.Context, event *domain.IntegrationEvent) error
	SaveSettingsAudit(ctx context.Context, audit *domain.SettingsAudit) error
	UpsertModelPrice(ctx context.Context, price *domain.ModelPrice) error
	GetModelPrice(ctx context.Context, model string) (*domain.ModelPrice, error)
	ListModelPrices(ctx context.Context) ([]*domain.ModelPrice, error)
	InsertBillingLedgerEntry(ctx context.Context, entry *domain.BillingLedgerEntry) error
	GetBillingLedgerEntryByIdempotencyKey(ctx context.Context, key string) (*domain.BillingLedgerEntry, error)
	ListBillingLedgerByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.BillingLedgerEntry, int, error)
	SummarizeBillingLedgerByUser(ctx context.Context, userID string) (*domain.BillingLedgerSummary, error)
	SumBillingLedgerAfter(ctx context.Context, userID string, afterSeq int64) (int64, int64, error)
	GetBillingBalanceCheckpoint(ctx context.Context, userID string) (*domain.BillingBalanceCheckpoint, error)
	UpsertBillingBalanceCheckpoint(ctx context.Context, checkpoint *domain.BillingBalanceCheckpoint) error
	CreateBillableRequest(ctx context.Context, br *domain.BillableRequest) error
	GetBillableRequest(ctx context.Context, requestID string) (*domain.BillableRequest, error)
	UpdateBillableRequestUsage(ctx context.Context, br *domain.BillableRequest) error
	MarkBillableRequestSettled(ctx context.Context, requestID, ledgerID string, settledAt time.Time) error
	MarkBillableRequestStatus(ctx context.Context, requestID, status, errMsg string) error
	ListUnsettledUsageObservedRequests(ctx context.Context, limit int) ([]*domain.BillableRequest, error)
	SumAPIKeyUsageMicros(ctx context.Context, apiKeyID string, since, until time.Time) (int64, error)
	SavePaymentOrder(ctx context.Context, order *domain.PaymentOrder) error
	GetPaymentOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*domain.PaymentOrder, error)
	ListPaymentOrdersByUser(ctx context.Context, userID string, limit int) ([]*domain.PaymentOrder, error)
	ListPaymentOrders(ctx context.Context, limit int) ([]*domain.PaymentOrder, error)
	SummarizePaymentOrders(ctx context.Context) (*domain.PaymentOrderSummary, error)
	MarkPaymentOrderPaid(ctx context.Context, outTradeNo, zpayTradeNo, paymentType string, paidAt time.Time) error
	FulfillPaymentOrderWithCredit(ctx context.Context, outTradeNo, zpayTradeNo, paymentType string, paidAt time.Time, credit *domain.BillingLedgerEntry) error
	SavePaymentEvent(ctx context.Context, event *domain.PaymentEvent) error
	CreateReferralWithCredits(ctx context.Context, referral *domain.Referral, inviteeCredit, inviterCredit *domain.BillingLedgerEntry) error
	GetReferralByInvitee(ctx context.Context, inviteeUserID string) (*domain.Referral, error)
	ReferralStatsByInviter(ctx context.Context, inviterUserID string) (*domain.ReferralStats, error)
	UpsertAdmissionLimit(ctx context.Context, limit *domain.AdmissionLimit) error
	GetAdmissionLimit(ctx context.Context, scope, scopeID string) (*domain.AdmissionLimit, error)
	ListAdmissionLimits(ctx context.Context) ([]*domain.AdmissionLimit, error)

	// Request log
	InsertRequestLog(ctx context.Context, log *domain.RequestLog) (int64, error)
	QueryRequestLogs(ctx context.Context, opts domain.RequestLogQuery) ([]*domain.RequestLog, int, error)
	QueryRelayOutcomeStats(ctx context.Context, since time.Time) ([]domain.RelayOutcomeStat, error)
	QueryCellRiskStats(ctx context.Context, since time.Time) ([]domain.CellRiskStat, error)
	PurgeOldLogs(ctx context.Context, before time.Time) (int64, error)

	// Dashboard & analytics
	QueryUsagePeriods(ctx context.Context, userID string, loc *time.Location) ([]domain.UsagePeriod, error)
	QueryUserTotalCosts(ctx context.Context) (map[string]float64, error)
	QueryUserTotalCostsByIDs(ctx context.Context, userIDs []string) (map[string]float64, error)
	QueryModelUsage(ctx context.Context, userID string) ([]domain.ModelUsageRow, error)
}
