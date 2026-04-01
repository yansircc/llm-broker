package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
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

func relayEffectKind(kind driver.EffectKind) string {
	switch kind {
	case driver.EffectSuccess:
		return "success"
	case driver.EffectCooldown:
		return "cooldown"
	case driver.EffectReject:
		return "reject"
	case driver.EffectOverload:
		return "overload"
	case driver.EffectBlock:
		return "block"
	case driver.EffectAuthFail:
		return "auth_fail"
	case driver.EffectServerError:
		return "server_error"
	default:
		return ""
	}
}

func requestValidationEffectKind(statusCode int) string {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return "overload"
	case statusCode == http.StatusUnauthorized:
		return "auth_fail"
	case statusCode >= 500:
		return "server_error"
	default:
		return "reject"
	}
}

func upstreamRequestID(headers http.Header) string {
	if headers == nil {
		return ""
	}
	if v := headers.Get("request-id"); v != "" {
		return v
	}
	return headers.Get("x-request-id")
}

func (r *Relay) baseRequestLog(prepared *preparedRelayRequest, acct *domain.Account, attempt int) *domain.RequestLog {
	entry := &domain.RequestLog{
		AttemptCount: attempt + 1,
		CreatedAt:    time.Now().UTC(),
	}
	if prepared != nil && prepared.keyInfo != nil {
		entry.UserID = prepared.keyInfo.ID
	}
	if prepared != nil && prepared.input != nil {
		entry.Surface = string(prepared.surface)
		entry.Model = prepared.input.Model
		entry.Path = requestLogPath(prepared)
		entry.RequestBytes = len(requestLogClientBody(prepared))
		entry.SessionUUID = prepared.sessionUUID
		entry.BindingSource = requestBindingSource(prepared)
		entry.ClientHeaders = requestClientHeaders(requestLogClientHeaders(prepared))
		entry.ClientBodyExcerpt = requestBodyExcerpt(requestLogClientBody(prepared))
		entry.RequestMeta = requestMeta(prepared)
	}
	if acct != nil {
		entry.AccountID = acct.ID
		entry.Provider = string(acct.Provider)
		entry.CellID = acct.CellID
		entry.BucketKey = acct.BucketKey
	}
	return entry
}

func setRequestLogUpstreamRequest(entry *domain.RequestLog, req *http.Request, body []byte) {
	if entry == nil || req == nil {
		return
	}
	entry.UpstreamURL = safeRequestURL(req)
	entry.UpstreamRequestHeaders = marshalObservationMapString(traceRequestHeaders(req.Header))
	entry.UpstreamRequestMeta = upstreamRequestMeta(req, body)
	entry.UpstreamRequestBodyExcerpt = requestBodyExcerpt(body)
}

