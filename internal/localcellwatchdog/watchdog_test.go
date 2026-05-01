package localcellwatchdog

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"
)

func TestTargetCellsFiltersLocalDantedActiveSocks5(t *testing.T) {
	now := time.Now().UTC()
	cells := []Cell{
		{
			ID:     "cell-ok",
			Name:   "UK Linode 01(local)",
			Status: "active",
			Proxy:  &Proxy{Type: "socks5", Host: "127.0.0.1", Port: 11080},
			Labels: map[string]string{"transport": "local-danted", "ipv6": "2600:3c13:e001:ae::100"},
		},
		{
			ID:     "cell-disabled",
			Status: "disabled",
			Proxy:  &Proxy{Type: "socks5", Host: "127.0.0.1", Port: 11081},
			Labels: map[string]string{"transport": "local-danted", "ipv6": "2600:3c13:e001:ae::101"},
		},
		{
			ID:     "cell-http",
			Status: "active",
			Proxy:  &Proxy{Type: "http", Host: "127.0.0.1", Port: 11082},
			Labels: map[string]string{"transport": "local-danted", "ipv6": "2600:3c13:e001:ae::102"},
		},
		{
			ID:     "cell-remote",
			Status: "active",
			Proxy:  &Proxy{Type: "socks5", Host: "10.0.0.2", Port: 12081},
			Labels: map[string]string{"transport": "wg-direct", "ipv6": "2600:3c1a:e001:16::101"},
		},
		{
			ID:            "cell-missing-ipv6",
			Status:        "active",
			Proxy:         &Proxy{Type: "socks5", Host: "127.0.0.1", Port: 11083},
			Labels:        map[string]string{"transport": "local-danted"},
			CooldownUntil: &now,
		},
	}

	got := TargetCells(cells)
	if len(got) != 1 {
		t.Fatalf("TargetCells() len = %d, want 1", len(got))
	}
	if got[0].ID != "cell-ok" {
		t.Fatalf("TargetCells()[0].ID = %q, want %q", got[0].ID, "cell-ok")
	}
}

