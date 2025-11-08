package toolid

import (
	"encoding/base64"
	"strings"
	"unicode"
)

const encodedPrefix = "call_enc_"

// Encode converts a provider-specific tool_call_id into an OpenAI-safe identifier.
// IDs containing characters outside [A-Za-z0-9_-] are base64url-encoded with a
// well-known prefix so they can be deterministically decoded later.
func Encode(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	if !needsEncoding(trimmed) {
		return trimmed
	}
	encoded := base64.RawURLEncoding.EncodeToString([]byte(trimmed))
	return encodedPrefix + encoded
}

// Decode converts an encoded OpenAI-safe identifier back into the original provider ID.
// If the ID is not encoded, it is returned as-is.
func Decode(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" || !strings.HasPrefix(trimmed, encodedPrefix) {
		return trimmed
	}
	payload := trimmed[len(encodedPrefix):]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return trimmed
	}
	return string(decoded)
}

func needsEncoding(id string) bool {
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return true
	}
	return false
}
