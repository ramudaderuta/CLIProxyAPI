// Package shared provides environment and utility helpers for testing
package shared

import (
	"math/rand"
	"os"
	"testing"
	"time"
)

// SetEnv sets an environment variable for the duration of a test
func SetEnv(t *testing.T, key, value string) {
	t.Helper()

	// Save original value
	original, hadOriginal := os.LookupEnv(key)

	// Set new value
	os.Setenv(key, value)

	// Restore on cleanup
	t.Cleanup(func() {
		if hadOriginal {
			os.Setenv(key, original)
		} else {
			os.Unsetenv(key)
		}
	})
}

// SetTestTime sets a fixed time for testing (useful for timestamp tests)
type TestClock struct {
	current time.Time
}

// NewTestClock creates a TestClock with a fixed time
func NewTestClock(t time.Time) *TestClock {
	return &TestClock{current: t}
}

// Now returns the current test time
func (tc *TestClock) Now() time.Time {
	return tc.current
}

// Advance advances the test clock by a duration
func (tc *TestClock) Advance(d time.Duration) {
	tc.current = tc.current.Add(d)
}

// Set sets the clock to a specific time
func (tc *TestClock) Set(t time.Time) {
	tc.current = t
}

// FixedTestTime returns a fixed time for reproducible tests
func FixedTestTime() time.Time {
	return time.Date(2024, 11, 23, 12, 0, 0, 0, time.UTC)
}

// RandomString generates a random string of given length (for test data)
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// RandomTestSeed creates a deterministic random seed for reproducible tests
func RandomTestSeed(t *testing.T) *rand.Rand {
	t.Helper()

	// Use test name as seed for reproducibility
	seed := int64(0)
	for _, r := range t.Name() {
		seed += int64(r)
	}

	return rand.New(rand.NewSource(seed))
}

// TempDir creates a temporary directory for testing
func TempDir(t *testing.T, pattern string) string {
	t.Helper()

	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	return dir
}

// TempFile creates a temporary file for testing
func TempFile(t *testing.T, pattern string) *os.File {
	t.Helper()

	file, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	t.Cleanup(func() {
		file.Close()
		os.Remove(file.Name())
	})

	return file
}

// SkipIfShort skips a test if -short flag is set
func SkipIfShort(t *testing.T, reason string) {
	if testing.Short() {
		t.Skip("Skipping test in short mode:", reason)
	}
}

// RequireEnv requires an environment variable to be set, or skips the test
func RequireEnv(t *testing.T, key string) string {
	value := os.Getenv(key)
	if value == "" {
		t.Skipf("Skipping test: %s environment variable not set", key)
	}
	return value
}
