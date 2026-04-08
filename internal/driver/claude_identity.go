package driver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

// ---------------------------------------------------------------------------
// Stainless header storage (provider-agnostic contract)
// ---------------------------------------------------------------------------

// StainlessStore provides raw get/set for per-account stainless header
// bindings.  Implemented by pool; consumed only by ClaudeDriver.
type StainlessStore interface {
	GetStainless(ctx context.Context, accountID string) (string, bool, error)
	SetStainlessNX(ctx context.Context, accountID, headersJSON string, ttl time.Duration) (bool, error)
}

// NoopStainlessStore is a no-op implementation for testing.
type NoopStainlessStore struct{}

func (NoopStainlessStore) GetStainless(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (NoopStainlessStore) SetStainlessNX(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}

// ---------------------------------------------------------------------------
// Compiled patterns
// ---------------------------------------------------------------------------

var (
	billingHeaderPattern = regexp.MustCompile(`(?i)x-anthropic-billing-header`)
	userIDPattern        = regexp.MustCompile(`^user_([a-fA-F0-9]{64})_account__session_([\w-]+)$`)
	sessionUUIDPattern   = regexp.MustCompile(`session_([a-f0-9-]{36})$`)
)

// ---------------------------------------------------------------------------
// Header filtering (Claude-specific allowlist)
// ---------------------------------------------------------------------------

const stainlessPrefix = "x-stainless-"

var claudeAllowedHeaders = map[string]bool{
	"accept":            true,
	"content-type":      true,
	"user-agent":        true,
	"anthropic-version": true,
	"anthropic-beta":    true,
	"anthropic-dangerous-direct-browser-access": true,
	"x-app": true,
}

func filterClaudeHeaders(original http.Header) http.Header {
	clean := make(http.Header)
	for key, vals := range original {
		lower := strings.ToLower(key)
		if claudeAllowedHeaders[lower] || strings.HasPrefix(lower, stainlessPrefix) {
			for _, v := range vals {
				clean.Add(key, v)
			}
		}
	}
	return clean
}

func removeAllStainless(h http.Header) {
	for key := range h {
		if strings.HasPrefix(strings.ToLower(key), stainlessPrefix) {
			h.Del(key)
		}
	}
}

const defaultClaudeVersion = "2.2.0"

func setClaudeRequiredHeaders(h http.Header, accessToken, apiVersion, betaHeader string) {
	h.Del("x-api-key")
	h.Del("Authorization")

	h.Set("Authorization", "Bearer "+accessToken)
	if h.Get("anthropic-version") == "" {
		h.Set("anthropic-version", apiVersion)
	}
	if mergedBeta := mergeBetaHeaders(h.Get("anthropic-beta"), betaHeader); mergedBeta != "" {
		h.Set("anthropic-beta", mergedBeta)
	}
	h.Set("Content-Type", "application/json")

	// Derive UA from bound stainless version for fingerprint consistency.
	version := defaultClaudeVersion
	if v := h.Get("x-stainless-package-version"); v != "" {
		version = v
	}
	h.Set("User-Agent", "claude-cli/"+version+" (external, cli)")
}

func mergeBetaHeaders(clientBeta, relayBeta string) string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, raw := range []string{clientBeta, relayBeta} {
		if raw == "" {
			continue
		}
		for _, part := range strings.Split(raw, ",") {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	return strings.Join(out, ",")
}

// ---------------------------------------------------------------------------
// Transform pipeline
// ---------------------------------------------------------------------------

type claudeTransformer struct {
	stainless        StainlessStore
	maxCacheControls int
	envMasker        *promptEnvMasker // nil when masking disabled
}

type transformResult struct {
	Body        map[string]interface{}
	Headers     http.Header
	SessionHash string
}

func (t *claudeTransformer) transform(
	ctx context.Context,
	body map[string]interface{},
	reqHeaders http.Header,
	acct *domain.Account,
	brokerUserID string,
) (*transformResult, error) {
	result := &transformResult{
		Body:    body,
		Headers: filterClaudeHeaders(reqHeaders),
	}

	stripBillingHeaders(body)

	// Mask Claude-injected environment fingerprints (opt-in via PromptEnvHome)
	if t.envMasker != nil {
		t.envMasker.maskSystem(body)
		t.envMasker.maskMessageReminders(body)
	}

	enforceCacheControl(body, t.maxCacheControls)

	accountUUID := acct.IdentityString("account_uuid")
	sessionTail := "compat-" + brokerUserID
	syntheticUserID := buildUserID(acct.ID, accountUUID, sessionTail)

	metadata, hasMeta := body["metadata"].(map[string]interface{})
	if hasMeta {
		if origUserID, ok := metadata["user_id"].(string); ok {
			metadata["user_id"] = rewriteUserID(origUserID, acct.ID, accountUUID)
		} else {
			metadata["user_id"] = syntheticUserID
		}
	} else {
		body["metadata"] = map[string]interface{}{
			"user_id": syntheticUserID,
		}
	}

	result.SessionHash = computeSessionHashFromBody(body)

	removeAllStainless(result.Headers)
	if err := t.bindStainless(ctx, acct.ID, reqHeaders, result.Headers); err != nil {
		return nil, err
	}

	// When prompt env masking is active, override stainless OS/arch headers
	// to match the canonical prompt environment.  Without this, a Linux
	// client would carry x-stainless-os=Linux alongside Platform: darwin.
	if t.envMasker != nil {
		t.envMasker.overrideStainless(result.Headers)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Billing header strip
// ---------------------------------------------------------------------------

func stripBillingHeaders(body map[string]interface{}) {
	system, ok := body["system"]
	if !ok {
		return
	}

	switch s := system.(type) {
	case []interface{}:
		filtered := make([]interface{}, 0, len(s))
		for _, entry := range s {
			if m, ok := entry.(map[string]interface{}); ok {
				if text, ok := m["text"].(string); ok {
					if billingHeaderPattern.MatchString(text) {
						continue
					}
				}
			}
			filtered = append(filtered, entry)
		}
		body["system"] = filtered
	}
}

// ---------------------------------------------------------------------------
// Cache control enforcement
// ---------------------------------------------------------------------------

func enforceCacheControl(body map[string]interface{}, maxBlocks int) {
	total := 0
	total += stripAndCountCacheControl(body, "system")
	total += stripAndCountCacheControl(body, "messages")

	if total <= maxBlocks {
		return
	}

	excess := total - maxBlocks
	excess = removeCacheControls(body, "messages", excess)
	if excess > 0 {
		removeCacheControls(body, "system", excess)
	}
}

func stripAndCountCacheControl(body map[string]interface{}, field string) int {
	count := 0
	walkContentBlocks(body[field], func(block map[string]interface{}) {
		if cc, ok := block["cache_control"]; ok {
			count++
			if ccMap, ok := cc.(map[string]interface{}); ok {
				delete(ccMap, "ttl")
			}
		}
	})
	return count
}

func removeCacheControls(body map[string]interface{}, field string, toRemove int) int {
	if toRemove <= 0 {
		return 0
	}
	removed := 0
	walkContentBlocks(body[field], func(block map[string]interface{}) {
		if removed >= toRemove {
			return
		}
		if _, ok := block["cache_control"]; ok {
			delete(block, "cache_control")
			removed++
		}
	})
	return toRemove - removed
}

func walkContentBlocks(v interface{}, fn func(map[string]interface{})) {
	switch s := v.(type) {
	case []interface{}:
		for _, item := range s {
			if m, ok := item.(map[string]interface{}); ok {
				fn(m)
				if content, ok := m["content"]; ok {
					walkContentBlocks(content, fn)
				}
			}
		}
	case string:
		// System prompt as string — no cache_control to process
	}
}

// ---------------------------------------------------------------------------
// User ID rewrite
// ---------------------------------------------------------------------------

func rewriteUserID(originalUserID, accountID, accountUUID string) string {
	matches := userIDPattern.FindStringSubmatch(originalUserID)
	if len(matches) < 3 {
		return buildUserID(accountID, accountUUID, "default")
	}
	return buildUserID(accountID, accountUUID, matches[2])
}

func extractSessionUUID(userID string) string {
	matches := sessionUUIDPattern.FindStringSubmatch(userID)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func buildUserID(accountID, accountUUID, sessionTail string) string {
	accountHash := deriveAccountHash(accountUUID, accountID)
	stableSession := deriveSessionUUID(accountID, sessionTail)
	return fmt.Sprintf("user_%s_account__session_%s", accountHash, stableSession)
}

func deriveAccountHash(accountUUID, accountID string) string {
	source := accountUUID
	if source == "" {
		source = accountID
	}
	h := sha256.Sum256([]byte(source))
	return hex.EncodeToString(h[:])
}

func deriveSessionUUID(accountID, sessionTail string) string {
	h := sha256.Sum256([]byte(accountID + ":" + sessionTail))
	hx := hex.EncodeToString(h[:16])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[0:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}

// ---------------------------------------------------------------------------
// Session hash
// ---------------------------------------------------------------------------

func computeSessionHashFromBody(body map[string]interface{}) string {
	var userID, systemPrompt, firstMsg string

	if metadata, ok := body["metadata"].(map[string]interface{}); ok {
		userID, _ = metadata["user_id"].(string)
	}
	if sys, ok := body["system"].(string); ok {
		systemPrompt = sys
	} else if sysList, ok := body["system"].([]interface{}); ok && len(sysList) > 0 {
		if m, ok := sysList[0].(map[string]interface{}); ok {
			systemPrompt, _ = m["text"].(string)
		}
	}
	if msgs, ok := body["messages"].([]interface{}); ok && len(msgs) > 0 {
		if m, ok := msgs[0].(map[string]interface{}); ok {
			if content, ok := m["content"].(string); ok {
				firstMsg = content
			}
		}
	}

	return computeSessionHash(userID, systemPrompt, firstMsg)
}

func computeSessionHash(userID, systemPrompt, firstMessage string) string {
	if idx := strings.LastIndex(userID, "session_"); idx >= 0 {
		session := userID[idx:]
		h := sha256.Sum256([]byte("session:" + session))
		return hex.EncodeToString(h[:16])
	}
	if systemPrompt != "" {
		end := len(systemPrompt)
		if end > 200 {
			end = 200
		}
		h := sha256.Sum256([]byte("system:" + systemPrompt[:end]))
		return hex.EncodeToString(h[:16])
	}
	if firstMessage != "" {
		end := len(firstMessage)
		if end > 200 {
			end = 200
		}
		h := sha256.Sum256([]byte("msg:" + firstMessage[:end]))
		return hex.EncodeToString(h[:16])
	}
	return ""
}

// ---------------------------------------------------------------------------
// Prompt environment masking
// ---------------------------------------------------------------------------

// promptEnvMasker rewrites Claude-injected environment fingerprints in system
// prompts and <system-reminder> tags.  Activated only when PROMPT_ENV_HOME is
// set.
type promptEnvMasker struct {
	home    string // canonical home, e.g. "/Users/user"
	workDir string // canonical workdir, e.g. "/Users/user/project"

	// Canonical values for prompt and stainless consistency
	platform string // "darwin"
	shell    string // "/bin/zsh"
	osVer    string // "Darwin 25.4.0"
	stOS     string // x-stainless-os value: "Darwin"
	stArch   string // x-stainless-arch value: "arm64"

	// Scoped patterns — match only Claude Code's " - Field: value" injection
	// format.  Safe for system fields that may contain user-authored content.
	reScopedPlatform *regexp.Regexp
	reScopedShell    *regexp.Regexp
	reScopedOS       *regexp.Regexp
	reScopedWorkDir  *regexp.Regexp
	reScopedGitUser  *regexp.Regexp
	reScopedHome     *regexp.Regexp

	// Broad patterns — used only inside <system-reminder> tags where all
	// content is Claude-injected.
	reBroadPlatform *regexp.Regexp
	reBroadShell    *regexp.Regexp
	reBroadOS       *regexp.Regexp
	reBroadWorkDir  *regexp.Regexp
	reBroadGitUser  *regexp.Regexp
	reBroadHome     *regexp.Regexp

	reReminder *regexp.Regexp
}

func newPromptEnvMasker(home string) *promptEnvMasker {
	return &promptEnvMasker{
		home:     home,
		workDir:  home + "/project",
		platform: "darwin",
		shell:    "/bin/zsh",
		osVer:    "Darwin 25.4.0",
		stOS:     "Darwin",
		stArch:   "arm64",

		// Scoped: require " - " prefix (Claude Code env block format)
		reScopedPlatform: regexp.MustCompile(`( - Platform:\s*)\S+`),
		reScopedShell:    regexp.MustCompile(`( - Shell:\s*)\S+`),
		reScopedOS:       regexp.MustCompile(`( - OS Version:\s*)[^\n]+`),
		reScopedWorkDir:  regexp.MustCompile(`( - (?:Primary )?[Ww]orking directory:\s*)/\S+`),
		reScopedGitUser:  regexp.MustCompile(`( - Git user:\s*)[^\n]+`),
		reScopedHome:     regexp.MustCompile(`( - (?:Primary )?[Ww]orking directory:\s*)/(?:Users|home)/[^/\s"')]+/`),

		// Broad: match anywhere (only applied inside <system-reminder>)
		reBroadPlatform: regexp.MustCompile(`(Platform:\s*)\S+`),
		reBroadShell:    regexp.MustCompile(`(Shell:\s*)\S+`),
		reBroadOS:       regexp.MustCompile(`(OS Version:\s*)[^\n<]+`),
		reBroadWorkDir:  regexp.MustCompile(`((?:Primary )?[Ww]orking directory:\s*)/\S+`),
		reBroadGitUser:  regexp.MustCompile(`(Git user:\s*)[^\n<]+`),
		reBroadHome:     regexp.MustCompile(`/(?:Users|home)/[^/\s"')]+/`),

		reReminder: regexp.MustCompile(`(<system-reminder>)([\s\S]*?)(</system-reminder>)`),
	}
}

// rewriteEnvBlock rewrites only Claude Code's " - Field: value" environment
// injection lines.  Safe for system fields that may mix injected and
// user-authored content.
func (m *promptEnvMasker) rewriteEnvBlock(text string) string {
	text = m.reScopedPlatform.ReplaceAllString(text, "${1}"+m.platform)
	text = m.reScopedShell.ReplaceAllString(text, "${1}"+m.shell)
	text = m.reScopedOS.ReplaceAllString(text, "${1}"+m.osVer)
	// Replace home path in working directory lines first (more specific)
	text = m.reScopedHome.ReplaceAllString(text, "${1}"+m.home+"/")
	text = m.reScopedWorkDir.ReplaceAllString(text, "${1}"+m.workDir)
	text = m.reScopedGitUser.ReplaceAllString(text, "${1}User")
	return text
}

// rewriteReminderText rewrites environment info broadly.  Only used inside
// <system-reminder> tags where all content is Claude-injected.
func (m *promptEnvMasker) rewriteReminderText(text string) string {
	text = m.reBroadPlatform.ReplaceAllString(text, "${1}"+m.platform)
	text = m.reBroadShell.ReplaceAllString(text, "${1}"+m.shell)
	text = m.reBroadOS.ReplaceAllString(text, "${1}"+m.osVer)
	text = m.reBroadWorkDir.ReplaceAllString(text, "${1}"+m.workDir)
	text = m.reBroadHome.ReplaceAllString(text, m.home+"/")
	text = m.reBroadGitUser.ReplaceAllString(text, "${1}User")
	return text
}

// overrideStainless forces OS/arch stainless headers to match the canonical
// prompt environment, preventing a Linux stainless fingerprint paired with a
// Darwin prompt fingerprint.
func (m *promptEnvMasker) overrideStainless(h http.Header) {
	if m.stOS != "" {
		h.Set("x-stainless-os", m.stOS)
	}
	if m.stArch != "" {
		h.Set("x-stainless-arch", m.stArch)
	}
}

// maskSystem rewrites only Claude Code's " - Field: value" environment lines
// in the system field.  User-authored text (e.g. compat system messages) that
// happens to contain "Platform:" without the " - " prefix is not touched.
func (m *promptEnvMasker) maskSystem(body map[string]interface{}) {
	system, ok := body["system"]
	if !ok {
		return
	}
	switch s := system.(type) {
	case string:
		body["system"] = m.rewriteEnvBlock(s)
	case []interface{}:
		for i, entry := range s {
			switch e := entry.(type) {
			case string:
				s[i] = m.rewriteEnvBlock(e)
			case map[string]interface{}:
				if text, ok := e["text"].(string); ok {
					e["text"] = m.rewriteEnvBlock(text)
				}
			}
		}
	}
}

// maskMessageReminders rewrites environment info only inside <system-reminder>
// tags in messages, leaving user text untouched.
func (m *promptEnvMasker) maskMessageReminders(body map[string]interface{}) {
	messages, ok := body["messages"].([]interface{})
	if !ok {
		return
	}
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		switch content := msgMap["content"].(type) {
		case string:
			msgMap["content"] = m.reReminder.ReplaceAllStringFunc(content, func(match string) string {
				parts := m.reReminder.FindStringSubmatch(match)
				if len(parts) < 4 {
					return match
				}
				return parts[1] + m.rewriteReminderText(parts[2]) + parts[3]
			})
		case []interface{}:
			for _, block := range content {
				if b, ok := block.(map[string]interface{}); ok {
					if text, ok := b["text"].(string); ok {
						b["text"] = m.reReminder.ReplaceAllStringFunc(text, func(match string) string {
							parts := m.reReminder.FindStringSubmatch(match)
							if len(parts) < 4 {
								return match
							}
							return parts[1] + m.rewriteReminderText(parts[2]) + parts[3]
						})
					}
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Warmup detection & response
// ---------------------------------------------------------------------------

func isWarmupRequest(body map[string]interface{}) bool {
	if messages, ok := body["messages"].([]interface{}); ok && len(messages) == 1 {
		if m, ok := messages[0].(map[string]interface{}); ok {
			if content, ok := m["content"].(string); ok && content == "Warmup" {
				return true
			}
			if content, ok := m["content"].([]interface{}); ok && len(content) == 1 {
				if block, ok := content[0].(map[string]interface{}); ok {
					if text, ok := block["text"].(string); ok && text == "Warmup" {
						return true
					}
				}
			}
		}
	}

	systemText := extractSystemText(body)
	if strings.Contains(systemText, "Please write a 5-10 word title") {
		return true
	}
	if strings.Contains(systemText, "nalyze if this message indicates a new conversation topic") {
		return true
	}

	return false
}

func extractSystemText(body map[string]interface{}) string {
	switch s := body["system"].(type) {
	case string:
		return s
	case []interface{}:
		var texts []string
		for _, entry := range s {
			if m, ok := entry.(map[string]interface{}); ok {
				if text, ok := m["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, " ")
	}
	return ""
}

func warmupEvents(model string) []string {
	id := "msg_warmup_" + generateShortID()
	return []string{
		`event: message_start` + "\n" + `data: {"type":"message_start","message":{"id":"` + id + `","type":"message","role":"assistant","content":[],"model":"` + model + `","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":5,"output_tokens":1}}}` + "\n\n",
		`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n",
		`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"OK"}}` + "\n\n",
		`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":0}` + "\n\n",
		`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":1}}` + "\n\n",
		`event: message_stop` + "\n" + `data: {"type":"message_stop"}` + "\n\n",
	}
}

func generateShortID() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(h[:8])
}

// ---------------------------------------------------------------------------
// Stainless header binding (capture/replay)
// ---------------------------------------------------------------------------

var boundStainlessKeys = []string{
	"x-stainless-os",
	"x-stainless-arch",
	"x-stainless-runtime",
	"x-stainless-runtime-version",
	"x-stainless-lang",
	"x-stainless-package-version",
}

var passthroughStainlessKeys = []string{
	"x-stainless-retry-count",
	"x-stainless-read-timeout",
}

func (t *claudeTransformer) bindStainless(ctx context.Context, accountID string, reqHeaders, outHeaders http.Header) error {
	stored, ok, err := t.stainless.GetStainless(ctx, accountID)
	if err != nil {
		return err
	}

	if ok {
		var headers map[string]string
		if json.Unmarshal([]byte(stored), &headers) == nil {
			for k, v := range headers {
				outHeaders.Set(k, v)
			}
		}
	} else {
		captured := make(map[string]string)
		for _, key := range boundStainlessKeys {
			if v := reqHeaders.Get(key); v != "" {
				captured[key] = v
				outHeaders.Set(key, v)
			}
		}
		if len(captured) > 0 {
			data, _ := json.Marshal(captured)
			won, err := t.stainless.SetStainlessNX(ctx, accountID, string(data), 24*time.Hour)
			if err != nil {
				return err
			}
			if !won {
				if reread, ok, err := t.stainless.GetStainless(ctx, accountID); err != nil {
					return err
				} else if ok {
					var headers map[string]string
					if json.Unmarshal([]byte(reread), &headers) == nil {
						for k, v := range headers {
							outHeaders.Set(k, v)
						}
					}
				}
			}
		}
	}

	for _, key := range passthroughStainlessKeys {
		if v := reqHeaders.Get(key); v != "" {
			outHeaders.Set(key, v)
		}
	}
	return nil
}
