package testutils

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// FixturePath returns the absolute path to a fixture file in a package's testdata directory
func FixturePath(pkgPath string, name string) string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(wd, pkgPath, "testdata", name)
}

// SetupTestDir provides a test setup helper that creates a temporary directory
// and returns cleanup function
func SetupTestDir() (string, func()) {
	tmpDir := filepath.Join(os.TempDir(), "compozy-test-"+uuid.New().String())
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		panic(err)
	}

	return tmpDir, func() {
		os.RemoveAll(tmpDir)
	}
}

// CopyFixture copies a fixture file to the temporary test directory
func CopyFixture(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// SetupFixture sets up a test fixture by copying it to a temporary directory
func SetupFixture(t *testing.T, pkgPath string, fixtureName string) string {
	t.Helper()

	// Construct source path relative to the package path
	srcPath := filepath.Join(pkgPath, "testdata", fixtureName)
	dstPath := filepath.Join(t.TempDir(), fixtureName)

	src, err := os.Open(srcPath)
	require.NoError(t, err)
	defer src.Close()

	dst, err := os.Create(dstPath)
	require.NoError(t, err)
	defer dst.Close()

	_, err = io.Copy(dst, src)
	require.NoError(t, err)

	return dstPath
}
