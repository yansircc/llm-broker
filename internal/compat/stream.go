package compat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// StreamConverter reads Anthropic SSE events and writes OpenAI-format chunks.
type StreamConverter struct {
	w       http.ResponseWriter
	flusher http.Flusher

	// State tracked across events
	id        string
	model     string
	created   int64
	toolIndex int // tracks current tool_call index for OpenAI delta format

	// Usage accumulation
	inputTokens  int
	outputTokens int

	includeUsage bool // from stream_options.include_usage
}

// StreamChatResponse reads an Anthropic streaming response and writes
// OpenAI-format SSE chunks to the client.
// Returns (completed, usage).
func StreamChatResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response, includeUsage bool) (bool, *ChatUsage) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return false, nil
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	sc := &StreamConverter{
		w:            w,
		flusher:      flusher,
		created:      time.Now().Unix(),
		toolIndex:    -1,
		includeUsage: includeUsage,
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var eventType string
	completed := true

	for scanner.Scan() {
		if ctx.Err() != nil {
			completed = false
			break
		}
		line := scanner.Text()

		if after, found := strings.CutPrefix(line, "event: "); found {
			eventType = after
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			data := line[6:]
			sc.handleEvent(eventType, []byte(data))
			eventType = ""
		}
	}

	// Emit final usage chunk if requested
	if completed && sc.includeUsage {
		sc.emitUsageChunk()
	}

	// Emit [DONE]
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()

	if sc.inputTokens == 0 && sc.outputTokens == 0 {
		return completed, nil
	}
	return completed, &ChatUsage{
		PromptTokens:     sc.inputTokens,
		CompletionTokens: sc.outputTokens,
		TotalTokens:      sc.inputTokens + sc.outputTokens,
	}
}

func (sc *StreamConverter) handleEvent(eventType string, data []byte) {
	switch eventType {
	case "message_start":
		sc.onMessageStart(data)
	case "content_block_start":
		sc.onContentBlockStart(data)
	case "content_block_delta":
		sc.onContentBlockDelta(data)
	case "message_delta":
		sc.onMessageDelta(data)
	case "ping", "content_block_stop", "message_stop":
		// Ignored — [DONE] is emitted after the loop
	}
}

func (sc *StreamConverter) onMessageStart(data []byte) {
	var ev struct {
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage struct {
				InputTokens int `json:"input_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}
	if json.Unmarshal(data, &ev) != nil {
		return
	}
	sc.id = "chatcmpl-" + ev.Message.ID
	sc.model = ev.Message.Model
	sc.inputTokens = ev.Message.Usage.InputTokens

	// Emit initial chunk with role
	sc.emitChunk(&ChatRespMessage{Role: "assistant", Content: strPtr("")}, nil)
}

func (sc *StreamConverter) onContentBlockStart(data []byte) {
	var ev struct {
		Index        int `json:"index"`
		ContentBlock struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"content_block"`
	}
	if json.Unmarshal(data, &ev) != nil {
		return
	}

	if ev.ContentBlock.Type == "tool_use" {
		sc.toolIndex++
		idx := sc.toolIndex
		sc.emitChunk(&ChatRespMessage{
			ToolCalls: []ToolCall{{
				Index: &idx,
				ID:    ev.ContentBlock.ID,
				Type:  "function",
				Function: ToolCallFunction{
					Name:      ev.ContentBlock.Name,
					Arguments: "",
				},
			}},
		}, nil)
	}
	// text blocks: nothing to emit at start
}

func (sc *StreamConverter) onContentBlockDelta(data []byte) {
	var ev struct {
		Index int `json:"index"`
		Delta struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
		} `json:"delta"`
	}
	if json.Unmarshal(data, &ev) != nil {
		return
	}

	switch ev.Delta.Type {
	case "text_delta":
		sc.emitChunk(&ChatRespMessage{Content: &ev.Delta.Text}, nil)

	case "input_json_delta":
		// Tool arguments chunk — index only, no id/type/name (omitempty drops them)
		idx := sc.toolIndex
		sc.emitChunk(&ChatRespMessage{
			ToolCalls: []ToolCall{{
				Index: &idx,
				Function: ToolCallFunction{
					Arguments: ev.Delta.PartialJSON,
				},
			}},
		}, nil)

	case "thinking":
		// Extended thinking — skip, not part of OpenAI format
	}
}

func (sc *StreamConverter) onMessageDelta(data []byte) {
	var ev struct {
		Delta struct {
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		Usage struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &ev) != nil {
		return
	}

	sc.outputTokens = ev.Usage.OutputTokens

	finishReason := mapStopReason(ev.Delta.StopReason)
	sc.emitChunk(&ChatRespMessage{}, &finishReason)
}

func (sc *StreamConverter) emitChunk(delta *ChatRespMessage, finishReason *string) {
	chunk := ChatCompletionChunk{
		ID:      sc.id,
		Object:  "chat.completion.chunk",
		Created: sc.created,
		Model:   sc.model,
		Choices: []ChatChoice{{
			Index:        0,
			Delta:        delta,
			FinishReason: finishReason,
		}},
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	fmt.Fprintf(sc.w, "data: %s\n\n", data)
	sc.flusher.Flush()
}

func (sc *StreamConverter) emitUsageChunk() {
	chunk := ChatCompletionChunk{
		ID:      sc.id,
		Object:  "chat.completion.chunk",
		Created: sc.created,
		Model:   sc.model,
		Choices: []ChatChoice{},
		Usage: &ChatUsage{
			PromptTokens:     sc.inputTokens,
			CompletionTokens: sc.outputTokens,
			TotalTokens:      sc.inputTokens + sc.outputTokens,
		},
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	fmt.Fprintf(sc.w, "data: %s\n\n", data)
	sc.flusher.Flush()
}

func strPtr(s string) *string { return &s }
