package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"

	"github.com/yansircc/llm-broker/internal/domain"
)

var billingHeaderPattern = regexp.MustCompile(`(?i)x-anthropic-billing-header`)

// StainlessBinder captures and replays stainless headers per account.
type StainlessBinder interface {
	BindStainlessFromRequest(ctx context.Context, accountID string, reqHeaders http.Header, outHeaders http.Header) error
}

// Transformer applies all identity transformations to a request.
type Transformer struct {
	stainless        StainlessBinder
	maxCacheControls int
}

func NewTransformer(sb StainlessBinder, maxCacheControls int) *Transformer {
	return &Transformer{stainless: sb, maxCacheControls: maxCacheControls}
}

// TransformResult holds the results of a transformation.
type TransformResult struct {
	Body        map[string]interface{}
	Headers     http.Header
	SessionHash string
}

// Transform applies all identity transformations to a request.
func (t *Transformer) Transform(
	ctx context.Context,
	body map[string]interface{},
	reqHeaders http.Header,
	acct *domain.Account,
	brokerUserID string,
) (*TransformResult, error) {
	result := &TransformResult{
		Body:    body,
		Headers: FilterHeaders(reqHeaders),
	}

	// 1. Strip billing headers from system prompt
	t.stripBillingHeaders(body)

	// 2. Enforce cache_control compliance (max N blocks, strip TTL)
	t.enforceCacheControl(body)

	// 3. Rewrite or inject metadata.user_id
	accountUUID := acct.IdentityString("account_uuid")
	metadata, hasMeta := body["metadata"].(map[string]interface{})
	if hasMeta {
		if origUserID, ok := metadata["user_id"].(string); ok {
			metadata["user_id"] = RewriteUserID(origUserID, acct.ID, accountUUID)
		}
	} else {
		// Inject synthetic Claude Code identity for non-native clients.
		// Session tail is keyed on (account, broker_user) so each broker
		// user gets a stable, unique session per Claude account.
		sessionTail := "compat-" + brokerUserID
		body["metadata"] = map[string]interface{}{
			"user_id": buildUserID(acct.ID, accountUUID, sessionTail),
		}
	}

	// 4. Compute session hash
	result.SessionHash = t.computeSessionHashFromBody(body)

	// 5. Bind/restore stainless headers
	RemoveAllStainless(result.Headers)
	if err := t.stainless.BindStainlessFromRequest(ctx, acct.ID, reqHeaders, result.Headers); err != nil {
		return nil, err
	}

	return result, nil
}

func (t *Transformer) stripBillingHeaders(body map[string]interface{}) {
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

func (t *Transformer) enforceCacheControl(body map[string]interface{}) {
	maxBlocks := t.maxCacheControls

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

func (t *Transformer) computeSessionHashFromBody(body map[string]interface{}) string {
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
