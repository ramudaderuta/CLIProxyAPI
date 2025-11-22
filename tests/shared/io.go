// Package shared provides common test utilities, fixtures, and helpers
// for testing the Kiro CLI provider across unit, integration, and regression tests.
package shared

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestDataDir returns the path to the shared testdata directory
func TestDataDir() string {
	// Find the tests/shared/testdata directory
	// This assumes we're running from the project root or a subdirectory
	candidates := []string{
		"tests/shared/testdata",
		"../shared/testdata",
		"../../shared/testdata",
		"../../../shared/testdata",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	return "tests/shared/testdata"
}

// LoadJSONFile loads a JSON file from the testdata directory
func LoadJSONFile(t *testing.T, filename string) []byte {
	t.Helper()

	path := filepath.Join(TestDataDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to load test data file %s: %v", filename, err)
	}

	return data
}

// LoadNonStreamRequest loads a non-streaming request JSON file
func LoadNonStreamRequest(t *testing.T, name string) []byte {
	t.Helper()
	return LoadJSONFile(t, filepath.Join("nonstream", name+".json"))
}

// LoadStreamingData loads an NDJSON streaming data file
func LoadStreamingData(t *testing.T, name string) []byte {
	t.Helper()
	return LoadJSONFile(t, filepath.Join("streaming", name+".ndjson"))
}

// ParseJSONFile loads and parses a JSON file into the provided interface
func ParseJSONFile(t *testing.T, filename string, v interface{}) {
	t.Helper()

	data := LoadJSONFile(t, filename)
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("Failed to parse JSON file %s: %v", filename, err)
	}
}

// WriteJSONFile writes data as JSON to a file (for test data generation)
func WriteJSONFile(t *testing.T, filename string, v interface{}) {
	t.Helper()

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal data: %v", err)
	}

	path := filepath.Join(TestDataDir(), filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// ReadLines reads a file and returns its lines
func ReadLines(t *testing.T, filename string) []string {
	t.Helper()

	path := filepath.Join(TestDataDir(), filename)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file %s: %v", filename, err)
	}
	defer file.Close()

	var lines []string
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", filename, err)
	}

	// Split by newlines
	for _, line := range splitLines(data) {
		if len(line) > 0 {
			lines = append(lines, string(line))
		}
	}

	return lines
}

// splitLines splits byte data into lines
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	var start int

	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}

	// Handle last line if no trailing newline
	if start < len(data) {
		lines = append(lines, data[start:])
	}

	return lines
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
