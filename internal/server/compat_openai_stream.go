package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type compatOpenAIStreamWriter struct {
	dst            http.ResponseWriter
	flusher        http.Flusher
	requestedModel string
	created        int64

	header         http.Header
	status         int
	headersWritten bool

	lineBuf       bytes.Buffer
	rawErrorBody  bytes.Buffer
	lastEventType string
	chunkID       string
	roleSent      bool
	doneSent      bool

	chunksEmitted      int
	contentChunks      int
	terminalSignalSeen bool
	syntheticDone      bool
	downstreamBytes    int
	downstreamErr      error
}

type compatGeminiStreamWriter struct {
	dst            http.ResponseWriter
	flusher        http.Flusher
	requestedModel string
	created        int64

	header         http.Header
	status         int
	headersWritten bool

	lineBuf      bytes.Buffer
	rawErrorBody bytes.Buffer
	doneSent     bool
	roleSent     bool
	chunkID      string

	chunksEmitted      int
	contentChunks      int
	terminalSignalSeen bool
	syntheticDone      bool
	downstreamBytes    int
	downstreamErr      error
}

func newCompatOpenAIStreamWriter(dst http.ResponseWriter, requestedModel string) *compatOpenAIStreamWriter {
	streamWriter := &compatOpenAIStreamWriter{
		dst:            dst,
		requestedModel: requestedModel,
		created:        time.Now().Unix(),
		header:         make(http.Header),
	}
	if flusher, ok := dst.(http.Flusher); ok {
		streamWriter.flusher = flusher
	}
	return streamWriter
}

func (w *compatOpenAIStreamWriter) Header() http.Header {
	return w.header
}

func (w *compatOpenAIStreamWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *compatOpenAIStreamWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.downstreamErr != nil {
		return len(p), w.downstreamErr
	}
	if w.status != http.StatusOK {
		_, _ = w.rawErrorBody.Write(p)
		return len(p), nil
	}

	_, _ = w.lineBuf.Write(p)
	for {
		line, ok := w.nextLine()
		if !ok {
			break
		}
		if !w.handleClaudeStreamLine(line) {
			return len(p), w.downstreamErr
		}
	}
	return len(p), nil
}

func (w *compatOpenAIStreamWriter) Flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}

func (w *compatOpenAIStreamWriter) nextLine() (string, bool) {
	data := w.lineBuf.Bytes()
	idx := bytes.IndexByte(data, '\n')
	if idx < 0 {
		return "", false
	}
	line := strings.TrimRight(string(data[:idx]), "\r")
	w.lineBuf.Next(idx + 1)
	return line, true
}

func (w *compatOpenAIStreamWriter) handleClaudeStreamLine(line string) bool {
	if line == "" {
		return true
	}
	if strings.HasPrefix(line, "event: ") {
		w.lastEventType = strings.TrimPrefix(line, "event: ")
		return true
	}
	if !strings.HasPrefix(line, "data: ") {
		return true
	}

	payload := strings.TrimPrefix(line, "data: ")
	switch w.lastEventType {
	case "message_start":
		var event struct {
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(payload), &event) == nil && strings.TrimSpace(event.Message.ID) != "" {
			w.chunkID = event.Message.ID
		}
		if !w.roleSent {
			w.roleSent = true
			return w.emitChunk(compatOpenAIChatDelta{Role: "assistant"}, nil)
		}

	case "content_block_delta":
		var event struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if json.Unmarshal([]byte(payload), &event) != nil {
			return true
		}
		if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
			return w.emitChunk(compatOpenAIChatDelta{Content: event.Delta.Text}, nil)
		}

	case "message_delta":
		var event struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
		}
		if json.Unmarshal([]byte(payload), &event) != nil {
			return true
		}
		if strings.TrimSpace(event.Delta.StopReason) == "" {
			return true
		}
		finishReason := compatClaudeFinishReason(event.Delta.StopReason)
		return w.emitChunk(compatOpenAIChatDelta{}, &finishReason)

	case "message_stop":
		w.terminalSignalSeen = true
		return w.emitDone(false)
	}
	return true
}

func (w *compatOpenAIStreamWriter) ensureSuccessHeaders() {
	if w.headersWritten {
		return
	}
	w.dst.Header().Set("Content-Type", "text/event-stream")
	w.dst.Header().Set("Cache-Control", "no-cache")
	w.dst.Header().Set("Connection", "keep-alive")
	w.dst.WriteHeader(http.StatusOK)
	w.headersWritten = true
}

func (w *compatOpenAIStreamWriter) streamChunkID() string {
	if strings.TrimSpace(w.chunkID) != "" {
		return w.chunkID
	}
	return "chatcmpl-compat"
}

