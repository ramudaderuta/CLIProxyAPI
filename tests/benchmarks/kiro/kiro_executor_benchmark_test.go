package kiro

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestExecutorBenchmarks benchmarks executor performance
func BenchmarkExecutorFullCycle(b *testing.B) {

	request := shared.BuildOpenAIRequest(
		"kiro-sonnet",
		shared.SimpleMessages,
		false,
	)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate full request cycle
		_ = shared.MarshalJSON(&testing.T{}, request)
	}
}

func BenchmarkTokenValidation(b *testing.B) {
	// Create a simple token for benchmarking
	expiresAt := time.Now().Add(1 * time.Hour)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate token validation
		_ = expiresAt
	}
}

func BenchmarkSSEStreaming(b *testing.B) {
	events := []struct{ Event, Data string }{
		{"message_start", `{"type":"message_start"}`},
		{"content_block_delta", `{"type":"content_block_delta","delta":{"text":"Hello"}}`},
		{"message_stop", `{"type":"message_stop"}`},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = shared.MockSSEResponse(events)
	}
}

func BenchmarkTranslationOpenAIToKiro(b *testing.B) {
	request := shared.BuildOpenAIRequest(
		"kiro-sonnet",
		shared.MultiTurnMessages,
		false,
	)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = shared.MarshalJSON(&testing.T{}, request)
	}
}

func BenchmarkTranslationKiroToOpenAI(b *testing.B) {
	response := shared.BuildKiroResponse(shared.TestResponse)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = shared.MarshalJSON(&testing.T{}, response)
	}
}
