package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/yansircc/llm-broker/internal/driver"
)

func (s *Server) runOverloadRecovery(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.probeOverloadedBuckets(ctx)
		}
	}
}

func (s *Server) probeOverloadedBuckets(ctx context.Context) {
	ready := s.pool.OverloadedBucketsReady()
	if len(ready) == 0 {
		return
	}

	// Limit concurrent probes.
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for _, info := range ready {
		acct := s.pool.AnyAccountInBucket(info.BucketKey)
		if acct == nil {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(bucketKey string) {
			defer wg.Done()
			defer func() { <-sem }()

			probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			result, err := s.probeAccount(probeCtx, acct)
			if err != nil {
				slog.Warn("overload probe: error", "bucketKey", bucketKey, "error", err)
				return
			}

			if result.ClearCooldown {
				// probeAccount already called pool.ClearCooldown; clear the overload tracking too.
				s.pool.ClearOverload(bucketKey)
				return
			}

			if result.Effect.Kind == driver.EffectOverload {
				s.pool.ExtendOverloadCooldown(bucketKey, s.cfg.ErrorPause529)
				return
			}
		}(info.BucketKey)
	}

	wg.Wait()
}
