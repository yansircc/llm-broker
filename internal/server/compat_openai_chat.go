package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/domain"
)

const compatClaudeDefaultMaxTokens = 4096
const compatDefaultRequestBodyMB = 60

type compatOpenAIChatRequest struct {
	Model               string          `json:"model"`
	Messages            []compatMessage `json:"messages"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	Stop                json.RawMessage `json:"stop,omitempty"`
	ResponseFormat      json.RawMessage `json:"response_format,omitempty"`
	Stream              bool            `json:"stream,omitempty"`
	Tools               json.RawMessage `json:"tools,omitempty"`
	ToolChoice          json.RawMessage `json:"tool_choice,omitempty"`
}

type compatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type compatClaudeRequest struct {
	Model         string                `json:"model"`
	System        string                `json:"system,omitempty"`
	Messages      []compatClaudeMessage `json:"messages"`
	MaxTokens     int                   `json:"max_tokens"`
	Stream        bool                  `json:"stream,omitempty"`
	Temperature   *float64              `json:"temperature,omitempty"`
	TopP          *float64              `json:"top_p,omitempty"`
	StopSequences []string              `json:"stop_sequences,omitempty"`
}

type compatClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type compatClaudeResponse struct {
	ID         string `json:"id"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
	Usage *struct {
		InputTokens       int `json:"input_tokens"`
		OutputTokens      int `json:"output_tokens"`
		CacheReadTokens   int `json:"cache_read_input_tokens,omitempty"`
		CacheCreateTokens int `json:"cache_creation_input_tokens,omitempty"`
	} `json:"usage"`
}

type compatGeminiRequest struct {
	Model             string                        `json:"model,omitempty"`
	Contents          []compatGeminiContent         `json:"contents"`
	SystemInstruction *compatGeminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *compatGeminiGenerationConfig `json:"generationConfig,omitempty"`
}

type compatGeminiContent struct {
	Role  string             `json:"role,omitempty"`
	Parts []compatGeminiPart `json:"parts"`
}

type compatGeminiPart struct {
	Text string `json:"text,omitempty"`
}

type compatGeminiGenerationConfig struct {
	MaxOutputTokens    int      `json:"maxOutputTokens,omitempty"`
	Temperature        *float64 `json:"temperature,omitempty"`
	TopP               *float64 `json:"topP,omitempty"`
	StopSequences      []string `json:"stopSequences,omitempty"`
	ResponseMIMEType   string   `json:"responseMimeType,omitempty"`
	ResponseJSONSchema any      `json:"responseJsonSchema,omitempty"`
}

type compatGeminiResponseEnvelope struct {
	Response *compatGeminiResponse `json:"response,omitempty"`
}

type compatGeminiResponse struct {
	ResponseID   string                     `json:"responseId,omitempty"`
	ModelVersion string                     `json:"modelVersion,omitempty"`
	Candidates   []compatGeminiCandidate    `json:"candidates,omitempty"`
	Usage        *compatGeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

type compatGeminiCandidate struct {
	Index        int                  `json:"index,omitempty"`
	FinishReason string               `json:"finishReason,omitempty"`
	Content      *compatGeminiContent `json:"content,omitempty"`
}

type compatGeminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}

type compatResponseFormatSpec struct {
	Type       string `json:"type"`
	JSONSchema *struct {
		Schema any `json:"schema"`
	} `json:"json_schema,omitempty"`
}

type compatOpenAIChatResponse struct {
	ID      string                     `json:"id"`
	Object  string                     `json:"object"`
	Created int64                      `json:"created"`
	Model   string                     `json:"model"`
	Choices []compatOpenAIChatChoice   `json:"choices"`
	Usage   *compatOpenAIChatUsageInfo `json:"usage,omitempty"`
}

type compatOpenAIChatChoice struct {
	Index        int                         `json:"index"`
	Message      compatOpenAIResponseMessage `json:"message"`
	FinishReason string                      `json:"finish_reason"`
}

type compatOpenAIResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type compatOpenAIChatUsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type compatOpenAIChatStreamChunk struct {
	ID      string                         `json:"id"`
	Object  string                         `json:"object"`
	Created int64                          `json:"created"`
	Model   string                         `json:"model"`
	Choices []compatOpenAIChatStreamChoice `json:"choices"`
}

type compatOpenAIChatStreamChoice struct {
	Index        int                   `json:"index"`
	Delta        compatOpenAIChatDelta `json:"delta"`
	FinishReason *string               `json:"finish_reason,omitempty"`
}

type compatOpenAIChatDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type compatOpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type compatTarget struct {
	provider        domain.Provider
	requestedModel  string
	relayPath       string
	upstreamBody    []byte
	stream          bool
	convertResponse func([]byte, string) (*compatOpenAIChatResponse, error)
	newStreamWriter func(http.ResponseWriter, string) compatStreamWriter
}

type compatStreamWriter interface {
	http.ResponseWriter
	finalize()
}

type compatResponseCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

type compatOpenAIStreamWriter struct {
	dst            http.ResponseWriter
	flusher        http.Flusher
	requestedModel string
	created        int64

	header         http.Header
	status         int
	headersWritten bool

	lineBuf       bytes.Buffer
	rawErrorBody  bytes.Buffer
	lastEventType string
	chunkID       string
	roleSent      bool
	doneSent      bool
}

type compatGeminiStreamWriter struct {
	dst            http.ResponseWriter
	flusher        http.Flusher
	requestedModel string
	created        int64

	header         http.Header
	status         int
	headersWritten bool

	lineBuf      bytes.Buffer
	rawErrorBody bytes.Buffer
	doneSent     bool
	roleSent     bool
	chunkID      string
}

func (c *compatResponseCapture) Header() http.Header {
	if c.header == nil {
		c.header = make(http.Header)
	}
	return c.header
}

func (c *compatResponseCapture) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	return c.body.Write(p)
}

func (c *compatResponseCapture) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}

func newCompatOpenAIStreamWriter(dst http.ResponseWriter, requestedModel string) *compatOpenAIStreamWriter {
	streamWriter := &compatOpenAIStreamWriter{
		dst:            dst,
		requestedModel: requestedModel,
		created:        time.Now().Unix(),
		header:         make(http.Header),
	}
	if flusher, ok := dst.(http.Flusher); ok {
		streamWriter.flusher = flusher
	}
	return streamWriter
}

func (w *compatOpenAIStreamWriter) Header() http.Header {
	return w.header
}

func (w *compatOpenAIStreamWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *compatOpenAIStreamWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.status != http.StatusOK {
		_, _ = w.rawErrorBody.Write(p)
		return len(p), nil
	}

	_, _ = w.lineBuf.Write(p)
	for {
		line, ok := w.nextLine()
		if !ok {
			break
		}
		w.handleClaudeStreamLine(line)
	}
	return len(p), nil
}

func (w *compatOpenAIStreamWriter) Flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}

func (w *compatOpenAIStreamWriter) nextLine() (string, bool) {
	data := w.lineBuf.Bytes()
	idx := bytes.IndexByte(data, '\n')
	if idx < 0 {
		return "", false
	}
	line := strings.TrimRight(string(data[:idx]), "\r")
	w.lineBuf.Next(idx + 1)
	return line, true
}

func (w *compatOpenAIStreamWriter) handleClaudeStreamLine(line string) {
	if line == "" {
		return
	}
	if strings.HasPrefix(line, "event: ") {
		w.lastEventType = strings.TrimPrefix(line, "event: ")
		return
	}
	if !strings.HasPrefix(line, "data: ") {
		return
	}

	payload := strings.TrimPrefix(line, "data: ")
	switch w.lastEventType {
	case "message_start":
		var event struct {
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(payload), &event) == nil && strings.TrimSpace(event.Message.ID) != "" {
			w.chunkID = event.Message.ID
		}
		if !w.roleSent {
			w.roleSent = true
			w.emitChunk(compatOpenAIChatDelta{Role: "assistant"}, nil)
		}

	case "content_block_delta":
		var event struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if json.Unmarshal([]byte(payload), &event) != nil {
			return
		}
		if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
			w.emitChunk(compatOpenAIChatDelta{Content: event.Delta.Text}, nil)
		}

	case "message_delta":
		var event struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
		}
		if json.Unmarshal([]byte(payload), &event) != nil {
			return
		}
		if strings.TrimSpace(event.Delta.StopReason) == "" {
			return
		}
		finishReason := compatClaudeFinishReason(event.Delta.StopReason)
		w.emitChunk(compatOpenAIChatDelta{}, &finishReason)

	case "message_stop":
		w.emitDone()
	}
}

