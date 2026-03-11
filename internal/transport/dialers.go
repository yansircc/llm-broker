package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"

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

		rawConn, err := dialer.Dial(network, addr)
		if err != nil {
			return nil, fmt.Errorf("socks5 dial: %w", err)
		}
		return rawConn, nil
	}
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
