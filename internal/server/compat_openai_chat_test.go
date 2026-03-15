package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if got.System != "system prompt\n\ndeveloper prompt" {
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
	if !got.Stream {
		t.Fatal("stream = false, want true")
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
		{"gemini/gemini-2.5-flash", domain.ProviderGemini, "gemini-2.5-flash", "gemini/gemini-2.5-flash"},
		{"google/gemini-2.5-pro", domain.ProviderGemini, "gemini-2.5-pro", "gemini/gemini-2.5-pro"},
		{"gemini-2.5-pro", domain.ProviderGemini, "gemini-2.5-pro", "gemini/gemini-2.5-pro"},
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
			wantErr: "model must be a claude or gemini model",
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

func TestCompatOpenAIChatToGeminiRequest(t *testing.T) {
	maxTokens := 512
	temperature := 0.3
	topP := 0.8
	req := &compatOpenAIChatRequest{
		Model:          "gemini/gemini-2.5-flash",
		MaxTokens:      &maxTokens,
		Temperature:    &temperature,
		TopP:           &topP,
		Stop:           json.RawMessage(`"STOP"`),
		ResponseFormat: json.RawMessage(`{"type":"json_object"}`),
		Messages: []compatMessage{
			{Role: "system", Content: json.RawMessage(`"be concise"`)},
			{Role: "user", Content: json.RawMessage(`"hello"`)},
			{Role: "assistant", Content: json.RawMessage(`"hi"`)},
		},
	}

	got, err := compatOpenAIChatToGeminiRequest(req)
	if err != nil {
		t.Fatalf("compatOpenAIChatToGeminiRequest() error = %v", err)
	}
	if got.SystemInstruction == nil || len(got.SystemInstruction.Parts) != 1 || got.SystemInstruction.Parts[0].Text != "be concise" {
		t.Fatalf("systemInstruction = %#v", got.SystemInstruction)
	}
	if len(got.Contents) != 2 {
		t.Fatalf("len(contents) = %d, want 2", len(got.Contents))
	}
	if got.Contents[0].Role != "user" || got.Contents[0].Parts[0].Text != "hello" {
		t.Fatalf("contents[0] = %#v", got.Contents[0])
	}
	if got.Contents[1].Role != "model" || got.Contents[1].Parts[0].Text != "hi" {
		t.Fatalf("contents[1] = %#v", got.Contents[1])
	}
	if got.GenerationConfig == nil {
		t.Fatal("generationConfig = nil")
	}
	if got.GenerationConfig.MaxOutputTokens != maxTokens {
		t.Fatalf("maxOutputTokens = %d, want %d", got.GenerationConfig.MaxOutputTokens, maxTokens)
	}
	if got.GenerationConfig.ResponseMIMEType != "application/json" {
		t.Fatalf("responseMimeType = %q", got.GenerationConfig.ResponseMIMEType)
	}
	if len(got.GenerationConfig.StopSequences) != 1 || got.GenerationConfig.StopSequences[0] != "STOP" {
		t.Fatalf("stopSequences = %#v", got.GenerationConfig.StopSequences)
	}
}

func TestCompatGeminiToOpenAIChatResponse(t *testing.T) {
	body := []byte(`{
		"responseId": "gem_resp_1",
		"modelVersion": "gemini-2.5-flash",
		"candidates": [
			{
				"index": 0,
				"finishReason": "MAX_TOKENS",
				"content": {
					"role": "model",
					"parts": [
						{"text":"hello"},
						{"text":" world"}
					]
				}
			}
		],
		"usageMetadata": {
			"promptTokenCount": 12,
			"candidatesTokenCount": 34
		}
	}`)

	got, err := compatGeminiToOpenAIChatResponse(body, "gemini/gemini-2.5-flash")
	if err != nil {
		t.Fatalf("compatGeminiToOpenAIChatResponse() error = %v", err)
	}
	if got.Model != "gemini/gemini-2.5-flash" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.ID != "gem_resp_1" {
		t.Fatalf("id = %q", got.ID)
	}
	if len(got.Choices) != 1 || got.Choices[0].Message.Content != "hello world" {
		t.Fatalf("choices = %#v", got.Choices)
	}
	if got.Choices[0].FinishReason != "length" {
		t.Fatalf("finish_reason = %q", got.Choices[0].FinishReason)
	}
	if got.Usage == nil || got.Usage.TotalTokens != 46 {
		t.Fatalf("usage = %#v", got.Usage)
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
		domain.ProviderGemini: driver.NewGeminiDriver(driver.GeminiConfig{}),
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
	if !ids["gemini/gemini-2.5-flash"] {
		t.Fatalf("compat models missing gemini model: %#v", ids)
	}
}

func TestHandleCompatOpenAIChatCompletions_MinimalLoop(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://claude.example/v1/messages" {
				t.Fatalf("upstream URL = %q", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
			}
			if req.Header.Get("anthropic-version") != "2023-06-01" {
				t.Fatalf("anthropic-version = %q", req.Header.Get("anthropic-version"))
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
			if body["system"] != "system prompt" {
				t.Fatalf("system = %#v", body["system"])
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

func TestHandleCompatOpenAIChatCompletions_StreamLoop(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://claude.example/v1/messages" {
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

func TestHandleCompatOpenAIChatCompletions_GeminiLoop(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://gemini.example/v1internal:generateContent" {
				t.Fatalf("upstream URL = %q", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
			}

			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll(req.Body) error = %v", err)
			}
			var body map[string]any
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Fatalf("json.Unmarshal(upstream body) error = %v", err)
			}
			if body["project"] != "proj-123" {
				t.Fatalf("project = %#v", body["project"])
			}
			if _, ok := body["model"]; ok {
				t.Fatalf("upstream body unexpectedly kept model field: %#v", body)
			}
			if _, ok := body["systemInstruction"]; !ok {
				t.Fatalf("systemInstruction missing: %#v", body)
			}

			respBody := `{
				"responseId":"gem_resp_1",
				"modelVersion":"gemini-2.5-flash",
				"candidates":[{"index":0,"finishReason":"STOP","content":{"role":"model","parts":[{"text":"gemini ok"}]}}],
				"usageMetadata":{"promptTokenCount":9,"candidatesTokenCount":4}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatMultiProviderTestServer(t, map[domain.Provider]*http.Client{
		domain.ProviderGemini: upstreamClient,
	})

	reqBody := `{
		"model":"gemini/gemini-2.5-flash",
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
	if resp.Model != "gemini/gemini-2.5-flash" {
		t.Fatalf("model = %q", resp.Model)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "gemini ok" {
		t.Fatalf("choices = %#v", resp.Choices)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish_reason = %q", resp.Choices[0].FinishReason)
	}
}

func TestHandleCompatOpenAIChatCompletions_GeminiStreamLoop(t *testing.T) {
	upstreamClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://gemini.example/v1internal:streamGenerateContent?alt=sse" {
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
			if _, ok := body["model"]; ok {
				t.Fatalf("upstream body unexpectedly kept model field: %#v", body)
			}

			respBody := strings.Join([]string{
				`data: {"response":{"responseId":"gem_stream_1","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"hello"}]}}]}}`,
				``,
				`data: {"response":{"responseId":"gem_stream_1","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":" world"}]}}]}}`,
				``,
				`data: {"response":{"responseId":"gem_stream_1","candidates":[{"index":0,"finishReason":"STOP","content":{"role":"model","parts":[]}}],"usageMetadata":{"promptTokenCount":9,"candidatesTokenCount":4}}}`,
				``,
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}
	srv := newCompatMultiProviderTestServer(t, map[domain.Provider]*http.Client{
		domain.ProviderGemini: upstreamClient,
	})

	reqBody := `{
		"model":"gemini/gemini-2.5-flash",
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

func newCompatTestServer(t *testing.T, upstreamClient *http.Client) *Server {
	return newCompatMultiProviderTestServer(t, map[domain.Provider]*http.Client{
		domain.ProviderClaude: upstreamClient,
	})
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
		{
			ID:                "acct-gemini-1",
			Email:             "gemini@example.com",
			Provider:          domain.ProviderGemini,
			Status:            domain.StatusActive,
			Priority:          1,
			CellID:            "cell-compat-gemini-1",
			ProviderStateJSON: `{"project_id":"proj-123"}`,
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
		{
			ID:        "cell-compat-gemini-1",
			Name:      "Compat Gemini 01",
			Status:    domain.EgressCellActive,
			Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 11082},
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
	if err := p.Update("acct-gemini-1", func(acct *domain.Account) {
		acct.ProviderStateJSON = `{"project_id":"proj-123"}`
	}); err != nil {
		t.Fatalf("pool.Update(gemini state) error = %v", err)
	}

	claudeDrv := driver.NewClaudeDriver(driver.ClaudeConfig{
		APIURL:     "https://claude.example/v1/messages",
		APIVersion: "2023-06-01",
	}, identity.NewTransformer(noopStainlessBinder{}, 8))
	geminiDrv := driver.NewGeminiDriver(driver.GeminiConfig{
		APIURL: "https://gemini.example",
	})

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
				MaxRequestBodyMB: 60,
				MaxRetryAccounts: 0,
			},
			staticTransportProvider{clients: upstreamClients},
			bus,
			map[domain.Provider]driver.ExecutionDriver{
				domain.ProviderClaude: claudeDrv,
				domain.ProviderGemini: geminiDrv,
			},
		),
		catalogDrivers: map[domain.Provider]driver.Descriptor{
			domain.ProviderClaude: claudeDrv,
			domain.ProviderGemini: geminiDrv,
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
