package main

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// parseSSEEventsForContent extracts content from SSE events - this is the FIXED version
func parseSSEEventsForContent(data []byte) string {
	var result strings.Builder
	dataStr := string(data)

	offset := 0
	for offset < len(dataStr) {
		jsonStart := -1

		// First try to find event-stream format marker
		eventMarker := strings.Index(dataStr[offset:], ":message-typeevent{")
		if eventMarker >= 0 {
			jsonStart = offset + eventMarker + len(":message-typeevent")
		} else {
			// Fall back to plain JSON format
			plainStart := strings.Index(dataStr[offset:], `{"`)
			if plainStart >= 0 {
				jsonStart = offset + plainStart
			}
		}

		if jsonStart < 0 {
			break
		}

		// Find the end of this JSON object by counting braces
		braceCount := 0
		jsonEnd := -1
		inString := false
		escaped := false

		for i := jsonStart; i < len(dataStr); i++ {
			ch := dataStr[i]

			if escaped {
				escaped = false
				continue
			}

			if ch == '\\' {
				escaped = true
				continue
			}

			if ch == '"' {
				inString = !inString
				continue
			}

			if !inString {
				if ch == '{' {
					braceCount++
				} else if ch == '}' {
					braceCount--
					if braceCount == 0 {
						jsonEnd = i + 1
						break
					}
				}
			}
		}

		if jsonEnd < 0 {
			break
		}

		// Extract and parse the JSON object
		jsonStr := dataStr[jsonStart:jsonEnd]
		content := gjson.Get(jsonStr, "content").String()
		if content != "" {
			result.WriteString(content)
		}

		offset = jsonEnd
	}

	return result.String()
}

func main() {
	fmt.Println("🧪 Testing Event-Stream Content Parsing Fix")
	fmt.Println("============================================\n")

	// Test 1: AWS event-stream decoded format (from debug_kiro.log)
	testData1 := `:message-typeevent{"conversationId":"a7bb608e-129a-4dc2-9074-7cab7e80c727"}
:message-typeevent{"content":"Hello! "}
:message-typeevent{"content":"How can I assist you with your software development needs today"}
:message-typeevent{"content":"?"}
:message-typeevent{"followupPrompt":{"content":"What specific challenges?"}}`

	expected1 := "Hello! How can I assist you with your software development needs today?"
	result1 := parseSSEEventsForContent([]byte(testData1))

	fmt.Println("Test 1: AWS Event-Stream Format")
	fmt.Printf("  Expected: %s\n", expected1)
	fmt.Printf("  Got:      %s\n", result1)
	if result1 == expected1 {
		fmt.Println("  ✅ PASS\n")
	} else {
		fmt.Println("  ❌ FAIL\n")
	}

	// Test 2: Plain JSON format (backward compatibility)
	testData2 := `{"content":"Hello"}{"content":" world"}`
	expected2 := "Hello world"
	result2 := parseSSEEventsForContent([]byte(testData2))

	fmt.Println("Test 2: Plain JSON Format (backward compat)")
	fmt.Printf("  Expected: %s\n", expected2)
	fmt.Printf("  Got:      %s\n", result2)
	if result2 == expected2 {
		fmt.Println("  ✅ PASS\n")
	} else {
		fmt.Println("  ❌ FAIL\n")
	}

	// Test 3: Mixed format with noise
	testData3 := `some noise
:message-typeevent{"content":"Test "}
more noise
:message-typeevent{"content":"content"}
{"conversationId":"ignore-this"}`

	expected3 := "Test content"
	result3 := parseSSEEventsForContent([]byte(testData3))

	fmt.Println("Test 3: Mixed Format with Noise")
	fmt.Printf("  Expected: %s\n", expected3)
	fmt.Printf("  Got:      %s\n", result3)
	if result3 == expected3 {
		fmt.Println("  ✅ PASS\n")
	} else {
		fmt.Println("  ❌ FAIL\n")
	}

	fmt.Println("============================================")
	fmt.Println("✅ All Parser Tests Complete!")
}
