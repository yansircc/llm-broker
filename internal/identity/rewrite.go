package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
)

var userIDPattern = regexp.MustCompile(`^user_([a-fA-F0-9]{64})_account__session_([\w-]+)$`)
var sessionUUIDPattern = regexp.MustCompile(`session_([a-f0-9-]{36})$`)

// claudeCodeUserID is the JSON structure used by Claude Code clients.
// Both device_id and session_id must be present to distinguish from
// arbitrary user-provided JSON in metadata.user_id.
type claudeCodeUserID struct {
	DeviceID  string `json:"device_id"`
	SessionID string `json:"session_id"`
}

func parseClaudeCodeUserID(raw string) (claudeCodeUserID, bool) {
	var parsed claudeCodeUserID
	if json.Unmarshal([]byte(raw), &parsed) == nil && parsed.DeviceID != "" && parsed.SessionID != "" {
		return parsed, true
	}
	return claudeCodeUserID{}, false
}

// RewriteUserID replaces the user_id to match the account's real identity
// while maintaining session consistency. Handles both the relay's own format
// (user_{hash}_account__session_{tail}) and Claude Code's JSON format
// ({"device_id":"...","session_id":"...","account_uuid":"..."}).
func RewriteUserID(originalUserID, accountID, accountUUID string) string {
	// Try relay format first
	if matches := userIDPattern.FindStringSubmatch(originalUserID); len(matches) >= 3 {
		return buildUserID(accountID, accountUUID, matches[2])
	}
	// Try Claude Code JSON format (requires both device_id and session_id)
	if parsed, ok := parseClaudeCodeUserID(originalUserID); ok {
		return buildUserID(accountID, accountUUID, parsed.SessionID)
	}
	return buildUserID(accountID, accountUUID, "default")
}

// ExtractSessionUUID extracts the session UUID from a user_id string.
// Handles both relay format and Claude Code JSON format.
func ExtractSessionUUID(userID string) string {
	// Try relay format
	if matches := sessionUUIDPattern.FindStringSubmatch(userID); len(matches) >= 2 {
		return matches[1]
	}
	// Try Claude Code JSON format (requires both device_id and session_id)
	if parsed, ok := parseClaudeCodeUserID(userID); ok {
		return parsed.SessionID
	}
	return ""
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