func TestServiceNameForPort(t *testing.T) {
	tests := []struct {
		port int
		want string
		ok   bool
	}{
		{port: 11080, want: "danted-linda.service", ok: true},
		{port: 11082, want: "danted-cell-uk-linode-02-local.service", ok: true},
		{port: 11083, want: "danted-cell-uk-linode-03-local.service", ok: true},
		{port: 9999, want: "", ok: false},
	}

	for _, tt := range tests {
		got, ok := ServiceNameForPort(tt.port)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("ServiceNameForPort(%d) = (%q, %v), want (%q, %v)", tt.port, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRunOnceRepairsFailedCellAndClearsCooldown(t *testing.T) {
	now := time.Now().UTC()
	cell := Cell{
		ID:            "cell-uk-linode-01",
		Name:          "UK Linode 01(local)",
		Status:        "active",
		Proxy:         &Proxy{Type: "socks5", Host: "127.0.0.1", Port: 11080},
		Labels:        map[string]string{"transport": "local-danted", "ipv6": "2600:3c13:e001:ae::100"},
		CooldownUntil: &now,
	}

	client := &fakeClient{
		cells: []Cell{cell},
		results: map[string][]ProbeResult{
			cell.ID: {
				{OK: false, Error: "network unreachable"},
				{OK: true, LatencyMs: 7},
			},
		},
	}
	runner := &fakeRunner{present: map[string]bool{}}
	w := New(Options{
		Iface:  "eth0",
		Client: client,
		Runner: runner,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if !reflect.DeepEqual(runner.addedIPv6, []string{"eth0|2600:3c13:e001:ae::100"}) {
		t.Fatalf("addedIPv6 = %#v", runner.addedIPv6)
	}
	if !reflect.DeepEqual(runner.restartedServices, []string{"danted-linda.service"}) {
		t.Fatalf("restartedServices = %#v", runner.restartedServices)
	}
	if !reflect.DeepEqual(client.clearedCooldown, []string{cell.ID}) {
		t.Fatalf("clearedCooldown = %#v", client.clearedCooldown)
	}
}

func TestRunOnceClearsCooldownWhenHealthy(t *testing.T) {
	now := time.Now().UTC()
	cell := Cell{
		ID:            "cell-uk-linode-02",
		Status:        "active",
		Proxy:         &Proxy{Type: "socks5", Host: "127.0.0.1", Port: 11082},
		Labels:        map[string]string{"transport": "local-danted", "ipv6": "2600:3c13:e001:ae::101"},
		CooldownUntil: &now,
	}

	client := &fakeClient{
		cells: []Cell{cell},
		results: map[string][]ProbeResult{
			cell.ID: {
				{OK: true, LatencyMs: 9},
			},
		},
	}
	runner := &fakeRunner{present: map[string]bool{"2600:3c13:e001:ae::101": true}}
	w := New(Options{
		Iface:  "eth0",
		Client: client,
		Runner: runner,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if len(runner.addedIPv6) != 0 {
		t.Fatalf("addedIPv6 = %#v, want none", runner.addedIPv6)
	}
	if len(runner.restartedServices) != 0 {
		t.Fatalf("restartedServices = %#v, want none", runner.restartedServices)
	}
	if !reflect.DeepEqual(client.clearedCooldown, []string{cell.ID}) {
		t.Fatalf("clearedCooldown = %#v", client.clearedCooldown)
	}
}

func TestRunOnceLeavesCooldownWhenRetestStillFails(t *testing.T) {
	now := time.Now().UTC()
	cell := Cell{
		ID:            "cell-uk-linode-03",
		Status:        "active",
		Proxy:         &Proxy{Type: "socks5", Host: "127.0.0.1", Port: 11083},
		Labels:        map[string]string{"transport": "local-danted", "ipv6": "2600:3c13:e001:ae::102"},
		CooldownUntil: &now,
	}

	client := &fakeClient{
		cells: []Cell{cell},
		results: map[string][]ProbeResult{
			cell.ID: {
				{OK: false, Error: "network unreachable"},
				{OK: false, Error: "still unreachable"},
			},
		},
	}
	runner := &fakeRunner{present: map[string]bool{"2600:3c13:e001:ae::102": true}}
	w := New(Options{
		Iface:  "eth0",
		Client: client,
		Runner: runner,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if !reflect.DeepEqual(runner.restartedServices, []string{"danted-cell-uk-linode-03-local.service"}) {
		t.Fatalf("restartedServices = %#v", runner.restartedServices)
	}
	if len(client.clearedCooldown) != 0 {
		t.Fatalf("clearedCooldown = %#v, want none", client.clearedCooldown)
	}
}

func TestRunOnceRetriesProbeAfterRestart(t *testing.T) {
	now := time.Now().UTC()
	cell := Cell{
		ID:            "cell-uk-linode-02",
		Status:        "active",
		Proxy:         &Proxy{Type: "socks5", Host: "127.0.0.1", Port: 11082},
		Labels:        map[string]string{"transport": "local-danted", "ipv6": "2600:3c13:e001:ae::101"},
		CooldownUntil: &now,
	}

	client := &fakeClient{
		cells: []Cell{cell},
		results: map[string][]ProbeResult{
			cell.ID: {
				{OK: false, Error: "network unreachable"},
				{OK: false, Error: "dial tcp 127.0.0.1:11082: connect: connection refused"},
				{OK: true, LatencyMs: 8},
			},
		},
	}
	runner := &fakeRunner{present: map[string]bool{"2600:3c13:e001:ae::101": true}}
	w := New(Options{
		Iface:  "eth0",
		Client: client,
		Runner: runner,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if !reflect.DeepEqual(runner.restartedServices, []string{"danted-cell-uk-linode-02-local.service"}) {
		t.Fatalf("restartedServices = %#v", runner.restartedServices)
	}
	if !reflect.DeepEqual(client.clearedCooldown, []string{cell.ID}) {
		t.Fatalf("clearedCooldown = %#v", client.clearedCooldown)
	}
}

type fakeClient struct {
	cells           []Cell
	results         map[string][]ProbeResult
	clearedCooldown []string
}

func (f *fakeClient) ListCells(context.Context) ([]Cell, error) {
	return append([]Cell(nil), f.cells...), nil
}

func (f *fakeClient) TestCell(_ context.Context, cellID string) (ProbeResult, error) {
	queue := f.results[cellID]
	if len(queue) == 0 {
		return ProbeResult{}, nil
	}
	result := queue[0]
	f.results[cellID] = queue[1:]
	return result, nil
}

func (f *fakeClient) ClearCooldown(_ context.Context, cellID string) error {
	f.clearedCooldown = append(f.clearedCooldown, cellID)
	return nil
}

type fakeRunner struct {
	present           map[string]bool
	addedIPv6         []string
	restartedServices []string
}

func (f *fakeRunner) IPv6Present(_ context.Context, iface, ipv6 string) (bool, error) {
	return f.present[ipv6], nil
}

func (f *fakeRunner) AddIPv6(_ context.Context, iface, ipv6 string) error {
	f.addedIPv6 = append(f.addedIPv6, iface+"|"+ipv6)
	f.present[ipv6] = true
	return nil
}

func (f *fakeRunner) RestartService(_ context.Context, service string) error {
	f.restartedServices = append(f.restartedServices, service)
	return nil
}
