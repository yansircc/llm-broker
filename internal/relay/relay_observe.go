package relay

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const compatClientMetaHeader = "X-Broker-Compat-Client-Meta"

func requestBindingSource(prepared *preparedRelayRequest) string {
	if prepared == nil || prepared.keyInfo == nil {
		return ""
	}
	if prepared.keyInfo.BoundAccountID != "" {
		return "user_bound"
	}
	if prepared.sessionBoundAccountID != "" {
		return "session_bound"
	}
	return "none"
}

func requestClientHeaders(headers http.Header) json.RawMessage {
	return marshalObservationMapString(traceRequestHeaders(headers))
}

func requestMeta(prepared *preparedRelayRequest) json.RawMessage {
	if prepared == nil || prepared.input == nil {
		return nil
	}

	meta := map[string]any{
		"stream": prepared.input.IsStream,
	}
	body := prepared.input.Body
	if keys := observationKeys(body); len(keys) > 0 {
		meta["top_level_keys"] = keys
		meta["top_level_key_count"] = len(keys)
	}
	if prepared.input.IsCountTokens {
		meta["is_count_tokens"] = true
	}
	if prepared.input.RawQuery != "" {
		meta["raw_query"] = prepared.input.RawQuery
	}
	if traceID := strings.TrimSpace(prepared.input.Headers.Get("X-Broker-Compat-Trace-Id")); traceID != "" {
		meta["compat_trace_id"] = traceID
	}
	if retryCount := strings.TrimSpace(prepared.input.Headers.Get("X-Stainless-Retry-Count")); retryCount != "" {
		if n, err := strconv.Atoi(retryCount); err == nil {
			meta["client_retry_count"] = n
		} else {
			meta["client_retry_count"] = retryCount
		}
	}
	if compatClient := compatClientMeta(prepared.input.Headers); len(compatClient) > 0 {
		meta["compat_client"] = compatClient
	}

	if len(body) == 0 {
		return marshalObservationMap(meta)
	}

	if messages, ok := body["messages"].([]interface{}); ok {
		meta["message_count"] = len(messages)
		summarizeMessages(messages, meta)
	}
	if contents, ok := body["contents"].([]interface{}); ok {
		meta["content_count"] = len(contents)
		summarizeContents(contents, meta)
	}
	if inputItems, ok := body["input"].([]interface{}); ok {
		meta["input_count"] = len(inputItems)
		summarizeInputItems(inputItems, meta)
	}
	if hasValue(body["system"]) {
		meta["has_system"] = true
	}
	if hasValue(body["instructions"]) {
		meta["has_instructions"] = true
	}
	if tools, ok := body["tools"].([]interface{}); ok {
		meta["tools_count"] = len(tools)
		if len(tools) > 0 {
			meta["has_tools"] = true
		}
	} else if hasValue(body["tools"]) {
		meta["has_tools"] = true
	}
	if choiceType := requestChoiceType(body["tool_choice"]); choiceType != "" {
		meta["has_tool_choice"] = true
		meta["tool_choice_type"] = choiceType
	}
	if responseFormatType := requestFormatType(body["response_format"]); responseFormatType != "" {
		meta["response_format_type"] = responseFormatType
	}
	if outputFormatType := outputConfigFormatType(body["output_config"]); outputFormatType != "" {
		meta["output_format_type"] = outputFormatType
	}
	if stopCount := requestStopCount(body["stop"]); stopCount > 0 {
		meta["stop_count"] = stopCount
	}
	if stopSequenceCount := requestStopCount(body["stop_sequences"]); stopSequenceCount > 0 {
		meta["stop_sequence_count"] = stopSequenceCount
	}
	if maxTokens, ok := requestNumber(body["max_tokens"]); ok {
		meta["max_tokens"] = maxTokens
	}
	if maxCompletionTokens, ok := requestNumber(body["max_completion_tokens"]); ok {
		meta["max_completion_tokens"] = maxCompletionTokens
	}
	if temperature, ok := requestFloat(body["temperature"]); ok {
		meta["temperature"] = temperature
	}
	if topP, ok := requestFloat(body["top_p"]); ok {
		meta["top_p"] = topP
	}
	if parallelToolCalls, ok := body["parallel_tool_calls"].(bool); ok {
		meta["parallel_tool_calls"] = parallelToolCalls
	}
	return marshalObservationMap(meta)
}