func (w *compatOpenAIStreamWriter) emitChunk(delta compatOpenAIChatDelta, finishReason *string) bool {
	w.ensureSuccessHeaders()

	body, err := json.Marshal(compatOpenAIChatStreamChunk{
		ID:      w.streamChunkID(),
		Object:  "chat.completion.chunk",
		Created: w.created,
		Model:   w.requestedModel,
		Choices: []compatOpenAIChatStreamChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
	})
	if err != nil {
		return true
	}
	if delta.Content != "" {
		w.contentChunks++
	}
	w.chunksEmitted++
	if !w.writeDownstream([]byte("data: ")) {
		return false
	}
	if !w.writeDownstream(body) {
		return false
	}
	if !w.writeDownstream([]byte("\n\n")) {
		return false
	}
	w.Flush()
	return w.downstreamErr == nil
}

func (w *compatOpenAIStreamWriter) emitDone(synthetic bool) bool {
	if w.doneSent {
		return w.downstreamErr == nil
	}
	w.ensureSuccessHeaders()
	w.syntheticDone = synthetic
	if !w.writeDownstream([]byte("data: [DONE]\n\n")) {
		return false
	}
	w.Flush()
	w.doneSent = true
	return w.downstreamErr == nil
}

func (w *compatOpenAIStreamWriter) finalize() {
	if w.downstreamErr != nil {
		return
	}
	if w.status != 0 && w.status != http.StatusOK {
		writeCompatOpenAIUpstreamError(w.dst, w.status, compatExtractErrorBody(w.rawErrorBody.Bytes()))
		return
	}

	if w.lineBuf.Len() > 0 {
		line := strings.TrimRight(w.lineBuf.String(), "\r\n")
		w.lineBuf.Reset()
		if line != "" {
			if !w.handleClaudeStreamLine(line) {
				return
			}
		}
	}
	_ = w.emitDone(!w.terminalSignalSeen)
}

func (w *compatOpenAIStreamWriter) writeDownstream(p []byte) bool {
	n, err := w.dst.Write(p)
	w.downstreamBytes += n
	if err != nil && w.downstreamErr == nil {
		w.downstreamErr = err
	}
	return err == nil
}

func (w *compatOpenAIStreamWriter) completed() bool {
	return (w.status == 0 || w.status == http.StatusOK) && w.downstreamErr == nil && w.terminalSignalSeen && w.doneSent
}

func (w *compatOpenAIStreamWriter) ClientResponseObservation() map[string]any {
	meta := map[string]any{
		"http_status":          w.statusOrOK(),
		"headers_written":      w.headersWritten,
		"chunks_emitted":       w.chunksEmitted,
		"content_chunks":       w.contentChunks,
		"role_sent":            w.roleSent,
		"done_sent":            w.doneSent,
		"terminal_signal_seen": w.terminalSignalSeen,
		"synthetic_done":       w.syntheticDone,
		"downstream_bytes":     w.downstreamBytes,
		"delivery_completed":   w.completed(),
	}
	if w.downstreamErr != nil {
		meta["downstream_error"] = w.downstreamErr.Error()
	}
	return meta
}

func (w *compatOpenAIStreamWriter) statusOrOK() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func newCompatGeminiStreamWriter(dst http.ResponseWriter, requestedModel string) *compatGeminiStreamWriter {
	streamWriter := &compatGeminiStreamWriter{
		dst:            dst,
		requestedModel: requestedModel,
		created:        time.Now().Unix(),
		header:         make(http.Header),
	}
	if flusher, ok := dst.(http.Flusher); ok {
		streamWriter.flusher = flusher
	}
	return streamWriter
}

func (w *compatGeminiStreamWriter) Header() http.Header {
	return w.header
}

func (w *compatGeminiStreamWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *compatGeminiStreamWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.downstreamErr != nil {
		return len(p), w.downstreamErr
	}
	if w.status != http.StatusOK {
		_, _ = w.rawErrorBody.Write(p)
		return len(p), nil
	}

	_, _ = w.lineBuf.Write(p)
	for {
		line, ok := w.nextLine()
		if !ok {
			break
		}
		if !w.handleGeminiStreamLine(line) {
			return len(p), w.downstreamErr
		}
	}
	return len(p), nil
}

func (w *compatGeminiStreamWriter) Flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}

func (w *compatGeminiStreamWriter) nextLine() (string, bool) {
	data := w.lineBuf.Bytes()
	idx := bytes.IndexByte(data, '\n')
	if idx < 0 {
		return "", false
	}
	line := strings.TrimRight(string(data[:idx]), "\r")
	w.lineBuf.Next(idx + 1)
	return line, true
}

