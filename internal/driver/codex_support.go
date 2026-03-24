package driver

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CodexFamilyLimits holds rate-limit state for one quota family.
type CodexFamilyLimits struct {
	PrimaryUtil    float64 `json:"pu"`
	PrimaryReset   int64   `json:"pr"`
	SecondaryUtil  float64 `json:"su"`
	SecondaryReset int64   `json:"sr"`
	LimitName      string  `json:"name,omitempty"` // e.g. "GPT-5.3-Codex-Spark"
}

// CodexState holds the provider-specific rate-limit state for Codex accounts.
// State is stored per quota family: "" = standard codex, "bengalfox" = spark, etc.
type CodexState struct {
	Families map[string]CodexFamilyLimits `json:"families,omitempty"`

	// Legacy flat fields — read for backward compat, cleared on first write.
	PrimaryUtil    float64 `json:"primary_util,omitempty"`
	PrimaryReset   int64   `json:"primary_reset,omitempty"`
	SecondaryUtil  float64 `json:"secondary_util,omitempty"`
	SecondaryReset int64   `json:"secondary_reset,omitempty"`
}

// family returns the limits for the given family prefix.
// Falls back to legacy flat fields for the standard ("") family.
func (s *CodexState) family(prefix string) CodexFamilyLimits {
	if s.Families != nil {
		if f, ok := s.Families[prefix]; ok {
			return f
		}
	}
	if prefix == "" && (s.PrimaryUtil > 0 || s.PrimaryReset > 0 || s.SecondaryUtil > 0 || s.SecondaryReset > 0) {
		return CodexFamilyLimits{
			PrimaryUtil: s.PrimaryUtil, PrimaryReset: s.PrimaryReset,
			SecondaryUtil: s.SecondaryUtil, SecondaryReset: s.SecondaryReset,
		}
	}
	return CodexFamilyLimits{}
}

