package store

import (
	"sync"
	"time"
)

// TTLMap is a generic in-memory map with per-entry TTL expiration.
type TTLMap[V any] struct {
	mu    sync.RWMutex
	items map[string]ttlEntry[V]
}

type ttlEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// TTLEntry is a public view of a TTL map entry (for iteration).
type TTLEntry[V any] struct {
	Key       string
	Value     V
	ExpiresAt time.Time
}

func NewTTLMap[V any]() *TTLMap[V] {
	return &TTLMap[V]{items: make(map[string]ttlEntry[V])}
}

func (m *TTLMap[V]) Get(key string) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.items[key]
	if !ok || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (m *TTLMap[V]) Set(key string, value V, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = ttlEntry[V]{value: value, expiresAt: time.Now().Add(ttl)}
}

func (m *TTLMap[V]) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

// GetAndDelete atomically retrieves and removes an entry.
func (m *TTLMap[V]) GetAndDelete(key string) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.items[key]
	if !ok || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	delete(m.items, key)
	return e.value, true
}

// Update modifies an existing entry's value and resets its TTL.
// The callback receives a pointer to the value for in-place mutation.
// Returns false if the key doesn't exist or is expired.
func (m *TTLMap[V]) Update(key string, fn func(*V), newTTL time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.items[key]
	if !ok || time.Now().After(e.expiresAt) {
		return false
	}
	fn(&e.value)
	e.expiresAt = time.Now().Add(newTTL)
	m.items[key] = e
	return true
}

// Entries returns all non-expired entries.
func (m *TTLMap[V]) Entries() []TTLEntry[V] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	result := make([]TTLEntry[V], 0)
	for k, e := range m.items {
		if !now.After(e.expiresAt) {
			result = append(result, TTLEntry[V]{Key: k, Value: e.value, ExpiresAt: e.expiresAt})
		}
	}
	return result
}

// SetNX sets key only if it doesn't exist or is expired. Returns true if set.
func (m *TTLMap[V]) SetNX(key string, value V, ttl time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.items[key]; ok && time.Now().Before(e.expiresAt) {
		return false
	}
	m.items[key] = ttlEntry[V]{value: value, expiresAt: time.Now().Add(ttl)}
	return true
}

// DeleteIf atomically deletes a key only if its current value matches.
func (m *TTLMap[V]) DeleteIf(key string, match func(V) bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.items[key]
	if !ok || time.Now().After(e.expiresAt) {
		return false
	}
	if match(e.value) {
		delete(m.items, key)
		return true
	}
	return false
}

// Cleanup removes all expired entries.
func (m *TTLMap[V]) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, e := range m.items {
		if now.After(e.expiresAt) {
			delete(m.items, k)
		}
	}
}
