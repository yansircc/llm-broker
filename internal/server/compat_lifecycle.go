package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/requestid"
)

type observedResponseWriter struct {
	dst     http.ResponseWriter
	status  int
	written int
	err     error
}

func newObservedResponseWriter(dst http.ResponseWriter) *observedResponseWriter {
	if existing, ok := dst.(*observedResponseWriter); ok {
		return existing
	}
	return &observedResponseWriter{dst: dst}
}

func (w *observedResponseWriter) Header() http.Header {
	return w.dst.Header()
}

func (w *observedResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.dst.WriteHeader(status)
}

func (w *observedResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.dst.Write(p)
	w.written += n
	if err != nil && w.err == nil {
		w.err = err
	}
	return n, err
}

func (w *observedResponseWriter) Flush() {
	if flusher, ok := w.dst.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *observedResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *observedResponseWriter) observation() map[string]any {
	if w == nil {
		return nil
	}
	meta := map[string]any{
		"http_status":   w.statusCode(),
		"written_bytes": w.written,
	}
	if contentType := strings.TrimSpace(w.Header().Get("Content-Type")); contentType != "" {
		meta["content_type"] = contentType
	}
	if w.err != nil {
		meta["write_error"] = w.err.Error()
	}
	return meta
}

func (s *Server) logCompatFailureRequest(
	r *http.Request,
	rawBody []byte,
	req *compatOpenAIChatRequest,
	target *compatTarget,
	phase string,
	ow *observedResponseWriter,
	status string,
	effectKind string,
	duration time.Duration,
	message string,
	extra map[string]any,
) {
	if s == nil || s.store == nil || r == nil {
		return
	}

	entry := &domain.RequestLog{
		Surface:           string(domain.SurfaceCompat),
		Path:              r.URL.Path,
		Status:            status,
		EffectKind:        effectKind,
		RequestBytes:      len(rawBody),
		DurationMs:        duration.Milliseconds(),
		CreatedAt:         time.Now().UTC(),
		ClientHeaders:     compatLifecycleHeaders(r.Header),
		ClientBodyExcerpt: compatLifecycleBodyExcerpt(rawBody),
	}

	if ki := auth.GetKeyInfo(r.Context()); ki != nil {
		entry.UserID = ki.ID
	}
	if req != nil {
		entry.Model = strings.TrimSpace(req.Model)
	}
	if target != nil {
		entry.Provider = string(target.provider)
		if strings.TrimSpace(target.requestedModel) != "" {
			entry.Model = target.requestedModel
		}
	}

	meta := map[string]any{
		"phase": phase,
	}
	if id := requestid.FromRequest(r); id != "" {
		meta["request_id"] = id
	}
	if req != nil {
		meta["stream"] = req.Stream
		if model := strings.TrimSpace(req.Model); model != "" {
			meta["requested_model"] = model
		}
	}
	if target != nil {
		meta["relay_path"] = target.relayPath
		meta["provider"] = string(target.provider)
		if target.stream {
			meta["translated_stream"] = true
		}
	}
	if compatMeta := compatLifecycleCompatClient(rawBody); len(compatMeta) > 0 {
		meta["compat_client"] = compatMeta
	}
	if owMeta := ow.observation(); len(owMeta) > 0 {
		meta["client_response"] = owMeta
	}
	if message = strings.TrimSpace(message); message != "" {
		meta["error_message"] = message
	}
	for key, value := range extra {
		if compatLifecycleHasValue(value) {
			meta[key] = value
		}
	}
	entry.RequestMeta = compatLifecycleMarshal(meta)

	go func() {
		if err := s.store.InsertRequestLog(context.Background(), entry); err != nil {
			slog.Warn("compat lifecycle request log failed", "status", status, "phase", phase, "error", err)
		}
	}()
}

func compatLifecycleHeaders(h http.Header) json.RawMessage {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string)
	for key, values := range h {
		if len(values) == 0 {
			continue
		}
		lower := strings.ToLower(key)
		if lower == "accept" ||
			lower == "content-type" ||
			lower == "user-agent" ||
			lower == "cf-ray" ||
			lower == "x-stainless-retry-count" ||
			lower == strings.ToLower(requestid.Header) ||
			strings.HasPrefix(lower, "x-broker-") {
			out[key] = strings.Join(values, ", ")
		}
	}
	if len(out) == 0 {
		return nil
	}
	data, err := json.Marshal(out)
	if err != nil || string(data) == "{}" {
		return nil
	}
	return json.RawMessage(data)
}

func compatLifecycleBodyExcerpt(body []byte) string {
	text, _ := formatCompatTraceBody(body)
	return text
}

func compatLifecycleCompatClient(rawBody []byte) map[string]any {
	if meta := buildCompatClientMeta(rawBody); meta != "" {
		var value map[string]any
		if json.Unmarshal([]byte(meta), &value) == nil {
			return value
		}
	}
	return nil
}

func compatLifecycleMarshal(value map[string]any) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) == "{}" {
		return nil
	}
	return json.RawMessage(data)
}

func compatLifecycleHasValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func (s *Server) logCompatCompletion(
	r *http.Request,
	req *compatOpenAIChatRequest,
	target *compatTarget,
	ow *observedResponseWriter,
	duration time.Duration,
	outcome string,
	extra map[string]any,
) {
	if r == nil {
		return
	}
	attrs := []any{
		"requestId", requestid.FromRequest(r),
		"path", r.URL.Path,
		"outcome", outcome,
		"durationMs", duration.Milliseconds(),
		"clientResponse", ow.observation(),
	}
	if ki := auth.GetKeyInfo(r.Context()); ki != nil {
		attrs = append(attrs, "userId", ki.ID, "userName", ki.Name)
	}
	if req != nil {
		if model := strings.TrimSpace(req.Model); model != "" {
			attrs = append(attrs, "requestedModel", model)
		}
		attrs = append(attrs, "stream", req.Stream)
	}
	if target != nil {
		attrs = append(attrs,
			"provider", target.provider,
			"relayPath", target.relayPath,
			"translatedStream", target.stream,
		)
	}
	for key, value := range extra {
		if compatLifecycleHasValue(value) {
			attrs = append(attrs, key, value)
		}
	}
	if outcome == "ok" {
		slog.Info("compat request completed", attrs...)
		return
	}
	slog.Warn("compat request completed", attrs...)
}

func compatFailureStatusCode(status int) string {
	if status <= 0 {
		return "compat_failed"
	}
	return "compat_" + strconv.Itoa(status)
}
