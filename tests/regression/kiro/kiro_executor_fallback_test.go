package kiro_test

import (
	"encoding/json"
	"strings"
	"testing"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	testutil "github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
	"github.com/stretchr/testify/require"
)

func TestKiroExecutorFallbackBuilders(t *testing.T) {
	payload := testutil.LoadTestData(t, "claude_format.json")
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}

	primaryBody, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	require.NoError(t, err, "translator should build primary Kiro request")

	flattened, err := executor.BuildFlattenedKiroRequest(primaryBody)
	require.NoError(t, err, "flattened fallback should build successfully")

	var flattenedReq map[string]any
	require.NoError(t, json.Unmarshal(flattened, &flattenedReq))
	flatConv := flattenedReq["conversationState"].(map[string]any)
	flatHistory := flatConv["history"].([]any)
	require.Len(t, flatHistory, 0, "flattened fallback should discard history")
	flatUser := flatConv["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	flatContent := flatUser["content"].(string)
	require.Contains(t, flatContent, "Structured tool transcripts were flattened", "flattened fallback should inject explanatory note")
	require.True(t, strings.Contains(flatContent, "Result of calling the Read tool"), "flattened fallback should carry prior transcript details")

	minimal, err := executor.BuildMinimalKiroRequest(primaryBody)
	require.NoError(t, err, "minimal fallback should build successfully")

	var minimalReq map[string]any
	require.NoError(t, json.Unmarshal(minimal, &minimalReq))
	minConv := minimalReq["conversationState"].(map[string]any)
	minHistory := minConv["history"].([]any)
	require.Len(t, minHistory, 0, "minimal fallback should also drop history")
	minUser := minConv["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	minContent := strings.TrimSpace(minUser["content"].(string))
	require.NotEmpty(t, minContent, "minimal fallback must still provide some guidance text")
	require.NotContains(t, minContent, "Structured tool transcripts were flattened", "minimal fallback should avoid the verbose transcript note")
}
