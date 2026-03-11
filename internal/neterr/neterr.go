package neterr

import (
	"context"
	"errors"
	"net"
	"strings"
)

// IsTransport reports whether err looks like a network or proxy transport failure.
func IsTransport(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	msg := strings.ToLower(err.Error())
	markers := []string{
		"socks5 dial",
		"can't complete socks5 connection",
		"proxy connect",
		"dial tcp",
		"connection refused",
		"connection reset by peer",
		"network is unreachable",
		"no route to host",
		"broken pipe",
		"i/o timeout",
		"tls handshake timeout",
	}
	for _, marker := range markers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
