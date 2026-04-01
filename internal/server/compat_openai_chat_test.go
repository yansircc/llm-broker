package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/identity"
	"github.com/yansircc/llm-broker/internal/pool"
	"github.com/yansircc/llm-broker/internal/relay"
	"github.com/yansircc/llm-broker/internal/store"
)

func TestCompatOpenAIChatToClaudeRequest(t *testing.T) {
	maxTokens := 256
	maxCompletionTokens := 512
	temperature := 0.2
	topP := 0.9

	req := &compatOpenAIChatRequest{
		Model:               " claude/claude-sonnet-4-5 ",
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temperature,
		TopP:                &topP,
		Stop:                json.RawMessage(`["STOP",""]`),
		Tools:               json.RawMessage(`[]`),
		ToolChoice:          json.RawMessage(`"none"`),
		Messages: []compatMessage{
			{Role: "system", Content: json.RawMessage(`"system prompt"`)},
			{Role: " developer ", Content: json.RawMessage(`[{"type":"text","text":"developer prompt"}]`)},
			{Role: "USER", Content: json.RawMessage(`"hello"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"text","text":"hi"},{"type":"input_text","text":"there"}]`)},
		},
	}

	got, requestedModel, err := compatOpenAIChatToClaudeRequest(req)
	if err != nil {
		t.Fatalf("compatOpenAIChatToClaudeRequest() error = %v", err)
	}

	if requestedModel != "claude/claude-sonnet-4-5" {
		t.Fatalf("requestedModel = %q", requestedModel)
	}
	if got.Model != "claude-sonnet-4-5" {
		t.Fatalf("model = %q", got.Model)
	}
	systemText, ok := got.System.(string)
	if !ok {
		t.Fatalf("system = %#v, want string", got.System)
	}
	if systemText != "system prompt\n\ndeveloper prompt" {
		t.Fatalf("system = %q", got.System)
	}
	if got.MaxTokens != maxCompletionTokens {
		t.Fatalf("max_tokens = %d, want %d", got.MaxTokens, maxCompletionTokens)
	}
	if got.Temperature != &temperature {
		t.Fatalf("temperature pointer was not preserved")
	}
	if got.TopP != &topP {
		t.Fatalf("top_p pointer was not preserved")
	}
	if got.Stream == nil {
		t.Fatal("stream = nil, want explicit false")
	}
	if *got.Stream {
		t.Fatal("stream = true, want false")
	}
	if len(got.StopSequences) != 1 || got.StopSequences[0] != "STOP" {
		t.Fatalf("stop_sequences = %#v", got.StopSequences)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(got.Messages))
	}
	if got.Messages[0] != (compatClaudeMessage{Role: "user", Content: "hello"}) {
		t.Fatalf("messages[0] = %#v", got.Messages[0])
	}
	if got.Messages[1] != (compatClaudeMessage{Role: "assistant", Content: "hi\n\nthere"}) {
		t.Fatalf("messages[1] = %#v", got.Messages[1])
	}
}

func TestCompatOpenAIChatToClaudeRequest_Stream(t *testing.T) {
	req := &compatOpenAIChatRequest{
		Model:    "claude/claude-sonnet-4-5",
		Stream:   true,
		Messages: []compatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
	}

	got, requestedModel, err := compatOpenAIChatToClaudeRequest(req)
	if err != nil {
		t.Fatalf("compatOpenAIChatToClaudeRequest() error = %v", err)
	}
	if requestedModel != "claude/claude-sonnet-4-5" {
		t.Fatalf("requestedModel = %q", requestedModel)
	}
	if got.Stream == nil || !*got.Stream {
		t.Fatal("stream = false, want true")
	}
}

