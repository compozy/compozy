package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVersion(t *testing.T) {
	t.Run("Should create valid version from string", func(t *testing.T) {
		version, err := NewVersion("1.2.3")
		require.NoError(t, err)
		assert.NotNil(t, version)
		assert.Equal(t, "v1.2.3", version.String())
	})
	t.Run("Should return error for invalid version string", func(t *testing.T) {
		version, err := NewVersion("invalid")
		assert.Error(t, err)
		assert.Nil(t, version)
	})
	t.Run("Should handle version with v prefix", func(t *testing.T) {
		version, err := NewVersion("v1.2.3")
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", version.String())
	})
}

func TestVersion_BumpMajor(t *testing.T) {
	t.Run("Should bump major version correctly", func(t *testing.T) {
		version, err := NewVersion("1.2.3")
		require.NoError(t, err)
		newVersion := version.BumpMajor()
		assert.Equal(t, "v2.0.0", newVersion.String())
	})
	t.Run("Should reset minor and patch when bumping major", func(t *testing.T) {
		version, err := NewVersion("1.5.8")
		require.NoError(t, err)
		newVersion := version.BumpMajor()
		assert.Equal(t, "v2.0.0", newVersion.String())
	})
}

func TestVersion_BumpMinor(t *testing.T) {
	t.Run("Should bump minor version correctly", func(t *testing.T) {
		version, err := NewVersion("1.2.3")
		require.NoError(t, err)
		newVersion := version.BumpMinor()
		assert.Equal(t, "v1.3.0", newVersion.String())
	})
	t.Run("Should reset patch when bumping minor", func(t *testing.T) {
		version, err := NewVersion("1.2.5")
		require.NoError(t, err)
		newVersion := version.BumpMinor()
		assert.Equal(t, "v1.3.0", newVersion.String())
	})
}

func TestVersion_BumpPatch(t *testing.T) {
	t.Run("Should bump patch version correctly", func(t *testing.T) {
		version, err := NewVersion("1.2.3")
		require.NoError(t, err)
		newVersion := version.BumpPatch()
		assert.Equal(t, "v1.2.4", newVersion.String())
	})
	t.Run("Should only increment patch version", func(t *testing.T) {
		version, err := NewVersion("2.5.0")
		require.NoError(t, err)
		newVersion := version.BumpPatch()
		assert.Equal(t, "v2.5.1", newVersion.String())
	})
}

func TestVersion_Compare(t *testing.T) {
	t.Run("Should compare versions correctly", func(t *testing.T) {
		v1, err := NewVersion("1.2.3")
		require.NoError(t, err)
		v2, err := NewVersion("1.2.4")
		require.NoError(t, err)
		v3, err := NewVersion("1.2.3")
		require.NoError(t, err)
		assert.Equal(t, -1, v1.Compare(v2))
		assert.Equal(t, 1, v2.Compare(v1))
		assert.Equal(t, 0, v1.Compare(v3))
	})
	t.Run("Should handle major version differences", func(t *testing.T) {
		v1, err := NewVersion("1.0.0")
		require.NoError(t, err)
		v2, err := NewVersion("2.0.0")
		require.NoError(t, err)
		assert.Equal(t, -1, v1.Compare(v2))
		assert.Equal(t, 1, v2.Compare(v1))
	})
}

func TestVersion_String(t *testing.T) {
	t.Run("Should return version string with v prefix", func(t *testing.T) {
		version, err := NewVersion("v1.2.3")
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", version.String())
	})
	t.Run("Should handle prerelease versions", func(t *testing.T) {
		version, err := NewVersion("1.2.3-alpha")
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3-alpha", version.String())
	})
	t.Run("Should handle build metadata", func(t *testing.T) {
		version, err := NewVersion("1.2.3+build123")
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3+build123", version.String())
	})
}
