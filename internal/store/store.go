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
	GetStainlessBinding(ctx context.Context, accountID string) (*domain.StainlessBinding, error)
	SetStainlessBindingNX(ctx context.Context, binding *domain.StainlessBinding) (bool, error)
	DeleteStainlessBinding(ctx context.Context, accountID string) error
	PurgeExpiredStainlessBindings(ctx context.Context, before time.Time) (int64, error)
	SaveOAuthSession(ctx context.Context, session *domain.OAuthSessionState) error
	GetAndDeleteOAuthSession(ctx context.Context, sessionID string) (*domain.OAuthSessionState, error)
	PurgeExpiredOAuthSessions(ctx context.Context, before time.Time) (int64, error)
	AcquireRefreshLock(ctx context.Context, lock *domain.RefreshLock) (bool, error)
	ReleaseRefreshLock(ctx context.Context, accountID, lockID string) error
	PurgeExpiredRefreshLocks(ctx context.Context, before time.Time) (int64, error)

	// Users
	CreateUser(ctx context.Context, u *domain.User) error
	GetUserByTokenHash(ctx context.Context, tokenHash string) (*domain.User, error)
	ListUsers(ctx context.Context) ([]*domain.User, error)
	DeleteUser(ctx context.Context, id string) error
	UpdateUserStatus(ctx context.Context, id, status string) error
	UpdateUserToken(ctx context.Context, id, tokenHash, tokenPrefix string) error
	UpdateUserPolicy(ctx context.Context, id string, allowedSurface domain.Surface, boundAccountID string) error
	UpdateUserLastActive(ctx context.Context, id string) error

	// Request log
	InsertRequestLog(ctx context.Context, log *domain.RequestLog) error
	QueryRequestLogs(ctx context.Context, opts domain.RequestLogQuery) ([]*domain.RequestLog, int, error)
	PurgeOldLogs(ctx context.Context, before time.Time) (int64, error)

	// Dashboard & analytics
	QueryUsagePeriods(ctx context.Context, userID string, loc *time.Location) ([]domain.UsagePeriod, error)
	QueryUserTotalCosts(ctx context.Context) (map[string]float64, error)
	QueryModelUsage(ctx context.Context, userID string) ([]domain.ModelUsageRow, error)
}
