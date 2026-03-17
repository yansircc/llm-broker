package driver

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if ts, err := time.Parse(time.RFC1123, strings.TrimSpace(value)); err == nil {
		if d := time.Until(ts); d > 0 {
			return d
		}
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
