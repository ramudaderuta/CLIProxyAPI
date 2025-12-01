package helpers

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	protocolNoisePrefixes = []string{
		"event-type",
		"message-type",
		"content-length",
		"amz-sdk-request",
		"x-amzn",
		"amzn-",
		"transfer-encoding",
	}
	protocolNoisePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)content-type\s*[: ]*\s*application/json`),
		regexp.MustCompile(`(?i)content-type`),
	}
	collapseWhitespaceRegex = regexp.MustCompile(`[ \f\r]+`)
)

// AssistantTextSanitizeOptions tunes sanitation behavior for assistant text and stream chunks.
type AssistantTextSanitizeOptions struct {
	AllowBlank         bool
	CollapseWhitespace bool
	TrimResult         bool
	DropEmptyLines     bool
}

// SanitizeAssistantText removes control characters, protocol noise, and collapses whitespace.
func SanitizeAssistantText(text string) string {
	return SanitizeAssistantTextWithOptions(text, AssistantTextSanitizeOptions{
		AllowBlank:         false,
		CollapseWhitespace: true,
		TrimResult:         true,
		DropEmptyLines:     true,
	})
}

// SanitizeAssistantTextWithOptions allows fine-grained control over sanitation behavior.
func SanitizeAssistantTextWithOptions(text string, opts AssistantTextSanitizeOptions) string {
	if strings.TrimSpace(text) == "" && !opts.AllowBlank {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\r':
			continue
		case r < 32:
			if r == '\n' {
				builder.WriteByte('\n')
			} else if r == '\t' {
				builder.WriteByte('\t')
			} else if r == '\v' {
				builder.WriteByte(' ')
			}
			continue
		case unicode.IsControl(r):
			continue
		}
		builder.WriteRune(r)
	}

	cleaned := builder.String()
	if cleaned == "" {
		return ""
	}

	for _, pattern := range protocolNoisePatterns {
		cleaned = pattern.ReplaceAllString(cleaned, "")
	}

	lines := strings.Split(cleaned, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if opts.DropEmptyLines {
				continue
			}
			filtered = append(filtered, line)
			continue
		}
		lower := strings.ToLower(trimmed)
		if ShouldDropProtocolLine(lower) {
			continue
		}
		if opts.CollapseWhitespace {
			filtered = append(filtered, trimmed)
		} else {
			filtered = append(filtered, line)
		}
	}

	result := strings.Join(filtered, "\n")
	if opts.CollapseWhitespace {
		result = collapseWhitespaceRegex.ReplaceAllString(result, " ")
	}
	if opts.TrimResult {
		result = strings.TrimSpace(result)
	}
	if result == "" && !opts.AllowBlank {
		return ""
	}
	return result
}

// ShouldDropProtocolLine filters lines that look like protocol noise.
func ShouldDropProtocolLine(line string) bool {
	if line == "" {
		return false
	}
	for _, prefix := range protocolNoisePrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// SanitizeStreamingTextChunk keeps newline/indentation while stripping controls for stream text.
func SanitizeStreamingTextChunk(text string) string {
	return SanitizeAssistantTextWithOptions(text, AssistantTextSanitizeOptions{
		AllowBlank:         true,
		CollapseWhitespace: false,
		TrimResult:         false,
		DropEmptyLines:     false,
	})
}
