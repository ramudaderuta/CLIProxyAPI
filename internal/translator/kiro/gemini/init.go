package gemini

import (
	"context"

	. "github.com/router-for-me/CLIProxyAPI/v6/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/translator"
)

func init() {
	translator.Register(
		Gemini,
		Kiro,
		func(model string, rawJSON []byte, stream bool) []byte {
			return ConvertGeminiRequestToKiro(model, rawJSON, stream)
		},
		interfaces.TranslateResponse{
			Stream: func(ctx context.Context, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
				chunk := ConvertKiroStreamChunkToGemini(rawJSON, model)
				if chunk == nil {
					return []string{}
				}
				return []string{string(chunk)}
			},
			NonStream: func(ctx context.Context, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
				return string(ConvertKiroResponseToGemini(rawJSON, model, false))
			},
		},
	)
}
