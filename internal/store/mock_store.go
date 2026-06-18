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
	userRouteBinds  map[string]*domain.UserRouteBinding
	stainless       map[string]*domain.StainlessBinding
	oauthSessions   map[string]*domain.OAuthSessionState
	refreshLocks    map[string]*domain.RefreshLock
	users           map[string]*domain.User
	apiKeys         map[string]*domain.APIKey
	webSessions     map[string]*domain.WebSession
	emailTokens     map[string]*domain.EmailVerification
	ledger          []*domain.BillingLedgerEntry
	checkpoints     map[string]*domain.BillingBalanceCheckpoint
	modelPrices     map[string]*domain.ModelPrice
	billingSettings map[string]string
	billable        map[string]*domain.BillableRequest
	paymentOrders   map[string]*domain.PaymentOrder
	paymentEvents   []*domain.PaymentEvent
	referrals       map[string]*domain.Referral
	admissionLimits map[string]*domain.AdmissionLimit
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
		userRouteBinds:  make(map[string]*domain.UserRouteBinding),
		stainless:       make(map[string]*domain.StainlessBinding),
		oauthSessions:   make(map[string]*domain.OAuthSessionState),
		refreshLocks:    make(map[string]*domain.RefreshLock),
		users:           make(map[string]*domain.User),
		apiKeys:         make(map[string]*domain.APIKey),
		webSessions:     make(map[string]*domain.WebSession),
		emailTokens:     make(map[string]*domain.EmailVerification),
		checkpoints:     make(map[string]*domain.BillingBalanceCheckpoint),
		modelPrices:     make(map[string]*domain.ModelPrice),
		billingSettings: make(map[string]string),
		billable:        make(map[string]*domain.BillableRequest),
		paymentOrders:   make(map[string]*domain.PaymentOrder),
		referrals:       make(map[string]*domain.Referral),
		admissionLimits: make(map[string]*domain.AdmissionLimit),
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

func userRouteBindingKey(userID string, provider domain.Provider, surface domain.Surface) string {
	return userID + "|" + string(provider) + "|" + string(domain.NormalizeSurface(string(surface)))
}

func (m *MockStore) GetUserRouteBinding(_ context.Context, userID string, provider domain.Provider, surface domain.Surface) (*domain.UserRouteBinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	binding, ok := m.userRouteBinds[userRouteBindingKey(userID, provider, surface)]
	if !ok {
		return nil, nil
	}
	copy := *binding
	return &copy, nil
}

func (m *MockStore) SaveUserRouteBinding(_ context.Context, binding *domain.UserRouteBinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *binding
	copy.Surface = domain.NormalizeSurface(string(copy.Surface))
	m.userRouteBinds[userRouteBindingKey(copy.UserID, copy.Provider, copy.Surface)] = &copy
	return nil
}

