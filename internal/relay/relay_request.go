package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
)

type preparedRelayRequest struct {
	keyInfo            *auth.KeyInfo
	input              *driver.RelayInput
	clientObservation  *ClientRequestObservation
	surface            domain.Surface
	affinityKey        string
	affinityContinuity driver.AffinityContinuity
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

	affinityKey := routeAffinityKey(keyInfo.ID, drv.Provider(), surface, plan.Affinity)
	if plan.Affinity.Continuity == driver.AffinityRequire && affinityKey == "" && keyInfo.BoundAccountID == "" {
		drv.WriteError(w, http.StatusBadRequest, "conversation continuity cannot be resolved; start a new conversation")
		return nil, true
	}

	return &preparedRelayRequest{
		keyInfo:            keyInfo,
		input:              input,
		clientObservation:  clientObservationFromContext(req.Context()),
		surface:            surface,
		affinityKey:        affinityKey,
		affinityContinuity: plan.Affinity.Continuity,
	}, false
}

func routeAffinityKey(userID string, provider domain.Provider, surface domain.Surface, affinity driver.RouteAffinity) string {
	rawKey := strings.TrimSpace(affinity.RawKey)
	if rawKey == "" {
		return ""
	}
	parts := []string{
		"llm-broker-route-affinity-v1",
		userID,
		string(provider),
		string(domain.NormalizeSurface(string(surface))),
		affinity.Kind,
		rawKey,
	}
	var namespace strings.Builder
	for _, part := range parts {
		namespace.WriteString(strconv.Itoa(len(part)))
		namespace.WriteByte(':')
		namespace.WriteString(part)
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(namespace.String())).String()
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
