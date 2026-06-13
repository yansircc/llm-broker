package transport

import (
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/yansircc/llm-broker/internal/domain"
)

func TestSocks5TargetAddrsResolveLocalProxyTargets(t *testing.T) {
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{
			{IP: net.ParseIP("2607:6bc0::10")},
			{IP: net.ParseIP("160.79.104.10")},
		}, nil
	}

	got, err := socks5TargetAddrs(context.Background(), "tcp", "platform.claude.com:443", &domain.ProxyConfig{
		Type: "socks5",
		Host: "127.0.0.1",
		Port: 11080,
	}, lookup)
	if err != nil {
		t.Fatalf("socks5TargetAddrs() error = %v", err)
	}

	want := []string{"[2607:6bc0::10]:443", "160.79.104.10:443"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("socks5TargetAddrs() = %#v, want %#v", got, want)
	}
}

func TestSocks5TargetAddrsLeavesRemoteProxyHostnames(t *testing.T) {
	lookupCalled := false
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		lookupCalled = true
		return nil, nil
	}

	got, err := socks5TargetAddrs(context.Background(), "tcp", "platform.claude.com:443", &domain.ProxyConfig{
		Type: "socks5",
		Host: "proxy.example.com",
		Port: 1080,
	}, lookup)
	if err != nil {
		t.Fatalf("socks5TargetAddrs() error = %v", err)
	}
	if lookupCalled {
		t.Fatal("remote proxy target should not be resolved locally")
	}

	want := []string{"platform.claude.com:443"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("socks5TargetAddrs() = %#v, want %#v", got, want)
	}
}

func TestSocks5TargetAddrsLeavesIPTargets(t *testing.T) {
	lookupCalled := false
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		lookupCalled = true
		return nil, nil
	}

	got, err := socks5TargetAddrs(context.Background(), "tcp", "[2607:6bc0::10]:443", &domain.ProxyConfig{
		Type: "socks5",
		Host: "localhost",
		Port: 11080,
	}, lookup)
	if err != nil {
		t.Fatalf("socks5TargetAddrs() error = %v", err)
	}
	if lookupCalled {
		t.Fatal("IP target should not be resolved locally")
	}

	want := []string{"[2607:6bc0::10]:443"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("socks5TargetAddrs() = %#v, want %#v", got, want)
	}
}
