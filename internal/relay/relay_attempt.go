package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/neterr"
	"github.com/yansircc/llm-broker/internal/pool"
)

type relayAttemptOutcome int

const (
	relayAttemptContinue relayAttemptOutcome = iota
	relayAttemptStop
	relayAttemptDone
)

type relayAttemptState struct {
	exclusions         []pool.Exclusion
	forbiddenRetries   map[string]int
	lastErr            error
	lastUpstreamStatus int
	lastUpstreamBody   []byte
}

func newRelayAttemptState() *relayAttemptState {
	return &relayAttemptState{
		forbiddenRetries: make(map[string]int),
	}
}

func (s *relayAttemptState) boundAccountID(prepared *preparedRelayRequest, attempt int) string {
	boundID := prepared.keyInfo.BoundAccountID
	if attempt == 0 && prepared.sessionBoundAccountID != "" && boundID == "" {
		return prepared.sessionBoundAccountID
	}
	return boundID
}

func (s *relayAttemptState) exclude(acct *domain.Account, effect driver.Effect) {
	if effect.Scope == driver.EffectScopeBucket && acct.BucketKey != "" {
		s.exclusions = append(s.exclusions, pool.ExcludeBucket(acct.BucketKey))
		return
	}
	s.exclusions = append(s.exclusions, pool.ExcludeAccount(acct.ID))
}