func (w *compatOpenAIStreamWriter) ensureSuccessHeaders() {
	if w.headersWritten {
		return
	}
	w.dst.Header().Set("Content-Type", "text/event-stream")
	w.dst.Header().Set("Cache-Control", "no-cache")
	w.dst.Header().Set("Connection", "keep-alive")
	w.dst.WriteHeader(http.StatusOK)
	w.headersWritten = true
}

func (w *compatOpenAIStreamWriter) streamChunkID() string {
	if strings.TrimSpace(w.chunkID) != "" {
		return w.chunkID
	}
	return "chatcmpl-compat"
}

func (w *compatOpenAIStreamWriter) emitChunk(delta compatOpenAIChatDelta, finishReason *string) {
	w.ensureSuccessHeaders()

	body, err := json.Marshal(compatOpenAIChatStreamChunk{
		ID:      w.streamChunkID(),
		Object:  "chat.completion.chunk",
		Created: w.created,
		Model:   w.requestedModel,
		Choices: []compatOpenAIChatStreamChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
	})
	if err != nil {
		return
	}

	_, _ = w.dst.Write([]byte("data: "))
	_, _ = w.dst.Write(body)
	_, _ = w.dst.Write([]byte("\n\n"))
	w.Flush()
}

func (w *compatOpenAIStreamWriter) emitDone() {
	if w.doneSent {
		return
	}
	w.ensureSuccessHeaders()
	_, _ = w.dst.Write([]byte("data: [DONE]\n\n"))
	w.Flush()
	w.doneSent = true
}

func (w *compatOpenAIStreamWriter) finalize() {
	if w.status != 0 && w.status != http.StatusOK {
		writeCompatOpenAIUpstreamError(w.dst, w.status, compatExtractErrorBody(w.rawErrorBody.Bytes()))
		return
	}

	if w.lineBuf.Len() > 0 {
		line := strings.TrimRight(w.lineBuf.String(), "\r\n")
		w.lineBuf.Reset()
		if line != "" {
			w.handleClaudeStreamLine(line)
		}
	}
	w.emitDone()
}

func newCompatGeminiStreamWriter(dst http.ResponseWriter, requestedModel string) *compatGeminiStreamWriter {
	streamWriter := &compatGeminiStreamWriter{
		dst:            dst,
		requestedModel: requestedModel,
		created:        time.Now().Unix(),
		header:         make(http.Header),
	}
	if flusher, ok := dst.(http.Flusher); ok {
		streamWriter.flusher = flusher
	}
	return streamWriter
}

func (w *compatGeminiStreamWriter) Header() http.Header {
	return w.header
}

func (w *compatGeminiStreamWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *compatGeminiStreamWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.status != http.StatusOK {
		_, _ = w.rawErrorBody.Write(p)
		return len(p), nil
	}

	_, _ = w.lineBuf.Write(p)
	for {
		line, ok := w.nextLine()
		if !ok {
			break
		}
		w.handleGeminiStreamLine(line)
	}
	return len(p), nil
}

func (w *compatGeminiStreamWriter) Flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}

func (w *compatGeminiStreamWriter) nextLine() (string, bool) {
	data := w.lineBuf.Bytes()
	idx := bytes.IndexByte(data, '\n')
	if idx < 0 {
		return "", false
	}
	line := strings.TrimRight(string(data[:idx]), "\r")
	w.lineBuf.Next(idx + 1)
	return line, true
}

