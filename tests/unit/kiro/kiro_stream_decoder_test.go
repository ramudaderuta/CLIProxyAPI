package kiro_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/helpers"
)

func TestIsEventStreamFormat(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "too_small",
			data:     []byte{0x00, 0x00},
			expected: false,
		},
		{
			name:     "plain_sse_event",
			data:     []byte("event: message_start\ndata: {}\n\n"),
			expected: false,
		},
		{
			name:     "plain_sse_data",
			data:     []byte("data: hello world\n\n"),
			expected: false,
		},
		{
			name: "valid_event_stream",
			data: func() []byte {
				// Create a minimal valid frame: 12 bytes total (8 prelude + 0 payload + 4 CRC)
				buf := make([]byte, 12)
				binary.BigEndian.PutUint32(buf[0:4], 12) // total length
				binary.BigEndian.PutUint32(buf[4:8], 0)  // headers length
				// CRC would be at buf[8:12]
				return buf
			}(),
			expected: true,
		},
		{
			name: "invalid_total_length_too_small",
			data: func() []byte {
				buf := make([]byte, 12)
				binary.BigEndian.PutUint32(buf[0:4], 5) // too small
				binary.BigEndian.PutUint32(buf[4:8], 0)
				return buf
			}(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helpers.IsEventStreamFormat(tt.data)
			if result != tt.expected {
				t.Errorf("IsEventStreamFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDecodeEventStreamFrame(t *testing.T) {
	tests := []struct {
		name            string
		data            []byte
		expectedPayload []byte
		expectedSize    int
		expectError     bool
	}{
		{
			name:        "frame_too_small",
			data:        []byte{0x00, 0x00},
			expectError: true,
		},
		{
			name: "minimal_frame_no_payload",
			data: func() []byte {
				buf := make([]byte, 12)
				binary.BigEndian.PutUint32(buf[0:4], 12) // total: 8 prelude + 0 payload + 4 crc
				binary.BigEndian.PutUint32(buf[4:8], 0)  // no headers
				// Calculate and set CRC
				crc := crc32ChecksumIEEE(buf[:8])
				binary.BigEndian.PutUint32(buf[8:12], crc)
				return buf
			}(),
			expectedPayload: nil,
			expectedSize:    12,
			expectError:     false,
		},
		{
			name: "frame_with_payload",
			data: func() []byte {
				payload := []byte("test data")
				totalLen := 8 + 0 + len(payload) + 4 // prelude + headers + payload + crc
				buf := make([]byte, totalLen)
				binary.BigEndian.PutUint32(buf[0:4], uint32(totalLen))
				binary.BigEndian.PutUint32(buf[4:8], 0) // no headers
				copy(buf[8:], payload)
				// Calculate and set CRC

				crc := crc32ChecksumIEEE(buf[:totalLen-4])
				binary.BigEndian.PutUint32(buf[totalLen-4:], crc)
				return buf
			}(),
			expectedPayload: []byte("test data"),
			expectedSize:    21, // 8 + 9 + 4
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, size, err := helpers.DecodeEventStreamFrame(tt.data)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if size != tt.expectedSize {
				t.Errorf("size = %d, want %d", size, tt.expectedSize)
			}

			if !bytes.Equal(payload, tt.expectedPayload) {
				t.Errorf("payload = %v, want %v", payload, tt.expectedPayload)
			}
		})
	}
}

func TestNormalizeKiroStreamPayload(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "plain_sse_unchanged",
			input:    []byte("event: test\ndata: hello\n\n"),
			expected: []byte("event: test\ndata: hello\n\n"),
		},
		{
			name: "event_stream_decoded",
			input: func() []byte {
				payload := []byte("data: decoded\n\n")
				totalLen := 8 + 0 + len(payload) + 4
				buf := make([]byte, totalLen)
				binary.BigEndian.PutUint32(buf[0:4], uint32(totalLen))
				binary.BigEndian.PutUint32(buf[4:8], 0)
				copy(buf[8:], payload)
				crc := crc32ChecksumIEEE(buf[:totalLen-4])
				binary.BigEndian.PutUint32(buf[totalLen-4:], crc)
				return buf
			}(),
			expected: []byte("data: decoded\n\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := helpers.NormalizeKiroStreamPayload(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !bytes.Equal(result, tt.expected) {
				t.Errorf("result = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFilterEventStreamMetadata(t *testing.T) {
	input := `event: message_start
data: {"type":"message_start"}

event: metering
data: {"usage":100}

event: content_block_delta
data: {"delta":"hello"}

event: contextUsage
data: {"tokens":50}

event: message_stop
data: {"type":"message_stop"}

`

	expected := `event: message_start
data: {"type":"message_start"}


event: content_block_delta
data: {"delta":"hello"}


event: message_stop
data: {"type":"message_stop"}

`

	result := helpers.FilterEventStreamMetadata([]byte(input))
	if string(result) != expected {
		t.Errorf("FilterEventStreamMetadata failed\nGot:\n%s\nWant:\n%s", result, expected)
	}
}

// Helper function for CRC32 calculation (import from crc32 package if needed)
func crc32ChecksumIEEE(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}