func (w *compatGeminiStreamWriter) handleGeminiStreamLine(line string) bool {
	if line == "" || !strings.HasPrefix(line, "data: ") {
		return true
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
	if payload == "" || payload == "[DONE]" {
		w.terminalSignalSeen = true
		return w.emitDone(false)
	}

	resp, err := compatParseGeminiResponse([]byte(payload))
	if err != nil || resp == nil {
		return true
	}
	if strings.TrimSpace(resp.ResponseID) != "" {
		w.chunkID = resp.ResponseID
	}
	if len(resp.Candidates) == 0 {
		return true
	}
	candidate := resp.Candidates[0]
	if candidate.Content != nil {
		if !w.roleSent {
			w.roleSent = true
			if !w.emitChunk(compatOpenAIChatDelta{Role: "assistant"}, nil) {
				return false
			}
		}

		var text strings.Builder
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				text.WriteString(part.Text)
			}
		}
		if text.Len() > 0 {
			if !w.emitChunk(compatOpenAIChatDelta{Content: text.String()}, nil) {
				return false
			}
		}
	}

	if strings.TrimSpace(candidate.FinishReason) != "" {
		finishReason := compatGeminiFinishReason(candidate.FinishReason)
		w.terminalSignalSeen = true
		return w.emitChunk(compatOpenAIChatDelta{}, &finishReason)
	}
	return true
}

func (w *compatGeminiStreamWriter) ensureSuccessHeaders() {
	if w.headersWritten {
		return
	}
	w.dst.Header().Set("Content-Type", "text/event-stream")
	w.dst.Header().Set("Cache-Control", "no-cache")
	w.dst.Header().Set("Connection", "keep-alive")
	w.dst.WriteHeader(http.StatusOK)
	w.headersWritten = true
}

func (w *compatGeminiStreamWriter) streamChunkID() string {
	if strings.TrimSpace(w.chunkID) != "" {
		return w.chunkID
	}
	return "chatcmpl-compat"
}

func (w *compatGeminiStreamWriter) emitChunk(delta compatOpenAIChatDelta, finishReason *string) bool {
	w.ensureSuccessHeaders()

	body, err := json.Marshal(compatOpenAIChatStreamChunk{
		ID:      w.streamChunkID(),
		Object:  "chat.completion.chunk",
		Created: w.created,
		Model:   w.requestedModel,
		Choices: []compatOpenAIChatStreamChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
	})
	if err != nil {
		return true
	}
	if delta.Content != "" {
		w.contentChunks++
	}
	w.chunksEmitted++
	if !w.writeDownstream([]byte("data: ")) {
		return false
	}
	if !w.writeDownstream(body) {
		return false
	}
	if !w.writeDownstream([]byte("\n\n")) {
		return false
	}
	w.Flush()
	return w.downstreamErr == nil
}

func (w *compatGeminiStreamWriter) emitDone(synthetic bool) bool {
	if w.doneSent {
		return w.downstreamErr == nil
	}
	w.ensureSuccessHeaders()
	w.syntheticDone = synthetic
	if !w.writeDownstream([]byte("data: [DONE]\n\n")) {
		return false
	}
	w.Flush()
	w.doneSent = true
	return w.downstreamErr == nil
}

func (w *compatGeminiStreamWriter) finalize() {
	if w.downstreamErr != nil {
		return
	}
	if w.status != 0 && w.status != http.StatusOK {
		writeCompatOpenAIUpstreamError(w.dst, w.status, compatExtractErrorBody(w.rawErrorBody.Bytes()))
		return
	}

	if w.lineBuf.Len() > 0 {
		line := strings.TrimRight(w.lineBuf.String(), "\r\n")
		w.lineBuf.Reset()
		if line != "" {
			if !w.handleGeminiStreamLine(line) {
				return
			}
		}
	}
	_ = w.emitDone(!w.terminalSignalSeen)
}

func (w *compatGeminiStreamWriter) writeDownstream(p []byte) bool {
	n, err := w.dst.Write(p)
	w.downstreamBytes += n
	if err != nil && w.downstreamErr == nil {
		w.downstreamErr = err
	}
	return err == nil
}

func (w *compatGeminiStreamWriter) completed() bool {
	return (w.status == 0 || w.status == http.StatusOK) && w.downstreamErr == nil && w.terminalSignalSeen && w.doneSent
}

func (w *compatGeminiStreamWriter) ClientResponseObservation() map[string]any {
	meta := map[string]any{
		"http_status":          w.statusOrOK(),
		"headers_written":      w.headersWritten,
		"chunks_emitted":       w.chunksEmitted,
		"content_chunks":       w.contentChunks,
		"role_sent":            w.roleSent,
		"done_sent":            w.doneSent,
		"terminal_signal_seen": w.terminalSignalSeen,
		"synthetic_done":       w.syntheticDone,
		"downstream_bytes":     w.downstreamBytes,
		"delivery_completed":   w.completed(),
	}
	if w.downstreamErr != nil {
		meta["downstream_error"] = w.downstreamErr.Error()
	}
	return meta
}

func (w *compatGeminiStreamWriter) statusOrOK() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