func (r *Relay) executeRelayAttempt(
	ctx context.Context,
	w http.ResponseWriter,
	drv driver.ExecutionDriver,
	prepared *preparedRelayRequest,
	state *relayAttemptState,
	attempt int,
) relayAttemptOutcome {
	acct, err := r.pool.PickForSurface(drv, state.exclusions, prepared.input.Model, state.boundAccountID(prepared, attempt), prepared.surface)
	if err != nil {
		state.lastErr = err
		return relayAttemptStop
	}

	accessToken, err := r.tokens.EnsureValidToken(ctx, acct.ID)
	if err != nil {
		slog.Warn("token invalid, excluding account", "accountId", acct.ID, "error", err)
		state.exclusions = append(state.exclusions, pool.ExcludeAccount(acct.ID))
		state.lastErr = err
		return relayAttemptContinue
	}

	upReq, err := drv.BuildRequest(ctx, prepared.input, acct, accessToken)
	if err != nil {
		state.lastErr = fmt.Errorf("build request: %w", err)
		return relayAttemptStop
	}
	if r.shouldTraceCompat(prepared) {
		upstreamBody, snapErr := snapshotRequestBody(upReq)
		if snapErr != nil {
			slog.Warn("compat trace request snapshot failed",
				"traceId", compatTraceID(prepared),
				"clientPath", safeInputPath(prepared),
				"upstreamURL", safeRequestURL(upReq),
				"error", snapErr,
			)
		}
		r.logCompatTraceRequest(prepared, acct, attempt, upReq, upstreamBody)
	}

	attemptStartedAt := time.Now()
	resp, err := r.transport.ClientForAccount(acct).Do(upReq)
	if err != nil {
		if r.shouldTraceCompat(prepared) {
			r.logCompatTraceTransportError(prepared, acct, attempt, upReq, err)
		}
		if acct.CellID != "" && r.cfg.CellErrorPause > 0 && neterr.IsTransport(err) {
			r.pool.CooldownCell(acct.CellID, time.Now().Add(r.cfg.CellErrorPause), fmt.Sprintf("relay transport error on account %s: %v", acct.Email, err))
		}
		r.logRequestAsync(&domain.RequestLog{
			UserID:     prepared.keyInfo.ID,
			AccountID:  acct.ID,
			Model:      prepared.input.Model,
			Status:     "transport_error",
			DurationMs: time.Since(attemptStartedAt).Milliseconds(),
			CreatedAt:  time.Now().UTC(),
		})
		slog.Error("upstream request failed",
			"accountId", acct.ID,
			"userId", prepared.keyInfo.ID,
			"userName", prepared.keyInfo.Name,
			"model", prepared.input.Model,
			"path", prepared.input.Path,
			"sessionUUID", prepared.sessionUUID,
			"clientRetryCount", prepared.input.Headers.Get("X-Stainless-Retry-Count"),
			"error", err,
		)
		state.exclusions = append(state.exclusions, pool.ExcludeAccount(acct.ID))
		state.lastErr = err
		return relayAttemptContinue
	}

	var tracedRespBody []byte
	if r.shouldTraceCompat(prepared) {
		if prepared.input.IsStream {
			r.logCompatTraceResponse(prepared, acct, attempt, upReq, resp, nil)
		} else {
			body, snapErr := snapshotResponseBody(resp)
			if snapErr != nil {
				slog.Warn("compat trace response snapshot failed",
					"traceId", compatTraceID(prepared),
					"clientPath", safeInputPath(prepared),
					"upstreamURL", safeRequestURL(upReq),
					"status", resp.StatusCode,
					"error", snapErr,
				)
			} else {
				tracedRespBody = body
				r.logCompatTraceResponse(prepared, acct, attempt, upReq, resp, tracedRespBody)
			}
		}
	}

	if drv.ShouldRetry(resp.StatusCode) && attempt < r.cfg.MaxRetryAccounts {
		errBody := tracedRespBody
		if errBody == nil {
			errBody, _ = io.ReadAll(resp.Body)
		}
		resp.Body.Close()

		r.logRequestAsync(&domain.RequestLog{
			UserID:     prepared.keyInfo.ID,
			AccountID:  acct.ID,
			Model:      prepared.input.Model,
			Status:     fmt.Sprintf("upstream_%d", resp.StatusCode),
			DurationMs: time.Since(attemptStartedAt).Milliseconds(),
			CreatedAt:  time.Now().UTC(),
		})

		slog.Warn("retriable upstream error",
			"status", resp.StatusCode,
			"accountId", acct.ID,
			"userId", prepared.keyInfo.ID,
			"userName", prepared.keyInfo.Name,
			"model", prepared.input.Model,
			"path", prepared.input.Path,
			"sessionUUID", prepared.sessionUUID,
			"clientRetryCount", prepared.input.Headers.Get("X-Stainless-Retry-Count"),
			"body", truncate(string(errBody), 500),
		)

		state.lastUpstreamStatus = resp.StatusCode
		state.lastUpstreamBody = errBody

		if drv.ParseNonRetriable(resp.StatusCode, errBody) {
			drv.WriteUpstreamError(w, resp.StatusCode, errBody, prepared.input.IsStream)
			return relayAttemptDone
		}

		if drv.RetrySameAccount(resp.StatusCode, errBody, state.forbiddenRetries[acct.ID]) {
			state.forbiddenRetries[acct.ID]++
			state.lastErr = fmt.Errorf("upstream %d (retry %d)", resp.StatusCode, state.forbiddenRetries[acct.ID])
			return relayAttemptContinue
		}

		effect := drv.Interpret(resp.StatusCode, resp.Header, errBody, prepared.input.Model, json.RawMessage(acct.ProviderStateJSON))
		r.pool.Observe(acct.ID, effect)
		state.exclude(acct, effect)
		state.lastErr = fmt.Errorf("upstream %d", resp.StatusCode)
		return relayAttemptContinue
	}

	if resp.StatusCode != http.StatusOK {
		errBody := tracedRespBody
		if errBody == nil {
			errBody, _ = io.ReadAll(resp.Body)
		}
		resp.Body.Close()

		r.logRequestAsync(&domain.RequestLog{
			UserID:     prepared.keyInfo.ID,
			AccountID:  acct.ID,
			Model:      prepared.input.Model,
			Status:     fmt.Sprintf("upstream_%d", resp.StatusCode),
			DurationMs: time.Since(attemptStartedAt).Milliseconds(),
			CreatedAt:  time.Now().UTC(),
		})

		effect := drv.Interpret(resp.StatusCode, resp.Header, errBody, prepared.input.Model, json.RawMessage(acct.ProviderStateJSON))
		r.pool.Observe(acct.ID, effect)

		slog.Warn("upstream non-retriable error",
			"status", resp.StatusCode,
			"accountId", acct.ID,
			"userId", prepared.keyInfo.ID,
			"userName", prepared.keyInfo.Name,
			"model", prepared.input.Model,
			"path", prepared.input.Path,
			"sessionUUID", prepared.sessionUUID,
			"clientRetryCount", prepared.input.Headers.Get("X-Stainless-Retry-Count"),
			"body", truncate(string(errBody), 500),
		)

		drv.WriteUpstreamError(w, resp.StatusCode, errBody, prepared.input.IsStream)
		return relayAttemptDone
	}

	r.finishRelaySuccess(ctx, w, drv, prepared, acct, resp)
	return relayAttemptDone
}

