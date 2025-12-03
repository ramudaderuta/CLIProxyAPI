package helpers

import "strings"

var modelMapping = map[string]string{
	"claude-sonnet-4-5":                  "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-5-20250929":         "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-20250514":           "CLAUDE_SONNET_4_20250514_V1_0",
	"claude-3-7-sonnet-20250219":         "CLAUDE_3_7_SONNET_20250219_V1_0",
	"amazonq-claude-sonnet-4-20250514":   "CLAUDE_SONNET_4_20250514_V1_0",
	"amazonq-claude-3-7-sonnet-20250219": "CLAUDE_3_7_SONNET_20250219_V1_0",
}

// MapModel returns the upstream Kiro model identifier for the provided alias.
func MapModel(model string) string {
	if mapped, ok := modelMapping[strings.TrimSpace(model)]; ok {
		return mapped
	}
	return modelMapping["claude-sonnet-4-5"]
}
