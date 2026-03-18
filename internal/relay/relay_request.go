package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
)

type preparedRelayRequest struct {
	keyInfo               *auth.KeyInfo
	input                 *driver.RelayInput
	clientObservation     *ClientRequestObservation
	surface               domain.Surface
	sessionUUID           string
	sessionBoundAccountID string
	userRouteAccountID    string
}

func (r *Relay) prepareRelayRequest(w http.ResponseWriter, req *http.Request, drv driver.ExecutionDriver, surface domain.Surface) (*preparedRelayRequest, bool) {
	keyInfo := auth.GetKeyInfo(req.Context())
	if keyInfo == nil {
		drv.WriteError(w, http.StatusUnauthorized, "not authenticated")
		return nil, true
	}

	input, plan, ok := r.parseRelayInput(w, req, drv, keyInfo)
	if !ok {
		return nil, true
	}

	if drv.InterceptRequest(w, input.Body, input.Model) {
		return nil, true
	}

	if plan.IsCountTokens {
		r.handleCountTokens(w, req, drv, input, keyInfo, surface)
		return nil, true
	}

	sessionBoundAccountID, ok := r.resolveSessionBoundAccount(req.Context(), w, drv, input.Model, plan, surface)
	if !ok {
		return nil, true
	}
	userRouteAccountID := r.resolveUserRouteAccount(req.Context(), drv, keyInfo, input.Model, surface, sessionBoundAccountID)

	return &preparedRelayRequest{
		keyInfo:               keyInfo,
		input:                 input,
		clientObservation:     clientObservationFromContext(req.Context()),
		surface:               surface,
		sessionUUID:           plan.SessionUUID,
		sessionBoundAccountID: sessionBoundAccountID,
		userRouteAccountID:    userRouteAccountID,
	}, false
}

func (r *Relay) parseRelayInput(w http.ResponseWriter, req *http.Request, drv driver.ExecutionDriver, keyInfo *auth.KeyInfo) (*driver.RelayInput, driver.RelayPlan, bool) {
	req.Body = http.MaxBytesReader(w, req.Body, int64(r.cfg.MaxRequestBodyMB)<<20)

	rawBody, err := io.ReadAll(req.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			drv.WriteError(w, http.StatusRequestEntityTooLarge, "request body exceeds size limit")
			return nil, driver.RelayPlan{}, false
		}
		drv.WriteError(w, http.StatusBadRequest, "failed to read request body")
		evt := events.Event{
			Type:    events.EventRelayError,
			Message: fmt.Sprintf("%s: failed to read request body: %s", drv.Provider(), err.Error()),
		}
		if keyInfo != nil {
			evt.UserID = keyInfo.ID
		}
		r.bus.Publish(evt)
		return nil, driver.RelayPlan{}, false
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		drv.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return nil, driver.RelayPlan{}, false
	}

	model, _ := body["model"].(string)
	input := &driver.RelayInput{
		Body:     body,
		RawBody:  rawBody,
		Headers:  req.Header,
		Path:     req.URL.Path,
		RawQuery: req.URL.RawQuery,
		Model:    model,
		UserID:   keyInfo.ID,
	}
	plan := drv.Plan(input)
	input.IsStream = plan.IsStream
	input.IsCountTokens = plan.IsCountTokens

	return input, plan, true
}

func (r *Relay) resolveSessionBoundAccount(ctx context.Context, w http.ResponseWriter, drv driver.ExecutionDriver, model string, plan driver.RelayPlan, surface domain.Surface) (string, bool) {
	if plan.SessionUUID == "" {
		return "", true
	}

	boundID, ok, err := r.pool.GetSessionBinding(ctx, plan.SessionUUID)
	if err != nil {
		slog.Error("load session binding failed", "sessionUUID", plan.SessionUUID, "error", err)
		drv.WriteError(w, http.StatusServiceUnavailable, "session state unavailable")
		return "", false
	}
	if !ok {
		return "", true
	}
	if r.pool.IsAvailableForSurface(boundID, drv, model, surface) {
		if err := r.pool.RenewSessionBinding(ctx, plan.SessionUUID, r.cfg.SessionBindingTTL); err != nil {
			slog.Warn("renew session binding failed", "sessionUUID", plan.SessionUUID, "accountId", boundID, "error", err)
		}
		return boundID, true
	}
	if plan.RejectUnavailableSession {
		slog.Warn("session pollution detected", "sessionUUID", plan.SessionUUID, "boundAccountId", boundID)
		drv.WriteError(w, http.StatusBadRequest, "bound account unavailable, please start a new session")
		return "", false
	}
	return "", true
}

func (r *Relay) resolveUserRouteAccount(ctx context.Context, drv driver.ExecutionDriver, keyInfo *auth.KeyInfo, model string, surface domain.Surface, sessionBoundAccountID string) string {
	if keyInfo == nil || keyInfo.IsAdmin || keyInfo.BoundAccountID != "" || sessionBoundAccountID != "" {
		return ""
	}

	accountID, ok, err := r.pool.GetUserRouteBinding(ctx, keyInfo.ID, drv.Provider(), surface)
	if err != nil {
		slog.Warn("load user route binding failed", "userId", keyInfo.ID, "provider", drv.Provider(), "surface", surface, "error", err)
		return ""
	}
	if !ok {
		return ""
	}
	if r.pool.IsAvailableForSurface(accountID, drv, model, surface) {
		return accountID
	}
	return ""
}
