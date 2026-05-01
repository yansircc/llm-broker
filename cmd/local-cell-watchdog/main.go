package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/localcellwatchdog"
)

const (
	defaultEnvFile = "/etc/llm-broker.env"
	defaultIface   = "eth0"
	requestTimeout = 15 * time.Second
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg, err := loadConfig(ctx)
	if err != nil {
		logger.Error("local cell watchdog config failed", "error", err)
		os.Exit(1)
	}

	client := &adminClient{
		baseURL: cfg.baseURL,
		token:   cfg.token,
		client: &http.Client{
			Timeout: requestTimeout,
		},
	}
	runner := &osRunner{}
	watchdog := localcellwatchdog.New(localcellwatchdog.Options{
		Iface:  cfg.iface,
		Client: client,
		Runner: runner,
		Logger: logger,
	})

	if err := watchdog.RunOnce(ctx); err != nil {
		logger.Error("local cell watchdog run failed", "error", err)
		os.Exit(1)
	}
	logger.Info("local cell watchdog run complete", "base_url", cfg.baseURL, "iface", cfg.iface)
}

type config struct {
	baseURL string
	token   string
	iface   string
}

func loadConfig(ctx context.Context) (config, error) {
	envFile := envOr("BROKER_ENV_FILE", defaultEnvFile)
	token := strings.TrimSpace(os.Getenv("BROKER_API_TOKEN"))
	if token == "" {
		value, err := envFileValue(envFile, "API_TOKEN")
		if err != nil {
			return config{}, err
		}
		token = value
	}
	if token == "" {
		return config{}, errors.New("missing broker API token")
	}

	baseURL := strings.TrimSpace(os.Getenv("BROKER_URL"))
	if baseURL == "" {
		detected, err := detectBrokerBaseURL(ctx, token)
		if err != nil {
			return config{}, err
		}
		baseURL = detected
	}

	return config{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		iface:   envOr("WATCHDOG_IFACE", defaultIface),
	}, nil
}

func detectBrokerBaseURL(ctx context.Context, token string) (string, error) {
	candidates := []string{
		"http://127.0.0.1:3002",
		"http://127.0.0.1:3001",
		"http://127.0.0.1:3000",
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, baseURL := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/admin/health", nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("x-api-key", token)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return baseURL, nil
		}
	}
	return "", errors.New("failed to detect broker admin base URL")
}

func envFileValue(path, key string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	prefix := key + "="
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), nil
		}
	}
	return "", fmt.Errorf("%s not found in %s", key, path)
}

func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

type adminClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func (c *adminClient) ListCells(ctx context.Context) ([]localcellwatchdog.Cell, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/admin/egress/cells", nil)
	if err != nil {
		return nil, err
	}
	var cells []localcellwatchdog.Cell
	if err := c.doJSON(req, &cells); err != nil {
		return nil, err
	}
	return cells, nil
}

func (c *adminClient) TestCell(ctx context.Context, cellID string) (localcellwatchdog.ProbeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/admin/egress/cells/"+cellID+"/test", nil)
	if err != nil {
		return localcellwatchdog.ProbeResult{}, err
	}
	var result localcellwatchdog.ProbeResult
	if err := c.doJSON(req, &result); err != nil {
		return localcellwatchdog.ProbeResult{}, err
	}
	return result, nil
}

func (c *adminClient) ClearCooldown(ctx context.Context, cellID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/admin/egress/cells/"+cellID+"/clear-cooldown", nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, nil)
}

func (c *adminClient) doJSON(req *http.Request, out any) error {
	req.Header.Set("x-api-key", c.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: http %d: %s", req.Method, req.URL.Path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type osRunner struct{}

func (r *osRunner) IPv6Present(ctx context.Context, iface, ipv6 string) (bool, error) {
	output, err := exec.CommandContext(ctx, "ip", "-6", "addr", "show", "dev", iface).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("ip -6 addr show dev %s: %w: %s", iface, err, strings.TrimSpace(string(output)))
	}
	return strings.Contains(string(output), ipv6+"/"), nil
}

func (r *osRunner) AddIPv6(ctx context.Context, iface, ipv6 string) error {
	output, err := exec.CommandContext(ctx, "ip", "-6", "addr", "add", ipv6+"/128", "dev", iface, "nodad").CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip -6 addr add %s/128 dev %s nodad: %w: %s", ipv6, iface, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (r *osRunner) RestartService(ctx context.Context, service string) error {
	output, err := exec.CommandContext(ctx, "systemctl", "restart", service).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s: %w: %s", service, err, strings.TrimSpace(string(output)))
	}
	return nil
}
