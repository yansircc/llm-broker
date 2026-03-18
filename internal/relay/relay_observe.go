package relay

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const compatClientMetaHeader = "X-Broker-Compat-Client-Meta"
const requestLogBodyExcerptLimit = 8 << 10

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
	if prepared.userRouteAccountID != "" {
		return "user_sticky"
	}
	return "none"
}

func requestClientHeaders(headers http.Header) json.RawMessage {
	return marshalObservationMapString(traceRequestHeaders(headers))
}

func requestBodyExcerpt(body []byte) string {
	text, _ := formatObservationBody(body)
	return text
}

func requestLogPath(prepared *preparedRelayRequest) string {
	if prepared == nil {
		return ""
	}
	if obs := prepared.clientObservation; obs != nil && strings.TrimSpace(obs.Path) != "" {
		return obs.Path
	}
	if prepared.input == nil {
		return ""
	}
	return prepared.input.Path
}

func requestLogRawQuery(prepared *preparedRelayRequest) string {
	if prepared == nil {
		return ""
	}
	if obs := prepared.clientObservation; obs != nil && strings.TrimSpace(obs.RawQuery) != "" {
		return obs.RawQuery
	}
	if prepared.input == nil {
		return ""
	}
	return prepared.input.RawQuery
}

func requestLogClientHeaders(prepared *preparedRelayRequest) http.Header {
	if prepared == nil {
		return nil
	}
	if obs := prepared.clientObservation; obs != nil && len(obs.Headers) > 0 {
		return obs.Headers
	}
	if prepared.input == nil {
		return nil
	}
	return prepared.input.Headers
}

func requestLogClientBody(prepared *preparedRelayRequest) []byte {
	if prepared == nil {
		return nil
	}
	if obs := prepared.clientObservation; obs != nil && len(obs.Body) > 0 {
		return obs.Body
	}
	if prepared.input == nil {
		return nil
	}
	return prepared.input.RawBody
}

func requestLogClientBodyObject(prepared *preparedRelayRequest) map[string]interface{} {
	if prepared == nil || prepared.input == nil {
		return nil
	}
	if obs := prepared.clientObservation; obs != nil && len(obs.Body) > 0 {
		return observationObject(obs.Body)
	}
	return prepared.input.Body
}

func requestMeta(prepared *preparedRelayRequest) json.RawMessage {
	if prepared == nil || prepared.input == nil {
		return nil
	}

	body := requestLogClientBodyObject(prepared)
	rawBody := requestLogClientBody(prepared)
	meta := map[string]any{
		"stream": prepared.input.IsStream,
	}
	if keys := observationKeys(body); len(keys) > 0 {
		meta["top_level_keys"] = keys
		meta["top_level_key_count"] = len(keys)
	}
	if prepared.input.IsCountTokens {
		meta["is_count_tokens"] = true
	}
	if rawQuery := requestLogRawQuery(prepared); rawQuery != "" {
		meta["raw_query"] = rawQuery
	}
	if traceID := strings.TrimSpace(prepared.input.Headers.Get("X-Broker-Compat-Trace-Id")); traceID != "" {
		meta["compat_trace_id"] = traceID
	}
	if retryCount := strings.TrimSpace(requestLogClientHeaders(prepared).Get("X-Stainless-Retry-Count")); retryCount != "" {
		if n, err := strconv.Atoi(retryCount); err == nil {
			meta["client_retry_count"] = n
		} else {
			meta["client_retry_count"] = retryCount
		}
	}
	if compatClient := compatClientMeta(prepared.input.Headers); len(compatClient) > 0 {
		meta["compat_client"] = compatClient
	}

	summarizeRequestBody(body, meta)
	if bodyHash := observationRawBodyHash(rawBody); bodyHash != "" {
		meta["body_sha256"] = bodyHash
	}
	if _, truncated := formatObservationBody(rawBody); truncated {
		meta["body_excerpt_truncated"] = true
	}
	return marshalObservationMap(meta)
}

func upstreamRequestMeta(req *http.Request, body []byte) json.RawMessage {
	if req == nil {
		return nil
	}
	meta := map[string]any{
		"method": req.Method,
	}
	if req.URL != nil && strings.TrimSpace(req.URL.RawQuery) != "" {
		meta["raw_query"] = req.URL.RawQuery
	}
	if contentType := strings.TrimSpace(req.Header.Get("Content-Type")); contentType != "" {
		meta["content_type"] = contentType
	}
	if accept := strings.TrimSpace(req.Header.Get("Accept")); accept != "" {
		meta["accept"] = accept
	}
	if req.ContentLength > 0 {
		meta["content_length"] = req.ContentLength
	}
	if len(body) > 0 {
		meta["body_bytes"] = len(body)
	}
	summarizeRequestBody(observationObject(body), meta)
	if bodyHash := observationRawBodyHash(body); bodyHash != "" {
		meta["body_sha256"] = bodyHash
	}
	if _, truncated := formatObservationBody(body); truncated {
		meta["body_excerpt_truncated"] = true
	}
	return marshalObservationMap(meta)
}

