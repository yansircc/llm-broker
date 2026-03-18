package driver

import "strings"

const claudeCodeSystemBlockText = "You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK."

func normalizeClaudeMessageEnvelope(body map[string]interface{}) {
	if !claudeNeedsCodeSystemEnvelope(body) {
		return
	}
	body["system"] = normalizeClaudeSystemValue(body["system"])
}

func claudeNeedsCodeSystemEnvelope(body map[string]interface{}) bool {
	model, _ := body["model"].(string)
	model = strings.ToLower(model)
	if model == "" {
		return false
	}
	return strings.Contains(model, "claude-sonnet-4") || strings.Contains(model, "claude-opus-4")
}

func normalizeClaudeSystemValue(system interface{}) []interface{} {
	switch s := system.(type) {
	case nil:
		return []interface{}{claudeCodeSystemBlock()}
	case string:
		if strings.TrimSpace(s) == "" {
			return []interface{}{claudeCodeSystemBlock()}
		}
		if strings.Contains(s, claudeCodeSystemSignatureNeedle()) {
			return []interface{}{claudeSystemTextBlock(s)}
		}
		return []interface{}{
			claudeCodeSystemBlock(),
			claudeSystemTextBlock(s),
		}
	case []interface{}:
		if len(s) > 0 && claudeSystemBlockHasCodeSignature(s[0]) {
			claudeEnsureEphemeralCacheControl(s[0])
			return s
		}
		return append([]interface{}{claudeCodeSystemBlock()}, s...)
	default:
		return []interface{}{claudeCodeSystemBlock()}
	}
}

func claudeCodeSystemSignatureNeedle() string {
	return "You are Claude Code, Anthropic's official CLI for Claude"
}

func claudeCodeSystemBlock() map[string]interface{} {
	return map[string]interface{}{
		"type": "text",
		"text": claudeCodeSystemBlockText,
		"cache_control": map[string]interface{}{
			"type": "ephemeral",
		},
	}
}

func claudeSystemTextBlock(text string) map[string]interface{} {
	block := map[string]interface{}{
		"type": "text",
		"text": text,
	}
	if strings.TrimSpace(text) != "" {
		block["cache_control"] = map[string]interface{}{
			"type": "ephemeral",
		}
	}
	return block
}

func claudeSystemBlockHasCodeSignature(v interface{}) bool {
	block, ok := v.(map[string]interface{})
	if !ok {
		return false
	}
	text, _ := block["text"].(string)
	return strings.Contains(text, claudeCodeSystemSignatureNeedle())
}

func claudeEnsureEphemeralCacheControl(v interface{}) {
	block, ok := v.(map[string]interface{})
	if !ok {
		return
	}
	if _, ok := block["cache_control"]; ok {
		return
	}
	block["cache_control"] = map[string]interface{}{
		"type": "ephemeral",
	}
}
