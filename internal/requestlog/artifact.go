package requestlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

// LogObservation carries the heavy fields that used to live on domain.RequestLog
// but now persist only to the per-request JSON file on disk.
type LogObservation struct {
	Path                 string
	BucketKey            string
	SessionUUID          string
	BindingSource        string
	UpstreamURL          string
	UpstreamRequestID    string
	UpstreamErrorMessage string
	RequestBytes         int
	AttemptCount         int

	ClientHeaders               json.RawMessage
	ClientBodyExcerpt           string
	RequestMeta                 json.RawMessage
	UpstreamRequestHeaders      json.RawMessage
	UpstreamRequestMeta         json.RawMessage
	UpstreamRequestBodyExcerpt  string
	UpstreamHeaders             json.RawMessage
	UpstreamResponseMeta        json.RawMessage
	UpstreamResponseBodyExcerpt string

	ClientBody           []byte
	UpstreamRequestBody  []byte
	UpstreamResponseBody []byte
}

// BlobMode controls which request log entries write a JSON file to disk.
//
//	BlobModeOff:    never write a file (SQL row only).
//	BlobModeErrors: write only when the entry's Status != "ok".
//	BlobModeAll:    write for every entry.
//
// The SQL row is always inserted regardless of mode — it powers dashboard
// stats and is much smaller than the observation payload.
type BlobMode string

const (
	BlobModeOff    BlobMode = "off"
	BlobModeErrors BlobMode = "errors"
	BlobModeAll    BlobMode = "all"
)

// ParseBlobMode interprets a LOG_BLOBS env value. Empty (unset) defaults to
// errors-only. Boolean values are accepted for backward compatibility: true→all,
// false→off. Unknown values fall back to errors.
func ParseBlobMode(raw string) BlobMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "errors", "error":
		return BlobModeErrors
	case "off", "false", "0", "no", "none":
		return BlobModeOff
	case "all", "true", "1", "yes":
		return BlobModeAll
	default:
		return BlobModeErrors
	}
}

// ShouldWrite reports whether a request log entry with the given status should
// have its observation file written under the receiver mode.
func (m BlobMode) ShouldWrite(status string) bool {
	switch m {
	case BlobModeOff:
		return false
	case BlobModeAll:
		return true
	case BlobModeErrors:
		return strings.TrimSpace(status) != "ok"
	default:
		return false
	}
}

// ResolveBlobDir returns the on-disk log directory when blobs are enabled and
// the database path is real. Returns "" when blobs are disabled, the path is
// missing, or the path is the in-memory sentinel.
func ResolveBlobDir(dbPath string, mode BlobMode) string {
	if mode == BlobModeOff {
		return ""
	}
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || dbPath == ":memory:" {
		return ""
	}
	return filepath.Join(filepath.Dir(dbPath), "request-log-blobs")
}

// WriteLogFile persists the per-request observation payload to
// <logDir>/YYYY/MM/DD/<id>.json. It is a no-op when logDir is empty, when the
// entry is nil, or when the entry's id has not yet been assigned by the store.
func WriteLogFile(logDir string, entry *domain.RequestLog, obs *LogObservation) error {
	if logDir == "" || entry == nil || entry.ID <= 0 {
		return nil
	}
	payload, err := buildLogPayload(entry, obs)
	if err != nil {
		slog.Warn("build request log payload failed", "id", entry.ID, "error", err)
		return err
	}
	dayDir := filepath.Join(logDir, entry.CreatedAt.UTC().Format("2006/01/02"))
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		return fmt.Errorf("mkdir log dir: %w", err)
	}
	target := filepath.Join(dayDir, strconv.FormatInt(entry.ID, 10)+".json")
	if err := os.WriteFile(target, payload, 0o600); err != nil {
		return fmt.Errorf("write log file: %w", err)
	}
	return nil
}

// PurgeLogsBefore removes day directories under logDir whose date is strictly
// older than before (interpreted in UTC). Best-effort: errors on individual
// directories are logged and skipped so a single bad path can't stall purges.
func PurgeLogsBefore(logDir string, before time.Time) {
	if logDir == "" {
		return
	}
	cutoff := before.UTC().Truncate(24 * time.Hour)
	years, err := os.ReadDir(logDir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("purge log dir read failed", "dir", logDir, "error", err)
		}
		return
	}
	for _, yEntry := range years {
		if !yEntry.IsDir() {
			continue
		}
		year, err := strconv.Atoi(yEntry.Name())
		if err != nil || year < 1970 || year > 9999 {
			continue
		}
		yearDir := filepath.Join(logDir, yEntry.Name())
		months, err := os.ReadDir(yearDir)
		if err != nil {
			slog.Warn("purge log year read failed", "dir", yearDir, "error", err)
			continue
		}
		for _, mEntry := range months {
			if !mEntry.IsDir() {
				continue
			}
			month, err := strconv.Atoi(mEntry.Name())
			if err != nil || month < 1 || month > 12 {
				continue
			}
			monthDir := filepath.Join(yearDir, mEntry.Name())
			days, err := os.ReadDir(monthDir)
			if err != nil {
				slog.Warn("purge log month read failed", "dir", monthDir, "error", err)
				continue
			}
			for _, dEntry := range days {
				if !dEntry.IsDir() {
					continue
				}
				day, err := strconv.Atoi(dEntry.Name())
				if err != nil || day < 1 || day > 31 {
					continue
				}
				dayDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
				if !dayDate.Before(cutoff) {
					continue
				}
				dayDir := filepath.Join(monthDir, dEntry.Name())
				if err := os.RemoveAll(dayDir); err != nil {
					slog.Warn("purge log day failed", "dir", dayDir, "error", err)
				}
			}
		}
	}
}

