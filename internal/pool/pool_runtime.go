package pool

import (
	"context"
	"fmt"
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
	if err := p.refreshState(ctx); err != nil {
		return "", false, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, acct := range p.accounts {
		if acct.Provider == binding.Provider && acct.Subject == binding.Subject {
			return acct.ID, true, nil
		}
	}
	return "", false, nil
}

func (p *Pool) SetSessionBinding(ctx context.Context, sessionUUID string, acct *domain.Account, ttl time.Duration) error {
	if acct == nil || acct.Subject == "" {
		return fmt.Errorf("session binding target missing provider identity")
	}
	now := time.Now().UTC()
	return p.store.SaveSessionBinding(ctx, &domain.SessionBinding{
		SessionUUID: sessionUUID,
		Provider:    acct.Provider,
		Subject:     acct.Subject,
		CreatedAt:   now,
		LastUsedAt:  now,
		ExpiresAt:   now.Add(ttl),
	})
}

func (p *Pool) ListSessionBindingsForAccount(ctx context.Context, accountID string) ([]domain.SessionBindingInfo, error) {
	acct := p.Get(accountID)
	if acct == nil {
		return nil, fmt.Errorf("account %s not found", accountID)
	}
	bindings, err := p.store.ListSessionBindingsByTarget(ctx, acct.Provider, acct.Subject)
	if err != nil {
		return nil, err
	}
	result := make([]domain.SessionBindingInfo, 0, len(bindings))
	for _, binding := range bindings {
		result = append(result, binding.Info(accountID))
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
