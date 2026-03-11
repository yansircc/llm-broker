package transport

import (
	"net/http"
	"testing"

	"github.com/yansircc/llm-broker/internal/domain"
)

func TestBuildRoundTripperProxyEnablesHTTP2(t *testing.T) {
	acct := &domain.Account{
		Proxy: &domain.ProxyConfig{
			Type: "socks5",
			Host: "127.0.0.1",
			Port: 11080,
		},
	}

	rt := buildRoundTripper(acct)
	tr, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("round tripper type = %T, want *http.Transport", rt)
	}
	if !tr.ForceAttemptHTTP2 {
		t.Fatal("proxy transport should enable ForceAttemptHTTP2")
	}
	if tr.DialContext == nil {
		t.Fatal("proxy transport should use raw proxy DialContext")
	}
	if tr.DialTLSContext != nil {
		t.Fatal("proxy transport should leave TLS negotiation to net/http")
	}
}

func TestTransportForProxyEnablesHTTP2(t *testing.T) {
	pool := NewPool(0)
	tr := pool.TransportForProxy(&domain.ProxyConfig{
		Type: "socks5",
		Host: "127.0.0.1",
		Port: 11080,
	})
	if tr == nil {
		t.Fatal("TransportForProxy() returned nil")
	}
	if !tr.ForceAttemptHTTP2 {
		t.Fatal("refresh proxy transport should enable ForceAttemptHTTP2")
	}
	if tr.DialContext == nil {
		t.Fatal("refresh proxy transport should use raw proxy DialContext")
	}
	if tr.DialTLSContext != nil {
		t.Fatal("refresh proxy transport should leave TLS negotiation to net/http")
	}
}
