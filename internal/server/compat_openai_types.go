package server

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/yansircc/llm-broker/internal/domain"
)

const compatClaudeDefaultMaxTokens = 4096
const compatDefaultRequestBodyMB = 60

type compatOpenAIChatRequest struct {
	Model               string          `json:"model"`
	Messages            []compatMessage `json:"messages"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	Stop                json.RawMessage `json:"stop,omitempty"`
	ResponseFormat      json.RawMessage `json:"response_format,omitempty"`
	Stream              bool            `json:"stream,omitempty"`
	Tools               json.RawMessage `json:"tools,omitempty"`
	ToolChoice          json.RawMessage `json:"tool_choice,omitempty"`
}

type compatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type compatClaudeRequest struct {
	Model         string                    `json:"model"`
	System        any                       `json:"system,omitempty"`
	Messages      []compatClaudeMessage     `json:"messages"`
	MaxTokens     int                       `json:"max_tokens"`
	Stream        *bool                     `json:"stream,omitempty"`
	OutputConfig  *compatClaudeOutputConfig `json:"output_config,omitempty"`
	Thinking      *compatClaudeThinking     `json:"thinking,omitempty"`
	Temperature   *float64                  `json:"temperature,omitempty"`
	TopP          *float64                  `json:"top_p,omitempty"`
	StopSequences []string                  `json:"stop_sequences,omitempty"`
}

type compatClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type compatClaudeSystemBlock struct {
	Type         string                          `json:"type"`
	Text         string                          `json:"text,omitempty"`
	CacheControl *compatClaudeCacheControlPolicy `json:"cache_control,omitempty"`
}

type compatClaudeCacheControlPolicy struct {
	Type string `json:"type"`
}

type compatClaudeOutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type compatClaudeThinking struct {
	Type string `json:"type,omitempty"`
}

type compatClaudeResponse struct {
	ID         string `json:"id"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
	Usage *struct {
		InputTokens       int `json:"input_tokens"`
		OutputTokens      int `json:"output_tokens"`
		CacheReadTokens   int `json:"cache_read_input_tokens,omitempty"`
		CacheCreateTokens int `json:"cache_creation_input_tokens,omitempty"`
	} `json:"usage"`
}

type compatGeminiRequest struct {
	Model             string                        `json:"model,omitempty"`
	Contents          []compatGeminiContent         `json:"contents"`
	SystemInstruction *compatGeminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *compatGeminiGenerationConfig `json:"generationConfig,omitempty"`
}

type compatGeminiContent struct {
	Role  string             `json:"role,omitempty"`
	Parts []compatGeminiPart `json:"parts"`
}

type compatGeminiPart struct {
	Text string `json:"text,omitempty"`
}

type compatGeminiGenerationConfig struct {
	MaxOutputTokens    int      `json:"maxOutputTokens,omitempty"`
	Temperature        *float64 `json:"temperature,omitempty"`
	TopP               *float64 `json:"topP,omitempty"`
	StopSequences      []string `json:"stopSequences,omitempty"`
	ResponseMIMEType   string   `json:"responseMimeType,omitempty"`
	ResponseJSONSchema any      `json:"responseJsonSchema,omitempty"`
}

type compatGeminiResponseEnvelope struct {
	Response *compatGeminiResponse `json:"response,omitempty"`
}

type compatGeminiResponse struct {
	ResponseID   string                     `json:"responseId,omitempty"`
	ModelVersion string                     `json:"modelVersion,omitempty"`
	Candidates   []compatGeminiCandidate    `json:"candidates,omitempty"`
	Usage        *compatGeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

type compatGeminiCandidate struct {
	Index        int                  `json:"index,omitempty"`
	FinishReason string               `json:"finishReason,omitempty"`
	Content      *compatGeminiContent `json:"content,omitempty"`
}

type compatGeminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}

type compatResponseFormatSpec struct {
	Type       string `json:"type"`
	JSONSchema *struct {
		Schema any `json:"schema"`
	} `json:"json_schema,omitempty"`
}

type compatOpenAIChatResponse struct {
	ID      string                     `json:"id"`
	Object  string                     `json:"object"`
	Created int64                      `json:"created"`
	Model   string                     `json:"model"`
	Choices []compatOpenAIChatChoice   `json:"choices"`
	Usage   *compatOpenAIChatUsageInfo `json:"usage,omitempty"`
}

type compatOpenAIChatChoice struct {
	Index        int                         `json:"index"`
	Message      compatOpenAIResponseMessage `json:"message"`
	FinishReason string                      `json:"finish_reason"`
}

type compatOpenAIResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type compatOpenAIChatUsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type compatOpenAIChatStreamChunk struct {
	ID      string                         `json:"id"`
	Object  string                         `json:"object"`
	Created int64                          `json:"created"`
	Model   string                         `json:"model"`
	Choices []compatOpenAIChatStreamChoice `json:"choices"`
}

type compatOpenAIChatStreamChoice struct {
	Index        int                   `json:"index"`
	Delta        compatOpenAIChatDelta `json:"delta"`
	FinishReason *string               `json:"finish_reason,omitempty"`
}

type compatOpenAIChatDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type compatOpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type compatTarget struct {
	provider        domain.Provider
	requestedModel  string
	relayPath       string
	upstreamBody    []byte
	upstreamAccept  string
	upstreamHeaders http.Header
	stream          bool
	convertResponse func([]byte, string) (*compatOpenAIChatResponse, error)
	newStreamWriter func(http.ResponseWriter, string) compatStreamWriter
}

type compatStreamWriter interface {
	http.ResponseWriter
	finalize()
}

type compatResponseCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *compatResponseCapture) Header() http.Header {
	if c.header == nil {
		c.header = make(http.Header)
	}
	return c.header
}

func (c *compatResponseCapture) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	return c.body.Write(p)
}

func (c *compatResponseCapture) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}

type compatError string

func (e compatError) Error() string { return string(e) }

func errCompat(message string) error { return compatError(message) }

var _ http.ResponseWriter = (*compatResponseCapture)(nil)
