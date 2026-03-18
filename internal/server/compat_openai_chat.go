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
	r.Body = http.MaxBytesReader(w, r.Body, compatRequestBodyLimitBytes(s.cfg))

	releaseCompatSlot := func() {}
	if ki := auth.GetKeyInfo(r.Context()); ki != nil && !ki.IsAdmin {
		release, err := s.compatLimiter.Acquire(ki.ID, time.Now())
		if err != nil {
			writeCompatOpenAIError(w, http.StatusTooManyRequests, "rate_limit_error", err.Error())
			return
		}
		releaseCompatSlot = release
	}
	defer releaseCompatSlot()

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeCompatOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body")
		return
	}

	var req compatOpenAIChatRequest
	if err := json.NewDecoder(bytes.NewReader(rawBody)).Decode(&req); err != nil {
		writeCompatOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body")
		return
	}

	target, err := buildCompatTarget(&req)
	if err != nil {
		writeCompatOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if s.catalogDrivers[target.provider] == nil {
		writeCompatOpenAIError(w, http.StatusServiceUnavailable, "server_error", "provider compat surface is unavailable")
		return
	}

	traceID := ""
	if s.cfg != nil && s.cfg.TraceCompat {
		traceID = s.nextCompatTraceID()
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
	relayReq.Header.Set("Content-Type", "application/json")
	if clientMeta := buildCompatClientMeta(rawBody); clientMeta != "" {
		relayReq.Header.Set("X-Broker-Compat-Client-Meta", clientMeta)
	}
	if traceID != "" {
		relayReq.Header.Set("X-Broker-Compat-Trace-Id", traceID)
	}
	if target.stream {
		relayReq.Header.Set("Accept", "text/event-stream")
	}

	if target.stream {
		streamWriter := target.newStreamWriter(w, target.requestedModel)
		s.relay.HandleProviderSurface(target.provider, domain.SurfaceCompat).ServeHTTP(streamWriter, relayReq)
		streamWriter.finalize()
		return
	}

	capture := &compatResponseCapture{}
	s.relay.HandleProviderSurface(target.provider, domain.SurfaceCompat).ServeHTTP(capture, relayReq)

	status := capture.status
	if status == 0 {
		status = http.StatusOK
	}
	if status != http.StatusOK {
		writeCompatOpenAIUpstreamError(w, status, capture.body.Bytes())
		return
	}

	openAIResp, err := target.convertResponse(capture.body.Bytes(), target.requestedModel)
	if err != nil {
		writeCompatOpenAIError(w, http.StatusBadGateway, "server_error", "failed to convert compat response")
		return
	}
	writeJSON(w, http.StatusOK, openAIResp)
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
			stream:          req.Stream,
			convertResponse: compatClaudeToOpenAIChatResponse,
			newStreamWriter: func(w http.ResponseWriter, requestedModel string) compatStreamWriter {
				return newCompatOpenAIStreamWriter(w, requestedModel)
			},
		}, nil

	case domain.ProviderGemini:
		geminiReq, err := compatOpenAIChatToGeminiRequest(req)
		if err != nil {
			return nil, err
		}
		upstreamBody, err := json.Marshal(geminiReq)
		if err != nil {
			return nil, errCompat("failed to marshal gemini compat request")
		}
		relayPath := "/gemini/v1internal:generateContent"
		if req.Stream {
			relayPath = "/gemini/v1internal:streamGenerateContent"
		}
		return &compatTarget{
			provider:        domain.ProviderGemini,
			requestedModel:  requestedModel,
			relayPath:       relayPath,
			upstreamBody:    upstreamBody,
			stream:          req.Stream,
			convertResponse: compatGeminiToOpenAIChatResponse,
			newStreamWriter: func(w http.ResponseWriter, requestedModel string) compatStreamWriter {
				return newCompatGeminiStreamWriter(w, requestedModel)
			},
		}, nil
	}

	return nil, errCompat("unsupported compat provider")
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
		if !strings.HasPrefix(baseModel, "claude-") {
			return "", "", "", errCompat("model must be a claude model, e.g. claude/claude-sonnet-4-5")
		}
		return domain.ProviderClaude, baseModel, "claude/" + baseModel, nil

	case providerPrefix == "gemini" || providerPrefix == "google" || (providerPrefix == "" && strings.HasPrefix(baseModel, "gemini-")):
		if !strings.HasPrefix(baseModel, "gemini-") {
			return "", "", "", errCompat("model must be a gemini model, e.g. gemini/gemini-2.5-flash")
		}
		return domain.ProviderGemini, baseModel, "gemini/" + baseModel, nil
	}

	return "", "", "", errCompat("model must be a claude or gemini model, e.g. claude/claude-sonnet-4-5 or gemini/gemini-2.5-flash")
}

func compatSupportedProviders() []domain.Provider {
	return []domain.Provider{
		domain.ProviderClaude,
		domain.ProviderGemini,
	}
}

func compatProviderPrefix(provider domain.Provider) string {
	switch provider {
	case domain.ProviderClaude:
		return "claude"
	case domain.ProviderGemini:
		return "gemini"
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