// MergeMeta merges extra keys into a JSON object stored as raw, returning a
// JSON object suitable for assigning back to an observation meta field. Empty
// or null inputs are treated as the empty object.
func MergeMeta(raw json.RawMessage, extra map[string]any) json.RawMessage {
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
	if len(merged) == 0 {
		return nil
	}
	data, err := json.Marshal(merged)
	if err != nil || string(data) == "{}" {
		return nil
	}
	return json.RawMessage(data)
}

func buildLogPayload(entry *domain.RequestLog, obs *LogObservation) ([]byte, error) {
	payload := map[string]any{
		"schema":     "llm-broker.request_log.v2",
		"id":         entry.ID,
		"created_at": entry.CreatedAt.UTC().Format(time.RFC3339Nano),
		"facts": compactMap(map[string]any{
			"user_id":             entry.UserID,
			"account_id":          entry.AccountID,
			"provider":            entry.Provider,
			"surface":             entry.Surface,
			"model":               entry.Model,
			"cell_id":             entry.CellID,
			"status":              entry.Status,
			"effect_kind":         entry.EffectKind,
			"upstream_status":     entry.UpstreamStatus,
			"upstream_error_type": entry.UpstreamErrorType,
			"input_tokens":        entry.InputTokens,
			"output_tokens":       entry.OutputTokens,
			"cache_read_tokens":   entry.CacheReadTokens,
			"cache_create_tokens": entry.CacheCreateTokens,
			"cost_usd":            entry.CostUSD,
			"duration_ms":         entry.DurationMs,
		}),
	}
	if obs == nil {
		return json.MarshalIndent(payload, "", "  ")
	}
	if extra := compactMap(map[string]any{
		"path":                   obs.Path,
		"bucket_key":             obs.BucketKey,
		"session_uuid":           obs.SessionUUID,
		"binding_source":         obs.BindingSource,
		"upstream_url":           obs.UpstreamURL,
		"upstream_request_id":    obs.UpstreamRequestID,
		"upstream_error_message": obs.UpstreamErrorMessage,
		"request_bytes":          obs.RequestBytes,
		"attempt_count":          obs.AttemptCount,
	}); len(extra) > 0 {
		facts := payload["facts"].(map[string]any)
		for k, v := range extra {
			facts[k] = v
		}
	}
	if client := compactMap(map[string]any{
		"headers":      jsonValue(obs.ClientHeaders),
		"meta":         jsonValue(obs.RequestMeta),
		"body_excerpt": obs.ClientBodyExcerpt,
		"body":         bodyValue(obs.ClientBody),
	}); len(client) > 0 {
		payload["client"] = client
	}
	if upstreamReq := compactMap(map[string]any{
		"url":          obs.UpstreamURL,
		"headers":      jsonValue(obs.UpstreamRequestHeaders),
		"meta":         jsonValue(obs.UpstreamRequestMeta),
		"body_excerpt": obs.UpstreamRequestBodyExcerpt,
		"body":         bodyValue(obs.UpstreamRequestBody),
	}); len(upstreamReq) > 0 {
		payload["upstream_request"] = upstreamReq
	}
	if upstreamResp := compactMap(map[string]any{
		"headers":       jsonValue(obs.UpstreamHeaders),
		"meta":          jsonValue(obs.UpstreamResponseMeta),
		"body_excerpt":  obs.UpstreamResponseBodyExcerpt,
		"body":          bodyValue(obs.UpstreamResponseBody),
		"error_type":    entry.UpstreamErrorType,
		"error_message": obs.UpstreamErrorMessage,
	}); len(upstreamResp) > 0 {
		payload["upstream_response"] = upstreamResp
	}
	return json.MarshalIndent(payload, "", "  ")
}

func compactMap(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		if hasValue(value) {
			out[key] = value
		}
	}
	return out
}

func jsonValue(raw json.RawMessage) any {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}")) {
		return nil
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return string(trimmed)
	}
	return value
}

func bodyValue(body []byte) any {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(trimmed, &value) == nil {
		return value
	}
	return string(body)
}

func hasValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}
