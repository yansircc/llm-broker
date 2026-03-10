package pool

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (p *Pool) GetSessionBinding(sessionUUID string) (string, bool) {
	b, ok := p.sessions.Get(sessionUUID)
	if !ok {
		return "", false
	}
	return b.AccountID, true
}

func (p *Pool) SetSessionBinding(sessionUUID, accountID string, ttl time.Duration) {
	now := time.Now()
	p.sessions.Set(sessionUUID, SessionBinding{
		AccountID:  accountID,
		CreatedAt:  now,
		LastUsedAt: now,
	}, ttl)
}

func (p *Pool) RenewSessionBinding(sessionUUID string, ttl time.Duration) {
	p.sessions.Update(sessionUUID, func(b *SessionBinding) {
		b.LastUsedAt = time.Now()
	}, ttl)
}

func (p *Pool) ListSessionBindingsForAccount(accountID string) []domain.SessionBindingInfo {
	entries := p.sessions.Entries()
	var result []domain.SessionBindingInfo
	for _, e := range entries {
		if e.Value.AccountID == accountID {
			result = append(result, domain.SessionBindingInfo{
				SessionUUID: e.Key,
				AccountID:   e.Value.AccountID,
				CreatedAt:   e.Value.CreatedAt.Format(time.RFC3339),
				LastUsedAt:  e.Value.LastUsedAt.Format(time.RFC3339),
				ExpiresAt:   e.ExpiresAt,
			})
		}
	}
	return result
}

func (p *Pool) UnbindSession(sessionUUID string) {
	p.sessions.Delete(sessionUUID)
}

func (p *Pool) GetStainless(accountID string) (string, bool) {
	return p.stainless.Get(accountID)
}

func (p *Pool) SetStainlessNX(accountID, headersJSON string) bool {
	if _, ok := p.stainless.Get(accountID); ok {
		return false
	}
	p.stainless.Set(accountID, headersJSON, 24*time.Hour)
	return true
}

func (p *Pool) SetOAuthSession(state, data string, ttl time.Duration) {
	p.oauthSessions.Set(state, data, ttl)
}

func (p *Pool) GetDelOAuthSession(state string) (string, bool) {
	return p.oauthSessions.GetAndDelete(state)
}

func (p *Pool) AcquireRefreshLock(accountID, lockID string) bool {
	return p.refreshLocks.SetNX(accountID, lockID, 30*time.Second)
}

func (p *Pool) ReleaseRefreshLock(accountID, lockID string) {
	p.refreshLocks.DeleteIf(accountID, func(held string) bool {
		return held == lockID
	})
}

func (p *Pool) BindStainlessFromRequest(accountID string, reqHeaders http.Header, outHeaders http.Header) {
	stored, ok := p.GetStainless(accountID)

	if ok {
		var headers map[string]string
		if json.Unmarshal([]byte(stored), &headers) == nil {
			for k, v := range headers {
				outHeaders.Set(k, v)
			}
		}
	} else {
		captured := make(map[string]string)
		for _, key := range boundStainlessKeys {
			if v := reqHeaders.Get(key); v != "" {
				captured[key] = v
				outHeaders.Set(key, v)
			}
		}
		if len(captured) > 0 {
			data, _ := json.Marshal(captured)
			if !p.SetStainlessNX(accountID, string(data)) {
				if reread, ok := p.GetStainless(accountID); ok {
					var headers map[string]string
					if json.Unmarshal([]byte(reread), &headers) == nil {
						for k, v := range headers {
							outHeaders.Set(k, v)
						}
					}
				}
			}
		}
	}

	for _, key := range passthroughStainlessKeys {
		if v := reqHeaders.Get(key); v != "" {
			outHeaders.Set(key, v)
		}
	}
}

var boundStainlessKeys = []string{
	"x-stainless-os",
	"x-stainless-arch",
	"x-stainless-runtime",
	"x-stainless-runtime-version",
	"x-stainless-lang",
	"x-stainless-package-version",
}

var passthroughStainlessKeys = []string{
	"x-stainless-retry-count",
	"x-stainless-read-timeout",
}
