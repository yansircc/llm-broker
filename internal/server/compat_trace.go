package server

import (
	"bytes"
	"encoding/json"
)

const compatTraceBodyLimit = 64 << 10

func formatCompatTraceBody(body []byte) (string, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", false
	}

	formatted := trimmed
	if bytes.HasPrefix(trimmed, []byte("{")) || bytes.HasPrefix(trimmed, []byte("[")) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, trimmed); err == nil {
			formatted = compact.Bytes()
		}
	}

	if len(formatted) <= compatTraceBodyLimit {
		return string(formatted), false
	}
	return string(formatted[:compatTraceBodyLimit]) + "...<truncated>", true
}
