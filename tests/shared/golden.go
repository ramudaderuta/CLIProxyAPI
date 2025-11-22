// Package shared provides common test utility functions
package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Golden file test utilities

var updateGolden = false

func init() {
	// Check for -golden flag
	for _, arg := range os.Args {
		if arg == "-golden" || arg == "--golden" {
			updateGolden = true
			break
		}
	}
}

// CompareWithGolden compares actual output with a golden file
// If -golden flag is set, it updates the golden file
func CompareWithGolden(t *testing.T, name string, actual []byte) {
	t.Helper()

	goldenPath := filepath.Join(TestDataDir(), "..", "unit", "kiro", "testdata", "golden", name+".golden")

	if updateGolden {
		// Update golden file
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("Failed to create golden directory: %v", err)
		}
		if err := os.WriteFile(goldenPath, actual, 0644); err != nil {
			t.Fatalf("Failed to write golden file: %v", err)
		}
		t.Log("Updated golden file:", name)
		return
	}

	// Compare with golden file
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file %s: %v (run with -golden to create)", name, err)
	}

	if string(actual) != string(expected) {
		t.Errorf("Output does not match golden file %s\n=== Expected ===\n%s\n=== Actual ===\n%s\n=== Run with -golden to update ===",
			name, string(expected), string(actual))
	}
}

// LoadGoldenFile loads a golden file for comparison
func LoadGoldenFile(t *testing.T, name string) []byte {
	t.Helper()

	goldenPath := filepath.Join(TestDataDir(), "..", "unit", "kiro", "testdata", "golden", name+".golden")
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file %s: %v", name, err)
	}

	return data
}

// SaveGoldenFile saves data as a golden file (for manual golden file creation)
func SaveGoldenFile(t *testing.T, name string, data []byte) {
	t.Helper()

	goldenPath := filepath.Join(TestDataDir(), "..", "unit", "kiro", "testdata", "golden", name+".golden")
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
		t.Fatalf("Failed to create golden directory: %v", err)
	}
	if err := os.WriteFile(goldenPath, data, 0644); err != nil {
		t.Fatalf("Failed to write golden file: %v", err)
	}
	t.Log("Saved golden file:", name)
}

// IsGoldenUpdate returns true if we're in golden file update mode
func IsGoldenUpdate() bool {
	return updateGolden
}

// CreateSymlink creates a symlink (for test data organization)
func CreateSymlink(t *testing.T, target, linkPath string) {
	t.Helper()

	// Remove existing symlink if it exists
	os.Remove(linkPath)

	// Create directory for link
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		t.Fatalf("Failed to create link directory: %v", err)
	}

	// Create symlink
	if err := os.Symlink(target, linkPath); err != nil {
		t.Logf("Warning: Failed to create symlink %s -> %s: %v (this is OK on Windows)", linkPath, target, err)
		// On Windows, try copying instead
		copyFile(t, target, linkPath)
	}
}

// copyFile copies a file (fallback for Windows symlinks)
func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("Failed to read source file: %v", err)
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("Failed to write destination file: %v", err)
	}
}

// TestFixtureInfo provides information about a test fixture
type TestFixtureInfo struct {
	Name        string
	Description string
	Category    string
}

// String implements fmt.Stringer
func (tfi TestFixtureInfo) String() string {
	return fmt.Sprintf("[%s] %s: %s", tfi.Category, tfi.Name, tfi.Description)
}
