package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"golang.org/x/net/http2"
)

// Pool reuses round trippers by transport shape (direct or proxy config).
type Pool struct {
	mu             sync.Mutex
	entries        map[string]*poolEntry
	requestTimeout time.Duration
}

type poolEntry struct {
	roundTripper http.RoundTripper
	lastUsed     time.Time
}

func NewPool(requestTimeout time.Duration) *Pool {
	return &Pool{
		entries:        make(map[string]*poolEntry),
		requestTimeout: requestTimeout,
	}
}

// ClientForAccount returns an http.Client backed by the matching shared transport.
func (m *Pool) ClientForAccount(acct *domain.Account) *http.Client {
	return &http.Client{
		Transport: m.getRoundTripper(acct),
		Timeout:   m.requestTimeout,
	}
}

// TransportForProxy returns a plain http.Transport for refresh flows that only need proxy routing.
func (m *Pool) TransportForProxy(pcfg *domain.ProxyConfig) *http.Transport {
	if pcfg == nil {
		return nil
	}
	return &http.Transport{
		ForceAttemptHTTP2: true,
		DialContext:       rawProxyDialer(pcfg),
	}
}

// RunCleanup starts the background cleanup goroutine. Blocks until ctx is canceled.
func (m *Pool) RunCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanup(5 * time.Minute)
		}
	}
}

// Close closes all pooled transports.
func (m *Pool) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, entry := range m.entries {
		if t, ok := entry.roundTripper.(interface{ CloseIdleConnections() }); ok {
			t.CloseIdleConnections()
		}
		delete(m.entries, key)
	}
}

func (m *Pool) getRoundTripper(acct *domain.Account) http.RoundTripper {
	key := transportKey(acct)

	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, ok := m.entries[key]; ok {
		entry.lastUsed = time.Now()
		return entry.roundTripper
	}

	rt := buildRoundTripper(acct)
	m.entries[key] = &poolEntry{roundTripper: rt, lastUsed: time.Now()}
	return rt
}

func (m *Pool) cleanup(idleTimeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-idleTimeout)
	for key, entry := range m.entries {
		if entry.lastUsed.Before(cutoff) {
			if t, ok := entry.roundTripper.(interface{ CloseIdleConnections() }); ok {
				t.CloseIdleConnections()
			}
			delete(m.entries, key)
		}
	}
}

func transportKey(acct *domain.Account) string {
	proxy := acct.TransportProxy()
	if proxy == nil {
		return "direct:" + acct.ID
	}
	return fmt.Sprintf("%s://%s:%d/%s", proxy.Type, proxy.Host, proxy.Port, acct.ID)
}

func buildRoundTripper(acct *domain.Account) http.RoundTripper {
	if proxy := acct.TransportProxy(); proxy != nil {
		dialRaw := rawProxyDialer(proxy)
		return &http2.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				rawConn, err := dialRaw(ctx, network, addr)
				if err != nil {
					return nil, err
				}
				host, _, err := net.SplitHostPort(addr)
				if err != nil {
					host = addr
				}
				return uTLSHandshake(ctx, rawConn, host)
			},
		}
	}
	return &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return dialUTLS(ctx, network, addr)
		},
	}
}
