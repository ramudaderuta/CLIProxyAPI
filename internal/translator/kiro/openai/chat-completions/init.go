package chat_completions

import (
	"context"

	. "github.com/router-for-me/CLIProxyAPI/v6/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/translator"
)

func init() {
	translator.Register(
		OpenAI,
		Kiro,
		func(model string, rawJSON []byte, stream bool) []byte {
			converted, err := ConvertOpenAIRequestToKiro(model, rawJSON, nil, nil)
			if err != nil {
				return rawJSON
			}
			return converted
		},
		interfaces.TranslateResponse{
			Stream: func(ctx context.Context, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
				chunk := ConvertKiroStreamChunkToOpenAI(rawJSON, model)
				if chunk == nil {
					return []string{}
				}
				return []string{string(chunk)}
			},
			NonStream: func(ctx context.Context, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
				return string(ConvertKiroResponseToOpenAI(rawJSON, model, false))
			},
		},
	)
}
