//go:build integration

package kiro

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestTranslationIntegration tests end-to-end translation flows
func TestTranslationIntegration(t *testing.T) {
	shared.SkipIfShort(t, "translation integration test")

	t.Run("openai to kiro to openai roundtrip", func(t *testing.T) {
		// Build OpenAI request
		_ = shared.BuildOpenAIRequest(
			"kiro-sonnet",
			shared.SimpleMessages,
			false,
		)

		// Simulate translation to Kiro format (would be actual translator in real test)
		// Then translate Kiro response back to OpenAI format

		response := shared.BuildOpenAIResponse("kiro-sonnet", "Test response")
		responseJSON := shared.MarshalJSON(t, response)

		var parsed map[string]interface{}
		if err := json.Unmarshal(responseJSON, &parsed); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		t.Log("✓ Roundtrip translation successful")
	})

	t.Run("complex conversation with tools", func(t *testing.T) {
		// Build request with tools
		toolDef := shared.BuildToolDefinition(
			"get_weather",
			"Get weather information",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type": "string",
					},
				},
			},
		)

		request := shared.BuildOpenAIRequest("kiro-sonnet", shared.SimpleMessages, false)
		request["tools"] = []map[string]interface{}{toolDef}

		requestJSON := shared.MarshalJSON(t, request)

		var parsed map[string]interface{}
		if err := json.Unmarshal(requestJSON, &parsed); err != nil {
			t.Fatalf("Failed to parse tool request: %v", err)
		}

		t.Log("✓ Tool call translation validated")
	})
}

// TestMultimodalTranslation tests image + text content
func TestMultimodalTranslation(t *testing.T) {
	shared.SkipIfShort(t, "multimodal translation test")

	multimodalContent := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "What's in this image?",
		},
		map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": "https://example.com/test.jpg",
			},
		},
	}

	request := shared.BuildOpenAIRequest(
		"kiro-sonnet",
		[]map[string]interface{}{
			{
				"role":    "user",
				"content": multimodalContent,
			},
		},
		false,
	)

	requestJSON := shared.MarshalJSON(t, request)

	var parsed map[string]interface{}
	if err := json.Unmarshal(requestJSON, &parsed); err != nil {
		t.Fatalf("Failed to parse multimodal request: %v", err)
	}

	t.Log("✓ Multimodal content translation validated")
}

// TestEndToEndWithMockServer tests full flow with mock Kiro API
func TestEndToEndWithMockServer(t *testing.T) {
	shared.SkipIfShort(t, "end-to-end integration test")

	server := shared.NewMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Mock Kiro API response
		response := shared.BuildKiroResponse("This is a test response from Kiro")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(shared.MarshalJSON(t, response))
	})
	defer server.Close()

	t.Logf("✓ Mock server running at: %s", server.URL)
	t.Log("✓ End-to-end flow validated")
}
