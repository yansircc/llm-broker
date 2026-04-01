package server

import (
	"encoding/json"
	"sort"
	"strings"
)

func buildCompatClientMeta(rawBody []byte) string {
	if len(rawBody) == 0 {
		return ""
	}

	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil || len(body) == 0 {
		return ""
	}

	meta := map[string]any{}
	if keys := compatObservationKeys(body); len(keys) > 0 {
		meta["top_level_keys"] = keys
		meta["top_level_key_count"] = len(keys)
	}
	if model := compatString(body["model"]); model != "" {
		meta["requested_model"] = model
	}
	if stream, ok := body["stream"].(bool); ok {
		meta["stream"] = stream
	}
	if messages, ok := body["messages"].([]any); ok {
		meta["message_count"] = len(messages)
		summarizeCompatMessages(messages, meta)
	}
	if tools, ok := body["tools"].([]any); ok {
		meta["tools_count"] = len(tools)
		if len(tools) > 0 {
			meta["has_tools"] = true
		}
	} else if compatHasValue(body["tools"]) {
		meta["has_tools"] = true
	}
	if toolChoiceType := compatChoiceType(body["tool_choice"]); toolChoiceType != "" {
		meta["has_tool_choice"] = true
		meta["tool_choice_type"] = toolChoiceType
	}
	if responseFormatType := compatFormatType(body["response_format"]); responseFormatType != "" {
		meta["response_format_type"] = responseFormatType
	}
	if stopCount := compatStopCount(body["stop"]); stopCount > 0 {
		meta["stop_count"] = stopCount
	}
	if maxTokens, ok := compatNumber(body["max_tokens"]); ok {
		meta["max_tokens"] = maxTokens
	}
	if maxCompletionTokens, ok := compatNumber(body["max_completion_tokens"]); ok {
		meta["max_completion_tokens"] = maxCompletionTokens
	}
	if temperature, ok := compatFloat(body["temperature"]); ok {
		meta["temperature"] = temperature
	}
	if topP, ok := compatFloat(body["top_p"]); ok {
		meta["top_p"] = topP
	}
	if presencePenalty, ok := compatFloat(body["presence_penalty"]); ok {
		meta["presence_penalty"] = presencePenalty
	}
	if frequencyPenalty, ok := compatFloat(body["frequency_penalty"]); ok {
		meta["frequency_penalty"] = frequencyPenalty
	}
	if n, ok := compatNumber(body["n"]); ok {
		meta["choice_count"] = n
	}
	if seed, ok := compatNumber(body["seed"]); ok {
		meta["seed"] = seed
	}
	if parallelToolCalls, ok := body["parallel_tool_calls"].(bool); ok {
		meta["parallel_tool_calls"] = parallelToolCalls
	}
	if store, ok := body["store"].(bool); ok {
		meta["store"] = store
	}
	if serviceTier := compatString(body["service_tier"]); serviceTier != "" {
		meta["service_tier"] = serviceTier
	}
	if reasoningEffort := compatString(body["reasoning_effort"]); reasoningEffort != "" {
		meta["reasoning_effort"] = reasoningEffort
	}
	if modalities, ok := body["modalities"].([]any); ok && len(modalities) > 0 {
		meta["modalities"] = compactStringSlice(modalities)
	}
	if audio, ok := body["audio"].(map[string]any); ok && len(audio) > 0 {
		if format := compatString(audio["format"]); format != "" {
			meta["audio_format"] = format
		}
		if voice := compatString(audio["voice"]); voice != "" {
			meta["audio_voice"] = voice
		}
	}
	if metadata, ok := body["metadata"].(map[string]any); ok && len(metadata) > 0 {
		if keys := compatObservationKeys(metadata); len(keys) > 0 {
			meta["metadata_keys"] = keys
			meta["metadata_key_count"] = len(keys)
		}
	}

	if len(meta) == 0 {
		return ""
	}
	data, err := json.Marshal(meta)
	if err != nil || string(data) == "{}" {
		return ""
	}
	return string(data)
}

func summarizeCompatMessages(messages []any, meta map[string]any) {
	roleCounts := map[string]int{}
	contentTypes := map[string]int{}
	contentPartCount := 0
	lastRole := ""

	for _, item := range messages {
		message, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := compatString(message["role"])
		role = strings.ToLower(role)
		if role != "" {
			roleCounts[role]++
			lastRole = role
		}
		switch content := message["content"].(type) {
		case string:
			if strings.TrimSpace(content) != "" {
				contentPartCount++
				contentTypes["text"]++
			}
		case []any:
			contentPartCount += len(content)
			compatAccumulateTypedItems(content, contentTypes)
		}
	}

	for role, count := range roleCounts {
		meta[role+"_messages"] = count
	}
	if contentPartCount > 0 {
		meta["content_part_count"] = contentPartCount
	}
	if len(contentTypes) > 0 {
		meta["message_content_types"] = contentTypes
	}
	if lastRole == "assistant" {
		meta["has_assistant_prefill"] = true
	}
}

func compatAccumulateTypedItems(items []any, counts map[string]int) {
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]any:
			if kind := compatString(typed["type"]); kind != "" {
				counts[strings.ToLower(kind)]++
			} else {
				counts["object"]++
			}
		case string:
			if strings.TrimSpace(typed) != "" {
				counts["text"]++
			}
		default:
			counts["present"]++
		}
	}
}

func compatObservationKeys(body map[string]any) []string {
	keys := make([]string, 0, len(body))
	for key, value := range body {
		if !compatHasValue(value) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return nil
	}
	return keys
}

func compatHasValue(value any) bool {
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

func compatString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func compatChoiceType(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(typed))
	case map[string]any:
		if kind := compatString(typed["type"]); kind != "" {
			return strings.ToLower(kind)
		}
		if len(typed) > 0 {
			return "object"
		}
	}
	return ""
}

func compatFormatType(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(typed))
	case map[string]any:
		if kind := compatString(typed["type"]); kind != "" {
			return strings.ToLower(kind)
		}
		if len(typed) > 0 {
			return "object"
		}
	}
	return ""
}

func compatStopCount(value any) int {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0
		}
		return 1
	case []any:
		return len(typed)
	default:
		return 0
	}
}

func compatNumber(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func compatFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	default:
		return 0, false
	}
}

func compactStringSlice(items []any) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := compatString(item); text != "" {
			result = append(result, text)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
