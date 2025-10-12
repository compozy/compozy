package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeIdentifier_AllowsExpectedCharacters(t *testing.T) {
	id, err := sanitizeIdentifier("Valid_ID-123", "vector store id")
	require.NoError(t, err)
	require.Equal(t, "Valid_ID-123", id)
}

func TestSanitizeIdentifier_RejectsWhitespace(t *testing.T) {
	_, err := sanitizeIdentifier(" bad-id ", "vector store id")
	require.Error(t, err)
}

func TestSanitizeIdentifier_RejectsUnsupportedCharacters(t *testing.T) {
	_, err := sanitizeIdentifier("bad$id", "vector store id")
	require.Error(t, err)
}
