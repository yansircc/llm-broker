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
// Ephemeral state (sessions, stainless, locks, OAuth) lives in Pool or Server memory.
type Store interface {
	Ping(ctx context.Context) error
	Close() error

	// Accounts — typed structs
	GetAccount(ctx context.Context, id string) (*domain.Account, error)
	ListAccounts(ctx context.Context) ([]*domain.Account, error)
	SaveAccount(ctx context.Context, acct *domain.Account) error // UPSERT
	DeleteAccount(ctx context.Context, id string) error
	GetQuotaBucket(ctx context.Context, bucketKey string) (*domain.QuotaBucket, error)
	ListQuotaBuckets(ctx context.Context) ([]*domain.QuotaBucket, error)
	SaveQuotaBucket(ctx context.Context, bucket *domain.QuotaBucket) error
	DeleteQuotaBucket(ctx context.Context, bucketKey string) error

	// Users
	CreateUser(ctx context.Context, u *domain.User) error
	GetUserByTokenHash(ctx context.Context, tokenHash string) (*domain.User, error)
	ListUsers(ctx context.Context) ([]*domain.User, error)
	DeleteUser(ctx context.Context, id string) error
	UpdateUserStatus(ctx context.Context, id, status string) error
	UpdateUserToken(ctx context.Context, id, tokenHash, tokenPrefix string) error
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
