package driver

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestDriverWriteErrorEscapesJSON(t *testing.T) {
	tests := []struct {
		name  string
		write func(*httptest.ResponseRecorder, int, string)
		check func(*testing.T, map[string]any)
	}{
		{
			name: "claude",
			write: func(w *httptest.ResponseRecorder, status int, msg string) {
				(&ClaudeDriver{}).WriteError(w, status, msg)
			},
			check: func(t *testing.T, body map[string]any) {
				t.Helper()
				errBody, ok := body["error"].(map[string]any)
				if !ok {
					t.Fatalf("error body = %#v", body["error"])
				}
				if errBody["message"] != `unsupported Claude model "claude-sonnet-4-20250514"` {
					t.Fatalf("message = %#v", errBody["message"])
				}
			},
		},
		{
			name: "codex",
			write: func(w *httptest.ResponseRecorder, status int, msg string) {
				(&CodexDriver{}).WriteError(w, status, msg)
			},
			check: func(t *testing.T, body map[string]any) {
				t.Helper()
				errBody, ok := body["error"].(map[string]any)
				if !ok {
					t.Fatalf("error body = %#v", body["error"])
				}
				if errBody["message"] != `unsupported model "gpt-5.4"` {
					t.Fatalf("message = %#v", errBody["message"])
				}
			},
		},
		{
			name: "gemini",
			write: func(w *httptest.ResponseRecorder, status int, msg string) {
				(&GeminiDriver{}).WriteError(w, status, msg)
			},
			check: func(t *testing.T, body map[string]any) {
				t.Helper()
				errBody, ok := body["error"].(map[string]any)
				if !ok {
					t.Fatalf("error body = %#v", body["error"])
				}
				if errBody["message"] != `unsupported model "gemini-foo"` {
					t.Fatalf("message = %#v", errBody["message"])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.write(recorder, 400, map[string]string{
				"claude": `unsupported Claude model "claude-sonnet-4-20250514"`,
				"codex":  `unsupported model "gpt-5.4"`,
				"gemini": `unsupported model "gemini-foo"`,
			}[tc.name])

			var body map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("json.Unmarshal() error = %v, body = %q", err, recorder.Body.String())
			}
			tc.check(t, body)
		})
	}
}
