package relay

import (
	"context"
	"net/http"
)

type clientObservationContextKey string

const clientObservationKey clientObservationContextKey = "relayClientObservation"

// ClientRequestObservation carries the original inbound request for log/replay
// when the relay input has already been translated by an upper layer.
type ClientRequestObservation struct {
	Path     string
	RawQuery string
	Headers  http.Header
	Body     []byte
}

func WithClientRequestObservation(req *http.Request, obs *ClientRequestObservation) *http.Request {
	if req == nil || obs == nil {
		return req
	}
	clone := &ClientRequestObservation{
		Path:     obs.Path,
		RawQuery: obs.RawQuery,
		Headers:  obs.Headers.Clone(),
		Body:     append([]byte(nil), obs.Body...),
	}
	ctx := context.WithValue(req.Context(), clientObservationKey, clone)
	return req.WithContext(ctx)
}

func clientObservationFromContext(ctx context.Context) *ClientRequestObservation {
	if ctx == nil {
		return nil
	}
	obs, _ := ctx.Value(clientObservationKey).(*ClientRequestObservation)
	return obs
}
