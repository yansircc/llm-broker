package events

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

type EventType string

const (
	EventBan        EventType = "ban"
	EventRefresh    EventType = "refresh"
	EventRateLimit  EventType = "ratelimit"
	EventRecover    EventType = "recover"
	EventFiveHStop  EventType = "5h_stop"
	EventOverload   EventType = "overload"
	EventRequest    EventType = "request"
	EventRelayError EventType = "relay_error"
)

type Event struct {
	Type          EventType  `json:"type"`
	AccountID     string     `json:"account_id,omitempty"`
	UserID        string     `json:"user_id,omitempty"`
	BucketKey     string     `json:"bucket_key,omitempty"`
	CellID        string     `json:"cell_id,omitempty"`
	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
	Message       string     `json:"message"`
	Timestamp     time.Time  `json:"ts"`
}

type Bus struct {
	mu          sync.RWMutex
	ring        []Event
	ringSize    int
	ringPos     int
	ringCount   int
	subscribers map[int]chan Event
	nextID      int
	log         *slog.Logger // dedicated logger, bypasses app log level
}

func NewBus(ringSize int) *Bus {
	if ringSize <= 0 {
		ringSize = 200
	}
	return &Bus{
		ring:        make([]Event, ringSize),
		ringSize:    ringSize,
		subscribers: make(map[int]chan Event),
		log:         slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
}

func (b *Bus) Publish(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	// slog sink: semantic events enter journald automatically.
	attrs := []any{"type", string(e.Type)}
	if e.AccountID != "" {
		attrs = append(attrs, "accountId", e.AccountID)
	}
	if e.UserID != "" {
		attrs = append(attrs, "userId", e.UserID)
	}
	if e.BucketKey != "" {
		attrs = append(attrs, "bucketKey", e.BucketKey)
	}
	if e.CellID != "" {
		attrs = append(attrs, "cellId", e.CellID)
	}
	if e.CooldownUntil != nil {
		attrs = append(attrs, "cooldownUntil", e.CooldownUntil.Format(time.RFC3339))
	}
	if e.Message != "" {
		attrs = append(attrs, "detail", e.Message)
	}
	b.log.Warn("event", attrs...)

	b.mu.Lock()
	defer b.mu.Unlock()

	b.ring[b.ringPos] = e
	b.ringPos = (b.ringPos + 1) % b.ringSize
	if b.ringCount < b.ringSize {
		b.ringCount++
	}

	for _, ch := range b.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}

func (b *Bus) Subscribe() (id int, ch <-chan Event, recent []Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	c := make(chan Event, 64)
	id = b.nextID
	b.nextID++
	b.subscribers[id] = c

	recent = b.recentLocked()
	return id, c, recent
}

func (b *Bus) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(ch)
	}
}

// Recent returns the most recent events from the ring buffer (up to limit).
func (b *Bus) Recent(limit int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	all := b.recentLocked()
	if limit > 0 && len(all) > limit {
		return all[len(all)-limit:]
	}
	return all
}

func (b *Bus) recentLocked() []Event {
	if b.ringCount == 0 {
		return nil
	}
	result := make([]Event, b.ringCount)
	start := (b.ringPos - b.ringCount + b.ringSize) % b.ringSize
	for i := range b.ringCount {
		result[i] = b.ring[(start+i)%b.ringSize]
	}
	return result
}

// Clear resets the ring buffer, discarding all events.
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ringPos = 0
	b.ringCount = 0
}
