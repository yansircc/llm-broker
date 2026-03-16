package neterr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
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

	t.Run("context canceled is not transport", func(t *testing.T) {
		if IsTransport(context.Canceled) {
			t.Fatal("bare context.Canceled should not be classified as transport")
		}
	})

	t.Run("url.Error wrapping context canceled is not transport", func(t *testing.T) {
		// http.Client.Do returns *url.Error which implements net.Error.
		// Without the context.Canceled guard, errors.As(err, &netErr)
		// matches and the error is misclassified as a transport failure.
		err := &url.Error{
			Op:  "Post",
			URL: "https://api.anthropic.com/v1/messages",
			Err: context.Canceled,
		}
		if IsTransport(err) {
			t.Fatal("*url.Error wrapping context.Canceled should not be classified as transport")
		}
	})
}
