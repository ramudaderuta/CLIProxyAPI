// Package helpers provides defensive utility functions for Kiro translation.
// It includes safe JSON parsing, tool ID sanitization, and other helper functions.
package helpers

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

var (
	// Regex patterns for sanitizing malformed JSON
	danglingBackslashRegex = regexp.MustCompile(`\\$`)
	incompleteUnicodeRegex = regexp.MustCompile(`\\u[0-9a-fA-F]{0,3}$`)
	incompleteEscapeRegex  = regexp.MustCompile(`\\[^"\\\/bfnrtu]`)
)

// SafeParseJSON attempts to parse JSON with defensive handling of malformed input.
// It sanitizes dangling backslashes and incomplete escape sequences before parsing.
//
// Parameters:
//   - raw: The raw JSON string to parse
//
// Returns:
//   - map[string]any: The parsed JSON as a map
//   - error: An error if parsing fails even after sanitization
func SafeParseJSON(raw string) (map[string]any, error) {
	// Quick path for empty strings
	if raw == "" {
		return make(map[string]any), nil
	}

	// Sanitize the input
	sanitized := sanitizeJSON(raw)

	// Attempt to parse
	var result map[string]any
	err := json.Unmarshal([]byte(sanitized), &result)
	if err != nil {
		log.Warnf("SafeParseJSON failed even after sanitization: %v", err)
		// Return empty map on failure
		return make(map[string]any), err
	}

	return result, nil
}

// sanitizeJSON sanitizes malformed JSON by removing dangerous escape sequences
func sanitizeJSON(raw string) string {
	// Remove dangling backslashes at the end
	sanitized := danglingBackslashRegex.ReplaceAllString(raw, "")

	// Remove incomplete Unicode escape sequences (\u123)
	sanitized = incompleteUnicodeRegex.ReplaceAllString(sanitized, "")

	// Replace invalid escape sequences with their literal equivalents
	sanitized = incompleteEscapeRegex.ReplaceAllStringFunc(sanitized, func(match string) string {
		// Keep the backslash but escape it
		return strings.Replace(match, `\`, `\\`, 1)
	})

	return sanitized
}

// SanitizeToolCallID ensures a tool call ID is non-empty.
// If the ID is empty or whitespace-only, generates a new UUID-based ID.
//
// Parameters:
//   - id: The tool call ID to sanitize
//
// Returns:
//   - string: The sanitized tool call ID
func SanitizeToolCallID(id string) string {
	// Trim whitespace
	id = strings.TrimSpace(id)

	// Generate new ID if empty
	if id == "" {
		newID := "call_" + uuid.New().String()[:12]
		log.Debugf("Generated new tool call ID: %s", newID)
		return newID
	}

	return id
}

// TruncateString truncates a string to the specified length and adds a suffix.
//
// Parameters:
//   - s: The string to truncate
//   - maxLen: The maximum length
//   - suffix: The suffix to add if truncated
//
// Returns:
//   - string: The truncated string with suffix
func TruncateString(s string, maxLen int, suffix string) string {
	if len(s) <= maxLen {
		return s
	}

	truncated := s[:maxLen]
	return truncated + suffix
}

// ExtractTextFromMultimodal extracts all text parts from multimodal content array.
//
// Parameters:
//   - content: The multimodal content array (as interface{})
//
// Returns:
//   - string: The concatenated text content
func ExtractTextFromMultimodal(content interface{}) string {
	// Type assert to array
	contentArray, ok := content.([]interface{})
	if !ok {
		return ""
	}

	var textParts []string

	for _, part := range contentArray {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}

		partType, _ := partMap["type"].(string)
		if partType == "text" {
			if text, ok := partMap["text"].(string); ok {
				textParts = append(textParts, text)
			}
		}
	}

	return strings.Join(textParts, "\n")
}

// CoalesceString returns the first non-empty string from the arguments.
//
// Parameters:
//   - values: Variadic string arguments
//
// Returns:
//   - string: The first non-empty string, or empty string if all are empty
func CoalesceString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// SafeStringValue safely extracts a string value from a map.
//
// Parameters:
//   - m: The map to extract from
//   - key: The key to look up
//
// Returns:
//   - string: The string value, or empty string if not found or not a string
func SafeStringValue(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}

	val, ok := m[key]
	if !ok {
		return ""
	}

	str, ok := val.(string)
	if !ok {
		return ""
	}

	return str
}

// SafeInt64Value safely extracts an int64 value from a map.
//
// Parameters:
//   - m: The map to extract from
//   - key: The key to look up
//
// Returns:
//   - int64: The int64 value, or 0 if not found or not numeric
func SafeInt64Value(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}

	val, ok := m[key]
	if !ok {
		return 0
	}

	// Handle different numeric types
	switch v := val.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

// SafeGetString safely extracts a string value from any interface.
//
// Parameters:
//   - m: The map to extract from
//   - key: The key to look up
//
// Returns:
//   - string: The string value, or empty string if not found
func SafeGetString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}

	val, ok := m[key]
	if !ok {
		return ""
	}

	str, ok := val.(string)
	if !ok {
		return ""
	}

	return str
}
