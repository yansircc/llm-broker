package relay

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/yansircc/llm-broker/internal/domain"
)

const compatTraceBodyLimit = 64 << 10

func (r *Relay) shouldTraceCompat(prepared *preparedRelayRequest) bool {
	return r != nil && r.cfg.TraceCompat && prepared != nil && prepared.surface == domain.SurfaceCompat
}

func snapshotRequestBody(req *http.Request) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}
	if req.GetBody != nil {
		rc, err := req.GetBody()
		if err == nil {
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return body, nil
}

func snapshotResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return body, nil
}

func (r *Relay) logCompatTraceRequest(prepared *preparedRelayRequest, _ *domain.Account, attempt int, upReq *http.Request, upstreamBody []byte) {
	upstreamText, upstreamTruncated := formatTraceBody(upstreamBody)

	slog.Info("compat upstream request",
		"traceId", compatTraceID(prepared),
		"attempt", attempt+1,
		"clientPath", safeInputPath(prepared),
		"upstreamURL", safeRequestURL(upReq),
		"upstreamHeaders", traceRequestHeaders(upReq.Header),
		"upstreamBody", upstreamText,
		"upstreamBodyBytes", len(upstreamBody),
		"upstreamBodyTruncated", upstreamTruncated,
	)
}

func (r *Relay) logCompatTraceResponse(prepared *preparedRelayRequest, _ *domain.Account, attempt int, upReq *http.Request, resp *http.Response, respBody []byte) {
	bodyText, bodyTruncated := formatTraceBody(respBody)
	statusCode := 0
	headers := http.Header(nil)
	isStream := false
	if resp != nil {
		statusCode = resp.StatusCode
		headers = resp.Header
		isStream = prepared != nil && prepared.input != nil && prepared.input.IsStream
	}

	slog.Info("compat upstream response",
		"traceId", compatTraceID(prepared),
		"attempt", attempt+1,
		"clientPath", safeInputPath(prepared),
		"upstreamURL", safeRequestURL(upReq),
		"status", statusCode,
		"stream", isStream,
		"responseHeaders", traceResponseHeaders(headers),
		"responseBody", bodyText,
		"responseBodyBytes", len(respBody),
		"responseBodyTruncated", bodyTruncated,
	)
}

func (r *Relay) logCompatTraceTransportError(prepared *preparedRelayRequest, _ *domain.Account, attempt int, upReq *http.Request, err error) {
	slog.Info("compat upstream transport error",
		"traceId", compatTraceID(prepared),
		"attempt", attempt+1,
		"clientPath", safeInputPath(prepared),
		"upstreamURL", safeRequestURL(upReq),
		"error", err,
	)
}

func formatTraceBody(body []byte) (string, bool) {
	return formatBodyWithLimit(body, compatTraceBodyLimit)
}

func formatObservationBody(body []byte) (string, bool) {
	return formatBodyWithLimit(body, requestLogBodyExcerptLimit)
}

func formatBodyWithLimit(body []byte, limit int) (string, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", false
	}

	formatted := trimmed
	if bytes.HasPrefix(trimmed, []byte("{")) || bytes.HasPrefix(trimmed, []byte("[")) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, trimmed); err == nil {
			formatted = compact.Bytes()
		}
	}

	if len(formatted) <= limit {
		return string(formatted), false
	}
	const marker = "...<truncated>..."
	if limit <= len(marker)+2 {
		if limit <= 0 {
			return "", true
		}
		return string(formatted[:limit]), true
	}
	head := (limit - len(marker)) / 2
	tail := limit - len(marker) - head
	return string(formatted[:head]) + marker + string(formatted[len(formatted)-tail:]), true
}

func traceRequestHeaders(h http.Header) map[string]string {
	return traceHeaders(h, func(lower string) bool {
		return lower == "accept" ||
			lower == "content-type" ||
			lower == "user-agent" ||
			lower == "x-app" ||
			lower == "x-stainless-retry-count" ||
			lower == "anthropic-version" ||
			lower == "anthropic-beta" ||
			lower == "anthropic-dangerous-direct-browser-access" ||
			strings.HasPrefix(lower, "x-stainless-")
	})
}

func traceResponseHeaders(h http.Header) map[string]string {
	return traceHeaders(h, func(lower string) bool {
		return lower == "content-type" ||
			lower == "retry-after" ||
			lower == "x-request-id" ||
			lower == "request-id" ||
			lower == "cf-ray" ||
			strings.HasPrefix(lower, "anthropic-")
	})
}

func traceHeaders(h http.Header, allow func(string) bool) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string)
	for key, vals := range h {
		lower := strings.ToLower(key)
		if !allow(lower) || len(vals) == 0 {
			continue
		}
		out[key] = strings.Join(vals, ", ")
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func safeInputPath(prepared *preparedRelayRequest) string {
	return requestLogPath(prepared)
}

func compatTraceID(prepared *preparedRelayRequest) string {
	if prepared == nil || prepared.input == nil {
		return ""
	}
	return prepared.input.Headers.Get("X-Broker-Compat-Trace-Id")
}

func safeRequestURL(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	return req.URL.String()
}