func (m *MockStore) DeleteUserRouteBindingsByUser(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, binding := range m.userRouteBinds {
		if binding.UserID == userID {
			delete(m.userRouteBinds, key)
		}
	}
	return nil
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

func (m *MockStore) GetOAuthSession(_ context.Context, sessionID string) (*domain.OAuthSessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.oauthSessions[sessionID]
	if !ok || !session.ExpiresAt.After(time.Now()) {
		return nil, nil
	}
	copy := *session
	return &copy, nil
}

func (m *MockStore) DeleteOAuthSession(_ context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.oauthSessions, sessionID)
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

func (m *MockStore) GetUser(_ context.Context, id string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	copy := *u
	return &copy, nil
}

func (m *MockStore) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email {
			copy := *u
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *MockStore) GetUserByReferralCode(_ context.Context, code string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.ReferralCode == code {
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
	for key, binding := range m.userRouteBinds {
		if binding.UserID == id {
			delete(m.userRouteBinds, key)
		}
	}
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

func (m *MockStore) UpdateUserLastLogin(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		now := time.Now()
		u.LastLoginAt = &now
	}
	return nil
}

func (m *MockStore) MarkUserEmailVerified(_ context.Context, id string, verifiedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrNotFound
	}
	u.EmailVerifiedAt = &verifiedAt
	return nil
}

func (m *MockStore) CreateAPIKey(_ context.Context, key *domain.APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *key
	m.apiKeys[key.ID] = &copy
	return nil
}

func (m *MockStore) GetAPIKeyByTokenHash(_ context.Context, tokenHash string) (*domain.APIKey, *domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range m.apiKeys {
		if key.TokenHash != tokenHash {
			continue
		}
		user := m.users[key.UserID]
		if user == nil {
			return nil, nil, nil
		}
		keyCopy := *key
		userCopy := *user
		return &keyCopy, &userCopy, nil
	}
	return nil, nil, nil
}

func (m *MockStore) ListAPIKeysByUser(_ context.Context, userID string) ([]*domain.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keys []*domain.APIKey
	for _, key := range m.apiKeys {
		if key.UserID != userID {
			continue
		}
		copy := *key
		keys = append(keys, &copy)
	}
	return keys, nil
}

func (m *MockStore) DeleteAPIKey(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apiKeys[id]; !ok {
		return ErrNotFound
	}
	delete(m.apiKeys, id)
	return nil
}

func (m *MockStore) UpdateAPIKeyStatus(_ context.Context, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key, ok := m.apiKeys[id]
	if !ok {
		return ErrNotFound
	}
	key.Status = status
	return nil
}

func (m *MockStore) UpdateAPIKeyLastUsed(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if key, ok := m.apiKeys[id]; ok {
		now := time.Now()
		key.LastUsedAt = &now
	}
	return nil
}

func (m *MockStore) CreateWebSession(_ context.Context, session *domain.WebSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *session
	m.webSessions[session.TokenHash] = &copy
	return nil
}

func (m *MockStore) GetWebSessionByTokenHash(_ context.Context, tokenHash string) (*domain.WebSession, *domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session := m.webSessions[tokenHash]
	if session == nil || !session.ExpiresAt.After(time.Now()) {
		return nil, nil, nil
	}
	user := m.users[session.UserID]
	if user == nil {
		return nil, nil, nil
	}
	sessionCopy := *session
	userCopy := *user
	return &sessionCopy, &userCopy, nil
}

func (m *MockStore) DeleteWebSessionByTokenHash(_ context.Context, tokenHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.webSessions[tokenHash]; !ok {
		return ErrNotFound
	}
	delete(m.webSessions, tokenHash)
	return nil
}

func (m *MockStore) TouchWebSession(_ context.Context, id string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, session := range m.webSessions {
		if session.ID == id {
			session.LastSeenAt = now
			return nil
		}
	}
	return nil
}

func (m *MockStore) CreateEmailVerification(_ context.Context, ev *domain.EmailVerification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *ev
	m.emailTokens[ev.TokenHash] = &copy
	return nil
}

func (m *MockStore) GetEmailVerificationByTokenHash(_ context.Context, tokenHash string) (*domain.EmailVerification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ev := m.emailTokens[tokenHash]
	if ev == nil {
		return nil, nil
	}
	copy := *ev
	return &copy, nil
}

func (m *MockStore) ConsumeEmailVerification(_ context.Context, id string, consumedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ev := range m.emailTokens {
		if ev.ID == id {
			ev.ConsumedAt = &consumedAt
			return nil
		}
	}
	return ErrNotFound
}

func (m *MockStore) DeletePendingEmailVerifications(_ context.Context, userID, purpose string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, ev := range m.emailTokens {
		if ev.UserID == userID && ev.Purpose == purpose && ev.ConsumedAt == nil {
			delete(m.emailTokens, token)
		}
	}
	return nil
}

func (m *MockStore) CountEmailVerificationsSince(_ context.Context, userID, purpose string, since time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, ev := range m.emailTokens {
		if ev.UserID == userID && ev.Purpose == purpose && !ev.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (m *MockStore) LastEmailVerification(_ context.Context, userID, purpose string) (*domain.EmailVerification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var latest *domain.EmailVerification
	for _, ev := range m.emailTokens {
		if ev.UserID != userID || ev.Purpose != purpose {
			continue
		}
		if latest == nil || ev.CreatedAt.After(latest.CreatedAt) {
			copy := *ev
			latest = &copy
		}
	}
	return latest, nil
}

func (m *MockStore) UpsertBillingSetting(_ context.Context, key, value string, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.billingSettings[key] = value
	return nil
}

func (m *MockStore) GetBillingSetting(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.billingSettings[key], nil
}

func (m *MockStore) UpsertModelPrice(_ context.Context, price *domain.ModelPrice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *price
	m.modelPrices[price.Model] = &copy
	return nil
}

func (m *MockStore) GetModelPrice(_ context.Context, model string) (*domain.ModelPrice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	price := m.modelPrices[model]
	if price == nil {
		return nil, nil
	}
	copy := *price
	return &copy, nil
}

func (m *MockStore) ListModelPrices(_ context.Context) ([]*domain.ModelPrice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var prices []*domain.ModelPrice
	for _, price := range m.modelPrices {
		copy := *price
		prices = append(prices, &copy)
	}
	return prices, nil
}

func (m *MockStore) InsertBillingLedgerEntry(_ context.Context, entry *domain.BillingLedgerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.ledger {
		if existing.IdempotencyKey == entry.IdempotencyKey || existing.ID == entry.ID {
			return nil
		}
	}
	copy := *entry
	copy.Seq = int64(len(m.ledger) + 1)
	m.ledger = append(m.ledger, &copy)
	entry.Seq = copy.Seq
	return nil
}

func (m *MockStore) GetBillingLedgerEntryByIdempotencyKey(_ context.Context, key string) (*domain.BillingLedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range m.ledger {
		if entry.IdempotencyKey == key {
			copy := *entry
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *MockStore) SumBillingLedgerAfter(_ context.Context, userID string, afterSeq int64) (int64, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sum, maxSeq int64
	maxSeq = afterSeq
	for _, entry := range m.ledger {
		if entry.UserID != userID || entry.Seq <= afterSeq {
			continue
		}
		sum += entry.AmountMicros
		if entry.Seq > maxSeq {
			maxSeq = entry.Seq
		}
	}
	return sum, maxSeq, nil
}

func (m *MockStore) GetBillingBalanceCheckpoint(_ context.Context, userID string) (*domain.BillingBalanceCheckpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	checkpoint := m.checkpoints[userID]
	if checkpoint == nil {
		return nil, nil
	}
	copy := *checkpoint
	return &copy, nil
}

func (m *MockStore) UpsertBillingBalanceCheckpoint(_ context.Context, checkpoint *domain.BillingBalanceCheckpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *checkpoint
	m.checkpoints[checkpoint.UserID] = &copy
	return nil
}

func (m *MockStore) CreateBillableRequest(_ context.Context, br *domain.BillableRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.billable[br.RequestID]; ok {
		return nil
	}
	copy := *br
	m.billable[br.RequestID] = &copy
	return nil
}

func (m *MockStore) GetBillableRequest(_ context.Context, requestID string) (*domain.BillableRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	br := m.billable[requestID]
	if br == nil {
		return nil, nil
	}
	copy := *br
	return &copy, nil
}

func (m *MockStore) UpdateBillableRequestUsage(_ context.Context, br *domain.BillableRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing := m.billable[br.RequestID]
	if existing == nil {
		return ErrNotFound
	}
	existing.Status = br.Status
	existing.InputTokens = br.InputTokens
	existing.OutputTokens = br.OutputTokens
	existing.CacheReadTokens = br.CacheReadTokens
	existing.CacheCreateTokens = br.CacheCreateTokens
	existing.PriceSnapshotJSON = br.PriceSnapshotJSON
	existing.UsageObservedAt = br.UsageObservedAt
	return nil
}

func (m *MockStore) MarkBillableRequestSettled(_ context.Context, requestID, ledgerID string, settledAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	br := m.billable[requestID]
	if br == nil {
		return ErrNotFound
	}
	br.Status = "settled"
	br.LedgerID = ledgerID
	br.SettledAt = &settledAt
	return nil
}

func (m *MockStore) MarkBillableRequestStatus(_ context.Context, requestID, status, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	br := m.billable[requestID]
	if br == nil {
		return ErrNotFound
	}
	br.Status = status
	br.Error = errMsg
	return nil
}

func (m *MockStore) ListUnsettledUsageObservedRequests(_ context.Context, limit int) ([]*domain.BillableRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*domain.BillableRequest
	for _, br := range m.billable {
		if br.Status != "usage_observed" || br.LedgerID != "" {
			continue
		}
		copy := *br
		out = append(out, &copy)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MockStore) SavePaymentOrder(_ context.Context, order *domain.PaymentOrder) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *order
	m.paymentOrders[order.OutTradeNo] = &copy
	return nil
}

func (m *MockStore) GetPaymentOrderByOutTradeNo(_ context.Context, outTradeNo string) (*domain.PaymentOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order := m.paymentOrders[outTradeNo]
	if order == nil {
		return nil, nil
	}
	copy := *order
	return &copy, nil
}

func (m *MockStore) ListPaymentOrdersByUser(_ context.Context, userID string, limit int) ([]*domain.PaymentOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var orders []*domain.PaymentOrder
	for _, order := range m.paymentOrders {
		if order.UserID != userID {
			continue
		}
		copy := *order
		orders = append(orders, &copy)
		if limit > 0 && len(orders) >= limit {
			break
		}
	}
	return orders, nil
}

func (m *MockStore) MarkPaymentOrderPaid(_ context.Context, outTradeNo, zpayTradeNo, paymentType string, paidAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	order := m.paymentOrders[outTradeNo]
	if order == nil {
		return ErrNotFound
	}
	order.Status = "paid"
	order.ZpayTradeNo = zpayTradeNo
	order.PaymentType = paymentType
	order.PaidAt = &paidAt
	order.UpdatedAt = paidAt
	return nil
}

func (m *MockStore) FulfillPaymentOrderWithCredit(_ context.Context, outTradeNo, zpayTradeNo, paymentType string, paidAt time.Time, credit *domain.BillingLedgerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	order := m.paymentOrders[outTradeNo]
	if order == nil {
		return ErrNotFound
	}
	if credit != nil && credit.AmountMicros != 0 {
		exists := false
		for _, existing := range m.ledger {
			if existing.IdempotencyKey == credit.IdempotencyKey || existing.ID == credit.ID {
				exists = true
				break
			}
		}
		if !exists {
			entryCopy := *credit
			entryCopy.Seq = int64(len(m.ledger) + 1)
			m.ledger = append(m.ledger, &entryCopy)
		}
	}
	if order.Status != "paid" {
		order.Status = "paid"
		order.ZpayTradeNo = zpayTradeNo
		order.PaymentType = paymentType
		order.PaidAt = &paidAt
		order.UpdatedAt = paidAt
	}
	return nil
}

func (m *MockStore) SavePaymentEvent(_ context.Context, event *domain.PaymentEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *event
	m.paymentEvents = append(m.paymentEvents, &copy)
	return nil
}

func (m *MockStore) CreateReferralWithCredits(_ context.Context, referral *domain.Referral, inviteeCredit, inviterCredit *domain.BillingLedgerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.referrals[referral.InviteeUserID]; ok {
		return nil
	}
	copy := *referral
	m.referrals[referral.InviteeUserID] = &copy
	for _, entry := range []*domain.BillingLedgerEntry{inviteeCredit, inviterCredit} {
		if entry == nil || entry.AmountMicros == 0 {
			continue
		}
		entryCopy := *entry
		entryCopy.Seq = int64(len(m.ledger) + 1)
		m.ledger = append(m.ledger, &entryCopy)
	}
	return nil
}

func (m *MockStore) GetReferralByInvitee(_ context.Context, inviteeUserID string) (*domain.Referral, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	referral := m.referrals[inviteeUserID]
	if referral == nil {
		return nil, nil
	}
	copy := *referral
	return &copy, nil
}

func (m *MockStore) UpsertAdmissionLimit(_ context.Context, limit *domain.AdmissionLimit) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *limit
	m.admissionLimits[limit.Scope+"|"+limit.ScopeID] = &copy
	return nil
}

func (m *MockStore) GetAdmissionLimit(_ context.Context, scope, scopeID string) (*domain.AdmissionLimit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	limit := m.admissionLimits[scope+"|"+scopeID]
	if limit == nil && scopeID != "" {
		limit = m.admissionLimits[scope+"|"]
	}
	if limit == nil {
		return nil, nil
	}
	copy := *limit
	return &copy, nil
}

func (m *MockStore) ListAdmissionLimits(_ context.Context) ([]*domain.AdmissionLimit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var limits []*domain.AdmissionLimit
	for _, limit := range m.admissionLimits {
		copy := *limit
		limits = append(limits, &copy)
	}
	return limits, nil
}

func (m *MockStore) InsertRequestLog(_ context.Context, log *domain.RequestLog) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.InsertRequestErr != nil {
		return 0, m.InsertRequestErr
	}
	copy := *log
	if copy.ID == 0 {
		copy.ID = int64(len(m.logs)) + 1
	}
	log.ID = copy.ID
	m.logs = append(m.logs, &copy)
	return copy.ID, nil
}

func (m *MockStore) QueryRequestLogs(_ context.Context, opts domain.RequestLogQuery) ([]*domain.RequestLog, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var logs []*domain.RequestLog
	for _, entry := range m.logs {
		if opts.UserID != "" && entry.UserID != opts.UserID {
			continue
		}
		if opts.AccountID != "" && entry.AccountID != opts.AccountID {
			continue
		}
		if opts.FailuresOnly && entry.Status == "ok" {
			continue
		}
		copy := *entry
		logs = append(logs, &copy)
	}
	return logs, len(logs), nil
}

func (m *MockStore) QueryRelayOutcomeStats(_ context.Context, since time.Time) ([]domain.RelayOutcomeStat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	type key struct {
		provider string
		surface  string
		effect   string
		status   int
	}
	stats := map[key]*domain.RelayOutcomeStat{}
	userSeen := map[key]map[string]struct{}{}
	accountSeen := map[key]map[string]struct{}{}
	for _, entry := range m.logs {
		if entry.CreatedAt.Before(since) {
			continue
		}
		k := key{provider: entry.Provider, surface: entry.Surface, effect: entry.EffectKind, status: entry.UpstreamStatus}
		stat := stats[k]
		if stat == nil {
			stat = &domain.RelayOutcomeStat{
				Provider:       entry.Provider,
				Surface:        entry.Surface,
				EffectKind:     entry.EffectKind,
				UpstreamStatus: entry.UpstreamStatus,
			}
			stats[k] = stat
			userSeen[k] = map[string]struct{}{}
			accountSeen[k] = map[string]struct{}{}
		}
		stat.Requests++
		if entry.CreatedAt.After(stat.LastSeenAt) {
			stat.LastSeenAt = entry.CreatedAt
		}
		userSeen[k][entry.UserID] = struct{}{}
		accountSeen[k][entry.AccountID] = struct{}{}
	}

	result := make([]domain.RelayOutcomeStat, 0, len(stats))
	for k, stat := range stats {
		stat.DistinctUsers = len(userSeen[k])
		stat.DistinctAccounts = len(accountSeen[k])
		result = append(result, *stat)
	}
	return result, nil
}

func (m *MockStore) QueryCellRiskStats(_ context.Context, since time.Time) ([]domain.CellRiskStat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	type key struct {
		cellID   string
		provider string
	}
	stats := map[key]*domain.CellRiskStat{}
	userSeen := map[key]map[string]struct{}{}
	accountSeen := map[key]map[string]struct{}{}
	for _, entry := range m.logs {
		if entry.CreatedAt.Before(since) {
			continue
		}
		k := key{cellID: entry.CellID, provider: entry.Provider}
		stat := stats[k]
		if stat == nil {
			stat = &domain.CellRiskStat{
				CellID:   entry.CellID,
				Provider: entry.Provider,
			}
			stats[k] = stat
			userSeen[k] = map[string]struct{}{}
			accountSeen[k] = map[string]struct{}{}
		}
		stat.Requests++
		if entry.Status == "ok" {
			stat.Successes++
		}
		switch entry.UpstreamStatus {
		case 400:
			stat.Status400++
		case 403:
			stat.Status403++
		case 429:
			stat.Status429++
		}
		if entry.EffectKind == "block" {
			stat.Blocks++
		}
		if entry.Status == "transport_error" {
			stat.TransportErrors++
		}
		if entry.CreatedAt.After(stat.LastSeenAt) {
			stat.LastSeenAt = entry.CreatedAt
		}
		userSeen[k][entry.UserID] = struct{}{}
		accountSeen[k][entry.AccountID] = struct{}{}
	}

	result := make([]domain.CellRiskStat, 0, len(stats))
	for k, stat := range stats {
		stat.DistinctUsers = len(userSeen[k])
		stat.DistinctAccounts = len(accountSeen[k])
		result = append(result, *stat)
	}
	return result, nil
}

func (m *MockStore) PurgeOldLogs(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (m *MockStore) QueryUsagePeriods(_ context.Context, _ string, _ *time.Location) ([]domain.UsagePeriod, error) {
	return nil, nil
}

func (m *MockStore) QueryUserTotalCosts(_ context.Context) (map[string]float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]float64)
	for _, entry := range m.logs {
		if entry.Status != "ok" {
			continue
		}
		result[entry.UserID] += entry.CostUSD
	}
	return result, nil
}

func (m *MockStore) QueryUserTotalCostsByIDs(_ context.Context, userIDs []string) (map[string]float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	allow := make(map[string]struct{}, len(userIDs))
	result := make(map[string]float64, len(userIDs))
	for _, userID := range userIDs {
		allow[userID] = struct{}{}
		result[userID] = 0
	}
	for _, entry := range m.logs {
		if entry.Status != "ok" {
			continue
		}
		if _, ok := allow[entry.UserID]; !ok {
			continue
		}
		result[entry.UserID] += entry.CostUSD
	}
	return result, nil
}

func (m *MockStore) QueryModelUsage(_ context.Context, _ string) ([]domain.ModelUsageRow, error) {
	return nil, nil
}
