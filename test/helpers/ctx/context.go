package ctxhelpers

import (
	"context"
	"testing"

	"github.com/compozy/compozy/pkg/logger"
)

func TestContext(t *testing.T) context.Context {
	t.Helper()
	return logger.ContextWithLogger(t.Context(), logger.NewForTests())
}
