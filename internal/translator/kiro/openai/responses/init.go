package responses

import (
	"context"

	. "github.com/router-for-me/CLIProxyAPI/v6/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/translator"
)

func init() {
	translator.Register(
		OpenaiResponse,
		Kiro,
		ConvertOpenAIResponsesRequestToKiro,
		interfaces.TranslateResponse{
			Stream:    ConvertKiroResponseToOpenAIResponsesStream,
			NonStream: ConvertKiroResponseToOpenAIResponsesNonStream,
		},
	)
	translator.Register(
		Kiro,
		OpenaiResponse,
		nil,
		interfaces.TranslateResponse{
			Stream: func(ctx context.Context, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
				return ConvertKiroResponseToOpenAIResponsesStream(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
			},
			NonStream: func(ctx context.Context, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
				return ConvertKiroResponseToOpenAIResponsesNonStream(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
			},
		},
	)
}
