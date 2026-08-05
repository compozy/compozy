package workspace

import (
	"testing"
)

func TestResolveCacheHitInvalidateAndEviction(t *testing.T) {
	t.Parallel()
	t.Run("Should isolate cached config and invalidate every watched input", func(t *testing.T) {
		t.Parallel()
		runResolveCacheHitInvalidateAndEviction(t)
	})
}
