package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

// AssertGoldenBytes compares bytes with golden file content or updates the golden file
func AssertGoldenBytes(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("fixtures", "golden", name+".golden")
	if *update {
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