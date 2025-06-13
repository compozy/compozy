package utils

import (
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/google/uuid"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
)

func SetupTest(t *testing.T, filename string) (*core.PathCWD, string) {
	testDir := filepath.Dir(filename)
	cwd, err := core.CWDFromPath(testDir)
	require.NoError(t, err)
	dstPath := SetupFixture(t, testDir)
	return cwd, dstPath
}

func SetupFixture(t *testing.T, pkgPath string) string {
	t.Helper()

	// Construct source path relative to the package path
	srcPath := filepath.Join(pkgPath, "fixtures")
	dstPath := filepath.Join(t.TempDir(), "compozy-test-"+uuid.New().String())

	// Use the copy library to recursively copy the directory
	err := copy.Copy(srcPath, dstPath)
	require.NoError(t, err)

	return dstPath
}