func upstreamResponseMeta(resp *http.Response, body []byte) json.RawMessage {
	if resp == nil {
		return nil
	}
	meta := map[string]any{
		"status": resp.StatusCode,
	}
	if contentType := strings.TrimSpace(resp.Header.Get("Content-Type")); contentType != "" {
		meta["content_type"] = contentType
	}
	if retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After")); retryAfter != "" {
		meta["retry_after"] = retryAfter
	}
	if resp.ContentLength >= 0 {
		meta["content_length"] = resp.ContentLength
	}
	if len(body) > 0 {
		meta["body_bytes"] = len(body)
	}
	if bodyHash := observationRawBodyHash(body); bodyHash != "" {
		meta["body_sha256"] = bodyHash
	}
	if _, truncated := formatObservationBody(body); truncated {
		meta["body_excerpt_truncated"] = true
	}
	bodyObj := observationObject(body)
	if keys := observationKeys(bodyObj); len(keys) > 0 {
		meta["top_level_keys"] = keys
		meta["top_level_key_count"] = len(keys)
	}
	if bodyType, _ := bodyObj["type"].(string); strings.TrimSpace(bodyType) != "" {
		meta["type"] = strings.TrimSpace(strings.ToLower(bodyType))
	}
	if hasValue(bodyObj["usage"]) {
		meta["has_usage"] = true
	}
	if hasValue(bodyObj["error"]) {
		meta["has_error"] = true
	}
	if stopReason, _ := bodyObj["stop_reason"].(string); strings.TrimSpace(stopReason) != "" {
		meta["stop_reason"] = stopReason
	}
	return marshalObservationMap(meta)
}

