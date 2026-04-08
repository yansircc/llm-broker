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

func TestTransportKeyIsolation(t *testing.T) {
	pool := NewPool(0)

	acctA := &domain.Account{ID: "acct-a"}
	acctB := &domain.Account{ID: "acct-b"}

	rtA1 := pool.getRoundTripper(acctA)
	rtA2 := pool.getRoundTripper(acctA)
	rtB := pool.getRoundTripper(acctB)

	if rtA1 != rtA2 {
		t.Fatal("same account should reuse round tripper")
	}
	if rtA1 == rtB {
		t.Fatal("different direct accounts must not share round tripper")
	}
}

func TestTransportKeyIsolationProxy(t *testing.T) {
	pool := NewPool(0)

	proxy := &domain.ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 11080}
	acctA := &domain.Account{ID: "acct-a", Proxy: proxy}
	acctB := &domain.Account{ID: "acct-b", Proxy: proxy}

	rtA := pool.getRoundTripper(acctA)
	rtB := pool.getRoundTripper(acctB)

	if rtA == rtB {
		t.Fatal("different proxy accounts sharing same proxy must not share round tripper")
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