func TestCompatOpenAIChatToClaudeRequest_ModernEnvelope(t *testing.T) {
	temperature := 0.2
	req := &compatOpenAIChatRequest{
		Model:          "claude/claude-sonnet-4-6",
		Temperature:    &temperature,
		ResponseFormat: json.RawMessage(`{"type":"json_object"}`),
		Messages: []compatMessage{
			{Role: "system", Content: json.RawMessage(`"system prompt"`)},
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
	}

	got, requestedModel, err := compatOpenAIChatToClaudeRequest(req)
	if err != nil {
		t.Fatalf("compatOpenAIChatToClaudeRequest() error = %v", err)
	}
	if requestedModel != "claude/claude-sonnet-4-6" {
		t.Fatalf("requestedModel = %q", requestedModel)
	}
	if got.Stream == nil || *got.Stream {
		t.Fatalf("stream = %#v, want explicit false", got.Stream)
	}
	if got.OutputConfig == nil || got.OutputConfig.Effort != "medium" {
		t.Fatalf("output_config = %#v", got.OutputConfig)
	}
	if got.Thinking == nil || got.Thinking.Type != "adaptive" {
		t.Fatalf("thinking = %#v", got.Thinking)
	}
	if got.Temperature != nil {
		t.Fatalf("temperature = %#v, want nil when thinking is enabled", got.Temperature)
	}
	systemBlocks, ok := got.System.([]compatClaudeSystemBlock)
	if !ok {
		t.Fatalf("system = %#v, want Claude system blocks", got.System)
	}
	if len(systemBlocks) != 2 {
		t.Fatalf("len(systemBlocks) = %d, want 2", len(systemBlocks))
	}
	if systemBlocks[0].Text != "You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK." {
		t.Fatalf("systemBlocks[0] = %#v", systemBlocks[0])
	}
	if systemBlocks[0].CacheControl == nil || systemBlocks[0].CacheControl.Type != "ephemeral" {
		t.Fatalf("systemBlocks[0].cache_control = %#v", systemBlocks[0].CacheControl)
	}
	if systemBlocks[1].Text != "system prompt\n\nReturn only a valid JSON object. Do not include markdown fences or extra commentary." {
		t.Fatalf("systemBlocks[1] = %#v", systemBlocks[1])
	}
	if systemBlocks[1].CacheControl == nil || systemBlocks[1].CacheControl.Type != "ephemeral" {
		t.Fatalf("systemBlocks[1].cache_control = %#v", systemBlocks[1].CacheControl)
	}
}

func TestCompatOpenAIChatToClaudeRequest_ModernEnvelopeAlias(t *testing.T) {
	req := &compatOpenAIChatRequest{
		Model: "claude/claude-sonnet-4-20250514",
		Messages: []compatMessage{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
	}

	got, requestedModel, err := compatOpenAIChatToClaudeRequest(req)
	if err != nil {
		t.Fatalf("compatOpenAIChatToClaudeRequest() error = %v", err)
	}
	if requestedModel != "claude/claude-sonnet-4-6" {
		t.Fatalf("requestedModel = %q", requestedModel)
	}
	if got.Model != "claude-sonnet-4-6" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.OutputConfig == nil || got.OutputConfig.Effort != "medium" {
		t.Fatalf("output_config = %#v", got.OutputConfig)
	}
	if got.Thinking == nil || got.Thinking.Type != "adaptive" {
		t.Fatalf("thinking = %#v", got.Thinking)
	}
}

func TestResolveCompatModelAliases(t *testing.T) {
	tests := []struct {
		model         string
		wantProvider  domain.Provider
		wantModel     string
		wantRequested string
	}{
		{"claude/claude-sonnet-4-5", domain.ProviderClaude, "claude-sonnet-4-5", "claude/claude-sonnet-4-5"},
		{"anthropic/claude-sonnet-4-5", domain.ProviderClaude, "claude-sonnet-4-5", "claude/claude-sonnet-4-5"},
		{"claude-sonnet-4-5", domain.ProviderClaude, "claude-sonnet-4-5", "claude/claude-sonnet-4-5"},
		{"anthropic/claude-sonnet-4.5", domain.ProviderClaude, "claude-sonnet-4-5", "claude/claude-sonnet-4-5"},
		{"claude-sonnet-4-5-20250929", domain.ProviderClaude, "claude-sonnet-4-5", "claude/claude-sonnet-4-5"},
		{"anthropic/claude-opus-4.6", domain.ProviderClaude, "claude-opus-4-6", "claude/claude-opus-4-6"},
		{"claude-opus-4-1-20250805", domain.ProviderClaude, "claude-opus-4-1", "claude/claude-opus-4-1"},
		{"anthropic/claude-haiku-4.5", domain.ProviderClaude, "claude-haiku-4-5", "claude/claude-haiku-4-5"},
		{"claude-sonnet-4-20250514", domain.ProviderClaude, "claude-sonnet-4-6", "claude/claude-sonnet-4-6"},
	}

	for _, tt := range tests {
		provider, model, requested, err := resolveCompatModel(tt.model)
		if err != nil {
			t.Fatalf("resolveCompatModel(%q) error = %v", tt.model, err)
		}
		if provider != tt.wantProvider || model != tt.wantModel || requested != tt.wantRequested {
			t.Fatalf("resolveCompatModel(%q) = (%q, %q, %q), want (%q, %q, %q)", tt.model, provider, model, requested, tt.wantProvider, tt.wantModel, tt.wantRequested)
		}
	}
}

func TestCompatOpenAIChatToClaudeRequestErrors(t *testing.T) {
	tests := []struct {
		name    string
		req     *compatOpenAIChatRequest
		wantErr string
	}{
		{
			name: "tools",
			req: &compatOpenAIChatRequest{
				Model:    "claude/claude-sonnet-4-5",
				Tools:    json.RawMessage(`[{"type":"function"}]`),
				Messages: []compatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
			},
			wantErr: "tools are not supported",
		},
		{
			name: "tool choice",
			req: &compatOpenAIChatRequest{
				Model:      "claude/claude-sonnet-4-5",
				ToolChoice: json.RawMessage(`"auto"`),
				Messages:   []compatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
			},
			wantErr: "tools are not supported",
		},
		{
			name: "invalid model",
			req: &compatOpenAIChatRequest{
				Model:    "gpt-4o",
				Messages: []compatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
			},
			wantErr: "model must be a claude model",
		},
		{
			name:    "missing messages",
			req:     &compatOpenAIChatRequest{Model: "claude/claude-sonnet-4-5"},
			wantErr: "messages is required",
		},
		{
			name: "unsupported role",
			req: &compatOpenAIChatRequest{
				Model: "claude/claude-sonnet-4-5",
				Messages: []compatMessage{
					{Role: "tool", Content: json.RawMessage(`"hello"`)},
				},
			},
			wantErr: "unsupported message role",
		},
		{
			name: "unsupported content part",
			req: &compatOpenAIChatRequest{
				Model: "claude/claude-sonnet-4-5",
				Messages: []compatMessage{
					{Role: "user", Content: json.RawMessage(`[{"type":"image_url","image_url":{"url":"https://example.com"}}]`)},
				},
			},
			wantErr: "only text content parts are supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := compatOpenAIChatToClaudeRequest(tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCompatClaudeToOpenAIChatResponse(t *testing.T) {
	body := []byte(`{
		"id": "msg_123",
		"model": "claude-sonnet-4-5",
		"stop_reason": "max_tokens",
		"content": [
			{"type":"text","text":"hello"},
			{"type":"tool_use"},
			{"type":"text","text":"world"}
		],
		"usage": {
			"input_tokens": 12,
			"output_tokens": 34
		}
	}`)

	got, err := compatClaudeToOpenAIChatResponse(body, "claude/claude-sonnet-4-5")
	if err != nil {
		t.Fatalf("compatClaudeToOpenAIChatResponse() error = %v", err)
	}

	if got.Object != "chat.completion" {
		t.Fatalf("object = %q", got.Object)
	}
	if got.Model != "claude/claude-sonnet-4-5" {
		t.Fatalf("model = %q", got.Model)
	}
	if len(got.Choices) != 1 {
		t.Fatalf("len(choices) = %d, want 1", len(got.Choices))
	}
	if got.Choices[0].Message.Role != "assistant" {
		t.Fatalf("role = %q", got.Choices[0].Message.Role)
	}
	if got.Choices[0].Message.Content != "hello\n\nworld" {
		t.Fatalf("content = %q", got.Choices[0].Message.Content)
	}
	if got.Choices[0].FinishReason != "length" {
		t.Fatalf("finish_reason = %q", got.Choices[0].FinishReason)
	}
	if got.Usage == nil || got.Usage.TotalTokens != 46 {
		t.Fatalf("usage = %#v", got.Usage)
	}
}

func TestHandleCompatListModels(t *testing.T) {
	srv := newTestServer(t)
	srv.catalogDrivers = map[domain.Provider]driver.Descriptor{
		domain.ProviderClaude: driver.NewClaudeDriver(driver.ClaudeConfig{}, nil),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/compat/v1/models", nil)
	srv.handleCompatListModels(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var body struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Object != "list" {
		t.Fatalf("object = %q", body.Object)
	}
	if len(body.Data) == 0 {
		t.Fatal("expected at least one compat model")
	}
	ids := make(map[string]bool, len(body.Data))
	for _, item := range body.Data {
		ids[item.ID] = true
	}
	if !ids["claude/claude-sonnet-4-5"] {
		t.Fatalf("compat models missing claude model: %#v", ids)
	}
}

func TestHandleCompatOpenAIChatCompletions_MinimalLoop(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://claude.example/v1/messages?beta=true" {
				t.Fatalf("upstream URL = %q", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
			}
			if req.Header.Get("anthropic-version") != "2023-06-01" {
				t.Fatalf("anthropic-version = %q", req.Header.Get("anthropic-version"))
			}
			if req.Header.Get("Accept") != "text/event-stream" {
				t.Fatalf("accept = %q", req.Header.Get("Accept"))
			}

			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll(req.Body) error = %v", err)
			}

			var body map[string]any
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Fatalf("json.Unmarshal(upstream body) error = %v", err)
			}
			if body["model"] != "claude-sonnet-4-5" {
				t.Fatalf("model = %#v", body["model"])
			}
			system, ok := body["system"].([]any)
			if !ok || len(system) != 2 {
				t.Fatalf("system = %#v, want 2 Claude Code blocks", body["system"])
			}
			firstSystem, _ := system[0].(map[string]any)
			secondSystem, _ := system[1].(map[string]any)
			if firstSystem["text"] != "You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK." {
				t.Fatalf("first system text = %#v", firstSystem["text"])
			}
			if secondSystem["text"] != "system prompt" {
				t.Fatalf("second system text = %#v", secondSystem["text"])
			}
			stream, ok := body["stream"].(bool)
			if !ok || stream {
				t.Fatalf("stream = %#v, want explicit false", body["stream"])
			}

			messages, _ := body["messages"].([]any)
			if len(messages) != 1 {
				t.Fatalf("len(messages) = %d, want 1", len(messages))
			}
			msg, _ := messages[0].(map[string]any)
			if msg["role"] != "user" || msg["content"] != "hello" {
				t.Fatalf("message = %#v", msg)
			}

			respBody := `{
				"id":"msg_compat_1",
				"model":"claude-sonnet-4-5",
				"stop_reason":"end_turn",
				"content":[{"type":"text","text":"compat ok"}],
				"usage":{"input_tokens":11,"output_tokens":7}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatTestServer(t, upstreamClient)

	reqBody := `{
		"model":"claude/claude-sonnet-4-5",
		"messages":[
			{"role":"system","content":"system prompt"},
			{"role":"user","content":"hello"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/compat/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{ID: "user-1", Name: "test"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp compatOpenAIChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.Object != "chat.completion" {
		t.Fatalf("object = %q", resp.Object)
	}
	if resp.Model != "claude/claude-sonnet-4-5" {
		t.Fatalf("model = %q", resp.Model)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "compat ok" {
		t.Fatalf("choices = %#v", resp.Choices)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish_reason = %q", resp.Choices[0].FinishReason)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 18 {
		t.Fatalf("usage = %#v", resp.Usage)
	}
}

func TestHandleCompatOpenAIChatCompletions_LogsRawCompatClientRequest(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			respBody := `{
				"id":"msg_compat_log_1",
				"model":"claude-sonnet-4-5",
				"stop_reason":"end_turn",
				"content":[{"type":"text","text":"compat ok"}],
				"usage":{"input_tokens":11,"output_tokens":7}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatTestServer(t, upstreamClient)

	reqBody := `{
		"model":"claude/claude-sonnet-4-5",
		"reasoning_effort":"max",
		"messages":[
			{"role":"system","content":"system prompt"},
			{"role":"user","content":"hello"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/compat/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "openclaw-test")
	ctx := context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{ID: "user-1", Name: "test"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	entry := waitForCompatRequestLog(t, srv)
	if entry.Path != "/compat/v1/chat/completions" {
		t.Fatalf("entry.Path = %q, want compat path", entry.Path)
	}
	if !strings.Contains(entry.ClientBodyExcerpt, `"claude/claude-sonnet-4-5"`) {
		t.Fatalf("ClientBodyExcerpt = %q, want raw compat model", entry.ClientBodyExcerpt)
	}
	if !strings.Contains(entry.ClientBodyExcerpt, `"reasoning_effort":"max"`) {
		t.Fatalf("ClientBodyExcerpt = %q, want raw compat-only field", entry.ClientBodyExcerpt)
	}
	if !strings.Contains(entry.UpstreamRequestBodyExcerpt, `"claude-sonnet-4-5"`) {
		t.Fatalf("UpstreamRequestBodyExcerpt = %q, want translated model", entry.UpstreamRequestBodyExcerpt)
	}
	if strings.Contains(entry.UpstreamRequestBodyExcerpt, "reasoning_effort") {
		t.Fatalf("UpstreamRequestBodyExcerpt = %q, unexpected raw compat field", entry.UpstreamRequestBodyExcerpt)
	}

	var requestMeta map[string]any
	if err := json.Unmarshal(entry.RequestMeta, &requestMeta); err != nil {
		t.Fatalf("Unmarshal RequestMeta: %v", err)
	}
	compatClient, ok := requestMeta["compat_client"].(map[string]any)
	if !ok {
		t.Fatalf("compat_client = %#v, want object", requestMeta["compat_client"])
	}
	if compatClient["reasoning_effort"] != "max" {
		t.Fatalf("compat_client.reasoning_effort = %#v, want max", compatClient["reasoning_effort"])
	}
	path, _ := requestMeta["body_artifact_path"].(string)
	if path == "" {
		t.Fatalf("body_artifact_path = %#v, want non-empty path", requestMeta["body_artifact_path"])
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if !strings.Contains(string(raw), `"reasoning_effort":"max"`) {
		t.Fatalf("raw artifact = %q, want raw compat request", string(raw))
	}
}

func TestHandleCompatOpenAIChatCompletions_Claude46Envelope(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://claude.example/v1/messages?beta=true" {
				t.Fatalf("upstream URL = %q", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
			}
			if req.Header.Get("anthropic-version") != "2023-06-01" {
				t.Fatalf("anthropic-version = %q", req.Header.Get("anthropic-version"))
			}
			if req.Header.Get("Accept") != "text/event-stream" {
				t.Fatalf("accept = %q", req.Header.Get("Accept"))
			}
			if req.Header.Get("Anthropic-Dangerous-Direct-Browser-Access") != "true" {
				t.Fatalf("anthropic-dangerous-direct-browser-access = %q", req.Header.Get("Anthropic-Dangerous-Direct-Browser-Access"))
			}
			if req.Header.Get("X-App") != "cli" {
				t.Fatalf("x-app = %q", req.Header.Get("X-App"))
			}
			beta := req.Header.Get("Anthropic-Beta")
			for _, want := range []string{
				"effort-2025-11-24",
				"prompt-caching-scope-2026-01-05",
				"context-management-2025-06-27",
				"redact-thinking-2026-02-12",
			} {
				if !strings.Contains(beta, want) {
					t.Fatalf("anthropic-beta = %q, want to contain %q", beta, want)
				}
			}

			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll(req.Body) error = %v", err)
			}

			var body map[string]any
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Fatalf("json.Unmarshal(upstream body) error = %v", err)
			}
			if body["model"] != "claude-sonnet-4-6" {
				t.Fatalf("model = %#v", body["model"])
			}
			system, ok := body["system"].([]any)
			if !ok || len(system) != 2 {
				t.Fatalf("system = %#v, want 2 Claude Code blocks", body["system"])
			}
			firstSystem, _ := system[0].(map[string]any)
			if firstSystem["text"] != "You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK." {
				t.Fatalf("system[0] = %#v", firstSystem)
			}
			secondSystem, _ := system[1].(map[string]any)
			if !strings.Contains(secondSystem["text"].(string), "system prompt") {
				t.Fatalf("system[1] = %#v", secondSystem)
			}
			outputConfig, _ := body["output_config"].(map[string]any)
			if outputConfig["effort"] != "medium" {
				t.Fatalf("output_config = %#v", outputConfig)
			}
			thinking, _ := body["thinking"].(map[string]any)
			if thinking["type"] != "adaptive" {
				t.Fatalf("thinking = %#v", thinking)
			}
			stream, ok := body["stream"].(bool)
			if !ok || stream {
				t.Fatalf("stream = %#v, want explicit false", body["stream"])
			}
			if _, ok := body["temperature"]; ok {
				t.Fatalf("temperature = %#v, want omitted when thinking is enabled", body["temperature"])
			}

			respBody := `{
				"id":"msg_compat_46",
				"model":"claude-sonnet-4-6",
				"stop_reason":"end_turn",
				"content":[
					{"type":"thinking","thinking":"hidden"},
					{"type":"text","text":"compat ok"}
				],
				"usage":{"input_tokens":11,"output_tokens":7}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatTestServer(t, upstreamClient)

	reqBody := `{
		"model":"claude/claude-sonnet-4-6",
		"temperature":0.2,
		"messages":[
			{"role":"system","content":"system prompt"},
			{"role":"user","content":"hello"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/compat/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{ID: "user-1", Name: "test"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp compatOpenAIChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "compat ok" {
		t.Fatalf("choices = %#v", resp.Choices)
	}
}

func TestHandleCompatOpenAIChatCompletions_StreamLoop(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://claude.example/v1/messages?beta=true" {
				t.Fatalf("upstream URL = %q", req.URL.String())
			}
			if req.Header.Get("Accept") != "text/event-stream" {
				t.Fatalf("accept = %q", req.Header.Get("Accept"))
			}

			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll(req.Body) error = %v", err)
			}

			var body map[string]any
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Fatalf("json.Unmarshal(upstream body) error = %v", err)
			}
			if body["stream"] != true {
				t.Fatalf("stream = %#v, want true", body["stream"])
			}

			respBody := strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"id":"msg_stream_1","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":11,"output_tokens":1}}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
				``,
				`event: message_delta`,
				`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":2}}`,
				``,
				`event: message_stop`,
				`data: {"type":"message_stop"}`,
				``,
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatTestServer(t, upstreamClient)

	reqBody := `{
		"model":"claude/claude-sonnet-4-5",
		"stream": true,
		"messages":[
			{"role":"system","content":"system prompt"},
			{"role":"user","content":"hello"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/compat/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{ID: "user-1", Name: "test"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("content-type = %q", got)
	}

	lines := strings.Split(w.Body.String(), "\n")
	var payloads []string
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			payloads = append(payloads, strings.TrimPrefix(line, "data: "))
		}
	}
	if len(payloads) != 5 {
		t.Fatalf("payloads = %#v, want 5 SSE payloads", payloads)
	}
	if payloads[len(payloads)-1] != "[DONE]" {
		t.Fatalf("last payload = %q, want [DONE]", payloads[len(payloads)-1])
	}

	var chunks []compatOpenAIChatStreamChunk
	for _, payload := range payloads[:len(payloads)-1] {
		var chunk compatOpenAIChatStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			t.Fatalf("json.Unmarshal(chunk) error = %v; payload = %s", err, payload)
		}
		chunks = append(chunks, chunk)
	}
	if len(chunks) != 4 {
		t.Fatalf("len(chunks) = %d, want 4", len(chunks))
	}
	if chunks[0].Choices[0].Delta.Role != "assistant" {
		t.Fatalf("first delta role = %q", chunks[0].Choices[0].Delta.Role)
	}
	if chunks[1].Choices[0].Delta.Content != "hello" {
		t.Fatalf("second delta content = %q", chunks[1].Choices[0].Delta.Content)
	}
	if chunks[2].Choices[0].Delta.Content != " world" {
		t.Fatalf("third delta content = %q", chunks[2].Choices[0].Delta.Content)
	}
	if chunks[3].Choices[0].FinishReason == nil || *chunks[3].Choices[0].FinishReason != "stop" {
		t.Fatalf("finish_reason = %#v", chunks[3].Choices[0].FinishReason)
	}
}

func TestHandleCompatOpenAIChatCompletions_StreamLoopForwardsClaudePing(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			respBody := strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"id":"msg_stream_ping","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","stop_reason":null,"stop_sequence":null}}`,
				``,
				`event: ping`,
				`data: {"type":"ping"}`,
				``,
				`event: ping`,
				`data: {"type":"ping"}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello after ping"}}`,
				``,
				`event: message_delta`,
				`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
				``,
				`event: message_stop`,
				`data: {"type":"message_stop"}`,
				``,
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatTestServer(t, upstreamClient)

	reqBody := `{
		"model":"claude/claude-sonnet-4-5",
		"stream": true,
		"messages":[{"role":"user","content":"hello"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/compat/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{ID: "user-1", Name: "test"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	lines := strings.Split(w.Body.String(), "\n")
	var comments []string
	var payloads []string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, ": "):
			comments = append(comments, strings.TrimPrefix(line, ": "))
		case strings.HasPrefix(line, "data: "):
			payloads = append(payloads, strings.TrimPrefix(line, "data: "))
		}
	}
	if len(comments) != 2 {
		t.Fatalf("comments = %#v, want 2 ping heartbeats", comments)
	}
	for _, comment := range comments {
		if comment != "ping" {
			t.Fatalf("comment = %q, want ping", comment)
		}
	}
	if len(payloads) != 4 {
		t.Fatalf("payloads = %#v, want 4 SSE payloads", payloads)
	}
	if payloads[len(payloads)-1] != "[DONE]" {
		t.Fatalf("last payload = %q, want [DONE]", payloads[len(payloads)-1])
	}

	var contentChunk compatOpenAIChatStreamChunk
	if err := json.Unmarshal([]byte(payloads[1]), &contentChunk); err != nil {
		t.Fatalf("json.Unmarshal(content chunk) error = %v; payload = %s", err, payloads[1])
	}
	if contentChunk.Choices[0].Delta.Content != "hello after ping" {
		t.Fatalf("content = %q, want hello after ping", contentChunk.Choices[0].Delta.Content)
	}

}

func TestHandleCompatOpenAIChatCompletions_RateLimited(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			respBody := `{
				"id":"msg_compat_1",
				"model":"claude-sonnet-4-5",
				"stop_reason":"end_turn",
				"content":[{"type":"text","text":"compat ok"}]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatTestServer(t, upstreamClient)
	srv.compatLimiter = newCompatRateLimiter(1, 1)

	reqBody := `{
		"model":"claude/claude-sonnet-4-5",
		"messages":[{"role":"user","content":"hello"}]
	}`
	makeReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/compat/v1/chat/completions", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{ID: "user-1", Name: "test"})
		return req.WithContext(ctx)
	}

	first := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(first, makeReq())
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(second, makeReq())
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d, body = %s", second.Code, http.StatusTooManyRequests, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "rate limit") {
		t.Fatalf("second body = %s", second.Body.String())
	}

	logs := waitForCompatRequestLogsCount(t, srv, 2)
	entry := findCompatRequestLogByStatus(t, logs, "compat_429")
	if entry.Status != "compat_429" {
		t.Fatalf("entry.Status = %q, want compat_429", entry.Status)
	}
	if entry.EffectKind != "overload" {
		t.Fatalf("entry.EffectKind = %q, want overload", entry.EffectKind)
	}
	var meta map[string]any
	if err := json.Unmarshal(entry.RequestMeta, &meta); err != nil {
		t.Fatalf("Unmarshal RequestMeta: %v", err)
	}
	if meta["phase"] != "compat_preflight" {
		t.Fatalf("phase = %#v, want compat_preflight", meta["phase"])
	}
}

func TestHandleCompatOpenAIChatCompletions_LogsConvertFailure(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			respBody := `{"id":"msg_bad","model":"claude-sonnet-4-5","content":"wrong-shape"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatTestServer(t, upstreamClient)

	reqBody := `{
		"model":"claude/claude-sonnet-4-5",
		"messages":[{"role":"user","content":"hello"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/compat/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{ID: "user-1", Name: "test"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusBadGateway, w.Body.String())
	}

	logs := waitForCompatRequestLogsCount(t, srv, 2)
	entry := findCompatRequestLogByStatus(t, logs, "compat_502")
	if entry.Status != "compat_502" {
		t.Fatalf("entry.Status = %q, want compat_502", entry.Status)
	}
	if entry.EffectKind != "server_error" {
		t.Fatalf("entry.EffectKind = %q, want server_error", entry.EffectKind)
	}
	var meta map[string]any
	if err := json.Unmarshal(entry.RequestMeta, &meta); err != nil {
		t.Fatalf("Unmarshal RequestMeta: %v", err)
	}
	if meta["phase"] != "compat_final" {
		t.Fatalf("phase = %#v, want compat_final", meta["phase"])
	}
}

func TestHandleCompatOpenAIChatCompletions_StreamIncompleteWritesLifecycleFailure(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			respBody := strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"id":"msg_stream_incomplete","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5"}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
				``,
				`event: message_delta`,
				`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
				``,
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatTestServer(t, upstreamClient)

	reqBody := `{
		"model":"claude/claude-sonnet-4-5",
		"stream": true,
		"messages":[{"role":"user","content":"hello"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/compat/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{ID: "user-1", Name: "test"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	logs := waitForCompatRequestLogsCount(t, srv, 2)
	last := findCompatRequestLogByStatus(t, logs, "compat_stream_incomplete")
	if last.Status != "compat_stream_incomplete" {
		t.Fatalf("entry.Status = %q, want compat_stream_incomplete", last.Status)
	}
	if last.EffectKind != "stream_incomplete" {
		t.Fatalf("entry.EffectKind = %q, want stream_incomplete", last.EffectKind)
	}
	var meta map[string]any
	if err := json.Unmarshal(last.RequestMeta, &meta); err != nil {
		t.Fatalf("Unmarshal RequestMeta: %v", err)
	}
	if meta["phase"] != "compat_final" {
		t.Fatalf("phase = %#v, want compat_final", meta["phase"])
	}
	streamWriter, ok := meta["stream_writer"].(map[string]any)
	if !ok {
		t.Fatalf("stream_writer = %#v, want object", meta["stream_writer"])
	}
	if streamWriter["delivery_completed"] != false {
		t.Fatalf("delivery_completed = %#v, want false", streamWriter["delivery_completed"])
	}
	if streamWriter["synthetic_done"] != true {
		t.Fatalf("synthetic_done = %#v, want true", streamWriter["synthetic_done"])
	}
}

func TestHandleCompatOpenAIChatCompletions_TraceLogsRawClientBody(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			respBody := `{
				"id":"msg_compat_trace_1",
				"model":"claude-sonnet-4-5",
				"stop_reason":"end_turn",
				"content":[{"type":"text","text":"compat ok"}]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatTestServer(t, upstreamClient)
	srv.cfg.TraceCompat = true

	logs := &serverCaptureHandler{}
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(logs))
	defer slog.SetDefault(oldLogger)

	reqBody := `{
		"model":"claude/claude-sonnet-4-5",
		"messages":[{"role":"user","content":"hello"}],
		"reasoning_effort":"max"
	}`
	req := httptest.NewRequest(http.MethodPost, "/compat/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{ID: "user-1", Name: "test"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	record := logs.find("compat translation")
	if record == nil {
		t.Fatal("missing compat translation log record")
	}
	clientBody, _ := record.attrs["clientBody"].(string)
	if !strings.Contains(clientBody, `"reasoning_effort":"max"`) {
		t.Fatalf("clientBody = %q, want raw compat envelope field", clientBody)
	}
	translatedBody, _ := record.attrs["translatedBody"].(string)
	if strings.Contains(translatedBody, "reasoning_effort") {
		t.Fatalf("translatedBody = %q, unexpected passthrough field", translatedBody)
	}
}

func newCompatTestServer(t *testing.T, upstreamClient *http.Client) *Server {
	return newCompatMultiProviderTestServer(t, map[domain.Provider]*http.Client{
		domain.ProviderClaude: upstreamClient,
	})
}

func waitForCompatRequestLog(t *testing.T, srv *Server) *domain.RequestLog {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		logs, total, err := srv.store.QueryRequestLogs(context.Background(), domain.RequestLogQuery{Limit: 10})
		if err != nil {
			t.Fatalf("QueryRequestLogs() error = %v", err)
		}
		if total > 0 && len(logs) > 0 {
			return logs[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("request log was not persisted in time")
	return nil
}

func waitForCompatRequestLogsCount(t *testing.T, srv *Server, want int) []*domain.RequestLog {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		logs, total, err := srv.store.QueryRequestLogs(context.Background(), domain.RequestLogQuery{Limit: want + 4})
		if err != nil {
			t.Fatalf("QueryRequestLogs() error = %v", err)
		}
		if total >= want && len(logs) >= want {
			return logs
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("request logs did not reach count %d in time", want)
	return nil
}

func findCompatRequestLogByStatus(t *testing.T, logs []*domain.RequestLog, want string) *domain.RequestLog {
	t.Helper()

	for _, entry := range logs {
		if entry.Status == want {
			return entry
		}
	}
	t.Fatalf("request logs do not contain status %q", want)
	return nil
}

func newCompatMultiProviderTestServer(t *testing.T, upstreamClients map[domain.Provider]*http.Client) *Server {
	t.Helper()

	ms := store.NewMockStore()
	bus := events.NewBus(16)
	for _, acct := range []*domain.Account{
		{
			ID:       "acct-claude-1",
			Email:    "claude@example.com",
			Provider: domain.ProviderClaude,
			Status:   domain.StatusActive,
			Priority: 1,
			CellID:   "cell-compat-claude-1",
		},
	} {
		if err := ms.SaveAccount(context.Background(), acct); err != nil {
			t.Fatalf("SaveAccount() error = %v", err)
		}
	}
	for _, cell := range []*domain.EgressCell{
		{
			ID:        "cell-compat-claude-1",
			Name:      "Compat Claude 01",
			Status:    domain.EgressCellActive,
			Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 11081},
			Labels:    map[string]string{"lane": "compat"},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	} {
		if err := ms.SaveEgressCell(context.Background(), cell); err != nil {
			t.Fatalf("SaveEgressCell() error = %v", err)
		}
	}

	p, err := pool.New(ms, bus)
	if err != nil {
		t.Fatalf("pool.New() error = %v", err)
	}

	claudeDrv := driver.NewClaudeDriver(driver.ClaudeConfig{
		APIURL:     "https://claude.example/v1/messages",
		APIVersion: "2023-06-01",
	}, identity.NewTransformer(noopStainlessBinder{}, 8))

	return &Server{
		cfg: &config.Config{
			MaxRequestBodyMB: 60,
		},
		store: ms,
		pool:  p,
		bus:   bus,
		relay: relay.New(
			p,
			staticTokenProvider{},
			ms,
			relay.Config{
				MaxRequestBodyMB:  60,
				MaxRetryAccounts:  0,
				RequestLogBlobDir: t.TempDir(),
			},
			staticTransportProvider{clients: upstreamClients},
			bus,
			map[domain.Provider]driver.ExecutionDriver{
				domain.ProviderClaude: claudeDrv,
			},
		),
		catalogDrivers: map[domain.Provider]driver.Descriptor{
			domain.ProviderClaude: claudeDrv,
		},
	}
}

type noopStainlessBinder struct{}

func (noopStainlessBinder) BindStainlessFromRequest(context.Context, string, http.Header, http.Header) error {
	return nil
}

type staticTokenProvider struct{}

func (staticTokenProvider) EnsureValidToken(context.Context, string) (string, error) {
	return "test-token", nil
}

type staticTransportProvider struct {
	client  *http.Client
	clients map[domain.Provider]*http.Client
}

func (p staticTransportProvider) ClientForAccount(acct *domain.Account) *http.Client {
	if p.clients != nil {
		if acct != nil {
			if acctClient := p.clients[acct.Provider]; acctClient != nil {
				return acctClient
			}
		}
	}
	return p.client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type serverCapturedRecord struct {
	msg   string
	attrs map[string]any
}

type serverCaptureHandler struct {
	mu      sync.Mutex
	records []serverCapturedRecord
}

func (h *serverCaptureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *serverCaptureHandler) Handle(_ context.Context, record slog.Record) error {
	captured := serverCapturedRecord{
		msg:   record.Message,
		attrs: make(map[string]any),
	}
	record.Attrs(func(attr slog.Attr) bool {
		captured.attrs[attr.Key] = serverValueAny(attr.Value)
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, captured)
	return nil
}

func (h *serverCaptureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h *serverCaptureHandler) WithGroup(_ string) slog.Handler { return h }

func (h *serverCaptureHandler) find(msg string) *serverCapturedRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.records {
		if h.records[i].msg == msg {
			record := h.records[i]
			return &record
		}
	}
	return nil
}

func serverValueAny(v slog.Value) any {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindDuration:
		return v.Duration()
	case slog.KindTime:
		return v.Time()
	default:
		return v.Any()
	}
}
