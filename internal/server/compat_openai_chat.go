package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/domain"
	relaypkg "github.com/yansircc/llm-broker/internal/relay"
	"github.com/yansircc/llm-broker/internal/requestid"
)

func (s *Server) handleCompatListModels(w http.ResponseWriter, r *http.Request) {
	data := make([]compatOpenAIModel, 0)
	for _, provider := range compatSupportedProviders() {
		drv := s.catalogDrivers[provider]
		if drv == nil {
			continue
		}
		prefix := compatProviderPrefix(provider)
		for _, model := range drv.Models() {
			data = append(data, compatOpenAIModel{
				ID:      prefix + "/" + model.ID,
				Object:  model.Object,
				Created: model.Created,
				OwnedBy: model.OwnedBy,
			})
		}
	}
	if len(data) == 0 {
		writeCompatOpenAIError(w, http.StatusServiceUnavailable, "server_error", "compat surface is unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   data,
	})
}

func (s *Server) handleCompatOpenAIChatCompletions(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	ow := newObservedResponseWriter(w)
	r = requestid.Ensure(r, ow)
	r.Body = http.MaxBytesReader(ow, r.Body, compatRequestBodyLimitBytes(s.cfg))

	var rawBody []byte
	var req compatOpenAIChatRequest
	var target *compatTarget
	reqRef := func() *compatOpenAIChatRequest {
		if strings.TrimSpace(req.Model) == "" && len(req.Messages) == 0 && !req.Stream {
			return nil
		}
		return &req
	}
	logFailure := func(phase, status, effectKind, message string, extra map[string]any) {
		s.logCompatFailureRequest(r, rawBody, reqRef(), target, phase, ow, status, effectKind, time.Since(startedAt), message, extra)
		s.logCompatCompletion(r, reqRef(), target, ow, time.Since(startedAt), "error", extra)
	}

	releaseCompatSlot := func() {}
	if ki := auth.GetKeyInfo(r.Context()); ki != nil && !ki.IsAdmin {
		release, err := s.compatLimiter.Acquire(ki.ID, time.Now())
		if err != nil {
			writeCompatOpenAIError(ow, http.StatusTooManyRequests, "rate_limit_error", err.Error())
			logFailure("compat_preflight", compatFailureStatusCode(ow.statusCode()), "overload", err.Error(), map[string]any{
				"error_type": "rate_limit_error",
			})
			return
		}
		releaseCompatSlot = release
	}
	defer releaseCompatSlot()

	var err error
	rawBody, err = io.ReadAll(r.Body)
	if err != nil {
		writeCompatOpenAIError(ow, http.StatusBadRequest, "invalid_request_error", "invalid JSON body")
		logFailure("compat_preflight", compatFailureStatusCode(ow.statusCode()), "reject", "invalid JSON body", map[string]any{
			"error_type": "invalid_request_error",
		})
		return
	}

	if err := json.NewDecoder(bytes.NewReader(rawBody)).Decode(&req); err != nil {
		writeCompatOpenAIError(ow, http.StatusBadRequest, "invalid_request_error", "invalid JSON body")
		logFailure("compat_preflight", compatFailureStatusCode(ow.statusCode()), "reject", "invalid JSON body", map[string]any{
			"error_type": "invalid_request_error",
		})
		return
	}

	target, err = buildCompatTarget(&req)
	if err != nil {
		writeCompatOpenAIError(ow, http.StatusBadRequest, "invalid_request_error", err.Error())
		logFailure("compat_preflight", compatFailureStatusCode(ow.statusCode()), "reject", err.Error(), map[string]any{
			"error_type": "invalid_request_error",
		})
		return
	}
	if s.catalogDrivers[target.provider] == nil {
		writeCompatOpenAIError(ow, http.StatusServiceUnavailable, "server_error", "provider compat surface is unavailable")
		logFailure("compat_preflight", compatFailureStatusCode(ow.statusCode()), "server_error", "provider compat surface is unavailable", map[string]any{
			"error_type": "server_error",
		})
		return
	}

	slog.Info("compat request translated",
		"requestId", requestid.FromRequest(r),
		"path", r.URL.Path,
		"provider", target.provider,
		"requestedModel", strings.TrimSpace(req.Model),
		"translatedModel", target.requestedModel,
		"relayPath", target.relayPath,
		"stream", req.Stream,
	)

	traceID := ""
	if s.cfg != nil && s.cfg.TraceCompat {
		traceID = requestid.FromRequest(r)
		s.logCompatTranslationTrace(traceID, rawBody, r.URL.Path, target.relayPath, target.upstreamBody)
	}

	relayReq := r.Clone(r.Context())
	relayURL := *r.URL
	relayURL.Path = target.relayPath
	relayURL.RawPath = ""
	relayReq.URL = &relayURL
	relayReq.Method = http.MethodPost
	relayReq.Body = io.NopCloser(bytes.NewReader(target.upstreamBody))
	relayReq.ContentLength = int64(len(target.upstreamBody))
	relayReq.Header = r.Header.Clone()
	relayReq = relaypkg.WithClientRequestObservation(relayReq, &relaypkg.ClientRequestObservation{
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Headers:  r.Header.Clone(),
		Body:     rawBody,
	})
	relayReq.Header.Set("Content-Type", "application/json")
	if clientMeta := buildCompatClientMeta(rawBody); clientMeta != "" {
		relayReq.Header.Set("X-Broker-Compat-Client-Meta", clientMeta)
	}
	if traceID != "" {
		relayReq.Header.Set("X-Broker-Compat-Trace-Id", traceID)
	}
	for key, values := range target.upstreamHeaders {
		relayReq.Header.Del(key)
		for _, value := range values {
			relayReq.Header.Add(key, value)
		}
	}
	if target.upstreamAccept != "" {
		relayReq.Header.Set("Accept", target.upstreamAccept)
	} else if target.stream {
		relayReq.Header.Set("Accept", "text/event-stream")
	}

	if target.stream {
		streamWriter := target.newStreamWriter(ow, target.requestedModel)
		s.relay.HandleProviderSurface(target.provider, domain.SurfaceCompat).ServeHTTP(streamWriter, relayReq)
		streamWriter.finalize()
		streamMeta := streamWriter.ClientResponseObservation()
		if !streamWriter.completed() && ow.statusCode() == http.StatusOK {
			logFailure("compat_final", "compat_stream_incomplete", "stream_incomplete", "stream response did not complete cleanly", map[string]any{
				"stream_writer": streamMeta,
			})
			return
		}
		s.logCompatCompletion(r, &req, target, ow, time.Since(startedAt), "ok", map[string]any{
			"stream_writer": streamMeta,
		})
		return
	}

	capture := &compatResponseCapture{}
	s.relay.HandleProviderSurface(target.provider, domain.SurfaceCompat).ServeHTTP(capture, relayReq)

	status := capture.status
	if status == 0 {
		status = http.StatusOK
	}
	if status != http.StatusOK {
		writeCompatOpenAIUpstreamError(ow, status, capture.body.Bytes())
		s.logCompatCompletion(r, &req, target, ow, time.Since(startedAt), "error", map[string]any{
			"relay_status": status,
		})
		return
	}

	openAIResp, err := target.convertResponse(capture.body.Bytes(), target.requestedModel)
	if err != nil {
		writeCompatOpenAIError(ow, http.StatusBadGateway, "server_error", "failed to convert compat response")
		logFailure("compat_final", compatFailureStatusCode(ow.statusCode()), "server_error", "failed to convert compat response", map[string]any{
			"relay_status":  status,
			"convert_error": err.Error(),
		})
		return
	}
	writeJSON(ow, http.StatusOK, openAIResp)
	s.logCompatCompletion(r, &req, target, ow, time.Since(startedAt), "ok", map[string]any{
		"relay_status": status,
	})
}