func (r *Relay) finishRelaySuccess(
	ctx context.Context,
	w http.ResponseWriter,
	drv driver.ExecutionDriver,
	prepared *preparedRelayRequest,
	acct *domain.Account,
	resp *http.Response,
) {
	defer resp.Body.Close()

	effect := drv.Interpret(http.StatusOK, resp.Header, nil, prepared.input.Model, json.RawMessage(acct.ProviderStateJSON))
	r.pool.Observe(acct.ID, effect)

	if prepared.sessionUUID != "" {
		if err := r.pool.SetSessionBinding(ctx, prepared.sessionUUID, acct.ID, r.cfg.SessionBindingTTL); err != nil {
			slog.Warn("save session binding failed", "sessionUUID", prepared.sessionUUID, "accountId", acct.ID, "error", err)
		}
	}

	startTime := time.Now()
	var usage *driver.Usage
	if prepared.input.IsStream {
		_, usage = drv.StreamResponse(ctx, w, resp)
	} else {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			drv.WriteError(w, http.StatusBadGateway, "failed to read upstream response")
			return
		}
		usage = drv.ParseJSONUsage(respBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	}

	if usage != nil {
		r.logUsageAsync(prepared.keyInfo.ID, acct.ID, prepared.input.Model, usage, drv.CalcCost(prepared.input.Model, usage), startTime)
	}
}

func (r *Relay) logRequestAsync(entry *domain.RequestLog) {
	if entry == nil {
		return
	}
	go func() {
		_ = r.store.InsertRequestLog(context.Background(), entry)
	}()
}

func (r *Relay) logUsageAsync(userID, accountID, model string, usage *driver.Usage, cost float64, startTime time.Time) {
	r.logRequestAsync(&domain.RequestLog{
		UserID:            userID,
		AccountID:         accountID,
		Model:             model,
		InputTokens:       usage.InputTokens,
		OutputTokens:      usage.OutputTokens,
		CacheReadTokens:   usage.CacheReadTokens,
		CacheCreateTokens: usage.CacheCreateTokens,
		CostUSD:           cost,
		Status:            "ok",
		DurationMs:        time.Since(startTime).Milliseconds(),
		CreatedAt:         time.Now().UTC(),
	})
}

func (r *Relay) finishRelayFailure(w http.ResponseWriter, drv driver.ExecutionDriver, prepared *preparedRelayRequest, state *relayAttemptState) {
	if state.lastErr != nil {
		slog.Error("all relay attempts failed", "error", state.lastErr, "provider", drv.Provider())
		evt := events.Event{
			Type:    events.EventRelayError,
			Message: fmt.Sprintf("%s: all relay attempts failed: %s", drv.Provider(), state.lastErr.Error()),
		}
		if prepared != nil && prepared.keyInfo != nil {
			evt.UserID = prepared.keyInfo.ID
		}
		r.bus.Publish(evt)
	}
	if state.lastUpstreamBody != nil {
		drv.WriteUpstreamError(w, state.lastUpstreamStatus, state.lastUpstreamBody, prepared.input.IsStream)
		return
	}
	drv.WriteError(w, http.StatusServiceUnavailable, "no available accounts")
}
