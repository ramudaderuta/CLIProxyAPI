// Package gemini provides request translation functionality for Gemini CLI to Gemini API compatibility.
// It handles parsing and transforming Gemini CLI API requests into Gemini API format,
// extracting model information, system instructions, message contents, and tool declarations.
// The package performs JSON data transformation to ensure compatibility
// between Gemini CLI API format and Gemini API's expected format.
package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/gemini/common"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertGeminiRequestToGeminiCLI parses and transforms a Gemini CLI API request into Gemini API format.
// It extracts the model name, system instruction, message contents, and tool declarations
// from the raw JSON request and returns them in the format expected by the Gemini API.
// The function performs the following transformations:
// 1. Extracts the model information from the request
// 2. Restructures the JSON to match Gemini API format
// 3. Converts system instructions to the expected format
// 4. Fixes CLI tool response format and grouping
//
// Parameters:
//   - modelName: The name of the model to use for the request (unused in current implementation)
//   - rawJSON: The raw JSON request data from the Gemini CLI API
//   - stream: A boolean indicating if the request is for a streaming response (unused in current implementation)
//
// Returns:
//   - []byte: The transformed request data in Gemini API format
func ConvertGeminiRequestToGeminiCLI(_ string, inputRawJSON []byte, _ bool) []byte {
	rawJSON := bytes.Clone(inputRawJSON)
	template := ""
	template = `{"project":"","request":{},"model":""}`
	template, _ = sjson.SetRaw(template, "request", string(rawJSON))
	template, _ = sjson.Set(template, "model", gjson.Get(template, "request.model").String())
	template, _ = sjson.Delete(template, "request.model")

	template, errFixCLIToolResponse := fixCLIToolResponse(template)
	if errFixCLIToolResponse != nil {
		return []byte{}
	}

	systemInstructionResult := gjson.Get(template, "request.system_instruction")
	if systemInstructionResult.Exists() {
		template, _ = sjson.SetRaw(template, "request.systemInstruction", systemInstructionResult.Raw)
		template, _ = sjson.Delete(template, "request.system_instruction")
	}
	rawJSON = []byte(template)

	// Normalize roles in request.contents: default to valid values if missing/invalid
	contents := gjson.GetBytes(rawJSON, "request.contents")
	if contents.Exists() {
		prevRole := ""
		idx := 0
		contents.ForEach(func(_ gjson.Result, value gjson.Result) bool {
			role := value.Get("role").String()
			valid := role == "user" || role == "model"
			if role == "" || !valid {
				var newRole string
				if prevRole == "" {
					newRole = "user"
				} else if prevRole == "user" {
					newRole = "model"
				} else {
					newRole = "user"
				}
				path := fmt.Sprintf("request.contents.%d.role", idx)
				rawJSON, _ = sjson.SetBytes(rawJSON, path, newRole)
				role = newRole
			}
			prevRole = role
			idx++
			return true
		})
	}

	toolsResult := gjson.GetBytes(rawJSON, "request.tools")
	if toolsResult.Exists() && toolsResult.IsArray() {
		toolResults := toolsResult.Array()
		for i := 0; i < len(toolResults); i++ {
			functionDeclarationsResult := gjson.GetBytes(rawJSON, fmt.Sprintf("request.tools.%d.function_declarations", i))
			if functionDeclarationsResult.Exists() && functionDeclarationsResult.IsArray() {
				functionDeclarationsResults := functionDeclarationsResult.Array()
				for j := 0; j < len(functionDeclarationsResults); j++ {
					parametersResult := gjson.GetBytes(rawJSON, fmt.Sprintf("request.tools.%d.function_declarations.%d.parameters", i, j))
					if parametersResult.Exists() {
						strJson, _ := util.RenameKey(string(rawJSON), fmt.Sprintf("request.tools.%d.function_declarations.%d.parameters", i, j), fmt.Sprintf("request.tools.%d.function_declarations.%d.parametersJsonSchema", i, j))
						rawJSON = []byte(strJson)
					}
				}
			}
		}
	}

	return common.AttachDefaultSafetySettings(rawJSON, "request.safetySettings")
}

// FunctionCallGroup represents a group of function calls and their responses
type FunctionCallGroup struct {
	ModelContent    map[string]interface{}
	FunctionCalls   []gjson.Result
	ResponsesNeeded int
}

