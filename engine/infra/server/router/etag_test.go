package router

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseStrongETag(t *testing.T) {
	t.Run("Should return empty string when header missing", func(t *testing.T) {
		etag, err := ParseStrongETag("")
		require.NoError(t, err)
		require.Equal(t, "", etag)
	})
	t.Run("Should trim quotes and return value", func(t *testing.T) {
		etag, err := ParseStrongETag("\"abc123\"")
		require.NoError(t, err)
		require.Equal(t, "abc123", etag)
	})
	t.Run("Should use first value when multiple provided", func(t *testing.T) {
		etag, err := ParseStrongETag("\"first\", \"second\"")
		require.NoError(t, err)
		require.Equal(t, "first", etag)
	})
	t.Run("Should reject weak validators", func(t *testing.T) {
		_, err := ParseStrongETag("W/\"weak\"")
		require.Error(t, err)
	})
	t.Run("Should reject wildcard value", func(t *testing.T) {
		_, err := ParseStrongETag("*")
		require.Error(t, err)
	})
	t.Run("Should reject empty value after trimming", func(t *testing.T) {
		_, err := ParseStrongETag("\"\"")
		require.Error(t, err)
	})
}
