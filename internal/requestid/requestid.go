package requestid

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
)

type contextKey string

const (
	Header            = "X-Broker-Request-Id"
	key    contextKey = "brokerRequestID"
)

var seq atomic.Uint64

func Get(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(key).(string)
	return strings.TrimSpace(value)
}

func FromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if id := strings.TrimSpace(Get(r.Context())); id != "" {
		return id
	}
	return strings.TrimSpace(r.Header.Get(Header))
}

func Ensure(r *http.Request, w http.ResponseWriter) *http.Request {
	if r == nil {
		return r
	}
	id := FromRequest(r)
	if id == "" {
		id = fmt.Sprintf("req-%d", seq.Add(1))
	}
	if w != nil {
		w.Header().Set(Header, id)
	}
	clone := r.Clone(context.WithValue(r.Context(), key, id))
	clone.Header = r.Header.Clone()
	clone.Header.Set(Header, id)
	return clone
}
