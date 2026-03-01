package store

import (
	"context"
	"sync"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
)

// MockStore implements Store for testing.
type MockStore struct {
	mu       sync.Mutex
	accounts map[string]*domain.Account
	users    map[string]*domain.User
	logs     []*domain.RequestLog

	// Error injection
	SaveAccountErr    error
	ListAccountsErr   error
	GetAccountErr     error
	DeleteAccountErr  error
	CreateUserErr     error
	InsertRequestErr  error
}

func NewMockStore() *MockStore {
	return &MockStore{
		accounts: make(map[string]*domain.Account),
		users:    make(map[string]*domain.User),
	}
}

func (m *MockStore) Ping(_ context.Context) error { return nil }
func (m *MockStore) Close() error                  { return nil }

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
	delete(m.users, id)
	return nil
}

func (m *MockStore) UpdateUserStatus(_ context.Context, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		u.Status = status
	}
	return nil
}

func (m *MockStore) UpdateUserToken(_ context.Context, id, tokenHash, tokenPrefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		u.TokenHash = tokenHash
		u.TokenPrefix = tokenPrefix
	}
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
