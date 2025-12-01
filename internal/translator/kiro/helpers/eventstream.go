package helpers

import (
	"bytes"
	"encoding/binary"
	"strings"
)

const (
	awsEventStreamPreludeLen = 8
	awsEventStreamOverhead   = 12 // prelude (8) + message CRC (4)
)

// NormalizeKiroStreamPayload detects Amazon event-stream encoded responses and strips the binary envelope
// so downstream parsers see the original text-based SSE payload. When the payload is not event-stream data,
// the original slice is returned unchanged.
func NormalizeKiroStreamPayload(raw []byte) []byte {
	if len(raw) < awsEventStreamOverhead {
		return raw
	}
	decoded, ok := decodeAmazonEventStream(raw)
	if !ok {
		return raw
	}
	return decoded
}

func decodeAmazonEventStream(raw []byte) ([]byte, bool) {
	if len(raw) < awsEventStreamOverhead {
		return nil, false
	}

	buf := raw
	var out bytes.Buffer
	decodedAny := false
	processed := false

	for len(buf) >= awsEventStreamOverhead {
		totalLen := int(binary.BigEndian.Uint32(buf[0:4]))
		headerLen := int(binary.BigEndian.Uint32(buf[4:8]))

		if totalLen < awsEventStreamOverhead || totalLen > len(buf) {
			break
		}
		if headerLen < 0 || headerLen > totalLen-awsEventStreamOverhead {
			break
		}

		payloadStart := awsEventStreamPreludeLen + headerLen
		payloadEnd := totalLen - 4 // strip CRC
		if payloadStart > payloadEnd || payloadEnd > len(buf) {
			break
		}

		processed = true
		payload := extractJSONPayload(buf[payloadStart:payloadEnd])
		if payload != "" {
			if decodedAny {
				out.WriteString("\n\n")
			}
			out.WriteString(payload)
			decodedAny = true
		}

		buf = buf[totalLen:]
	}

	if !processed {
		return nil, false
	}

	return out.Bytes(), true
}

func extractJSONPayload(payload []byte) string {
	lines := strings.Split(string(payload), "\n")
	var builder strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ":") {
			continue
		}
		if strings.HasPrefix(trimmed, "event") || strings.HasPrefix(trimmed, "vent") {
			if idx := strings.Index(trimmed, "{"); idx >= 0 {
				trimmed = trimmed[idx:]
			} else {
				continue
			}
		}
		if IsMeteringPayloadString(trimmed) || IsContextUsagePayloadString(trimmed) {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(trimmed)
	}
	return strings.TrimSpace(builder.String())
}
