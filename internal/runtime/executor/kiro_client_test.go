package executor

import "testing"

func TestSanitizePayloadForLogRemovesControl(t *testing.T) {
	raw := []byte(":message-type event\r\n{\"content\":\"Hello\"}\x1e\r\n:event-type assistantResponseEvent\x90\r\n")
	got := sanitizePayloadForLog(raw)
	expected := ":message-type event\n{\"content\":\"Hello\"}\n:event-type assistantResponseEvent"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestSanitizePayloadForLogPreservesPrintable(t *testing.T) {
	raw := []byte("Tool output says 30°C\nand rising.")
	got := sanitizePayloadForLog(raw)
	expected := "Tool output says 30°C\nand rising."
	if got != expected {
		t.Fatalf("expected printable text to remain, want %q got %q", expected, got)
	}
}