func (w *compatGeminiStreamWriter) handleGeminiStreamLine(line string) {
	if line == "" || !strings.HasPrefix(line, "data: ") {
		return
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
	if payload == "" || payload == "[DONE]" {
		w.emitDone()
		return
	}

	resp, err := compatParseGeminiResponse([]byte(payload))
	if err != nil || resp == nil {
		return
	}
	if strings.TrimSpace(resp.ResponseID) != "" {
		w.chunkID = resp.ResponseID
	}
	if len(resp.Candidates) == 0 {
		return
	}
	candidate := resp.Candidates[0]
	if candidate.Content != nil {
		if !w.roleSent {
			w.roleSent = true
			w.emitChunk(compatOpenAIChatDelta{Role: "assistant"}, nil)
		}

		var text strings.Builder
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				text.WriteString(part.Text)
			}
		}
		if text.Len() > 0 {
			w.emitChunk(compatOpenAIChatDelta{Content: text.String()}, nil)
		}
	}

	if strings.TrimSpace(candidate.FinishReason) != "" {
		finishReason := compatGeminiFinishReason(candidate.FinishReason)
		w.emitChunk(compatOpenAIChatDelta{}, &finishReason)
	}
}

func (w *compatGeminiStreamWriter) ensureSuccessHeaders() {
	if w.headersWritten {
		return
	}
	w.dst.Header().Set("Content-Type", "text/event-stream")
	w.dst.Header().Set("Cache-Control", "no-cache")
	w.dst.Header().Set("Connection", "keep-alive")
	w.dst.WriteHeader(http.StatusOK)
	w.headersWritten = true
}

func (w *compatGeminiStreamWriter) streamChunkID() string {
	if strings.TrimSpace(w.chunkID) != "" {
		return w.chunkID
	}
	return "chatcmpl-compat"
}

func (w *compatGeminiStreamWriter) emitChunk(delta compatOpenAIChatDelta, finishReason *string) {
	w.ensureSuccessHeaders()

	body, err := json.Marshal(compatOpenAIChatStreamChunk{
		ID:      w.streamChunkID(),
		Object:  "chat.completion.chunk",
		Created: w.created,
		Model:   w.requestedModel,
		Choices: []compatOpenAIChatStreamChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
	})
	if err != nil {
		return
	}

	_, _ = w.dst.Write([]byte("data: "))
	_, _ = w.dst.Write(body)
	_, _ = w.dst.Write([]byte("\n\n"))
	w.Flush()
}

func (w *compatGeminiStreamWriter) emitDone() {
	if w.doneSent {
		return
	}
	w.ensureSuccessHeaders()
	_, _ = w.dst.Write([]byte("data: [DONE]\n\n"))
	w.Flush()
	w.doneSent = true
}

func (w *compatGeminiStreamWriter) finalize() {
	if w.status != 0 && w.status != http.StatusOK {
		writeCompatOpenAIUpstreamError(w.dst, w.status, compatExtractErrorBody(w.rawErrorBody.Bytes()))
		return
	}

	if w.lineBuf.Len() > 0 {
		line := strings.TrimRight(w.lineBuf.String(), "\r\n")
		w.lineBuf.Reset()
		if line != "" {
			w.handleGeminiStreamLine(line)
		}
	}
	w.emitDone()
}

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

	var req compatOpenAIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

