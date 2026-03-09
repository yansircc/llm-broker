package driver

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/yansircc/llm-broker/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCodexBuildRequestUsesSubjectForAccountHeader(t *testing.T) {
	d := NewCodexDriver(CodexConfig{APIURL: "https://chatgpt.com/backend-api/codex"})
	input := &RelayInput{
		RawBody: []byte(`{"model":"gpt-5.1-codex"}`),
		Headers: make(http.Header),
	}
	acct := &domain.Account{
		Provider: domain.ProviderCodex,
		Subject:  "acct-subject-123",
		Identity: map[string]interface{}{"chatgptAccountId": "stale-identity"},
	}

	req, err := d.BuildRequest(context.Background(), input, acct, "tok")
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if got := req.Header.Get("Chatgpt-Account-Id"); got != acct.Subject {
		t.Fatalf("Chatgpt-Account-Id = %q, want %q", got, acct.Subject)
	}
}

func TestCodexProbeRequiresSubject(t *testing.T) {
	d := NewCodexDriver(CodexConfig{APIURL: "https://chatgpt.com/backend-api/codex"})
	acct := &domain.Account{Provider: domain.ProviderCodex}
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("event: response.output_text.delta\n")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := d.Probe(context.Background(), acct, "tok", client)
	if err == nil {
		t.Fatal("Probe() error = nil, want missing subject error")
	}
}
