package kiro

import (
	"strings"

	"github.com/tidwall/gjson"
)

// isMeteringPayload returns true when the JSON payload represents a meter/usage event
// (unit/unitPlural/usage trio with no other semantic data).
func isMeteringPayload(node gjson.Result) bool {
	if !node.Exists() {
		return false
	}
	if !node.Get("unit").Exists() || !node.Get("unitPlural").Exists() || !node.Get("usage").Exists() {
		return false
	}

	// Reject payloads that also include assistant/tool fields.
	if node.Get("name").Exists() || node.Get("content").Exists() || node.Get("type").Exists() ||
		node.Get("toolUseId").Exists() || node.Get("input").Exists() || node.Get("delta").Exists() {
		return false
	}

	extra := false
	node.ForEach(func(key, value gjson.Result) bool {
		switch key.String() {
		case "unit", "unitPlural", "usage":
			return true
		default:
			extra = true
			return false
		}
	})
	return !extra
}

func isMeteringPayloadString(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !gjson.Valid(trimmed) {
		return false
	}
	return isMeteringPayload(gjson.Parse(trimmed))
}