// allFamilies returns all known family prefixes in stable order.
func (s *CodexState) allFamilies() []string {
	if len(s.Families) > 0 {
		keys := make([]string, 0, len(s.Families))
		for k := range s.Families {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}
	if s.PrimaryUtil > 0 || s.PrimaryReset > 0 || s.SecondaryUtil > 0 || s.SecondaryReset > 0 {
		return []string{""}
	}
	return nil
}

// codexModelFamily maps a model name to its quota family prefix.
func codexModelFamily(model string) string {
	if strings.Contains(strings.ToLower(model), "spark") {
		return "bengalfox"
	}
	return ""
}

// codexFamilyDisplayName returns a human-readable label prefix for a family.
func codexFamilyDisplayName(prefix string, f CodexFamilyLimits) string {
	if f.LimitName != "" {
		return f.LimitName
	}
	if prefix == "" {
		return ""
	}
	return prefix
}

func (d *CodexDriver) captureHeaders(headers http.Header, prevState json.RawMessage) json.RawMessage {
	if headers == nil {
		if len(prevState) > 0 {
			return prevState
		}
		return nil
	}

	var state CodexState
	if len(prevState) > 0 {
		json.Unmarshal(prevState, &state)
	}

	// Migrate legacy flat fields into Families map.
	if state.Families == nil {
		state.Families = make(map[string]CodexFamilyLimits)
	}
	if state.PrimaryUtil > 0 || state.PrimaryReset > 0 || state.SecondaryUtil > 0 || state.SecondaryReset > 0 {
		if _, ok := state.Families[""]; !ok {
			state.Families[""] = CodexFamilyLimits{
				PrimaryUtil: state.PrimaryUtil, PrimaryReset: state.PrimaryReset,
				SecondaryUtil: state.SecondaryUtil, SecondaryReset: state.SecondaryReset,
			}
		}
		state.PrimaryUtil = 0
		state.PrimaryReset = 0
		state.SecondaryUtil = 0
		state.SecondaryReset = 0
	}

	// Discover family prefixes from headers.
	// Standard: x-codex-primary-used-percent
	// Extra:    x-codex-{family}-primary-used-percent
	prefixes := discoverCodexFamilyPrefixes(headers)
	now := time.Now().Unix()
	for _, prefix := range prefixes {
		f := parseCodexFamilyHeaders(headers, prefix, now)
		// Capture limit-name if available.
		if prefix != "" {
			if name := headers.Get("x-codex-" + prefix + "-limit-name"); name != "" {
				f.LimitName = name
			}
		}
		state.Families[prefix] = f
	}

	data, _ := json.Marshal(state)
	return data
}

// discoverCodexFamilyPrefixes finds all quota-family prefixes in the response headers.
// Returns "" for the standard family and e.g. "bengalfox" for spark.
func discoverCodexFamilyPrefixes(headers http.Header) []string {
	seen := map[string]bool{}
	for key := range headers {
		lower := strings.ToLower(key)
		if !strings.HasPrefix(lower, "x-codex-") {
			continue
		}
		rest := lower[len("x-codex-"):]
		if rest == "primary-used-percent" || rest == "secondary-used-percent" {
			seen[""] = true
		} else if idx := strings.Index(rest, "-primary-used-percent"); idx > 0 {
			seen[rest[:idx]] = true
		} else if idx := strings.Index(rest, "-secondary-used-percent"); idx > 0 {
			seen[rest[:idx]] = true
		}
	}
	prefixes := make([]string, 0, len(seen))
	for p := range seen {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)
	return prefixes
}

// parseCodexFamilyHeaders extracts primary/secondary limits for a specific family prefix.
func parseCodexFamilyHeaders(headers http.Header, prefix string, nowUnix int64) CodexFamilyLimits {
	hp := "x-codex-"
	if prefix != "" {
		hp = "x-codex-" + prefix + "-"
	}
	var f CodexFamilyLimits
	if v := headers.Get(hp + "primary-used-percent"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			f.PrimaryUtil = pct / 100
		}
	}
	if v := headers.Get(hp + "primary-reset-after-seconds"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			f.PrimaryReset = nowUnix + int64(secs)
		}
	}
	if v := headers.Get(hp + "secondary-used-percent"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			f.SecondaryUtil = pct / 100
		}
	}
	if v := headers.Get(hp + "secondary-reset-after-seconds"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			f.SecondaryReset = nowUnix + int64(secs)
		}
	}
	return f
}

type codexUsageFields struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Details      *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func codexUsageToUsage(u *codexUsageFields) *Usage {
	if u == nil {
		return nil
	}
	result := &Usage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
	}
	if u.Details != nil {
		result.CacheReadTokens = u.Details.CachedTokens
	}
	return result
}

func parseCodexUsage(data string) *Usage {
	var wrapper struct {
		Type     string `json:"type"`
		Response struct {
			Usage *codexUsageFields `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal([]byte(data), &wrapper) != nil {
		return nil
	}
	return codexUsageToUsage(wrapper.Response.Usage)
}

func parseCodexResetsIn(body []byte) time.Duration {
	var envelope struct {
		Error struct {
			ResetsInSeconds int `json:"resets_in_seconds"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.ResetsInSeconds > 0 {
		return time.Duration(envelope.Error.ResetsInSeconds) * time.Second
	}
	return 0
}

func parseCodexErrorInfo(body []byte) (string, string) {
	var envelope struct {
		Error struct {
			Type    string `json:"type"`
			Code    any    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return "", ""
	}
	errType := envelope.Error.Type
	if errType == "" && envelope.Error.Code != nil {
		errType = stringifyCodexErrorCode(envelope.Error.Code)
	}
	return errType, envelope.Error.Message
}

func extractCodexErrorMessage(body []byte) string {
	_, message := parseCodexErrorInfo(body)
	return message
}

func stringifyCodexErrorCode(code any) string {
	switch v := code.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}
