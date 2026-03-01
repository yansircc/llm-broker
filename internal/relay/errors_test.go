package relay

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeError_DirectMap(t *testing.T) {
	tests := []struct {
		code       int
		wantStatus int
		wantType   string
	}{
		{401, 401, "authentication_error"},
		{403, 403, "permission_error"},
		{429, 429, "rate_limit_error"},
		{529, 529, "overloaded_error"},
	}

	for _, tt := range tests {
		status, body := SanitizeError(tt.code, []byte("some error"))
		if status != tt.wantStatus {
			t.Errorf("SanitizeError(%d) status = %d, want %d", tt.code, status, tt.wantStatus)
		}
		var parsed struct {
			Error struct{ Type string } `json:"error"`
		}
		if json.Unmarshal(body, &parsed) != nil || parsed.Error.Type != tt.wantType {
			t.Errorf("SanitizeError(%d) type = %q, want %q", tt.code, parsed.Error.Type, tt.wantType)
		}
	}
}

func TestSanitizeError_PatternMatch(t *testing.T) {
	status, body := SanitizeError(500, []byte(`{"error":"rate_limit exceeded"}`))
	if status != 429 {
		t.Errorf("expected 429, got %d", status)
	}
	if !strings.Contains(string(body), "rate_limit_error") {
		t.Errorf("expected rate_limit_error in body")
	}
}

func TestSanitizeError_PreserveJSON(t *testing.T) {
	originalBody := []byte(`{"type":"error","error":{"type":"custom_error","message":"custom msg"}}`)
	// Use a status code that's not in directStatuses and body won't match patterns
	status, body := SanitizeError(418, originalBody)
	var parsed struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		t.Fatal("should be valid JSON")
	}
	if status != 418 {
		t.Errorf("expected 418, got %d", status)
	}
	if parsed.Error.Type != "custom_error" {
		t.Errorf("expected custom_error, got %s", parsed.Error.Type)
	}
}

func TestSanitizeError_Fallback(t *testing.T) {
	status, body := SanitizeError(599, []byte("random garbage"))
	if status != 500 {
		t.Errorf("expected 500 fallback, got %d", status)
	}
	if !strings.Contains(string(body), "unexpected upstream error") {
		t.Error("expected fallback message")
	}
}

func TestSanitizeSSEError(t *testing.T) {
	sse := SanitizeSSEError(429, []byte("rate limited"))
	if !strings.HasPrefix(sse, "event: error\n") {
		t.Error("should start with event: error")
	}
	if !strings.Contains(sse, "data: ") {
		t.Error("should contain data: line")
	}
	if !strings.HasSuffix(sse, "\n\n") {
		t.Error("should end with double newline")
	}
	// data portion should be valid JSON
	dataIdx := strings.Index(sse, "data: ")
	dataStr := strings.TrimSpace(sse[dataIdx+6:])
	var parsed map[string]interface{}
	if json.Unmarshal([]byte(dataStr), &parsed) != nil {
		t.Error("data should be valid JSON")
	}
}
