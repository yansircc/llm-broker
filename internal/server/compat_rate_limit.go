package server

import (
	"fmt"
	"sync"
	"time"
)

type compatRateLimiter struct {
	mu            sync.Mutex
	maxPerMinute  int
	maxConcurrent int
	state         map[string]*compatRateState
}

type compatRateState struct {
	started  []time.Time
	inflight int
}

func newCompatRateLimiter(maxPerMinute, maxConcurrent int) *compatRateLimiter {
	if maxPerMinute < 0 {
		maxPerMinute = 0
	}
	if maxConcurrent < 0 {
		maxConcurrent = 0
	}
	if maxPerMinute == 0 && maxConcurrent == 0 {
		return nil
	}
	return &compatRateLimiter{
		maxPerMinute:  maxPerMinute,
		maxConcurrent: maxConcurrent,
		state:         make(map[string]*compatRateState),
	}
}

func (l *compatRateLimiter) Acquire(key string, now time.Time) (func(), error) {
	if l == nil || key == "" {
		return func() {}, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.state[key]
	if entry == nil {
		entry = &compatRateState{}
		l.state[key] = entry
	}

	cutoff := now.Add(-time.Minute)
	kept := entry.started[:0]
	for _, ts := range entry.started {
		if !ts.Before(cutoff) {
			kept = append(kept, ts)
		}
	}
	entry.started = kept

	if l.maxConcurrent > 0 && entry.inflight >= l.maxConcurrent {
		return nil, fmt.Errorf("compat concurrency limit reached (%d)", l.maxConcurrent)
	}
	if l.maxPerMinute > 0 && len(entry.started) >= l.maxPerMinute {
		return nil, fmt.Errorf("compat request rate limit reached (%d/min)", l.maxPerMinute)
	}

	entry.inflight++
	if l.maxPerMinute > 0 {
		entry.started = append(entry.started, now)
	}

	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()

		current := l.state[key]
		if current == nil {
			return
		}
		if current.inflight > 0 {
			current.inflight--
		}

		if l.maxPerMinute > 0 {
			cutoff := time.Now().Add(-time.Minute)
			kept := current.started[:0]
			for _, ts := range current.started {
				if !ts.Before(cutoff) {
					kept = append(kept, ts)
				}
			}
			current.started = kept
		}
		if current.inflight == 0 && len(current.started) == 0 {
			delete(l.state, key)
		}
	}, nil
}