func summarizeMessages(messages []interface{}, meta map[string]any) {
	roleCounts := map[string]int{}
	contentBlocks := 0
	lastRole := ""
	contentTypes := map[string]int{}

	for _, item := range messages {
		message, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := message["role"].(string)
		role = strings.TrimSpace(strings.ToLower(role))
		if role != "" {
			roleCounts[role]++
			lastRole = role
		}
		switch content := message["content"].(type) {
		case []interface{}:
			contentBlocks += len(content)
			accumulateTypedItems(content, contentTypes)
		case string:
			if strings.TrimSpace(content) != "" {
				contentBlocks++
				contentTypes["text"]++
			}
		}
	}

	for role, count := range roleCounts {
		meta[role+"_messages"] = count
	}
	if contentBlocks > 0 {
		meta["content_block_count"] = contentBlocks
	}
	if len(contentTypes) > 0 {
		meta["message_content_types"] = contentTypes
	}
	if lastRole == "assistant" {
		meta["has_assistant_prefill"] = true
	}
}

func summarizeContents(contents []interface{}, meta map[string]any) {
	roleCounts := map[string]int{}
	partTypes := map[string]int{}
	partCount := 0

	for _, item := range contents {
		content, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := content["role"].(string)
		role = strings.TrimSpace(strings.ToLower(role))
		if role != "" {
			roleCounts[role]++
		}
		parts, ok := content["parts"].([]interface{})
		if !ok {
			continue
		}
		partCount += len(parts)
		accumulateTypedItems(parts, partTypes)
	}

	for role, count := range roleCounts {
		meta["contents_"+role] = count
	}
	if partCount > 0 {
		meta["content_part_count"] = partCount
	}
	if len(partTypes) > 0 {
		meta["content_part_types"] = partTypes
	}
}

func summarizeInputItems(items []interface{}, meta map[string]any) {
	itemTypes := map[string]int{}
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]interface{}:
			if kind, _ := typed["type"].(string); strings.TrimSpace(kind) != "" {
				itemTypes[strings.TrimSpace(strings.ToLower(kind))]++
				continue
			}
			itemTypes["object"]++
		case string:
			if strings.TrimSpace(typed) != "" {
				itemTypes["text"]++
			}
		default:
			itemTypes["present"]++
		}
	}
	if len(itemTypes) > 0 {
		meta["input_item_types"] = itemTypes
	}
}

func accumulateTypedItems(items []interface{}, counts map[string]int) {
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]interface{}:
			if kind, _ := typed["type"].(string); strings.TrimSpace(kind) != "" {
				counts[strings.TrimSpace(strings.ToLower(kind))]++
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

func observationKeys(body map[string]interface{}) []string {
	if len(body) == 0 {
		return nil
	}
	keys := make([]string, 0, len(body))
	for key, value := range body {
		if !hasValue(value) {
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

func compatClientMeta(headers http.Header) map[string]any {
	if headers == nil {
		return nil
	}
	raw := strings.TrimSpace(headers.Get(compatClientMetaHeader))
	if raw == "" {
		return nil
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(raw), &meta); err != nil || len(meta) == 0 {
		return nil
	}
	return meta
}

func requestChoiceType(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(strings.ToLower(v))
	case map[string]interface{}:
		if kind, _ := v["type"].(string); strings.TrimSpace(kind) != "" {
			return strings.TrimSpace(strings.ToLower(kind))
		}
		return "object"
	default:
		if hasValue(v) {
			return "present"
		}
		return ""
	}
}

func requestFormatType(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(strings.ToLower(v))
	case map[string]interface{}:
		if kind, _ := v["type"].(string); strings.TrimSpace(kind) != "" {
			return strings.TrimSpace(strings.ToLower(kind))
		}
		return "object"
	default:
		return ""
	}
}

func outputConfigFormatType(value any) string {
	cfg, ok := value.(map[string]interface{})
	if !ok {
		return ""
	}
	return requestFormatType(cfg["format"])
}

func requestStopCount(value any) int {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return 0
		}
		return 1
	case []interface{}:
		return len(v)
	default:
		return 0
	}
}

func requestNumber(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

func requestFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

func hasValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	default:
		return true
	}
}

func marshalObservationMap(value map[string]any) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) == "{}" {
		return nil
	}
	return json.RawMessage(data)
}

func marshalObservationMapString(value map[string]string) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) == "{}" {
		return nil
	}
	return json.RawMessage(data)
}
