package relay

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

func TestRequestMetaSummarizesNestedEnvelope(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}},
					{"type":"tool_result","tool_use_id":"toolu_1","is_error":true,"content":"boom"}
				]
			}
		],
		"stream":true,
		"max_tokens":32000,
		"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral"}}],
		"tools":[
			{"name":"exec","type":"custom","input_schema":{"type":"object","properties":{"command":{"type":"string"}}}},
			{"name":"read","type":"custom","input_schema":{"type":"object","properties":{"path":{"type":"string"}}}}
		],
		"tool_choice":{"type":"auto"},
		"thinking":{"type":"enabled","budget_tokens":2048},
		"output_config":{"format":{"type":"json_object"}},
		"context_management":{"edits":"auto"},
		"metadata":{"user_id":"u1","session":"s1"}
	}`)

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	metaRaw := requestMeta(&preparedRelayRequest{
		input: &driver.RelayInput{
			Headers: http.Header{
				"X-Stainless-Retry-Count": []string{"2"},
			},
			RawBody:  raw,
			Body:     body,
			IsStream: true,
			Model:    "claude-sonnet-4-6",
		},
	})

	var meta map[string]any
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatalf("Unmarshal requestMeta: %v", err)
	}

	for _, key := range []string{
		"body_sha256",
		"messages_sha256",
		"tools_sha256",
		"thinking_sha256",
		"output_config_sha256",
		"context_management_sha256",
		"metadata_sha256",
	} {
		if _, ok := meta[key].(string); !ok {
			t.Fatalf("%s missing from meta: %s", key, string(metaRaw))
		}
	}
	if meta["message_cache_control_count"] != float64(1) {
		t.Fatalf("message_cache_control_count = %#v, want 1", meta["message_cache_control_count"])
	}
	if meta["tool_result_error_count"] != float64(1) {
		t.Fatalf("tool_result_error_count = %#v, want 1", meta["tool_result_error_count"])
	}
	if meta["system_block_count"] != float64(1) {
		t.Fatalf("system_block_count = %#v, want 1", meta["system_block_count"])
	}
	if meta["system_cache_control_count"] != float64(1) {
		t.Fatalf("system_cache_control_count = %#v, want 1", meta["system_cache_control_count"])
	}
	if meta["has_thinking"] != true {
		t.Fatalf("has_thinking = %#v, want true", meta["has_thinking"])
	}
	if meta["has_output_config"] != true {
		t.Fatalf("has_output_config = %#v, want true", meta["has_output_config"])
	}
	if meta["has_context_management"] != true {
		t.Fatalf("has_context_management = %#v, want true", meta["has_context_management"])
	}
	if meta["has_metadata"] != true {
		t.Fatalf("has_metadata = %#v, want true", meta["has_metadata"])
	}
	if meta["tool_choice_type"] != "auto" {
		t.Fatalf("tool_choice_type = %#v, want auto", meta["tool_choice_type"])
	}

	toolNames, ok := meta["tool_names"].([]any)
	if !ok || len(toolNames) != 2 {
		t.Fatalf("tool_names = %#v, want 2 names", meta["tool_names"])
	}
	if toolNames[0] != "exec" || toolNames[1] != "read" {
		t.Fatalf("tool_names = %#v, want [exec read]", toolNames)
	}
	toolSignatures, ok := meta["tool_signatures"].([]any)
	if !ok || len(toolSignatures) != 2 {
		t.Fatalf("tool_signatures = %#v, want 2 signatures", meta["tool_signatures"])
	}
}

func TestRequestBodyExcerptKeepsHeadAndTailWhenTruncated(t *testing.T) {
	body := []byte(`{"head":"` + strings.Repeat("A", 5000) + `","middle":"` + strings.Repeat("B", 5000) + `","tail":"TAIL_MARKER"}`)
	excerpt := requestBodyExcerpt(body)
	if !strings.Contains(excerpt, `"head":"`) {
		t.Fatalf("excerpt missing head: %q", excerpt)
	}
	if !strings.Contains(excerpt, `"tail":"TAIL_MARKER"`) {
		t.Fatalf("excerpt missing tail marker: %q", excerpt)
	}
	if !strings.Contains(excerpt, "...<truncated>...") {
		t.Fatalf("excerpt missing truncation marker: %q", excerpt)
	}
}

