package tool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetool "github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/testutil"
)

type recordingLogger struct {
	debugMessages []string
}

func (l *recordingLogger) Debug(msg string, _ ...any) {
	l.debugMessages = append(l.debugMessages, msg)
}

func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(string, ...any)  {}
func (l *recordingLogger) Error(string, ...any) {}
func (l *recordingLogger) With(...any) logger.Logger {
	return l
}

func TestNewTrimsIdentifier(t *testing.T) {
	builder := New("  formatter  ")
	require.Equal(t, "formatter", builder.config.ID)
	require.Equal(t, string(core.ConfigTool), builder.config.Resource)
}

func TestNilBuilderMethodsReturnNil(t *testing.T) {
	var builder *Builder
	require.Nil(t, builder.WithName("tool"))
	require.Nil(t, builder.WithDescription("desc"))
	require.Nil(t, builder.WithRuntime("bun"))
	require.Nil(t, builder.WithCode("code"))
	require.Nil(t, builder.WithInput(&engineschema.Schema{"type": "object"}))
	require.Nil(t, builder.WithOutput(&engineschema.Schema{"type": "object"}))
	cfg, err := builder.Build(testutil.NewTestContext(t))
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tool builder is required")
}

func TestWithSchemasAssignPointers(t *testing.T) {
	input := engineschema.Schema{"type": "object"}
	output := engineschema.Schema{"type": "object"}
	builder := New("formatter").
		WithInput(&input).
		WithOutput(&output)
	require.Equal(t, &input, builder.config.InputSchema)
	require.Equal(t, &output, builder.config.OutputSchema)
}

func TestWithRuntimeValidation(t *testing.T) {
	builder := New("formatter").
		WithRuntime("  BUN  ")
	require.Equal(t, "bun", builder.config.Runtime)
	builder.WithRuntime("python")
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "runtime must be bun")
}

func TestWithRuntimeEmptyAddsError(t *testing.T) {
	builder := New("formatter").
		WithRuntime(" ")
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "runtime cannot be empty")
}

func TestBuildSuccessReturnsClone(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	input := engineschema.Schema{"type": "object"}
	output := engineschema.Schema{"type": "object"}
	builder := New("formatter").
		WithName("Formatter").
		WithDescription("Formats code").
		WithRuntime("bun").
		WithCode("export default {}").
		WithInput(&input).
		WithOutput(&output)
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotSame(t, builder.config, cfg)
	cfg.Name = "mutated"
	require.Equal(t, "Formatter", builder.config.Name)
}

func TestBuildAggregatesValidationErrors(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	builder := New(" bad id ").
		WithName(" ").
		WithDescription("").
		WithRuntime("python").
		WithCode("")
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.GreaterOrEqual(t, len(buildErr.Errors), 5)
}

func TestBuildFailsWithNilContext(t *testing.T) {
	builder := New("formatter").
		WithName("Formatter").
		WithDescription("Formats code").
		WithRuntime("bun").
		WithCode("export default {}")
	cfg, err := builder.Build(nil)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "context is required")
}

func TestBuildRequiresRuntime(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	builder := New("formatter").
		WithName("Formatter").
		WithDescription("Formats code").
		WithCode("export default {}")
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "tool runtime")
}

func TestBuildCloneFailure(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	builder := New("formatter").
		WithName("Formatter").
		WithDescription("Formats code").
		WithRuntime("bun").
		WithCode("export default {}")
	original := cloneToolConfig
	cloneToolConfig = func(*enginetool.Config) (*enginetool.Config, error) {
		return nil, fmt.Errorf("clone failed")
	}
	t.Cleanup(func() {
		cloneToolConfig = original
	})
	cfg, err := builder.Build(ctx)
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to clone tool config")
}

func TestBuildLogsDebugMessage(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	recLogger := &recordingLogger{}
	ctx = logger.ContextWithLogger(ctx, recLogger)
	builder := New("formatter").
		WithName("Formatter").
		WithDescription("Formats code").
		WithRuntime("bun").
		WithCode("export default {}")
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotEmpty(t, recLogger.debugMessages)
	require.Contains(t, recLogger.debugMessages[0], "building tool configuration")
}
