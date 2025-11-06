package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("golden", false, "update golden files")
var updateGoldenEnv = os.Getenv("UPDATE_GOLDEN") == "1"

// AssertGoldenBytes compares bytes with golden file content or updates the golden file
func AssertGoldenBytes(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("testdata", "golden", name+".golden")
	if *updateGolden || updateGoldenEnv {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, got, 0o644))
	}
	want, err := os.ReadFile(p)
	require.NoError(t, err, "missing golden: %s", p)
	require.Equal(t, string(want), string(got))
}

// AssertGoldenString compares string with golden file content or updates the golden file
func AssertGoldenString(t *testing.T, name string, got string) {
	t.Helper()
	AssertGoldenBytes(t, name, []byte(got))
}

// AssertMatchesGolden compares bytes with golden file content from shared golden directory
func AssertMatchesGolden(t *testing.T, got []byte, relPath string) {
	t.Helper()

	// Try to find the shared golden directory relative to the current test
	sharedGoldenPath := filepath.Join("..", "..", "..", "shared", "golden", relPath)

	// Check if running from test directory structure
	if _, err := os.Stat(sharedGoldenPath); os.IsNotExist(err) {
		// Fallback to direct path
		sharedGoldenPath = filepath.Join("tests", "shared", "golden", relPath)
	}

	if *updateGolden || updateGoldenEnv {
		require.NoError(t, os.MkdirAll(filepath.Dir(sharedGoldenPath), 0o755))
		require.NoError(t, os.WriteFile(sharedGoldenPath, got, 0o644))
	}

	want, err := os.ReadFile(sharedGoldenPath)
	require.NoError(t, err, "missing golden file: %s", sharedGoldenPath)
	require.Equal(t, string(want), string(got))
}