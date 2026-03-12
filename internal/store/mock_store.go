package store

import (
	"context"
	"sync"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

// MockStore implements Store for testing.
type MockStore struct {
	mu              sync.Mutex
	accounts        map[string]*domain.Account
	cells           map[string]*domain.EgressCell
	buckets         map[string]*domain.QuotaBucket
	sessionBindings map[string]*domain.SessionBinding
	stainless       map[string]*domain.StainlessBinding
	oauthSessions   map[string]*domain.OAuthSessionState
	refreshLocks    map[string]*domain.RefreshLock
	users           map[string]*domain.User
	logs            []*domain.RequestLog

	// Error injection
	SaveAccountErr   error
	ListAccountsErr  error
	GetAccountErr    error
	DeleteAccountErr error
	CreateUserErr    error
	InsertRequestErr error
}

func NewMockStore() *MockStore {
	return &MockStore{
		accounts:        make(map[string]*domain.Account),
		cells:           make(map[string]*domain.EgressCell),
		buckets:         make(map[string]*domain.QuotaBucket),
		sessionBindings: make(map[string]*domain.SessionBinding),
		stainless:       make(map[string]*domain.StainlessBinding),
		oauthSessions:   make(map[string]*domain.OAuthSessionState),
		refreshLocks:    make(map[string]*domain.RefreshLock),
		users:           make(map[string]*domain.User),
	}
}

func (m *MockStore) Ping(_ context.Context) error { return nil }
func (m *MockStore) Close() error                 { return nil }

func (m *MockStore) GetAccount(_ context.Context, id string) (*domain.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetAccountErr != nil {
		return nil, m.GetAccountErr
	}
	a, ok := m.accounts[id]
	if !ok {
		return nil, nil
	}
	copy := *a
	return &copy, nil
}

func (m *MockStore) ListAccounts(_ context.Context) ([]*domain.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ListAccountsErr != nil {
		return nil, m.ListAccountsErr
	}
	result := make([]*domain.Account, 0, len(m.accounts))
	for _, a := range m.accounts {
		copy := *a
		result = append(result, &copy)
	}
	return result, nil
}

func (m *MockStore) SaveAccount(_ context.Context, acct *domain.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SaveAccountErr != nil {
		return m.SaveAccountErr
	}
	copy := *acct
	m.accounts[acct.ID] = &copy
	return nil
}

func (m *MockStore) DeleteAccount(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.DeleteAccountErr != nil {
		return m.DeleteAccountErr
	}
	delete(m.accounts, id)
	return nil
}

func (m *MockStore) GetEgressCell(_ context.Context, id string) (*domain.EgressCell, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cell, ok := m.cells[id]
	if !ok {
		return nil, nil
	}
	copy := *cell
	return &copy, nil
}

func (m *MockStore) ListEgressCells(_ context.Context) ([]*domain.EgressCell, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*domain.EgressCell, 0, len(m.cells))
	for _, cell := range m.cells {
		copy := *cell
		result = append(result, &copy)
	}
	return result, nil
}

func (m *MockStore) SaveEgressCell(_ context.Context, cell *domain.EgressCell) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *cell
	m.cells[cell.ID] = &copy
	return nil
}

func (m *MockStore) DeleteEgressCell(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cells, id)
	return nil
}

func (m *MockStore) GetQuotaBucket(_ context.Context, bucketKey string) (*domain.QuotaBucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.buckets[bucketKey]
	if !ok {
		return nil, nil
	}
	copy := *b
	return &copy, nil
}

func (m *MockStore) ListQuotaBuckets(_ context.Context) ([]*domain.QuotaBucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*domain.QuotaBucket, 0, len(m.buckets))
	for _, b := range m.buckets {
		copy := *b
		result = append(result, &copy)
	}
	return result, nil
}

func (m *MockStore) SaveQuotaBucket(_ context.Context, bucket *domain.QuotaBucket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *bucket
	m.buckets[bucket.BucketKey] = &copy
	return nil
}

func (m *MockStore) DeleteQuotaBucket(_ context.Context, bucketKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.buckets, bucketKey)
	return nil
}

func (m *MockStore) GetSessionBinding(_ context.Context, sessionUUID string) (*domain.SessionBinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	binding, ok := m.sessionBindings[sessionUUID]
	if !ok || !binding.ExpiresAt.After(time.Now()) {
		return nil, nil
	}
	copy := *binding
	return &copy, nil
}

func (m *MockStore) ListSessionBindingsByAccount(_ context.Context, accountID string) ([]domain.SessionBinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	var result []domain.SessionBinding
	for _, binding := range m.sessionBindings {
		if binding.AccountID != accountID || !binding.ExpiresAt.After(now) {
			continue
		}
		result = append(result, *binding)
	}
	return result, nil
}

func (m *MockStore) SaveSessionBinding(_ context.Context, binding *domain.SessionBinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *binding
	m.sessionBindings[binding.SessionUUID] = &copy
	return nil
}

func (m *MockStore) DeleteSessionBinding(_ context.Context, sessionUUID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessionBindings, sessionUUID)
	return nil
}