// fixCLIToolResponse performs sophisticated tool response format conversion and grouping.
// This function transforms the CLI tool response format by intelligently grouping function calls
// with their corresponding responses, ensuring proper conversation flow and API compatibility.
// It converts from a linear format (1.json) to a grouped format (2.json) where function calls
// and their responses are properly associated and structured.
//
// Parameters:
//   - input: The input JSON string to be processed
//
// Returns:
//   - string: The processed JSON string with grouped function calls and responses
//   - error: An error if the processing fails
func fixCLIToolResponse(input string) (string, error) {
	// Parse the input JSON to extract the conversation structure
	parsed := gjson.Parse(input)

	// Extract the contents array which contains the conversation messages
	contents := parsed.Get("request.contents")
	if !contents.Exists() {
		// log.Debugf(input)
		return input, fmt.Errorf("contents not found in input")
	}

	// Initialize data structures for processing and grouping
	var newContents []interface{}          // Final processed contents array
	var pendingGroups []*FunctionCallGroup // Groups awaiting completion with responses
	var collectedResponses []gjson.Result  // Standalone responses to be matched

	// Process each content object in the conversation
	// This iterates through messages and groups function calls with their responses
	contents.ForEach(func(key, value gjson.Result) bool {
		role := value.Get("role").String()
		parts := value.Get("parts")

		// Check if this content has function responses
		var responsePartsInThisContent []gjson.Result
		parts.ForEach(func(_, part gjson.Result) bool {
			if part.Get("functionResponse").Exists() {
				responsePartsInThisContent = append(responsePartsInThisContent, part)
			}
			return true
		})

		// If this content has function responses, collect them
		if len(responsePartsInThisContent) > 0 {
			collectedResponses = append(collectedResponses, responsePartsInThisContent...)

			// Check if any pending groups can be satisfied
			for i := len(pendingGroups) - 1; i >= 0; i-- {
				group := pendingGroups[i]
				if len(collectedResponses) >= group.ResponsesNeeded {
					// Take the needed responses for this group
					groupResponses := collectedResponses[:group.ResponsesNeeded]
					collectedResponses = collectedResponses[group.ResponsesNeeded:]

					// Create merged function response content
					var responseParts []interface{}
					for _, response := range groupResponses {
						var responseMap map[string]interface{}
						errUnmarshal := json.Unmarshal([]byte(response.Raw), &responseMap)
						if errUnmarshal != nil {
							log.Warnf("failed to unmarshal function response: %v\n", errUnmarshal)
							continue
						}
						responseParts = append(responseParts, responseMap)
					}

					if len(responseParts) > 0 {
						functionResponseContent := map[string]interface{}{
							"parts": responseParts,
							"role":  "function",
						}
						newContents = append(newContents, functionResponseContent)
					}

					// Remove this group as it's been satisfied
					pendingGroups = append(pendingGroups[:i], pendingGroups[i+1:]...)
					break
				}
			}

			return true // Skip adding this content, responses are merged
		}

		// If this is a model with function calls, create a new group
		if role == "model" {
			var functionCallsInThisModel []gjson.Result
			parts.ForEach(func(_, part gjson.Result) bool {
				if part.Get("functionCall").Exists() {
					functionCallsInThisModel = append(functionCallsInThisModel, part)
				}
				return true
			})

			if len(functionCallsInThisModel) > 0 {
				// Add the model content
				var contentMap map[string]interface{}
				errUnmarshal := json.Unmarshal([]byte(value.Raw), &contentMap)
				if errUnmarshal != nil {
					log.Warnf("failed to unmarshal model content: %v\n", errUnmarshal)
					return true
				}
				newContents = append(newContents, contentMap)

				// Create a new group for tracking responses
				group := &FunctionCallGroup{
					ModelContent:    contentMap,
					FunctionCalls:   functionCallsInThisModel,
					ResponsesNeeded: len(functionCallsInThisModel),
				}
				pendingGroups = append(pendingGroups, group)
			} else {
				// Regular model content without function calls
				var contentMap map[string]interface{}
				errUnmarshal := json.Unmarshal([]byte(value.Raw), &contentMap)
				if errUnmarshal != nil {
					log.Warnf("failed to unmarshal content: %v\n", errUnmarshal)
					return true
				}
				newContents = append(newContents, contentMap)
			}
		} else {
			// Non-model content (user, etc.)
			var contentMap map[string]interface{}
			errUnmarshal := json.Unmarshal([]byte(value.Raw), &contentMap)
			if errUnmarshal != nil {
				log.Warnf("failed to unmarshal content: %v\n", errUnmarshal)
				return true
			}
			newContents = append(newContents, contentMap)
		}

		return true
	})

	// Handle any remaining pending groups with remaining responses
	for _, group := range pendingGroups {
		if len(collectedResponses) >= group.ResponsesNeeded {
			groupResponses := collectedResponses[:group.ResponsesNeeded]
			collectedResponses = collectedResponses[group.ResponsesNeeded:]

			var responseParts []interface{}
			for _, response := range groupResponses {
				var responseMap map[string]interface{}
				errUnmarshal := json.Unmarshal([]byte(response.Raw), &responseMap)
				if errUnmarshal != nil {
					log.Warnf("failed to unmarshal function response: %v\n", errUnmarshal)
					continue
				}
				responseParts = append(responseParts, responseMap)
			}

			if len(responseParts) > 0 {
				functionResponseContent := map[string]interface{}{
					"parts": responseParts,
					"role":  "function",
				}
				newContents = append(newContents, functionResponseContent)
			}
		}
	}

	// Update the original JSON with the new contents
	result := input
	newContentsJSON, _ := json.Marshal(newContents)
	result, _ = sjson.Set(result, "request.contents", json.RawMessage(newContentsJSON))

	return result, nil
}
