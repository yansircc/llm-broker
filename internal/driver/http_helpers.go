package driver

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func parseRetryAfter(value string) time.Duration {
	return parseRetryAfterAt(value, time.Now())
}

func parseRetryAfterAt(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if secs, err := strconv.ParseInt(value, 10, 64); err == nil && secs > 0 && secs <= int64(time.Duration(1<<63-1)/time.Second) {
		return time.Duration(secs) * time.Second
	}
	if ts, err := http.ParseTime(value); err == nil && ts.After(now) {
		return ts.Sub(now)
	}
	return 0
}

func appendRawQuery(rawURL, rawQuery string) (string, error) {
	if rawQuery == "" {
		return rawURL, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	additional, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", err
	}
	for k, vals := range additional {
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ensureBetaParam adds beta=true to the query string if not already present.
// Claude Code always sends this parameter; its absence is a fingerprint signal.
func ensureBetaParam(rawQuery string) string {
	if strings.Contains(rawQuery, "beta=true") {
		return rawQuery
	}
	if rawQuery == "" {
		return "beta=true"
	}
	return rawQuery + "&beta=true"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func httpClientOrDefault(client *http.Client, timeout time.Duration) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: timeout}
}