func (m *MockStore) PurgeExpiredSessionBindings(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var purged int64
	for sessionUUID, binding := range m.sessionBindings {
		if !binding.ExpiresAt.After(before) {
			delete(m.sessionBindings, sessionUUID)
			purged++
		}
	}
	return purged, nil
}

func (m *MockStore) GetStainlessBinding(_ context.Context, accountID string) (*domain.StainlessBinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	binding, ok := m.stainless[accountID]
	if !ok || !binding.ExpiresAt.After(time.Now()) {
		return nil, nil
	}
	copy := *binding
	return &copy, nil
}

func (m *MockStore) SetStainlessBindingNX(_ context.Context, binding *domain.StainlessBinding) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.stainless[binding.AccountID]; ok && existing.ExpiresAt.After(binding.CreatedAt) {
		return false, nil
	}
	copy := *binding
	m.stainless[binding.AccountID] = &copy
	return true, nil
}

func (m *MockStore) DeleteStainlessBinding(_ context.Context, accountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.stainless, accountID)
	return nil
}

func (m *MockStore) PurgeExpiredStainlessBindings(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var purged int64
	for accountID, binding := range m.stainless {
		if !binding.ExpiresAt.After(before) {
			delete(m.stainless, accountID)
			purged++
		}
	}
	return purged, nil
}

func (m *MockStore) SaveOAuthSession(_ context.Context, session *domain.OAuthSessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *session
	m.oauthSessions[session.SessionID] = &copy
	return nil
}

func (m *MockStore) GetAndDeleteOAuthSession(_ context.Context, sessionID string) (*domain.OAuthSessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.oauthSessions[sessionID]
	if !ok || !session.ExpiresAt.After(time.Now()) {
		return nil, nil
	}
	copy := *session
	delete(m.oauthSessions, sessionID)
	return &copy, nil
}

func (m *MockStore) PurgeExpiredOAuthSessions(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var purged int64
	for sessionID, session := range m.oauthSessions {
		if !session.ExpiresAt.After(before) {
			delete(m.oauthSessions, sessionID)
			purged++
		}
	}
	return purged, nil
}

func (m *MockStore) AcquireRefreshLock(_ context.Context, lock *domain.RefreshLock) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.refreshLocks[lock.AccountID]; ok && existing.ExpiresAt.After(lock.CreatedAt) {
		return false, nil
	}
	copy := *lock
	m.refreshLocks[lock.AccountID] = &copy
	return true, nil
}

func (m *MockStore) ReleaseRefreshLock(_ context.Context, accountID, lockID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.refreshLocks[accountID]; ok && existing.LockID == lockID {
		delete(m.refreshLocks, accountID)
	}
	return nil
}

func (m *MockStore) PurgeExpiredRefreshLocks(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var purged int64
	for accountID, lock := range m.refreshLocks {
		if !lock.ExpiresAt.After(before) {
			delete(m.refreshLocks, accountID)
			purged++
		}
	}
	return purged, nil
}

func (m *MockStore) CreateUser(_ context.Context, u *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.CreateUserErr != nil {
		return m.CreateUserErr
	}
	copy := *u
	m.users[u.ID] = &copy
	return nil
}

func (m *MockStore) GetUserByTokenHash(_ context.Context, hash string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.TokenHash == hash {
			copy := *u
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *MockStore) ListUsers(_ context.Context) ([]*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*domain.User, 0, len(m.users))
	for _, u := range m.users {
		copy := *u
		result = append(result, &copy)
	}
	return result, nil
}

func (m *MockStore) DeleteUser(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[id]; !ok {
		return ErrNotFound
	}
	delete(m.users, id)
	return nil
}

func (m *MockStore) UpdateUserStatus(_ context.Context, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrNotFound
	}
	u.Status = status
	return nil
}

func (m *MockStore) UpdateUserToken(_ context.Context, id, tokenHash, tokenPrefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrNotFound
	}
	u.TokenHash = tokenHash
	u.TokenPrefix = tokenPrefix
	return nil
}

func (m *MockStore) UpdateUserPolicy(_ context.Context, id string, allowedSurface domain.Surface, boundAccountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrNotFound
	}
	if allowedSurface == "" {
		allowedSurface = domain.SurfaceNative
	}
	u.AllowedSurface = allowedSurface
	u.BoundAccountID = boundAccountID
	return nil
}

func (m *MockStore) UpdateUserLastActive(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		now := time.Now()
		u.LastActiveAt = &now
	}
	return nil
}

func (m *MockStore) InsertRequestLog(_ context.Context, log *domain.RequestLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.InsertRequestErr != nil {
		return m.InsertRequestErr
	}
	m.logs = append(m.logs, log)
	return nil
}

func (m *MockStore) QueryRequestLogs(_ context.Context, _ domain.RequestLogQuery) ([]*domain.RequestLog, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs, len(m.logs), nil
}

func (m *MockStore) PurgeOldLogs(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (m *MockStore) QueryUsagePeriods(_ context.Context, _ string, _ *time.Location) ([]domain.UsagePeriod, error) {
	return nil, nil
}

func (m *MockStore) QueryUserTotalCosts(_ context.Context) (map[string]float64, error) {
	return nil, nil
}

func (m *MockStore) QueryModelUsage(_ context.Context, _ string) ([]domain.ModelUsageRow, error) {
	return nil, nil
}