func compatOpenAIChatToClaudeRequest(req *compatOpenAIChatRequest) (*compatClaudeRequest, string, error) {
	if req == nil {
		return nil, "", errCompat("request is required")
	}
	if compatHasTools(req.Tools) || compatHasToolChoice(req.ToolChoice) {
		return nil, "", errCompat("tools are not supported on the claude compat surface yet")
	}

	_, model, requestedModel, err := resolveCompatModel(req.Model)
	if err != nil {
		return nil, "", err
	}
	if !compatProviderMatches(domain.ProviderClaude, requestedModel) {
		return nil, "", errCompat("model must be a claude model, e.g. claude/claude-sonnet-4-5")
	}
	if len(req.Messages) == 0 {
		return nil, "", errCompat("messages is required")
	}

	stopSequences, err := parseCompatStop(req.Stop)
	if err != nil {
		return nil, "", err
	}
	responseFormat, err := parseCompatResponseFormat(req.ResponseFormat)
	if err != nil {
		return nil, "", err
	}

	claudeReq := &compatClaudeRequest{
		Model:         model,
		MaxTokens:     compatMaxTokens(req),
		Stream:        req.Stream,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: stopSequences,
	}

	var systemParts []string
	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		content, err := compatExtractTextContent(message.Content)
		if err != nil {
			return nil, "", err
		}
		switch role {
		case "system", "developer":
			if content != "" {
				systemParts = append(systemParts, content)
			}
		case "user", "assistant":
			claudeReq.Messages = append(claudeReq.Messages, compatClaudeMessage{
				Role:    role,
				Content: content,
			})
		default:
			return nil, "", errCompat("unsupported message role: " + strings.TrimSpace(message.Role))
		}
	}

	if len(claudeReq.Messages) == 0 {
		return nil, "", errCompat("at least one user or assistant message is required")
	}
	if instruction := compatClaudeResponseFormatInstruction(responseFormat); instruction != "" {
		systemParts = append(systemParts, instruction)
	}
	claudeReq.System = strings.Join(systemParts, "\n\n")

	return claudeReq, requestedModel, nil
}

func compatClaudeToOpenAIChatResponse(body []byte, requestedModel string) (*compatOpenAIChatResponse, error) {
	var resp compatClaudeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	content := make([]string, 0, len(resp.Content))
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			content = append(content, block.Text)
		}
	}

	openAIResp := &compatOpenAIChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []compatOpenAIChatChoice{
			{
				Index: 0,
				Message: compatOpenAIResponseMessage{
					Role:    "assistant",
					Content: strings.Join(content, "\n\n"),
				},
				FinishReason: compatClaudeFinishReason(resp.StopReason),
			},
		},
	}

	if resp.Usage != nil {
		openAIResp.Usage = &compatOpenAIChatUsageInfo{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return openAIResp, nil
}

func compatOpenAIChatToGeminiRequest(req *compatOpenAIChatRequest) (*compatGeminiRequest, error) {
	if req == nil {
		return nil, errCompat("request is required")
	}
	if compatHasTools(req.Tools) || compatHasToolChoice(req.ToolChoice) {
		return nil, errCompat("tools are not supported on the gemini compat surface yet")
	}
	_, model, requestedModel, err := resolveCompatModel(req.Model)
	if err != nil {
		return nil, err
	}
	if !compatProviderMatches(domain.ProviderGemini, requestedModel) {
		return nil, errCompat("model must be a gemini model, e.g. gemini/gemini-2.5-flash")
	}
	if len(req.Messages) == 0 {
		return nil, errCompat("messages is required")
	}

	stopSequences, err := parseCompatStop(req.Stop)
	if err != nil {
		return nil, err
	}
	responseFormat, err := parseCompatResponseFormat(req.ResponseFormat)
	if err != nil {
		return nil, err
	}

	geminiReq := &compatGeminiRequest{
		Model: model,
		GenerationConfig: &compatGeminiGenerationConfig{
			MaxOutputTokens: compatMaxTokens(req),
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			StopSequences:   stopSequences,
		},
	}
	if geminiReq.GenerationConfig.MaxOutputTokens <= 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = compatClaudeDefaultMaxTokens
	}

	if err := applyCompatGeminiResponseFormat(geminiReq.GenerationConfig, responseFormat); err != nil {
		return nil, err
	}

	var systemParts []string
	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		content, err := compatExtractTextContent(message.Content)
		if err != nil {
			return nil, err
		}
		switch role {
		case "system", "developer":
			if content != "" {
				systemParts = append(systemParts, content)
			}
		case "user":
			geminiReq.Contents = append(geminiReq.Contents, compatGeminiContent{
				Role:  "user",
				Parts: []compatGeminiPart{{Text: content}},
			})
		case "assistant":
			geminiReq.Contents = append(geminiReq.Contents, compatGeminiContent{
				Role:  "model",
				Parts: []compatGeminiPart{{Text: content}},
			})
		default:
			return nil, errCompat("unsupported message role: " + strings.TrimSpace(message.Role))
		}
	}
	if len(geminiReq.Contents) == 0 {
		return nil, errCompat("at least one user or assistant message is required")
	}
	if len(systemParts) > 0 {
		geminiReq.SystemInstruction = &compatGeminiContent{
			Parts: []compatGeminiPart{{Text: strings.Join(systemParts, "\n\n")}},
		}
	}
	return geminiReq, nil
}

