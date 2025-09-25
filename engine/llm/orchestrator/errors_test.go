package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRetryableError(t *testing.T) {
	t.Run("Should treat deadline exceeded as retryable", func(t *testing.T) {
		assert.True(t, isRetryableError(context.DeadlineExceeded))
	})
	t.Run("Should treat context canceled as non retryable", func(t *testing.T) {
		assert.False(t, isRetryableError(context.Canceled))
	})
}
