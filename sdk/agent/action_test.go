package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

func TestNewActionBuilderTrimsID(t *testing.T) {
	t.Parallel()
	builder := NewAction("  action-id  ")
	require.NotNil(t, builder)
	require.Equal(t, "action-id", builder.config.ID)
}

func TestWithPromptValidatesNonEmpty(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("   ")
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
}

func TestWithOutputAssignsSchema(t *testing.T) {
	t.Parallel()
	out := schema.Schema{"type": "object"}
	builder := NewAction("action").WithPrompt("prompt").WithOutput(&out)
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.OutputSchema)
	require.Equal(t, &out, cfg.OutputSchema)
}

func TestAddToolScopesToolToAction(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").AddTool("search").AddTool("calc")
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Len(t, cfg.Tools, 2)
	require.Equal(t, "search", cfg.Tools[0].ID)
	require.Equal(t, "calc", cfg.Tools[1].ID)
}

func TestWithSuccessTransitionSetsNextTask(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").WithSuccessTransition("next-task")
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.OnSuccess)
	require.NotNil(t, cfg.OnSuccess.Next)
	require.Equal(t, "next-task", *cfg.OnSuccess.Next)
}

func TestWithErrorTransitionSetsNextTask(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").WithErrorTransition("recover-task")
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.OnError)
	require.NotNil(t, cfg.OnError.Next)
	require.Equal(t, "recover-task", *cfg.OnError.Next)
}

func TestWithRetryConfiguresPolicy(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").WithRetry(3, 2*time.Second)
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.RetryPolicy)
	require.Equal(t, int32(3), cfg.RetryPolicy.MaximumAttempts)
	require.Equal(t, "2s", cfg.RetryPolicy.InitialInterval)
}

func TestWithRetryRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").WithRetry(0, time.Second)
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestWithTimeoutStoresDuration(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").WithTimeout(5 * time.Second)
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Equal(t, "5s", cfg.Timeout)
}

func TestWithTimeoutRejectsNonPositive(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").WithTimeout(0)
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestActionBuildProducesClone(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").AddTool("one")
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	cfg.Tools[0].ID = "mutated"
	cloned, cloneErr := builder.Build(ctx)
	require.NoError(t, cloneErr)
	require.Equal(t, "one", cloned.Tools[0].ID)
}

func TestActionBuilderFailureIncludesIDValidation(t *testing.T) {
	t.Parallel()
	builder := NewAction("   ").WithPrompt("prompt")
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
}

func TestWithErrorTransitionClearsPreviousValue(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").WithErrorTransition("a").WithErrorTransition("b")
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.OnError)
	require.Equal(t, "b", *cfg.OnError.Next)
}

func TestWithSuccessTransitionClearsPreviousValue(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").WithSuccessTransition("a").WithSuccessTransition("b")
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.OnSuccess)
	require.Equal(t, "b", *cfg.OnSuccess.Next)
}

func TestAddToolRejectsEmptyIdentifiers(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").AddTool(" ")
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestWithRetryOverridesPreviousPolicy(t *testing.T) {
	t.Parallel()
	builder := NewAction("action").WithPrompt("prompt").WithRetry(2, time.Second).WithRetry(5, 3*time.Second)
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Equal(t, int32(5), cfg.RetryPolicy.MaximumAttempts)
	require.Equal(t, "3s", cfg.RetryPolicy.InitialInterval)
}