func buildCompatTarget(req *compatOpenAIChatRequest) (*compatTarget, error) {
	provider, _, requestedModel, err := resolveCompatModel(req.Model)
	if err != nil {
		return nil, err
	}

	switch provider {
	case domain.ProviderClaude:
		claudeReq, _, err := compatOpenAIChatToClaudeRequest(req)
		if err != nil {
			return nil, err
		}
		upstreamBody, err := json.Marshal(claudeReq)
		if err != nil {
			return nil, errCompat("failed to marshal claude compat request")
		}
		return &compatTarget{
			provider:        domain.ProviderClaude,
			requestedModel:  requestedModel,
			relayPath:       "/v1/messages",
			upstreamBody:    upstreamBody,
			upstreamAccept:  "text/event-stream",
			upstreamHeaders: compatClaudeUpstreamHeaders(claudeReq.Model),
			stream:          req.Stream,
			convertResponse: compatClaudeToOpenAIChatResponse,
			newStreamWriter: func(w http.ResponseWriter, requestedModel string) compatStreamWriter {
				return newCompatOpenAIStreamWriter(w, requestedModel)
			},
		}, nil

	default:
		return nil, errCompat("unsupported provider")
	}
}

func compatClaudeUpstreamHeaders(model string) http.Header {
	headers := make(http.Header)
	if !compatClaudeUsesModernEnvelope(model) {
		return headers
	}
	headers.Set("Anthropic-Beta", strings.Join([]string{
		"redact-thinking-2026-02-12",
		"context-management-2025-06-27",
		"prompt-caching-scope-2026-01-05",
		"effort-2025-11-24",
	}, ","))
	headers.Set("Anthropic-Dangerous-Direct-Browser-Access", "true")
	headers.Set("X-App", "cli")
	return headers
}