func compatGeminiToOpenAIChatResponse(body []byte, requestedModel string) (*compatOpenAIChatResponse, error) {
	resp, err := compatParseGeminiResponse(body)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errCompat("empty gemini response")
	}

	content := ""
	finishReason := "stop"
	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		finishReason = compatGeminiFinishReason(candidate.FinishReason)
		if candidate.Content != nil {
			var builder strings.Builder
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					builder.WriteString(part.Text)
				}
			}
			content = builder.String()
		}
	}

	openAIResp := &compatOpenAIChatResponse{
		ID:      compatGeminiResponseID(resp),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []compatOpenAIChatChoice{
			{
				Index: 0,
				Message: compatOpenAIResponseMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
	}
	if resp.Usage != nil {
		openAIResp.Usage = &compatOpenAIChatUsageInfo{
			PromptTokens:     resp.Usage.PromptTokenCount,
			CompletionTokens: resp.Usage.CandidatesTokenCount,
			TotalTokens:      resp.Usage.PromptTokenCount + resp.Usage.CandidatesTokenCount,
		}
	}
	return openAIResp, nil
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

func parseCompatResponseFormat(raw json.RawMessage) (*compatResponseFormatSpec, error) {
	if !hasCompatValue(raw) {
		return nil, nil
	}
	var spec compatResponseFormatSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, errCompat("response_format must be an object")
	}
	spec.Type = strings.ToLower(strings.TrimSpace(spec.Type))
	switch spec.Type {
	case "", "text":
		return &spec, nil
	case "json_object":
		return &spec, nil
	case "json_schema":
		if spec.JSONSchema == nil || spec.JSONSchema.Schema == nil {
			return nil, errCompat("response_format json_schema requires json_schema.schema")
		}
		return &spec, nil
	default:
		return nil, errCompat("unsupported response_format type: " + spec.Type)
	}
}

func compatClaudeResponseFormatInstruction(spec *compatResponseFormatSpec) string {
	if spec == nil {
		return ""
	}
	switch spec.Type {
	case "json_object":
		return "Return only a valid JSON object. Do not include markdown fences or extra commentary."
	case "json_schema":
		schema, err := json.Marshal(spec.JSONSchema.Schema)
		if err != nil {
			return "Return only valid JSON that matches the requested JSON Schema."
		}
		return "Return only valid JSON that matches this JSON Schema: " + string(schema)
	default:
		return ""
	}
}

func applyCompatGeminiResponseFormat(cfg *compatGeminiGenerationConfig, spec *compatResponseFormatSpec) error {
	if cfg == nil || spec == nil {
		return nil
	}
	switch spec.Type {
	case "", "text":
		return nil
	case "json_object":
		cfg.ResponseMIMEType = "application/json"
		return nil
	case "json_schema":
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseJSONSchema = spec.JSONSchema.Schema
		return nil
	default:
		return errCompat("unsupported response_format type: " + spec.Type)
	}
}

func compatParseGeminiResponse(body []byte) (*compatGeminiResponse, error) {
	var resp compatGeminiResponse
	if json.Unmarshal(body, &resp) == nil {
		if resp.ResponseID != "" || resp.ModelVersion != "" || len(resp.Candidates) > 0 || resp.Usage != nil {
			return &resp, nil
		}
	}

	var envelope compatGeminiResponseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Response == nil {
		return nil, errCompat("invalid gemini response")
	}
	if envelope.Response.ResponseID == "" && envelope.Response.ModelVersion == "" && len(envelope.Response.Candidates) == 0 && envelope.Response.Usage == nil {
		return nil, errCompat("invalid gemini response")
	}
	return envelope.Response, nil
}

func compatGeminiResponseID(resp *compatGeminiResponse) string {
	if resp == nil || strings.TrimSpace(resp.ResponseID) == "" {
		return "chatcmpl-compat"
	}
	return resp.ResponseID
}

func compatGeminiFinishReason(reason string) string {
	switch strings.ToUpper(strings.TrimSpace(reason)) {
	case "MAX_TOKENS":
		return "length"
	case "UNEXPECTED_TOOL_CALL":
		return "tool_calls"
	case "SAFETY", "RECITATION", "LANGUAGE", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "IMAGE_SAFETY":
		return "content_filter"
	default:
		return "stop"
	}
}

func compatMaxTokens(req *compatOpenAIChatRequest) int {
	if req == nil {
		return compatClaudeDefaultMaxTokens
	}
	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		return *req.MaxCompletionTokens
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		return *req.MaxTokens
	}
	return compatClaudeDefaultMaxTokens
}

