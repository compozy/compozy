package memory

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

type capturingLogger struct {
	debugMessages []string
}

func (l *capturingLogger) Debug(msg string, _ ...any) {
	l.debugMessages = append(l.debugMessages, msg)
}

func (l *capturingLogger) Info(string, ...any)  {}
func (l *capturingLogger) Warn(string, ...any)  {}
func (l *capturingLogger) Error(string, ...any) {}
func (l *capturingLogger) With(...any) logger.Logger {
	return l
}

func TestNewReferenceInitializesDefaults(t *testing.T) {
	t.Parallel()
	builder := NewReference("  session-store  ")
	require.NotNil(t, builder)
	require.Equal(t, "session-store", builder.config.ID)
	require.Equal(t, core.MemoryModeReadWrite, builder.config.Mode)
	require.Empty(t, builder.errors)
}

func TestBuildProducesReferenceConfig(t *testing.T) {
	t.Parallel()
	rec := &capturingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), rec)
	builder := NewReference("conversation-memory").WithKey(" conversation-{{.conversation.id}} ")
	ref, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, ref)
	assert.Equal(t, "conversation-memory", ref.ID)
	assert.Equal(t, core.MemoryModeReadWrite, ref.Mode)
	assert.Equal(t, "conversation-{{.conversation.id}}", ref.Key)
	require.NotSame(t, builder.config, ref)
	ref.Key = "override"
	assert.NotEqual(t, ref.Key, builder.config.Key)
	assert.NotEmpty(t, rec.debugMessages)
}

func TestBuildRequiresContext(t *testing.T) {
	t.Parallel()
	_, err := NewReference("memory").Build(nil)
	require.Error(t, err)
	assert.EqualError(t, err, "context is required")
}

func TestBuildValidatesIdentifier(t *testing.T) {
	t.Parallel()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	ref, err := NewReference("   ").Build(ctx)
	require.Error(t, err)
	require.Nil(t, ref)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.ErrorContains(t, buildErr, "id is required")
	assert.True(t, errors.Is(err, buildErr))
}

func TestWithKeyRejectsEmptyTemplate(t *testing.T) {
	t.Parallel()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	builder := NewReference("memory").WithKey("   ")
	ref, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, ref)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.ErrorContains(t, buildErr, "memory key template cannot be empty")
}

func TestBuildAggregatesErrors(t *testing.T) {
	t.Parallel()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	builder := NewReference("   ").WithKey(" ")
	ref, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, ref)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.GreaterOrEqual(t, len(buildErr.Errors), 2)
}
