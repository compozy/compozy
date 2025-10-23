package monitoring

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func attrString(t *testing.T, set attribute.Set, key string) string {
	t.Helper()
	value, ok := set.Value(attribute.Key(key))
	require.True(t, ok, "expected attribute %q", key)
	return value.AsString()
}

func attrInt(t *testing.T, set attribute.Set, key string) int64 {
	t.Helper()
	value, ok := set.Value(attribute.Key(key))
	require.True(t, ok, "expected attribute %q", key)
	return value.AsInt64()
}
