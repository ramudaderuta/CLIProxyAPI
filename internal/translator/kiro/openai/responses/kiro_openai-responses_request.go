package responses

import (
	. "github.com/router-for-me/CLIProxyAPI/v6/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/chat-completions"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/translator"
)

// ConvertOpenAIResponsesRequestToKiro converts OpenAI Responses API requests to Kiro format
// by first translating them into OpenAI Chat Completions format and then delegating to the
// chat completions converter.
func ConvertOpenAIResponsesRequestToKiro(model string, rawJSON []byte, stream bool) []byte {
	translated := translator.Request(OpenaiResponse, OpenAI, model, rawJSON, stream)
	result, err := chat_completions.ConvertOpenAIRequestToKiro(model, translated, nil, nil)
	if err != nil {
		return translated
	}
	return result
}
