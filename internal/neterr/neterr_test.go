package neterr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestIsTransport(t *testing.T) {
	t.Run("net op error", func(t *testing.T) {
		err := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
		if !IsTransport(err) {
			t.Fatal("expected net.OpError to be transport")
		}
	})

	t.Run("wrapped socks error", func(t *testing.T) {
		err := fmt.Errorf("socks5 dial: %w", errors.New("proxy failure"))
		if !IsTransport(err) {
			t.Fatal("expected socks5 error to be transport")
		}
	})

	t.Run("deadline", func(t *testing.T) {
		if !IsTransport(context.DeadlineExceeded) {
			t.Fatal("expected context deadline to be transport")
		}
	})

	t.Run("application error", func(t *testing.T) {
		if IsTransport(errors.New("oauth returned 400: bad request")) {
			t.Fatal("unexpected transport classification")
		}
	})
}
