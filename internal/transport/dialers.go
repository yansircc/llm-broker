package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"

	utls "github.com/refraction-networking/utls"
	"github.com/yansircc/llm-broker/internal/domain"
	"golang.org/x/net/proxy"
)

func dialUTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	dialer := &net.Dialer{}
	rawConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	return uTLSHandshake(ctx, rawConn, host)
}

func uTLSHandshake(_ context.Context, rawConn net.Conn, serverName string) (net.Conn, error) {
	tlsConn := utls.UClient(rawConn, &utls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}, utls.HelloChrome_Auto)

	if err := tlsConn.HandshakeContext(context.Background()); err != nil {
		rawConn.Close()
		return nil, err
	}

	return tlsConn, nil
}

func rawProxyDialer(pcfg *domain.ProxyConfig) func(ctx context.Context, network, addr string) (net.Conn, error) {
	switch pcfg.Type {
	case "socks5":
		return rawSocks5Dialer(pcfg)
	default:
		return rawHTTPConnectDialer(pcfg)
	}
}

func rawSocks5Dialer(pcfg *domain.ProxyConfig) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		proxyAddr := fmt.Sprintf("%s:%d", pcfg.Host, pcfg.Port)

		var auth *proxy.Auth
		if pcfg.Username != "" {
			auth = &proxy.Auth{
				User:     pcfg.Username,
				Password: pcfg.Password,
			}
		}

		dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("socks5 dialer: %w", err)
		}

		targets, err := socks5TargetAddrs(ctx, network, addr, pcfg, net.DefaultResolver.LookupIPAddr)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, target := range targets {
			rawConn, err := dialer.Dial(network, target)
			if err == nil {
				return rawConn, nil
			}
			lastErr = err
		}
		return nil, fmt.Errorf("socks5 dial: %w", lastErr)
	}
}

type ipLookupFunc func(context.Context, string) ([]net.IPAddr, error)

func socks5TargetAddrs(ctx context.Context, network, addr string, pcg *domain.ProxyConfig, lookup ipLookupFunc) ([]string, error) {
	if pcg == nil || !isLocalProxyHost(pcg.Host) {
		return []string{addr}, nil
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return []string{addr}, nil
	}
	if net.ParseIP(host) != nil {
		return []string{addr}, nil
	}

	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve socks5 target %s: %w", host, err)
	}
	targets := make([]string, 0, len(ips))
	for _, ip := range ips {
		if ip.IP == nil {
			continue
		}
		if network == "tcp4" && ip.IP.To4() == nil {
			continue
		}
		if network == "tcp6" && ip.IP.To4() != nil {
			continue
		}
		targets = append(targets, net.JoinHostPort(ip.IP.String(), port))
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("resolve socks5 target %s: no %s addresses", host, network)
	}
	return targets, nil
}

func isLocalProxyHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func rawHTTPConnectDialer(pcfg *domain.ProxyConfig) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		proxyAddr := fmt.Sprintf("%s:%d", pcfg.Host, pcfg.Port)

		dialer := &net.Dialer{}
		rawConn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("proxy tcp dial: %w", err)
		}

		connectReq := &http.Request{
			Method: http.MethodConnect,
			URL:    nil,
			Host:   addr,
			Header: make(http.Header),
		}

		if pcfg.Username != "" {
			cred := base64.StdEncoding.EncodeToString([]byte(pcfg.Username + ":" + pcfg.Password))
			connectReq.Header.Set("Proxy-Authorization", "Basic "+cred)
		}

		if err := connectReq.Write(rawConn); err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("proxy CONNECT write: %w", err)
		}

		resp, err := http.ReadResponse(bufio.NewReader(rawConn), connectReq)
		if err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("proxy CONNECT read: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			rawConn.Close()
			return nil, fmt.Errorf("proxy CONNECT failed: %s", resp.Status)
		}
		return rawConn, nil
	}
}
