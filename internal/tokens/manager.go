package tokens

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/crypto"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

const tokenSalt = "salt"

// PoolAccess provides the methods Manager needs from Pool.
type PoolAccess interface {
	Get(accountID string) *domain.Account
	StoreTokens(accountID, accessTokenEnc, refreshTokenEnc string, expiresAt int64) error
	MarkError(accountID, msg string)
	AcquireRefreshLock(accountID, lockID string) bool
	ReleaseRefreshLock(accountID, lockID string)
}

// TransportProvider supplies plain proxy transports for refresh flows.
type TransportProvider interface {
	TransportForProxy(proxy *domain.ProxyConfig) *http.Transport
}

// Manager handles OAuth token refresh with locking.
type Manager struct {
	pool                PoolAccess
	crypto              *crypto.Crypto
	transport           TransportProvider
	drivers             map[domain.Provider]driver.RefreshDriver
	client              *http.Client
	tokenRefreshAdvance time.Duration
}

// NewManager creates a token manager.
func NewManager(pool PoolAccess, c *crypto.Crypto, tp TransportProvider, refreshAdvance time.Duration, drivers map[domain.Provider]driver.RefreshDriver) *Manager {
	return &Manager{
		pool:                pool,
		crypto:              c,
		transport:           tp,
		drivers:             drivers,
		client:              &http.Client{Timeout: 30 * time.Second},
		tokenRefreshAdvance: refreshAdvance,
	}
}

// EnsureValidToken checks if the account's access token is valid.
// If expired (within advance window), triggers a refresh.
// Returns the decrypted access token.
func (tm *Manager) EnsureValidToken(ctx context.Context, accountID string) (string, error) {
	acct := tm.pool.Get(accountID)
	if acct == nil {
		return "", fmt.Errorf("account %s not found", accountID)
	}

	now := time.Now().UnixMilli()

	if acct.ExpiresAt > 0 && now < acct.ExpiresAt-tm.tokenRefreshAdvance.Milliseconds() {
		if acct.AccessTokenEnc != "" {
			token, err := tm.crypto.Decrypt(acct.AccessTokenEnc, tokenSalt)
			if err != nil {
				return "", fmt.Errorf("decrypt access token: %w", err)
			}
			if token != "" {
				return token, nil
			}
		}
	}

	return tm.refresh(ctx, accountID)
}

func (tm *Manager) refresh(ctx context.Context, accountID string) (string, error) {
	lockID := uuid.New().String()

	acquired := tm.pool.AcquireRefreshLock(accountID, lockID)
	if !acquired {
		slog.Info("token refresh locked, waiting", "accountId", accountID)
		time.Sleep(2 * time.Second)

		acct := tm.pool.Get(accountID)
		if acct == nil {
			return "", fmt.Errorf("account not found after wait")
		}
		if acct.AccessTokenEnc != "" && acct.ExpiresAt > time.Now().UnixMilli() {
			token, err := tm.crypto.Decrypt(acct.AccessTokenEnc, tokenSalt)
			if err == nil && token != "" {
				return token, nil
			}
		}
		return "", fmt.Errorf("token refresh in progress by another process")
	}

	defer tm.pool.ReleaseRefreshLock(accountID, lockID)

	acct := tm.pool.Get(accountID)
	if acct == nil {
		return "", fmt.Errorf("account %s not found", accountID)
	}

	refreshToken, err := tm.crypto.Decrypt(acct.RefreshTokenEnc, tokenSalt)
	if err != nil {
		tm.markError(accountID, "decrypt refresh token: "+err.Error())
		return "", fmt.Errorf("decrypt refresh token: %w", err)
	}
	if refreshToken == "" {
		tm.markError(accountID, "empty refresh token")
		return "", fmt.Errorf("empty refresh token for account %s", accountID)
	}

	slog.Info("refreshing token", "accountId", accountID)

	if acct.Provider == "" {
		tm.markError(accountID, "unknown provider")
		return "", fmt.Errorf("unknown provider for account %s", accountID)
	}

	drv, ok := tm.drivers[acct.Provider]
	if !ok {
		tm.markError(accountID, "no refresh driver")
		return "", fmt.Errorf("no refresh driver for provider %s", acct.Provider)
	}

	client := tm.client
	if tm.transport != nil && acct.Proxy != nil {
		client = &http.Client{
			Transport: tm.transport.TransportForProxy(acct.Proxy),
			Timeout:   30 * time.Second,
		}
	}

	tokenResp, err := drv.RefreshToken(ctx, client, refreshToken)
	if err != nil {
		tm.markError(accountID, err.Error())
		return "", fmt.Errorf("oauth refresh: %w", err)
	}

	encAccess, err := tm.crypto.Encrypt(tokenResp.AccessToken, tokenSalt)
	if err != nil {
		return "", fmt.Errorf("encrypt access token: %w", err)
	}
	encRefresh := acct.RefreshTokenEnc
	if tokenResp.RefreshToken != "" {
		encRefresh, err = tm.crypto.Encrypt(tokenResp.RefreshToken, tokenSalt)
		if err != nil {
			return "", fmt.Errorf("encrypt refresh token: %w", err)
		}
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()
	if err := tm.pool.StoreTokens(accountID, encAccess, encRefresh, expiresAt); err != nil {
		return "", fmt.Errorf("store tokens: %w", err)
	}

	slog.Info("token refreshed", "accountId", accountID, "expiresIn", tokenResp.ExpiresIn)
	return tokenResp.AccessToken, nil
}

func (tm *Manager) markError(accountID, msg string) {
	slog.Error("token refresh failed", "accountId", accountID, "error", msg)
	tm.pool.MarkError(accountID, msg)
}

// ForceRefresh triggers an immediate token refresh, ignoring expiry.
func (tm *Manager) ForceRefresh(ctx context.Context, accountID string) (string, error) {
	return tm.refresh(ctx, accountID)
}

// EncryptToken encrypts a token for storage (used during account creation).
func (tm *Manager) EncryptToken(token string) (string, error) {
	return tm.crypto.Encrypt(token, tokenSalt)
}
