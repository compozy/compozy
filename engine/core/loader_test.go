package core

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRejectDollarKeys(t *testing.T) {
	ctx := t.Context()
	t.Run("Should allow $schema under schema context", func(t *testing.T) {
		y := []byte("input:\n  schema:\n    $schema: http://json-schema.org/draft-07/schema#\n    type: object\n")
		require.NoError(t, rejectDollarKeys(ctx, y, "test.yaml"))
	})
	t.Run("Should reject $ at root outside schema context", func(t *testing.T) {
		y := []byte("$ref: something")
		err := rejectDollarKeys(ctx, y, "test.yaml")
		require.Error(t, err)
		require.Contains(t, err.Error(), "test.yaml:1:1:")
		require.Contains(t, err.Error(), "unsupported directive key '$ref'")
	})
	t.Run("Should allow nested $ref inside schema", func(t *testing.T) {
		y := []byte(
			"input:\n  schema:\n    type: object\n    properties:\n      user:\n        $ref: '#/components/schemas/User'\n",
		)
		require.NoError(t, rejectDollarKeys(ctx, y, "test.yaml"))
	})
	t.Run("Should allow pure schema documents", func(t *testing.T) {
		y := []byte("$schema: http://json-schema.org/draft-07/schema#\ntitle: T\n")
		require.NoError(t, rejectDollarKeys(ctx, y, "schema.yaml"))
	})
}

func TestIsSchemaMapping(t *testing.T) {
	t.Run("Should return false for mapping with only type", func(t *testing.T) {
		n := &yaml.Node{Kind: yaml.MappingNode}
		n.Content = append(
			n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "type"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "string"},
		)
		result := isSchemaMapping(n)
		require.False(t, result)
	})
	t.Run("Should return true when schema sentinel present", func(t *testing.T) {
		n := &yaml.Node{Kind: yaml.MappingNode}
		n.Content = append(
			n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "$schema"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "http://json-schema"},
		)
		result := isSchemaMapping(n)
		require.True(t, result)
	})
	t.Run("Should return true when structural keys present", func(t *testing.T) {
		n := &yaml.Node{Kind: yaml.MappingNode}
		n.Content = append(
			n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "properties"},
			&yaml.Node{Kind: yaml.SequenceNode},
		)
		result := isSchemaMapping(n)
		require.True(t, result)
	})
}
