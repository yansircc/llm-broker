package admission

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/store"
)

type BalanceReader interface {
	Balance(ctx context.Context, userID string) (int64, int64, error)
}

type Service struct {
	store   store.Store
	billing BalanceReader
	now     func() time.Time

	mu         sync.Mutex
	concurrent map[string]int
	recent     map[string][]time.Time
}

type Request struct {
	UserID        string
	APIKeyID      string
	EmailVerified bool
	RewardOnly    bool
}

type Decision struct {
	BalanceMicros int64
	Reason        string
}

type ReleaseFunc func()

func NewService(s store.Store, billing BalanceReader) *Service {
	return &Service{
		store:      s,
		billing:    billing,
		now:        func() time.Time { return time.Now().UTC() },
		concurrent: make(map[string]int),
		recent:     make(map[string][]time.Time),
	}
}

func (s *Service) Admit(ctx context.Context, req Request) (Decision, ReleaseFunc, error) {
	if req.UserID == "" || req.APIKeyID == "" {
		return Decision{Reason: "missing_principal"}, nil, fmt.Errorf("missing principal")
	}
	if !req.EmailVerified {
		return Decision{Reason: "email_unverified"}, nil, fmt.Errorf("email not verified")
	}
	balance, _, err := s.billing.Balance(ctx, req.UserID)
	if err != nil {
		return Decision{Reason: "balance_unavailable"}, nil, err
	}
	limits, err := s.effectiveLimits(ctx, req)
	if err != nil {
		return Decision{BalanceMicros: balance, Reason: "limits_unavailable"}, nil, err
	}
	minBalance := maxInt64(limits.global.MinBalanceMicros, limits.user.MinBalanceMicros, limits.key.MinBalanceMicros)
	if req.RewardOnly {
		minBalance = maxInt64(minBalance, limits.reward.MinBalanceMicros)
	}
	if balance < minBalance {
		slog.Info("admission rejected", "user_id", req.UserID, "api_key_id", req.APIKeyID, "reason", "insufficient_balance", "balance_micros", balance, "min_balance_micros", minBalance)
		return Decision{BalanceMicros: balance, Reason: "insufficient_balance"}, nil, fmt.Errorf("insufficient balance")
	}

	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := []string{"global:", "user:" + req.UserID, "api_key:" + req.APIKeyID}
	if req.RewardOnly {
		keys = append(keys, "reward_only:"+req.UserID)
	}
	if over, key := s.overConcurrent(keys, limits); over {
		slog.Info("admission rejected", "user_id", req.UserID, "api_key_id", req.APIKeyID, "reason", "concurrency_limit", "limit_key", key, "balance_micros", balance)
		return Decision{BalanceMicros: balance, Reason: "concurrency_limit"}, nil, fmt.Errorf("concurrency limit")
	}
	if over, key := s.overRate(now, keys, limits); over {
		slog.Info("admission rejected", "user_id", req.UserID, "api_key_id", req.APIKeyID, "reason", "rate_limit", "limit_key", key, "balance_micros", balance)
		return Decision{BalanceMicros: balance, Reason: "rate_limit"}, nil, fmt.Errorf("rate limit")
	}
	for _, key := range keys {
		s.concurrent[key]++
		s.recent[key] = append(s.recent[key], now)
	}
	released := false
	release := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if released {
			return
		}
		released = true
		for _, key := range keys {
			if s.concurrent[key] > 0 {
				s.concurrent[key]--
			}
		}
	}
	return Decision{BalanceMicros: balance}, release, nil
}

type limits struct {
	global domain.AdmissionLimit
	user   domain.AdmissionLimit
	key    domain.AdmissionLimit
	reward domain.AdmissionLimit
}

func (s *Service) effectiveLimits(ctx context.Context, req Request) (limits, error) {
	global, err := s.limit(ctx, "global", "")
	if err != nil {
		return limits{}, err
	}
	user, err := s.limit(ctx, "user", req.UserID)
	if err != nil {
		return limits{}, err
	}
	key, err := s.limit(ctx, "api_key", req.APIKeyID)
	if err != nil {
		return limits{}, err
	}
	reward, err := s.limit(ctx, "reward_only", "")
	if err != nil {
		return limits{}, err
	}
	return limits{global: global, user: user, key: key, reward: reward}, nil
}

func (s *Service) limit(ctx context.Context, scope, scopeID string) (domain.AdmissionLimit, error) {
	limit, err := s.store.GetAdmissionLimit(ctx, scope, scopeID)
	if err != nil {
		return domain.AdmissionLimit{}, err
	}
	if limit == nil && scopeID != "" {
		limit, err = s.store.GetAdmissionLimit(ctx, scope, "")
	}
	if err != nil {
		return domain.AdmissionLimit{}, err
	}
	if limit == nil {
		return domain.AdmissionLimit{Scope: scope, ScopeID: scopeID, MinBalanceMicros: 1}, nil
	}
	return *limit, nil
}

func (s *Service) overConcurrent(keys []string, limits limits) (bool, string) {
	checks := []struct {
		key   string
		limit int
	}{
		{"global:", limits.global.MaxConcurrent},
		{keys[1], limits.user.MaxConcurrent},
		{keys[2], limits.key.MaxConcurrent},
	}
	if len(keys) > 3 {
		checks = append(checks, struct {
			key   string
			limit int
		}{keys[3], limits.reward.MaxConcurrent})
	}
	for _, check := range checks {
		if check.limit > 0 && s.concurrent[check.key] >= check.limit {
			return true, check.key
		}
	}
	return false, ""
}

func (s *Service) overRate(now time.Time, keys []string, limits limits) (bool, string) {
	checks := []struct {
		key   string
		limit int
	}{
		{"global:", limits.global.RequestsPerMinute},
		{keys[1], limits.user.RequestsPerMinute},
		{keys[2], limits.key.RequestsPerMinute},
	}
	cutoff := now.Add(-time.Minute)
	for _, check := range checks {
		if check.limit <= 0 {
			continue
		}
		recent := s.recent[check.key]
		keep := recent[:0]
		for _, ts := range recent {
			if ts.After(cutoff) {
				keep = append(keep, ts)
			}
		}
		s.recent[check.key] = keep
		if len(keep) >= check.limit {
			return true, check.key
		}
	}
	return false, ""
}

func maxInt64(values ...int64) int64 {
	var out int64
	for _, value := range values {
		if value > out {
			out = value
		}
	}
	return out
}
