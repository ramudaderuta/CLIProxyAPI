package testutil

import (
	"math/rand"
	"testing"
	"time"
)

// Setenv sets an environment variable for testing
func Setenv(t testing.TB, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

// SeedRandom seeds randomness deterministically for tests
func SeedRandom(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

// FixedTime returns a fixed time for testing
func FixedTime() time.Time {
	return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
}