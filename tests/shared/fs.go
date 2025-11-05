package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ReadTestData reads a file from fixtures directory
func ReadTestData(t *testing.T, relativePath string) []byte {
	t.Helper()
	path := filepath.Join("fixtures", relativePath)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read fixtures file: %s", path)
	return data
}

// WriteTestData writes data to a file in fixtures directory
func WriteTestData(t *testing.T, relativePath string, data []byte) {
	t.Helper()
	path := filepath.Join("fixtures", relativePath)
	dir := filepath.Dir(path)
	require.NoError(t, os.MkdirAll(dir, 0o755), "failed to create directory: %s", dir)
	require.NoError(t, os.WriteFile(path, data, 0o644), "failed to write fixtures file: %s", path)
}