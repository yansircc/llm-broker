package driver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

// Driver encapsulates all provider-specific behavior.
// Core code (pool, relay, admin) only interacts with this interface.
type Driver interface {
	Provider() domain.Provider
	Info() ProviderInfo
	Models() []Model

	// --- Relay ---

	// BuildRequest creates the upstream HTTP request.
	BuildRequest(ctx context.Context, input *RelayInput, acct *domain.Account, token string) (*http.Request, error)

	// Interpret classifies an upstream response into a provider-agnostic Effect.
	Interpret(statusCode int, headers http.Header, body []byte, model string, state json.RawMessage) Effect

	// StreamResponse streams an SSE response to the client, returning completion status and usage.
	StreamResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response) (completed bool, usage *Usage)

	// ForwardResponse passes a non-streaming response (count_tokens, errors) to the client.
	ForwardResponse(w http.ResponseWriter, resp *http.Response)

	// ParseJSONUsage extracts usage from a non-streaming JSON response body.
	ParseJSONUsage(body []byte) *Usage

	// ShouldRetry returns true if the status code is retriable.
	ShouldRetry(statusCode int) bool

	// RetrySameAccount returns true if a failed request should retry the same account
	// without Observe/cooldown (e.g., Claude 403 non-ban retry ×2).
	RetrySameAccount(statusCode int, body []byte, priorAttempts int) bool

	// ParseNonRetriable returns true if the error body indicates a permanent rejection
	// that should be passed through without retry (e.g., "Extra usage is required").
	ParseNonRetriable(statusCode int, body []byte) bool

	// WriteError writes a provider-formatted error response.
	WriteError(w http.ResponseWriter, status int, msg string)

	// WriteUpstreamError passes through or sanitizes an upstream error body.
	WriteUpstreamError(w http.ResponseWriter, statusCode int, body []byte, isStream bool)

	// --- Provider-specific hooks ---

	// InterceptRequest handles provider-specific request interception (Claude: warmup).
	// Returns true if the request was handled and should not continue.
	InterceptRequest(w http.ResponseWriter, body map[string]interface{}, model string) bool

	// ExtractSessionUUID extracts a session UUID from the request body (Claude: metadata.user_id).
	// Returns "" for providers without session binding.
	ExtractSessionUUID(body map[string]interface{}) string

	// --- Lifecycle (OAuth) ---

	// GenerateAuthURL creates a PKCE-secured authorization URL.
	GenerateAuthURL() (string, OAuthSession, error)

	// ExchangeCode exchanges an authorization code for tokens and identity.
	// Must return a non-empty Subject or error.
	ExchangeCode(ctx context.Context, code, verifier, state string) (*ExchangeResult, error)

	// RefreshToken refreshes an OAuth access token.
	// Receives a pre-configured *http.Client (caller selects based on account proxy).
	RefreshToken(ctx context.Context, client *http.Client, refreshToken string) (*TokenResponse, error)

	// --- Admin ---

	// Probe performs a minimal health check and returns the resulting account effect.
	Probe(ctx context.Context, acct *domain.Account, token string, client *http.Client) (ProbeResult, error)

	// DescribeAccount returns provider-specific fields suitable for admin display.
	DescribeAccount(acct *domain.Account) []AccountField

	// AutoPriority computes the effective priority for an auto-mode account.
	AutoPriority(state json.RawMessage) int

	// IsStale returns true if the account's rate-limit data needs refreshing.
	IsStale(state json.RawMessage, now time.Time) bool

	// ComputeExhaustedCooldown returns the cooldown-until time if rate limits are exhausted.
	// Returns zero time if no cooldown needed.
	ComputeExhaustedCooldown(state json.RawMessage, now time.Time) time.Time

	// CanServe reports whether the provider state allows serving the given model now.
	CanServe(state json.RawMessage, model string, now time.Time) bool

	// CalcCost computes the estimated cost in USD.
	CalcCost(model string, usage *Usage) float64

	// GetUtilization extracts utilization windows from provider state.
	GetUtilization(state json.RawMessage) []UtilWindow
}
