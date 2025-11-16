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

// isContextUsagePayload identifies upstream context-usage telemetry payloads so
// we can drop them before surfacing a response to Claude Code.
func isContextUsagePayload(node gjson.Result) bool {
	if !node.Exists() || !node.IsObject() {
		return false
	}

	if node.Get("content").Exists() || node.Get("message").Exists() ||
		node.Get("type").Exists() || node.Get("name").Exists() ||
		node.Get("delta").Exists() || node.Get("toolUseId").Exists() ||
		node.Get("tool_use_id").Exists() {
		return false
	}

	hasMetric := false
	allowed := true
	node.ForEach(func(key, value gjson.Result) bool {
		lower := strings.ToLower(strings.TrimSpace(key.String()))
		switch {
		case strings.HasPrefix(lower, "contextusage"), strings.HasPrefix(lower, "context_usage"):
			hasMetric = true
			return true
		case lower == "timestamp" || lower == "time" || lower == "source" || lower == "eventid":
			return true
		default:
			allowed = false
			return false
		}
	})

	return hasMetric && allowed
}

func isContextUsagePayloadString(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !gjson.Valid(trimmed) {
		return false
	}
	return isContextUsagePayload(gjson.Parse(trimmed))
}