func setRequestLogUpstreamResponse(entry *domain.RequestLog, resp *http.Response, body []byte, effect *driver.Effect) {
	if entry == nil {
		return
	}
	if resp != nil {
		entry.UpstreamRequestID = upstreamRequestID(resp.Header)
		entry.UpstreamHeaders = marshalObservationMapString(traceResponseHeaders(resp.Header))
		entry.UpstreamResponseMeta = upstreamResponseMeta(resp, body)
		entry.UpstreamResponseBodyExcerpt = requestBodyExcerpt(body)
	}
	if effect == nil {
		return
	}
	entry.UpstreamErrorType = effect.UpstreamErrorType
	entry.UpstreamErrorMessage = effect.UpstreamErrorMessage
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
	if attempt == 0 && prepared.userRouteAccountID != "" && boundID == "" {
		return prepared.userRouteAccountID
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
	boundAccountID := state.boundAccountID(prepared, attempt)
	acct, err := r.pool.PickForSurface(drv, state.exclusions, prepared.input.Model, boundAccountID, prepared.surface)
	if err != nil && boundAccountID != "" && boundAccountID == prepared.userRouteAccountID {
		slog.Info("sticky account unavailable, rerouting request",
			"userId", prepared.keyInfo.ID,
			"userName", prepared.keyInfo.Name,
			"provider", drv.Provider(),
			"surface", prepared.surface,
			"accountId", boundAccountID,
			"model", prepared.input.Model,
			"path", prepared.input.Path,
		)
		acct, err = r.pool.PickForSurface(drv, state.exclusions, prepared.input.Model, "", prepared.surface)
	}
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

	attemptStartedAt := time.Now()
	upReq, err := drv.BuildRequest(ctx, prepared.input, acct, accessToken)
	if err != nil {
		var requestErr *driver.RequestValidationError
		if errors.As(err, &requestErr) {
			entry := r.baseRequestLog(prepared, acct, attempt)
			entry.Status = fmt.Sprintf("validation_%d", requestErr.StatusCode)
			entry.EffectKind = requestValidationEffectKind(requestErr.StatusCode)
			entry.UpstreamStatus = requestErr.StatusCode
			entry.UpstreamErrorType = "request_validation_error"
			entry.UpstreamErrorMessage = requestErr.Message
			entry.DurationMs = time.Since(attemptStartedAt).Milliseconds()
			entry.RequestMeta = mergeObservationMeta(entry.RequestMeta, map[string]any{
				"error_phase": "build_request",
			})
			r.attachRequestLogArtifacts(entry, prepared, nil, nil)
			r.logRequestAsync(entry)

			slog.Warn("driver rejected request before upstream",
				"accountId", acct.ID,
				"userId", prepared.keyInfo.ID,
				"userName", prepared.keyInfo.Name,
				"model", prepared.input.Model,
				"path", prepared.input.Path,
				"sessionUUID", prepared.sessionUUID,
				"status", requestErr.StatusCode,
				"error", requestErr.Message,
			)
			drv.WriteError(w, requestErr.StatusCode, requestErr.Message)
			return relayAttemptDone
		}
		state.lastErr = fmt.Errorf("build request: %w", err)
		return relayAttemptStop
	}
	upstreamReqBody, snapErr := snapshotRequestBody(upReq)
	if snapErr != nil {
		slog.Warn("upstream request snapshot failed",
			"accountId", acct.ID,
			"userId", prepared.keyInfo.ID,
			"userName", prepared.keyInfo.Name,
			"model", prepared.input.Model,
			"path", prepared.input.Path,
			"sessionUUID", prepared.sessionUUID,
			"error", snapErr,
		)
	}
	if r.shouldTraceCompat(prepared) {
		if snapErr != nil {
			slog.Warn("compat trace request snapshot failed",
				"traceId", compatTraceID(prepared),
				"clientPath", safeInputPath(prepared),
				"upstreamURL", safeRequestURL(upReq),
				"error", snapErr,
			)
		} else {
			r.logCompatTraceRequest(prepared, acct, attempt, upReq, upstreamReqBody)
		}
	}

	// Anti-fingerprint jitter: small random delay before upstream request.
	// Respects context cancellation to avoid blocking cancelled requests.
	select {
	case <-time.After(time.Duration(rand.IntN(300)) * time.Millisecond):
	case <-ctx.Done():
		return relayAttemptDone
	}

	resp, err := r.transport.ClientForAccount(acct).Do(upReq)
	if err != nil {
		if r.shouldTraceCompat(prepared) {
			r.logCompatTraceTransportError(prepared, acct, attempt, upReq, err)
		}
		if acct.CellID != "" && r.cfg.CellErrorPause > 0 && neterr.IsTransport(err) {
			r.pool.CooldownCell(acct.CellID, time.Now().Add(r.cfg.CellErrorPause), fmt.Sprintf("relay transport error on account %s: %v", acct.Email, err))
		}
		entry := r.baseRequestLog(prepared, acct, attempt)
		entry.Status = "transport_error"
		entry.EffectKind = "transport_error"
		entry.DurationMs = time.Since(attemptStartedAt).Milliseconds()
		setRequestLogUpstreamRequest(entry, upReq, upstreamReqBody)
		r.attachRequestLogArtifacts(entry, prepared, upstreamReqBody, nil)
		r.logRequestAsync(entry)
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
		entry := r.baseRequestLog(prepared, acct, attempt)
		entry.Status = fmt.Sprintf("upstream_%d", resp.StatusCode)
		entry.UpstreamStatus = resp.StatusCode
		entry.DurationMs = time.Since(attemptStartedAt).Milliseconds()
		setRequestLogUpstreamRequest(entry, upReq, upstreamReqBody)
		effect := drv.Interpret(resp.StatusCode, resp.Header, errBody, prepared.input.Model, json.RawMessage(acct.ProviderStateJSON))
		setRequestLogUpstreamResponse(entry, resp, errBody, &effect)
		r.attachRequestLogArtifacts(entry, prepared, upstreamReqBody, errBody)

		if drv.ParseNonRetriable(resp.StatusCode, errBody) {
			r.logRequestAsync(entry)
			drv.WriteUpstreamError(w, resp.StatusCode, errBody, prepared.input.IsStream)
			return relayAttemptDone
		}

		if drv.RetrySameAccount(resp.StatusCode, errBody, state.forbiddenRetries[acct.ID]) {
			r.logRequestAsync(entry)
			state.forbiddenRetries[acct.ID]++
			state.lastErr = fmt.Errorf("upstream %d (retry %d)", resp.StatusCode, state.forbiddenRetries[acct.ID])
			return relayAttemptContinue
		}

		entry.EffectKind = relayEffectKind(effect.Kind)
		r.logRequestAsync(entry)
		r.pool.Observe(acct.ID, effect)
		r.maybeSetUserRouteBinding(ctx, prepared, drv, acct, effect)
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

		effect := drv.Interpret(resp.StatusCode, resp.Header, errBody, prepared.input.Model, json.RawMessage(acct.ProviderStateJSON))
		entry := r.baseRequestLog(prepared, acct, attempt)
		entry.Status = fmt.Sprintf("upstream_%d", resp.StatusCode)
		entry.UpstreamStatus = resp.StatusCode
		entry.EffectKind = relayEffectKind(effect.Kind)
		entry.DurationMs = time.Since(attemptStartedAt).Milliseconds()
		setRequestLogUpstreamRequest(entry, upReq, upstreamReqBody)
		setRequestLogUpstreamResponse(entry, resp, errBody, &effect)
		r.attachRequestLogArtifacts(entry, prepared, upstreamReqBody, errBody)
		r.logRequestAsync(entry)
		r.pool.Observe(acct.ID, effect)
		r.maybeSetUserRouteBinding(ctx, prepared, drv, acct, effect)

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

	r.finishRelaySuccess(ctx, w, drv, prepared, acct, upReq, upstreamReqBody, resp, attemptStartedAt, attempt)
	return relayAttemptDone
}

func (r *Relay) finishRelaySuccess(
	ctx context.Context,
	w http.ResponseWriter,
	drv driver.ExecutionDriver,
	prepared *preparedRelayRequest,
	acct *domain.Account,
	upReq *http.Request,
	upstreamReqBody []byte,
	resp *http.Response,
	attemptStartedAt time.Time,
	attempt int,
) {
	defer resp.Body.Close()

	effect := drv.Interpret(http.StatusOK, resp.Header, nil, prepared.input.Model, json.RawMessage(acct.ProviderStateJSON))
	r.pool.Observe(acct.ID, effect)
	r.maybeSetUserRouteBinding(ctx, prepared, drv, acct, effect)

	if prepared.sessionUUID != "" {
		if err := r.pool.SetSessionBinding(ctx, prepared.sessionUUID, acct.ID, r.cfg.SessionBindingTTL); err != nil {
			slog.Warn("save session binding failed", "sessionUUID", prepared.sessionUUID, "accountId", acct.ID, "error", err)
		}
	}

	var usage *driver.Usage
	var respBody []byte
	streamCompleted := true
	if prepared.input.IsStream {
		streamCompleted, usage = drv.StreamResponse(ctx, w, resp)
	} else {
		var err error
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			drv.WriteError(w, http.StatusBadGateway, "failed to read upstream response")
			return
		}
		usage = drv.ParseJSONUsage(respBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	}

	entry := r.baseRequestLog(prepared, acct, attempt)
	entry.Status = "ok"
	if prepared.input.IsStream && !streamCompleted {
		entry.Status = "stream_incomplete"
	}
	entry.EffectKind = relayEffectKind(effect.Kind)
	entry.DurationMs = time.Since(attemptStartedAt).Milliseconds()
	setRequestLogUpstreamRequest(entry, upReq, upstreamReqBody)
	setRequestLogUpstreamResponse(entry, resp, respBody, &effect)
	r.attachRequestLogArtifacts(entry, prepared, upstreamReqBody, respBody)
	if prepared.input.IsStream {
		entry.RequestMeta = mergeObservationMeta(entry.RequestMeta, map[string]any{
			"stream_completed": streamCompleted,
		})
		if observer, ok := w.(interface{ ClientResponseObservation() map[string]any }); ok {
			entry.RequestMeta = mergeObservationMeta(entry.RequestMeta, map[string]any{
				"client_response": observer.ClientResponseObservation(),
			})
		}
		if !streamCompleted {
			slog.Warn("stream relay ended before completion",
				"accountId", acct.ID,
				"userId", prepared.keyInfo.ID,
				"userName", prepared.keyInfo.Name,
				"model", prepared.input.Model,
				"path", prepared.input.Path,
				"sessionUUID", prepared.sessionUUID,
			)
		}
	}
	if usage != nil {
		entry.InputTokens = usage.InputTokens
		entry.OutputTokens = usage.OutputTokens
		entry.CacheReadTokens = usage.CacheReadTokens
		entry.CacheCreateTokens = usage.CacheCreateTokens
		entry.CostUSD = drv.CalcCost(prepared.input.Model, usage)
	}
	r.logRequestAsync(entry)
}

func (r *Relay) maybeSetUserRouteBinding(ctx context.Context, prepared *preparedRelayRequest, drv driver.ExecutionDriver, acct *domain.Account, effect driver.Effect) {
	if r == nil || prepared == nil || prepared.keyInfo == nil || acct == nil {
		return
	}
	if prepared.keyInfo.IsAdmin || prepared.keyInfo.BoundAccountID != "" {
		return
	}
	if effect.Kind != driver.EffectSuccess && effect.Kind != driver.EffectReject {
		return
	}
	if err := r.pool.SetUserRouteBinding(ctx, prepared.keyInfo.ID, drv.Provider(), prepared.surface, acct.ID); err != nil {
		slog.Warn("save user route binding failed",
			"userId", prepared.keyInfo.ID,
			"provider", drv.Provider(),
			"surface", prepared.surface,
			"accountId", acct.ID,
			"error", err,
		)
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
