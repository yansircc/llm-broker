package relay

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (r *Relay) attachRequestLogArtifacts(entry *domain.RequestLog, prepared *preparedRelayRequest, upstreamReqBody, upstreamRespBody []byte) {
	if r == nil || entry == nil || strings.TrimSpace(r.cfg.RequestLogBlobDir) == "" {
		return
	}
	if prepared != nil {
		entry.RequestMeta = r.attachBodyArtifact(entry.RequestMeta, "client-request", requestLogClientBody(prepared))
	}
	entry.UpstreamRequestMeta = r.attachBodyArtifact(entry.UpstreamRequestMeta, "upstream-request", upstreamReqBody)
	entry.UpstreamResponseMeta = r.attachBodyArtifact(entry.UpstreamResponseMeta, "upstream-response", upstreamRespBody)
}

func (r *Relay) attachBodyArtifact(raw json.RawMessage, kind string, body []byte) json.RawMessage {
	if len(bytes.TrimSpace(body)) == 0 {
		return raw
	}

	path, sha, err := r.writeBodyArtifact(kind, body)
	extra := map[string]any{
		"body_artifact_sha256": sha,
		"body_artifact_bytes":  len(body),
	}
	if err != nil {
		extra["body_artifact_error"] = err.Error()
		slog.Warn("write request log artifact failed", "kind", kind, "sha256", sha, "error", err)
		return mergeObservationMeta(raw, extra)
	}
	extra["body_artifact_path"] = path
	return mergeObservationMeta(raw, extra)
}

func (r *Relay) writeBodyArtifact(kind string, body []byte) (string, string, error) {
	sha := observationRawBodyHash(body)
	if sha == "" {
		return "", "", nil
	}
	safeKind := sanitizeArtifactKind(kind)
	ext := ".txt"
	trimmed := bytes.TrimSpace(body)
	if bytes.HasPrefix(trimmed, []byte("{")) || bytes.HasPrefix(trimmed, []byte("[")) {
		ext = ".json"
	}
	absPath := filepath.Join(r.cfg.RequestLogBlobDir, safeKind, sha[:2], sha+ext)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", sha, err
	}
	if _, err := os.Stat(absPath); err == nil {
		now := time.Now()
		_ = os.Chtimes(absPath, now, now) // refresh mtime so purge won't delete a still-referenced blob
		return absPath, sha, nil
	} else if !os.IsNotExist(err) {
		return "", sha, err
	}
	if err := os.WriteFile(absPath, body, 0o644); err != nil {
		return "", sha, err
	}
	return absPath, sha, nil
}

func sanitizeArtifactKind(kind string) string {
	kind = strings.TrimSpace(strings.ToLower(kind))
	if kind == "" {
		return "body"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	return replacer.Replace(kind)
}

func mergeObservationMeta(raw json.RawMessage, extra map[string]any) json.RawMessage {
	if len(extra) == 0 {
		return raw
	}
	merged := map[string]any{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
		_ = json.Unmarshal(trimmed, &merged)
	}
	for key, value := range extra {
		if hasValue(value) {
			merged[key] = value
		}
	}
	return marshalObservationMap(merged)
}
