package filesystem

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

func testContext(t *testing.T, root string, additional ...string) context.Context {
	t.Helper()
	ctx := t.Context()
	ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
	manager := config.NewManager(t.Context(), config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	cfg.Runtime.NativeTools.RootDir = root
	if len(additional) > 0 {
		cfg.Runtime.NativeTools.AdditionalRoots = append([]string{}, additional...)
	}
	return config.ContextWithManager(ctx, manager)
}

func callHandler(
	ctx context.Context,
	t *testing.T,
	handler builtin.Handler,
	payload map[string]any,
) (core.Output, *core.Error) {
	t.Helper()
	output, err := handler(ctx, payload)
	if err == nil {
		return output, nil
	}
	var coreErr *core.Error
	if errors.As(err, &coreErr) {
		return nil, coreErr
	}
	t.Fatalf("expected core.Error, got %v", err)
	return nil, nil
}
