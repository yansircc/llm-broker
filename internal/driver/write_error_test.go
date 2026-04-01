package driver

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestDriverWriteErrorEscapesJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&ClaudeDriver{}).WriteError(recorder, 400, `unsupported Claude model "claude-sonnet-4-20250514"`)

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, body = %q", err, recorder.Body.String())
	}
	errBody, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("error body = %#v", body["error"])
	}
	if errBody["message"] != `unsupported Claude model "claude-sonnet-4-20250514"` {
		t.Fatalf("message = %#v", errBody["message"])
	}
}
