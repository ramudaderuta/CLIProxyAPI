package kiro

import (
	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/claude"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/helpers"
)

// Re-export core types for backward compatibility.
type (
	OpenAIToolCall   = claude.OpenAIToolCall
	JSONProcessor    = claude.JSONProcessor
	ContentExtractor = claude.ContentExtractor
	ResponseParser   = claude.ResponseParser
)

// Constructors / helpers
func NewJSONProcessor() JSONProcessor       { return claude.NewJSONProcessor() }
func NewContentExtractor() ContentExtractor { return claude.NewContentExtractor() }
func NewResponseParser(processor JSONProcessor, extractor ContentExtractor) ResponseParser {
	return claude.NewResponseParser(processor, extractor)
}

// Request/response translation
func BuildRequest(model string, payload []byte, token *authkiro.KiroTokenStorage, metadata map[string]any) ([]byte, error) {
	return claude.BuildRequest(model, payload, token, metadata)
}

func ParseResponse(data []byte) (string, []OpenAIToolCall) { return claude.ParseResponse(data) }

func BuildAnthropicMessagePayload(model, content string, toolCalls []OpenAIToolCall, promptTokens, completionTokens int64) ([]byte, error) {
	return claude.BuildAnthropicMessagePayload(model, content, toolCalls, promptTokens, completionTokens)
}

func BuildOpenAIChatCompletionPayload(model, content string, toolCalls []OpenAIToolCall, promptTokens, completionTokens int64) ([]byte, error) {
	return claude.BuildOpenAIChatCompletionPayload(model, content, toolCalls, promptTokens, completionTokens)
}

func BuildAnthropicStreamingChunks(id, model string, created int64, content string, toolCalls []OpenAIToolCall, promptTokens, completionTokens int64) [][]byte {
	return claude.BuildAnthropicStreamingChunks(id, model, created, content, toolCalls, promptTokens, completionTokens)
}

func BuildStreamingChunks(id, model string, created int64, content string, toolCalls []OpenAIToolCall) [][]byte {
	return claude.BuildStreamingChunks(id, model, created, content, toolCalls)
}

// Streaming helpers
func NormalizeKiroStreamPayload(raw []byte) []byte { return helpers.NormalizeKiroStreamPayload(raw) }
func ConvertKiroStreamToAnthropic(raw []byte, model string, promptTokens, completionTokens int64) [][]byte {
	return helpers.ConvertKiroStreamToAnthropic(raw, model, promptTokens, completionTokens)
}

// Model mapping
func MapModel(model string) string { return helpers.MapModel(model) }

func SanitizeToolCallID(id string) string { return helpers.SanitizeToolCallID(id) }
func ValidateToolCallID(id string) bool   { return helpers.ValidateToolCallID(id) }