func TestAttachRequestLogArtifactsSpillsTruncatedBodiesToFiles(t *testing.T) {
	body := []byte(`{"head":"` + strings.Repeat("A", 5000) + `","middle":"` + strings.Repeat("B", 5000) + `","tail":"TAIL_MARKER"}`)
	entry := &domain.RequestLog{
		RequestMeta: json.RawMessage(`{"stream":true}`),
	}
	relaySvc := &Relay{
		cfg: Config{
			RequestLogBlobDir: t.TempDir(),
		},
	}
	prepared := &preparedRelayRequest{
		input: &driver.RelayInput{
			RawBody: body,
		},
	}

	relaySvc.attachRequestLogArtifacts(entry, prepared, nil, nil)

	var meta map[string]any
	if err := json.Unmarshal(entry.RequestMeta, &meta); err != nil {
		t.Fatalf("Unmarshal RequestMeta: %v", err)
	}
	path, ok := meta["body_artifact_path"].(string)
	if !ok || path == "" {
		t.Fatalf("body_artifact_path = %#v, want non-empty path", meta["body_artifact_path"])
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("body_artifact_path = %q, want absolute path", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if string(got) != string(body) {
		t.Fatalf("artifact content mismatch")
	}
}

func TestAttachRequestLogArtifactsSpillsSmallBodiesToFiles(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}]}`)
	entry := &domain.RequestLog{
		RequestMeta: json.RawMessage(`{"stream":false}`),
	}
	relaySvc := &Relay{
		cfg: Config{
			RequestLogBlobDir: t.TempDir(),
		},
	}
	prepared := &preparedRelayRequest{
		input: &driver.RelayInput{
			RawBody: body,
		},
	}

	relaySvc.attachRequestLogArtifacts(entry, prepared, nil, nil)

	var meta map[string]any
	if err := json.Unmarshal(entry.RequestMeta, &meta); err != nil {
		t.Fatalf("Unmarshal RequestMeta: %v", err)
	}
	path, ok := meta["body_artifact_path"].(string)
	if !ok || path == "" {
		t.Fatalf("body_artifact_path = %#v, want non-empty path", meta["body_artifact_path"])
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if string(got) != string(body) {
		t.Fatalf("artifact content mismatch")
	}
}

func TestRequestMetaUsesObservedClientRequest(t *testing.T) {
	rawClientBody := []byte(`{"model":"claude/claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}],"temperature":0.2}`)
	translatedBody := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}],"thinking":{"type":"adaptive"},"stream":false}`)

	var translated map[string]any
	if err := json.Unmarshal(translatedBody, &translated); err != nil {
		t.Fatalf("Unmarshal translatedBody: %v", err)
	}

	metaRaw := requestMeta(&preparedRelayRequest{
		input: &driver.RelayInput{
			Headers: http.Header{
				"X-Broker-Compat-Client-Meta": []string{`{"requested_model":"claude/claude-sonnet-4-6","message_count":1}`},
			},
			RawBody: translatedBody,
			Body:    translated,
			Path:    "/v1/messages",
			Model:   "claude-sonnet-4-6",
		},
		clientObservation: &ClientRequestObservation{
			Path:    "/compat/v1/chat/completions",
			Headers: http.Header{"Content-Type": []string{"application/json"}},
			Body:    rawClientBody,
		},
	})

	var meta map[string]any
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatalf("Unmarshal requestMeta: %v", err)
	}
	if meta["body_sha256"] != observationRawBodyHash(rawClientBody) {
		t.Fatalf("body_sha256 = %#v, want hash of raw compat body", meta["body_sha256"])
	}
	if _, ok := meta["temperature"].(float64); !ok {
		t.Fatalf("temperature = %#v, want from raw compat body", meta["temperature"])
	}
	if _, ok := meta["has_thinking"]; ok {
		t.Fatalf("has_thinking = %#v, want absent for raw compat body", meta["has_thinking"])
	}
	compatClient, ok := meta["compat_client"].(map[string]any)
	if !ok {
		t.Fatalf("compat_client = %#v, want object", meta["compat_client"])
	}
	if compatClient["requested_model"] != "claude/claude-sonnet-4-6" {
		t.Fatalf("compat_client.requested_model = %#v", compatClient["requested_model"])
	}
}
