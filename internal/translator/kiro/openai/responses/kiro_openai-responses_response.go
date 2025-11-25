package responses

import (
	"context"

	. "github.com/router-for-me/CLIProxyAPI/v6/internal/constant"
	chat_completions "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/chat-completions"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/translator"
)

// ConvertKiroResponseToOpenAIResponsesNonStream converts a full Kiro response to OpenAI Responses format.
func ConvertKiroResponseToOpenAIResponsesNonStream(ctx context.Context, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	openAI := chat_completions.ConvertKiroResponseToOpenAI(rawJSON, model, false)
	return translator.ResponseNonStream(OpenAI, OpenaiResponse, ctx, model, originalRequestRawJSON, requestRawJSON, openAI, param)
}

// ConvertKiroResponseToOpenAIResponsesStream converts streaming chunks from Kiro to OpenAI Responses SSE events.
func ConvertKiroResponseToOpenAIResponsesStream(ctx context.Context, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	openAIChunk := chat_completions.ConvertKiroStreamChunkToOpenAI(rawJSON, model)
	if openAIChunk == nil {
		return []string{}
	}
	return translator.Response(OpenAI, OpenaiResponse, ctx, model, originalRequestRawJSON, requestRawJSON, openAIChunk, param)
}
