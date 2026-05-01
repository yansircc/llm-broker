package localcellwatchdog

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"
)

type Proxy struct {
	Type string `json:"type,omitempty"`
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

type Cell struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Status        string            `json:"status"`
	Proxy         *Proxy            `json:"proxy,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	CooldownUntil *time.Time        `json:"cooldown_until,omitempty"`
}

type ProbeResult struct {
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

type Client interface {
	ListCells(ctx context.Context) ([]Cell, error)
	TestCell(ctx context.Context, cellID string) (ProbeResult, error)
	ClearCooldown(ctx context.Context, cellID string) error
}

type Runner interface {
	IPv6Present(ctx context.Context, iface, ipv6 string) (bool, error)
	AddIPv6(ctx context.Context, iface, ipv6 string) error
	RestartService(ctx context.Context, service string) error
}

type Options struct {
	Iface              string
	Client             Client
	Runner             Runner
	Logger             *slog.Logger
	RepairProbeRetries int
	RepairProbeDelay   time.Duration
}

type Watchdog struct {
	iface              string
	client             Client
	runner             Runner
	logger             *slog.Logger
	repairProbeRetries int
	repairProbeDelay   time.Duration
}

func New(opts Options) *Watchdog {
	if opts.Iface == "" {
		opts.Iface = "eth0"
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.RepairProbeRetries <= 0 {
		opts.RepairProbeRetries = 5
	}
	if opts.RepairProbeDelay < 0 {
		opts.RepairProbeDelay = 0
	}
	if opts.RepairProbeDelay == 0 {
		opts.RepairProbeDelay = 300 * time.Millisecond
	}
	return &Watchdog{
		iface:              opts.Iface,
		client:             opts.Client,
		runner:             opts.Runner,
		logger:             opts.Logger,
		repairProbeRetries: opts.RepairProbeRetries,
		repairProbeDelay:   opts.RepairProbeDelay,
	}
}

func TargetCells(cells []Cell) []Cell {
	targets := make([]Cell, 0, len(cells))
	for _, cell := range cells {
		if !isTargetCell(cell) {
			continue
		}
		targets = append(targets, cell)
	}
	return targets
}

func ServiceNameForPort(port int) (string, bool) {
	switch port {
	case 11080:
		return "danted-linda.service", true
	case 11082:
		return "danted-cell-uk-linode-02-local.service", true
	case 11083:
		return "danted-cell-uk-linode-03-local.service", true
	default:
		return "", false
	}
}

func (w *Watchdog) RunOnce(ctx context.Context) error {
	cells, err := w.client.ListCells(ctx)
	if err != nil {
		return err
	}

	var firstErr error
	for _, cell := range TargetCells(cells) {
		if err := w.reconcileCell(ctx, cell); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			w.logger.Error("local cell watchdog failed", "cell_id", cell.ID, "cell_name", cell.Name, "error", err)
		}
	}
	return firstErr
}

func (w *Watchdog) reconcileCell(ctx context.Context, cell Cell) error {
	ipv6 := strings.TrimSpace(cell.Labels["ipv6"])
	service, ok := ServiceNameForPort(cell.Proxy.Port)
	if !ok || ipv6 == "" {
		w.logger.Warn("local cell watchdog skipped", "cell_id", cell.ID, "proxy_port", cell.Proxy.Port, "ipv6", ipv6)
		return nil
	}

	probe, err := w.client.TestCell(ctx, cell.ID)
	if err != nil {
		return err
	}
	if probe.OK {
		return w.clearCooldownIfNeeded(ctx, cell, "healthy")
	}

	present, err := w.runner.IPv6Present(ctx, w.iface, ipv6)
	if err != nil {
		return err
	}
	if !present {
		if err := w.runner.AddIPv6(ctx, w.iface, ipv6); err != nil {
			return err
		}
		w.logger.Info("local cell watchdog added ipv6", "cell_id", cell.ID, "ipv6", ipv6, "iface", w.iface)
	}

	if err := w.runner.RestartService(ctx, service); err != nil {
		return err
	}
	w.logger.Warn("local cell watchdog restarted service", "cell_id", cell.ID, "service_name", service, "stage", "repair")

	probe, err = w.probeAfterRepair(ctx, cell.ID)
	if err != nil {
		return err
	}
	if !probe.OK {
		w.logger.Error("local cell watchdog retest failed", "cell_id", cell.ID, "service_name", service, "result", "failed", "error", probe.Error)
		return nil
	}
	return w.clearCooldownIfNeeded(ctx, cell, "recovered")
}

func (w *Watchdog) probeAfterRepair(ctx context.Context, cellID string) (ProbeResult, error) {
	attempts := w.repairProbeRetries
	var last ProbeResult
	for i := 0; i < attempts; i++ {
		probe, err := w.client.TestCell(ctx, cellID)
		if err != nil {
			return ProbeResult{}, err
		}
		last = probe
		if probe.OK {
			return probe, nil
		}
		if i == attempts-1 {
			break
		}
		timer := time.NewTimer(w.repairProbeDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ProbeResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	return last, nil
}

func (w *Watchdog) clearCooldownIfNeeded(ctx context.Context, cell Cell, stage string) error {
	if cell.CooldownUntil == nil {
		w.logger.Info("local cell watchdog probe ok", "cell_id", cell.ID, "stage", stage, "result", "ok")
		return nil
	}
	if err := w.client.ClearCooldown(ctx, cell.ID); err != nil {
		return err
	}
	w.logger.Info("local cell watchdog cleared cooldown", "cell_id", cell.ID, "stage", stage, "result", "ok")
	return nil
}

func isTargetCell(cell Cell) bool {
	if strings.TrimSpace(cell.Status) != "active" {
		return false
	}
	if cell.Proxy == nil {
		return false
	}
	if strings.TrimSpace(cell.Proxy.Type) != "socks5" {
		return false
	}
	if strings.TrimSpace(cell.Proxy.Host) != "127.0.0.1" {
		return false
	}
	if cell.Proxy.Port <= 0 {
		return false
	}
	if strings.TrimSpace(cell.Labels["transport"]) != "local-danted" {
		return false
	}
	return strings.TrimSpace(cell.Labels["ipv6"]) != ""
}
