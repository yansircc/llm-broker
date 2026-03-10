package driver

import (
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
