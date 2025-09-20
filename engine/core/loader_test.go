package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRejectDollarKeys(t *testing.T) {
	t.Run("Should allow $schema under schema context", func(t *testing.T) {
		y := []byte("input:\n  schema:\n    $schema: http://json-schema.org/draft-07/schema#\n    type: object\n")
		require.NoError(t, rejectDollarKeys(y, "test.yaml"))
	})
	t.Run("Should reject $ at root outside schema context", func(t *testing.T) {
		y := []byte("$ref: something")
		err := rejectDollarKeys(y, "test.yaml")
		require.Error(t, err)
		require.Contains(t, err.Error(), "test.yaml:1:1:")
		require.Contains(t, err.Error(), "unsupported directive key '$ref'")
	})
	t.Run("Should allow nested $ref inside schema", func(t *testing.T) {
		y := []byte(
			"input:\n  schema:\n    type: object\n    properties:\n      user:\n        $ref: '#/components/schemas/User'\n",
		)
		require.NoError(t, rejectDollarKeys(y, "test.yaml"))
	})
	t.Run("Should allow pure schema documents", func(t *testing.T) {
		y := []byte("$schema: http://json-schema.org/draft-07/schema#\ntitle: T\n")
		require.NoError(t, rejectDollarKeys(y, "schema.yaml"))
	})
}
