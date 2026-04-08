package pool

import (
	"context"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (p *Pool) GetSessionBinding(ctx context.Context, sessionUUID string) (string, bool, error) {
	binding, err := p.store.GetSessionBinding(ctx, sessionUUID)
	if err != nil {
		return "", false, err
	}
	if binding == nil {
		return "", false, nil
	}
	return binding.AccountID, true, nil
}

func (p *Pool) SetSessionBinding(ctx context.Context, sessionUUID, accountID string, ttl time.Duration) error {
	now := time.Now().UTC()
	return p.store.SaveSessionBinding(ctx, &domain.SessionBinding{
		SessionUUID: sessionUUID,
		AccountID:   accountID,
		CreatedAt:   now,
		LastUsedAt:  now,
		ExpiresAt:   now.Add(ttl),
	})
}

func (p *Pool) RenewSessionBinding(ctx context.Context, sessionUUID string, ttl time.Duration) error {
	binding, err := p.store.GetSessionBinding(ctx, sessionUUID)
	if err != nil || binding == nil {
		return err
	}
	now := time.Now().UTC()
	binding.LastUsedAt = now
	binding.ExpiresAt = now.Add(ttl)
	return p.store.SaveSessionBinding(ctx, binding)
}

func (p *Pool) GetUserRouteBinding(ctx context.Context, userID string, provider domain.Provider, surface domain.Surface) (string, bool, error) {
	binding, err := p.store.GetUserRouteBinding(ctx, userID, provider, surface)
	if err != nil {
		return "", false, err
	}
	if binding == nil {
		return "", false, nil
	}
	return binding.AccountID, true, nil
}

func (p *Pool) SetUserRouteBinding(ctx context.Context, userID string, provider domain.Provider, surface domain.Surface, accountID string) error {
	now := time.Now().UTC()
	return p.store.SaveUserRouteBinding(ctx, &domain.UserRouteBinding{
		UserID:     userID,
		Provider:   provider,
		Surface:    surface,
		AccountID:  accountID,
		CreatedAt:  now,
		LastUsedAt: now,
	})
}

func (p *Pool) ListSessionBindingsForAccount(ctx context.Context, accountID string) ([]domain.SessionBindingInfo, error) {
	bindings, err := p.store.ListSessionBindingsByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.SessionBindingInfo, 0, len(bindings))
	for _, binding := range bindings {
		result = append(result, binding.Info())
	}
	return result, nil
}

func (p *Pool) UnbindSession(ctx context.Context, sessionUUID string) error {
	return p.store.DeleteSessionBinding(ctx, sessionUUID)
}

func (p *Pool) GetStainless(ctx context.Context, accountID string) (string, bool, error) {
	binding, err := p.store.GetStainlessBinding(ctx, accountID)
	if err != nil {
		return "", false, err
	}
	if binding == nil {
		return "", false, nil
	}
	return binding.HeadersJSON, true, nil
}

func (p *Pool) SetStainlessNX(ctx context.Context, accountID, headersJSON string, ttl time.Duration) (bool, error) {
	now := time.Now().UTC()
	return p.store.SetStainlessBindingNX(ctx, &domain.StainlessBinding{
		AccountID:   accountID,
		HeadersJSON: headersJSON,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	})
}

func (p *Pool) SetOAuthSession(ctx context.Context, state, data string, ttl time.Duration) error {
	now := time.Now().UTC()
	return p.store.SaveOAuthSession(ctx, &domain.OAuthSessionState{
		SessionID: state,
		DataJSON:  data,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	})
}

func (p *Pool) GetOAuthSession(ctx context.Context, state string) (string, bool, error) {
	session, err := p.store.GetOAuthSession(ctx, state)
	if err != nil {
		return "", false, err
	}
	if session == nil {
		return "", false, nil
	}
	return session.DataJSON, true, nil
}

func (p *Pool) DelOAuthSession(ctx context.Context, state string) error {
	return p.store.DeleteOAuthSession(ctx, state)
}

func (p *Pool) GetDelOAuthSession(ctx context.Context, state string) (string, bool, error) {
	session, err := p.store.GetAndDeleteOAuthSession(ctx, state)
	if err != nil {
		return "", false, err
	}
	if session == nil {
		return "", false, nil
	}
	return session.DataJSON, true, nil
}

func (p *Pool) AcquireRefreshLock(ctx context.Context, accountID, lockID string) (bool, error) {
	now := time.Now().UTC()
	return p.store.AcquireRefreshLock(ctx, &domain.RefreshLock{
		AccountID: accountID,
		LockID:    lockID,
		CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Second),
	})
}

func (p *Pool) ReleaseRefreshLock(ctx context.Context, accountID, lockID string) error {
	return p.store.ReleaseRefreshLock(ctx, accountID, lockID)
}
