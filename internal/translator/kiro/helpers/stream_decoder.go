// Package helpers provides defensive utility functions for Kiro translation.
// It includes safe JSON parsing, tool ID sanitization, event-stream decoding, and other helper functions.
package helpers

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"

	log "github.com/sirupsen/logrus"
)

const (
	// Event-stream frame structure sizes
	preludeSize  = 8 // 4 bytes total length + 4 bytes headers length
	crcSize      = 4 // CRC32 checksum
	minFrameSize = preludeSize + crcSize

	// Event-stream header types
	headerTypeBool      = 0
	headerTypeByte      = 1
	headerTypeShort     = 2
	headerTypeInt       = 3
	headerTypeLong      = 4
	headerTypeBytes     = 5
	headerTypeString    = 6
	headerTypeTimestamp = 7
	headerTypeUUID      = 8
)

// NormalizeKiroStreamPayload decodes Amazon event-stream binary format to plain SSE.
// If the input is already plain text SSE, it returns it unchanged.
//
// Amazon event-stream format:
//
//	[prelude: 8 bytes][headers: variable][payload: variable][message_crc: 4 bytes]
//
// Prelude structure:
//
//	[total_length: 4 bytes BE][headers_length: 4 bytes BE]
//
// Parameters:
//   - data: Raw response data that may be in event-stream format
//
// Returns:
//   - []byte: Decoded SSE payload or original data if not event-stream
//   - error: An error if decoding fails
func NormalizeKiroStreamPayload(data []byte) ([]byte, error) {
	// Quick check: if data doesn't look like binary event-stream, return as-is
	if !IsEventStreamFormat(data) {
		return data, nil
	}

	log.Debug("Detected Amazon event-stream format, decoding...")

	var result bytes.Buffer
	offset := 0

	for offset < len(data) {
		// Check if we have enough data for a frame
		if offset+minFrameSize > len(data) {
			log.Debugf("Incomplete frame at offset %d, stopping decode", offset)
			break
		}

		// Decode frame
		framePayload, frameSize, err := DecodeEventStreamFrame(data[offset:])
		if err != nil {
			// If decoding fails, return what we've decoded so far
			log.Warnf("Event-stream decode error at offset %d: %v", offset, err)
			if result.Len() > 0 {
				return result.Bytes(), nil
			}
			// If nothing decoded yet, return original data
			return data, nil
		}

		// Append payload to result
		if len(framePayload) > 0 {
			result.Write(framePayload)
		}

		offset += frameSize
	}

	decoded := result.Bytes()
	log.Debugf("Event-stream decoded: %d bytes -> %d bytes", len(data), len(decoded))

	return decoded, nil
}

// IsEventStreamFormat detects if data is in Amazon event-stream binary format.
// Event-stream always starts with a 4-byte big-endian total length.
func IsEventStreamFormat(data []byte) bool {
	if len(data) < minFrameSize {
		return false
	}

	// Read total length from first 4 bytes
	totalLength := binary.BigEndian.Uint32(data[0:4])

	// Event-stream frames should have reasonable size (not too small, not huge)
	// and total length should match or be close to data length
	if totalLength < uint32(minFrameSize) || totalLength > uint32(len(data))*2 {
		return false
	}

	// Additional heuristic: check if it looks like plain text SSE
	// SSE typically starts with "event:" or "data:" or "id:"
	if len(data) > 6 {
		prefix := string(data[0:6])
		if prefix == "event:" || prefix == "data: " || prefix == "id: " {
			return false
		}
	}

	return true
}

// DecodeEventStreamFrame decodes a single event-stream frame.
//
// Returns:
//   - payload: The decoded payload bytes
//   - frameSize: Total size of the frame (including headers and CRC)
//   - error: An error if decoding fails
func DecodeEventStreamFrame(data []byte) (payload []byte, frameSize int, err error) {
	if len(data) < minFrameSize {
		return nil, 0, fmt.Errorf("frame too small: %d bytes", len(data))
	}

	// Read prelude
	totalLength := binary.BigEndian.Uint32(data[0:4])
	headersLength := binary.BigEndian.Uint32(data[4:8])

	// Validate lengths
	if int(totalLength) > len(data) {
		return nil, 0, fmt.Errorf("total length %d exceeds data size %d", totalLength, len(data))
	}

	if headersLength > totalLength-uint32(minFrameSize) {
		return nil, 0, fmt.Errorf("invalid headers length %d for total %d", headersLength, totalLength)
	}

	// Calculate payload offset and length
	payloadOffset := preludeSize + int(headersLength)
	payloadLength := int(totalLength) - payloadOffset - crcSize

	if payloadLength < 0 {
		return nil, 0, fmt.Errorf("invalid payload length: %d", payloadLength)
	}

	// Extract payload
	if payloadLength > 0 {
		payload = data[payloadOffset : payloadOffset+payloadLength]
	}

	// Verify CRC32 (optional but recommended)
	messageCRC := binary.BigEndian.Uint32(data[totalLength-crcSize : totalLength])
	calculatedCRC := crc32.ChecksumIEEE(data[:totalLength-crcSize])

	if messageCRC != calculatedCRC {
		log.Warnf("CRC mismatch: expected %08x, got %08x", messageCRC, calculatedCRC)
		// Still return payload but log warning
	}

	return payload, int(totalLength), nil
}

// FilterEventStreamMetadata removes non-content event-stream events.
// Kiro may send metering and context usage events that should be filtered.
func FilterEventStreamMetadata(data []byte) []byte {
	// This is a simple filter - remove lines containing known metadata event types
	lines := bytes.Split(data, []byte("\n"))
	var filtered [][]byte

	skipNext := false
	for _, line := range lines {
		// Skip metering and usage events
		if bytes.Contains(line, []byte("event: metering")) ||
			bytes.Contains(line, []byte("event: contextUsage")) ||
			bytes.Contains(line, []byte("event: billingMetrics")) {
			skipNext = true
			continue
		}

		// Skip data line after metadata event
		if skipNext && bytes.HasPrefix(line, []byte("data:")) {
			skipNext = false
			continue
		}

		filtered = append(filtered, line)
	}

	return bytes.Join(filtered, []byte("\n"))
}