func parseCompatStop(raw json.RawMessage) ([]string, error) {
	if !hasCompatValue(raw) {
		return nil, nil
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if strings.TrimSpace(single) == "" {
			return nil, nil
		}
		return []string{single}, nil
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		out := make([]string, 0, len(many))
		for _, item := range many {
			if strings.TrimSpace(item) != "" {
				out = append(out, item)
			}
		}
		return out, nil
	}

	return nil, errCompat("stop must be a string or string array")
}

func compatExtractTextContent(raw json.RawMessage) (string, error) {
	if !hasCompatValue(raw) {
		return "", errCompat("message content is required")
	}

	var content string
	if err := json.Unmarshal(raw, &content); err == nil {
		return content, nil
	}

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		texts := make([]string, 0, len(parts))
		for _, part := range parts {
			switch part.Type {
			case "text", "input_text":
				texts = append(texts, part.Text)
			default:
				return "", errCompat("only text content parts are supported on the compat surface")
			}
		}
		return strings.Join(texts, "\n\n"), nil
	}

	return "", errCompat("message content must be a string or text-only content array")
}

func compatClaudeFinishReason(stopReason string) string {
	switch stopReason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

func writeCompatOpenAIUpstreamError(w http.ResponseWriter, status int, body []byte) {
	message := "unexpected upstream error"
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && strings.TrimSpace(parsed.Error.Message) != "" {
		message = parsed.Error.Message
	}
	var geminiParsed []struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &geminiParsed) == nil {
		for _, item := range geminiParsed {
			if item.Error != nil && strings.TrimSpace(item.Error.Message) != "" {
				message = item.Error.Message
				break
			}
		}
	}
	writeCompatOpenAIError(w, status, "server_error", message)
}

func compatExtractErrorBody(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return trimmed
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return trimmed
	}

	lines := strings.Split(string(trimmed), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if strings.HasPrefix(payload, "{") || strings.HasPrefix(payload, "[") {
				return []byte(payload)
			}
		}
	}
	return trimmed
}

func writeCompatOpenAIError(w http.ResponseWriter, status int, errType, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errType,
		},
	})
}

func hasCompatValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func compatHasTools(raw json.RawMessage) bool {
	if !hasCompatValue(raw) {
		return false
	}

	var tools []json.RawMessage
	if err := json.Unmarshal(raw, &tools); err == nil {
		return len(tools) > 0
	}

	return true
}

func compatHasToolChoice(raw json.RawMessage) bool {
	if !hasCompatValue(raw) {
		return false
	}

	var choice string
	if err := json.Unmarshal(raw, &choice); err == nil {
		trimmed := strings.ToLower(strings.TrimSpace(choice))
		return trimmed != "" && trimmed != "none"
	}

	return true
}

func compatRequestBodyLimitBytes(cfg *config.Config) int64 {
	if cfg != nil && cfg.MaxRequestBodyMB > 0 {
		return int64(cfg.MaxRequestBodyMB) << 20
	}
	return int64(compatDefaultRequestBodyMB) << 20
}

type compatError string

func (e compatError) Error() string { return string(e) }

func errCompat(message string) error { return compatError(message) }

var _ http.ResponseWriter = (*compatResponseCapture)(nil)
