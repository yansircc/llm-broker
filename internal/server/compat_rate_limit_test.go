package server

import (
	"strings"
	"testing"
	"time"
)

func TestCompatRateLimiter_DisabledRequestRateAllowsBurst(t *testing.T) {
	limiter := newCompatRateLimiter(0, 0)
	now := time.Now()

	for range 20 {
		release, err := limiter.Acquire("user-1", now)
		if err != nil {
			t.Fatalf("Acquire() err = %v, want nil", err)
		}
		release()
	}
}

func TestCompatRateLimiter_ConcurrentOnly(t *testing.T) {
	limiter := newCompatRateLimiter(0, 1)
	now := time.Now()

	release, err := limiter.Acquire("user-1", now)
	if err != nil {
		t.Fatalf("first Acquire() err = %v, want nil", err)
	}

	_, err = limiter.Acquire("user-1", now)
	if err == nil || !strings.Contains(err.Error(), "concurrency limit") {
		t.Fatalf("second Acquire() err = %v, want concurrency limit", err)
	}

	release()

	release, err = limiter.Acquire("user-1", now)
	if err != nil {
		t.Fatalf("third Acquire() err = %v, want nil after release", err)
	}
	release()
}
