package toolid

import (
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		encoded bool
	}{
		{
			name:    "safe id unchanged",
			input:   "call_123abcXYZ",
			encoded: false,
		},
		{
			name:    "claude style id encoded",
			input:   "***.TodoWrite:3",
			encoded: true,
		},
		{
			name:    "id with spaces encoded",
			input:   "tool call 1",
			encoded: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			encoded := Encode(tc.input)
			if tc.encoded {
				if encoded == tc.input {
					t.Fatalf("expected encoded ID to differ for %q", tc.input)
				}
				if decoded := Decode(encoded); decoded != tc.input {
					t.Fatalf("expected decode %q, got %q", tc.input, decoded)
				}
			} else {
				if encoded != tc.input {
					t.Fatalf("expected safe ID to remain unchanged, got %q", encoded)
				}
			}
		})
	}
}

func TestDecodeHandlesPlainIDs(t *testing.T) {
	if got := Decode("call_abc"); got != "call_abc" {
		t.Fatalf("plain IDs should pass through, got %q", got)
	}
}