var compatClaudeModelAliases = map[string]string{
	"claude-haiku-4.5":           "claude-haiku-4-5",
	"claude-opus-4.0":            "claude-opus-4",
	"claude-opus-4-0":            "claude-opus-4",
	"claude-opus-4.1":            "claude-opus-4-1",
	"claude-opus-4-1-20250805":   "claude-opus-4-1",
	"claude-opus-4.5":            "claude-opus-4-5",
	"claude-opus-4-5-20251101":   "claude-opus-4-5",
	"claude-opus-4.6":            "claude-opus-4-6",
	"claude-opus-4-20250514":     "claude-opus-4",
	"claude-sonnet-4.0":          "claude-sonnet-4",
	"claude-sonnet-4-0":          "claude-sonnet-4",
	"claude-sonnet-4.5":          "claude-sonnet-4-5",
	"claude-sonnet-4-5-20250929": "claude-sonnet-4-5",
	"claude-sonnet-4.6":          "claude-sonnet-4-6",
	"claude-sonnet-4-20250514":   "claude-sonnet-4-6",
}

func compatCanonicalClaudeModel(model string) string {
	trimmed := strings.ToLower(strings.TrimSpace(model))
	if canonical, ok := compatClaudeModelAliases[trimmed]; ok {
		return canonical
	}
	return trimmed
}

func resolveCompatModel(model string) (domain.Provider, string, string, error) {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "", "", "", errCompat("model is required")
	}

	providerPrefix := ""
	baseModel := trimmed
	if head, tail, ok := strings.Cut(trimmed, "/"); ok {
		providerPrefix = strings.ToLower(strings.TrimSpace(head))
		baseModel = strings.TrimSpace(tail)
	}

	switch {
	case providerPrefix == "claude" || providerPrefix == "anthropic" || (providerPrefix == "" && strings.HasPrefix(baseModel, "claude-")):
		baseModel = compatCanonicalClaudeModel(baseModel)
		if !strings.HasPrefix(baseModel, "claude-") {
			return "", "", "", errCompat("model must be a claude model, e.g. claude/claude-sonnet-4-5")
		}
		return domain.ProviderClaude, baseModel, "claude/" + baseModel, nil
	}

	return "", "", "", errCompat("model must be a claude model, e.g. claude/claude-sonnet-4-5")
}

func compatSupportedProviders() []domain.Provider {
	return []domain.Provider{
		domain.ProviderClaude,
	}
}

func compatProviderPrefix(provider domain.Provider) string {
	switch provider {
	case domain.ProviderClaude:
		return "claude"
	default:
		return string(provider)
	}
}

func compatProviderMatches(provider domain.Provider, requestedModel string) bool {
	return strings.HasPrefix(requestedModel, compatProviderPrefix(provider)+"/")
}

func compatRequestBodyLimitBytes(cfg *config.Config) int64 {
	if cfg != nil && cfg.MaxRequestBodyMB > 0 {
		return int64(cfg.MaxRequestBodyMB) << 20
	}
	return int64(compatDefaultRequestBodyMB) << 20
}

func (s *Server) nextCompatTraceID() string {
	return "compat-" + strconv.FormatUint(s.requestSeq.Add(1), 10)
}

func (s *Server) logCompatTranslationTrace(
	traceID string,
	clientBody []byte,
	clientPath string,
	relayPath string,
	translatedBody []byte,
) {
	clientText, clientTruncated := formatCompatTraceBody(clientBody)
	translatedText, translatedTruncated := formatCompatTraceBody(translatedBody)

	slog.Info("compat translation",
		"traceId", traceID,
		"clientPath", clientPath,
		"relayPath", relayPath,
		"clientBody", clientText,
		"clientBodyBytes", len(clientBody),
		"clientBodyTruncated", clientTruncated,
		"translatedBody", translatedText,
		"translatedBodyBytes", len(translatedBody),
		"translatedBodyTruncated", translatedTruncated,
	)
}
