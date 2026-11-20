package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/store"
)

func fableRejection(d *driver.ClaudeDriver) driver.Effect {
	h := http.Header{}
	for k, v := range map[string]string{"7d-status": "allowed", "7d-utilization": "0.66", "7d_oi-status": "rejected", "7d_oi-utilization": "1.01", "7d_oi-reset": fmt.Sprint(time.Now().Add(24 * time.Hour).Unix()), "representative-claim": "seven_day_overage_included"} {
		h.Set("anthropic-ratelimit-unified-"+k, v)
	}
	h.Set("Retry-After", "86400")
	return d.Interpret(429, h, nil, "claude-fable-5-2", nil)
}

func TestClaudeQuotaObserveSchedulingAndProbeRace(t *testing.T) {
	p := newTestPool(t, activeAccount("a", "a@example.test"))
	d := driver.NewClaudeDriver(driver.ClaudeConfig{}, driver.NoopStainlessStore{}, 4)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{domain.ProviderClaude: d})
	p.Observe("a", fableRejection(d))
	a := p.Get("a")
	if a.CooldownUntil != nil {
		t.Fatalf("Fable wrote public cooldown: %s", a.CooldownUntil)
	}
	for _, tc := range []struct {
		model string
		want  bool
	}{{"claude-opus-5-5", true}, {"claude-sonnet-5-5", true}, {"claude-fable-5-2", false}} {
		_, lease, err := p.AcquireRoute(context.Background(), d, RouteRequest{Model: tc.model, Surface: domain.SurfaceNative})
		if lease != nil {
			lease.Finish()
		}
		if (err == nil) != tc.want {
			t.Errorf("%s route: %v", tc.model, err)
		}
	}
	until := time.Now().Add(time.Hour)
	p.Observe("a", driver.Effect{Kind: driver.EffectCooldown, Scope: driver.EffectScopeBucket, CooldownUntil: until})
	start := p.Get("a")
	// A newer rejection arriving during a probe must survive the stale success.
	p.Observe("a", driver.Effect{Kind: driver.EffectCooldown, Scope: driver.EffectScopeBucket, CooldownUntil: until.Add(time.Hour)})
	success := d.Interpret(200, nil, nil, "claude-opus-5-5", nil)
	p.ObserveProbe("a", success, start)
	if p.Get("a").CooldownUntil == nil {
		t.Fatal("stale probe erased newer rejection")
	}
	start = p.Get("a")
	p.Observe("a", fableRejection(d)) // Same shared deadline, newer provider observation.
	p.ObserveProbe("a", success, start)
	if p.Get("a").CooldownUntil == nil {
		t.Fatal("stale probe ignored a newer provider observation")
	}
	start = p.Get("a")
	p.ObserveProbe("a", d.Interpret(200, nil, nil, "claude-opus-5-5", nil), start)
	a = p.Get("a")
	if a.CooldownUntil != nil {
		t.Fatal("unchanged cooldown not cleared")
	}
	if d.CanServe(json.RawMessage(a.ProviderStateJSON), "claude-fable-5-2", time.Now()) {
		t.Fatal("ordinary probe erased Fable restriction")
	}
}

func TestClaudeQuotaConcurrentObservationsPersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quota.db")
	if err := store.Migrate(path); err != nil {
		t.Fatal(err)
	}
	db, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := New(db, events.NewBus(10))
	if err != nil {
		t.Fatal(err)
	}
	d := driver.NewClaudeDriver(driver.ClaudeConfig{}, driver.NoopStainlessStore{}, 4)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{domain.ProviderClaude: d})
	if err = p.Add(activeAccount("a", "a@example.test")); err != nil {
		t.Fatal(err)
	}
	older := d.Interpret(200, nil, nil, "claude-opus-5-5", nil)
	rejection := fableRejection(d)
	var wg sync.WaitGroup
	for _, effect := range []driver.Effect{older, rejection} {
		wg.Add(1)
		go func(e driver.Effect) { defer wg.Done(); p.Observe("a", e) }(effect)
	}
	wg.Wait()
	reopened, err := New(db, events.NewBus(10))
	if err != nil {
		t.Fatal(err)
	}
	a := reopened.Get("a")
	if a.CooldownUntil != nil || d.CanServe(json.RawMessage(a.ProviderStateJSON), "claude-fable-5-2", time.Now()) {
		t.Fatalf("restriction not persisted: %+v", a)
	}
}
