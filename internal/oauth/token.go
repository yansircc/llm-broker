package oauth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yansir/cc-relayer/internal/crypto"
	"github.com/yansir/cc-relayer/internal/domain"
)

const claudeSalt = "salt"

// PoolAccess provides the methods TokenManager needs from Pool.
type PoolAccess interface {
	GetTokenInfo(accountID string) (refreshEnc, accessEnc string, expiresAt int64, ok bool)
	GetProvider(accountID string) (domain.Provider, bool)
	GetProxy(accountID string) *domain.ProxyConfig
	StoreTokens(accountID, accessTokenEnc, refreshTokenEnc string, expiresAt int64) error
	MarkError(accountID, msg string)
	AcquireRefreshLock(accountID, lockID string) bool
	ReleaseRefreshLock(accountID, lockID string)
}

// TransportProvider returns per-account HTTP transports for proxy support.
type TransportProvider interface {
	GetHTTPTransport(proxy *domain.ProxyConfig) *http.Transport
}

// TokenManager handles OAuth token refresh with locking.
type TokenManager struct {
	pool                PoolAccess
	crypto              *crypto.Crypto
	transport           TransportProvider
	client              *http.Client
	tokenRefreshAdvance time.Duration
}

// NewTokenManager creates a token manager.
func NewTokenManager(pool PoolAccess, c *crypto.Crypto, tp TransportProvider, refreshAdvance time.Duration) *TokenManager {
	return &TokenManager{
		pool:                pool,
		crypto:              c,
		transport:           tp,
		client:              &http.Client{Timeout: 30 * time.Second},
		tokenRefreshAdvance: refreshAdvance,
	}
}

// EnsureValidToken checks if the account's access token is valid.
// If expired (within advance window), triggers a refresh.
// Returns the decrypted access token.
func (tm *TokenManager) EnsureValidToken(ctx context.Context, accountID string) (string, error) {
	refreshEnc, accessEnc, expiresAt, ok := tm.pool.GetTokenInfo(accountID)
	if !ok {
		return "", fmt.Errorf("account %s not found", accountID)
	}

	now := time.Now().UnixMilli()

	// Token still valid
	if expiresAt > 0 && now < expiresAt-tm.tokenRefreshAdvance.Milliseconds() {
		if accessEnc != "" {
			token, err := tm.crypto.Decrypt(accessEnc, claudeSalt)
			if err != nil {
				return "", fmt.Errorf("decrypt access token: %w", err)
			}
			if token != "" {
				return token, nil
			}
		}
	}

	// Token expired or about to expire — refresh
	_ = refreshEnc // will be read again in refresh()
	return tm.refresh(ctx, accountID)
}

// refresh performs the OAuth token refresh with locking.
func (tm *TokenManager) refresh(ctx context.Context, accountID string) (string, error) {
	lockID := uuid.New().String()

	acquired := tm.pool.AcquireRefreshLock(accountID, lockID)
	if !acquired {
		// Another goroutine is refreshing — wait and re-read
		slog.Info("token refresh locked, waiting", "accountId", accountID)
		time.Sleep(2 * time.Second)

		_, accessEnc, expiresAt, ok := tm.pool.GetTokenInfo(accountID)
		if !ok {
			return "", fmt.Errorf("account not found after wait")
		}
		if accessEnc != "" && expiresAt > time.Now().UnixMilli() {
			token, err := tm.crypto.Decrypt(accessEnc, claudeSalt)
			if err == nil && token != "" {
				return token, nil
			}
		}
		return "", fmt.Errorf("token refresh in progress by another process")
	}

	defer tm.pool.ReleaseRefreshLock(accountID, lockID)

	refreshEnc, _, _, ok := tm.pool.GetTokenInfo(accountID)
	if !ok {
		return "", fmt.Errorf("account %s not found", accountID)
	}

	refreshToken, err := tm.crypto.Decrypt(refreshEnc, claudeSalt)
	if err != nil {
		tm.markError(accountID, "decrypt refresh token: "+err.Error())
		return "", fmt.Errorf("decrypt refresh token: %w", err)
	}
	if refreshToken == "" {
		tm.markError(accountID, "empty refresh token")
		return "", fmt.Errorf("empty refresh token for account %s", accountID)
	}

	slog.Info("refreshing token", "accountId", accountID)

	provider, _ := tm.pool.GetProvider(accountID)
	var tokenResp *TokenResponse
	if provider == domain.ProviderCodex {
		tokenResp, err = CallCodexRefresh(ctx, tm.client, refreshToken)
	} else {
		// Use account-specific proxy transport for Claude refresh
		client := tm.client
		if tm.transport != nil {
			proxy := tm.pool.GetProxy(accountID)
			if proxy != nil {
				client = &http.Client{
					Transport: tm.transport.GetHTTPTransport(proxy),
					Timeout:   30 * time.Second,
				}
			}
		}
		tokenResp, err = CallClaudeRefresh(ctx, client, refreshToken)
	}
	if err != nil {
		tm.markError(accountID, err.Error())
		return "", fmt.Errorf("oauth refresh: %w", err)
	}

	// Encrypt new tokens
	encAccess, err := tm.crypto.Encrypt(tokenResp.AccessToken, claudeSalt)
	if err != nil {
		return "", fmt.Errorf("encrypt access token: %w", err)
	}
	encRefresh, err := tm.crypto.Encrypt(tokenResp.RefreshToken, claudeSalt)
	if err != nil {
		return "", fmt.Errorf("encrypt refresh token: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()

	if err := tm.pool.StoreTokens(accountID, encAccess, encRefresh, expiresAt); err != nil {
		return "", fmt.Errorf("store tokens: %w", err)
	}

	slog.Info("token refreshed", "accountId", accountID, "expiresIn", tokenResp.ExpiresIn)
	return tokenResp.AccessToken, nil
}

func (tm *TokenManager) markError(accountID, msg string) {
	slog.Error("token refresh failed", "accountId", accountID, "error", msg)
	tm.pool.MarkError(accountID, msg)
}

// ForceRefresh triggers an immediate token refresh, ignoring expiry.
func (tm *TokenManager) ForceRefresh(ctx context.Context, accountID string) (string, error) {
	return tm.refresh(ctx, accountID)
}

// EncryptToken encrypts a token for storage (used during account creation).
func (tm *TokenManager) EncryptToken(token string) (string, error) {
	return tm.crypto.Encrypt(token, claudeSalt)
}

// DecryptToken decrypts a stored token (used for admin display).
func (tm *TokenManager) DecryptToken(encrypted string) (string, error) {
	return tm.crypto.Decrypt(encrypted, claudeSalt)
}