func summarizeRequestBody(body map[string]interface{}, meta map[string]any) {
	if len(body) == 0 {
		return
	}
	if bodyHash := observationValueHash(body); bodyHash != "" {
		meta["body_object_sha256"] = bodyHash
	}
	if keys := observationKeys(body); len(keys) > 0 {
		meta["top_level_keys"] = keys
		meta["top_level_key_count"] = len(keys)
	}
	if messages, ok := body["messages"].([]interface{}); ok {
		if messagesHash := observationValueHash(messages); messagesHash != "" {
			meta["messages_sha256"] = messagesHash
		}
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
		summarizeStructuredValue("system", body["system"], meta)
		summarizeSystem(body["system"], meta)
	}
	if hasValue(body["instructions"]) {
		meta["has_instructions"] = true
	}
	if tools, ok := body["tools"].([]interface{}); ok {
		meta["tools_count"] = len(tools)
		if len(tools) > 0 {
			meta["has_tools"] = true
			if toolsHash := observationValueHash(tools); toolsHash != "" {
				meta["tools_sha256"] = toolsHash
			}
			summarizeTools(tools, meta)
		}
	} else if hasValue(body["tools"]) {
		meta["has_tools"] = true
		summarizeStructuredValue("tools", body["tools"], meta)
	}
	if choiceType := requestChoiceType(body["tool_choice"]); choiceType != "" {
		meta["has_tool_choice"] = true
		meta["tool_choice_type"] = choiceType
		summarizeStructuredValue("tool_choice", body["tool_choice"], meta)
	}
	if responseFormatType := requestFormatType(body["response_format"]); responseFormatType != "" {
		meta["response_format_type"] = responseFormatType
		summarizeStructuredValue("response_format", body["response_format"], meta)
	}
	if outputFormatType := outputConfigFormatType(body["output_config"]); outputFormatType != "" {
		meta["output_format_type"] = outputFormatType
	}
	if hasValue(body["thinking"]) {
		meta["has_thinking"] = true
		summarizeStructuredValue("thinking", body["thinking"], meta)
	}
	if hasValue(body["output_config"]) {
		meta["has_output_config"] = true
		summarizeStructuredValue("output_config", body["output_config"], meta)
	}
	if hasValue(body["context_management"]) {
		meta["has_context_management"] = true
		summarizeStructuredValue("context_management", body["context_management"], meta)
	}
	if hasValue(body["metadata"]) {
		meta["has_metadata"] = true
		summarizeStructuredValue("metadata", body["metadata"], meta)
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
}

func observationObject(body []byte) map[string]interface{} {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || !bytes.HasPrefix(trimmed, []byte("{")) {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(trimmed, &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

func summarizeMessages(messages []interface{}, meta map[string]any) {
	roleCounts := map[string]int{}
	contentBlocks := 0
	lastRole := ""
	contentTypes := map[string]int{}
	cacheControlCount := 0
	toolResultErrorCount := 0

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
			cacheControlCount += countItemsWithCacheControl(content)
			toolResultErrorCount += countToolResultErrors(content)
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
	if cacheControlCount > 0 {
		meta["message_cache_control_count"] = cacheControlCount
	}
	if toolResultErrorCount > 0 {
		meta["tool_result_error_count"] = toolResultErrorCount
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

func summarizeSystem(value any, meta map[string]any) {
	switch typed := value.(type) {
	case []interface{}:
		meta["system_block_count"] = len(typed)
		systemTypes := map[string]int{}
		accumulateTypedItems(typed, systemTypes)
		if len(systemTypes) > 0 {
			meta["system_content_types"] = systemTypes
		}
		if cacheControlCount := countItemsWithCacheControl(typed); cacheControlCount > 0 {
			meta["system_cache_control_count"] = cacheControlCount
		}
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			meta["system_text_len"] = len(trimmed)
		}
	}
}

func summarizeTools(tools []interface{}, meta map[string]any) {
	if len(tools) == 0 {
		return
	}
	names := make([]string, 0, len(tools))
	signatures := make([]string, 0, len(tools))
	toolTypes := map[string]int{}
	for _, item := range tools {
		switch typed := item.(type) {
		case map[string]interface{}:
			toolType, _ := typed["type"].(string)
			toolType = strings.TrimSpace(strings.ToLower(toolType))
			if toolType == "" {
				toolType = "object"
			}
			toolTypes[toolType]++
			name, _ := typed["name"].(string)
			name = strings.TrimSpace(name)
			if name != "" {
				names = append(names, name)
			}
			if hash := observationValueHash(typed); hash != "" {
				parts := []string{}
				if name != "" {
					parts = append(parts, name)
				}
				parts = append(parts, toolType, shortObservationHash(hash))
				signatures = append(signatures, strings.Join(parts, ":"))
			}
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				toolTypes["text"]++
				names = append(names, trimmed)
			}
		default:
			toolTypes["present"]++
		}
	}
	if len(toolTypes) > 0 {
		meta["tool_types"] = toolTypes
	}
	if len(names) > 0 {
		sort.Strings(names)
		meta["tool_names"] = names
	}
	if len(signatures) > 0 {
		sort.Strings(signatures)
		meta["tool_signatures"] = signatures
	}
}

func summarizeStructuredValue(prefix string, value any, meta map[string]any) {
	if !hasValue(value) {
		return
	}
	meta[prefix+"_kind"] = observationKind(value)
	if hash := observationValueHash(value); hash != "" {
		meta[prefix+"_sha256"] = hash
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		if keys := observationKeys(typed); len(keys) > 0 {
			meta[prefix+"_keys"] = keys
			meta[prefix+"_key_count"] = len(keys)
		}
		if kind, _ := typed["type"].(string); strings.TrimSpace(kind) != "" {
			meta[prefix+"_type"] = strings.TrimSpace(strings.ToLower(kind))
		}
	case []interface{}:
		meta[prefix+"_count"] = len(typed)
		itemTypes := map[string]int{}
		accumulateTypedItems(typed, itemTypes)
		if len(itemTypes) > 0 {
			meta[prefix+"_item_types"] = itemTypes
		}
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			meta[prefix+"_text_len"] = len(trimmed)
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

func observationKind(value any) string {
	switch value.(type) {
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case bool:
		return "bool"
	case float64, int:
		return "number"
	default:
		if hasValue(value) {
			return "present"
		}
		return ""
	}
}

func observationRawBodyHash(body []byte) string {
	canonical := canonicalObservationBytes(body)
	if len(canonical) == 0 {
		return ""
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func observationValueHash(value any) string {
	if !hasValue(value) {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	return observationRawBodyHash(raw)
}

func canonicalObservationBytes(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	if bytes.HasPrefix(trimmed, []byte("{")) || bytes.HasPrefix(trimmed, []byte("[")) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, trimmed); err == nil {
			return compact.Bytes()
		}
	}
	return trimmed
}

func shortObservationHash(hash string) string {
	const n = 12
	if len(hash) <= n {
		return hash
	}
	return hash[:n]
}

func countItemsWithCacheControl(items []interface{}) int {
	count := 0
	for _, item := range items {
		typed, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if hasValue(typed["cache_control"]) {
			count++
		}
	}
	return count
}

func countToolResultErrors(items []interface{}) int {
	count := 0
	for _, item := range items {
		typed, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		itemType, _ := typed["type"].(string)
		if strings.TrimSpace(strings.ToLower(itemType)) != "tool_result" {
			continue
		}
		if isError, _ := typed["is_error"].(bool); isError {
			count++
		}
	}
	return count
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
