package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// LoadTestData loads test data from the shared testdata directory
func LoadTestData(t *testing.T, relPath string) []byte {
	t.Helper()

	// Try multiple path strategies to find the shared testdata directory
	var paths []string

	// Strategy 1: Relative to current test location (for unit/integration/regression tests)
	// From tests/unit/kiro or tests/integration/kiro, we need to go up 2 levels to reach tests/
	paths = append(paths, filepath.Join("..", "..", "shared", "testdata", relPath))

	// Strategy 2: Direct path from repo root
	paths = append(paths, filepath.Join("tests", "shared", "testdata", relPath))

	var data []byte
	var err error

	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			return data
		}
	}

	require.NoError(t, err, "failed to load test data from any of these paths: %v", paths)
	return nil
}

// TempDir returns a temporary directory for the test, wrapped for consistency
func TempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// WriteFile writes data to a file path, creating directories as needed
func WriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0o755)
	require.NoError(t, err, "failed to create directory: %s", dir)
	err = os.WriteFile(path, data, 0o644)
	require.NoError(t, err, "failed to write file: %s", path)
}