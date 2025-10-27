package tool

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	engineschema "github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
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

func TestNewCreatesBuilderWithTrimmedID(t *testing.T) {
	t.Parallel()

	builder := New("  sample-tool  ")
	require.NotNil(t, builder)
	require.Equal(t, "sample-tool", builder.config.ID)
	require.Equal(t, string(core.ConfigTool), builder.config.Resource)
}

func TestWithNameTrimsAndStoresValue(t *testing.T) {
	t.Parallel()

	builder := New("tool-id").WithName("  Formatter Tool ")
	require.Equal(t, "Formatter Tool", builder.config.Name)
}

func TestWithNameEmptyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("tool-id").WithName(" ")
	require.NotEmpty(t, builder.errors)
}

func TestWithDescriptionTrimsAndStoresValue(t *testing.T) {
	t.Parallel()

	builder := New("tool-id").WithDescription("  Formats input ")
	require.Equal(t, "Formats input", builder.config.Description)
}

func TestWithDescriptionEmptyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("tool-id").WithDescription("")
	require.NotEmpty(t, builder.errors)
}

func TestWithRuntimeNormalizesCase(t *testing.T) {
	t.Parallel()

	builder := New("tool-id").WithRuntime("  BUN  ")
	require.Equal(t, "bun", builder.config.Runtime)
}

func TestWithRuntimeInvalidAddsError(t *testing.T) {
	t.Parallel()

	builder := New("tool-id").WithRuntime("python")
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "runtime must be bun")
}

func TestWithCodeTrimsAndStoresValue(t *testing.T) {
	t.Parallel()

	builder := New("tool-id").WithCode("  export default {} ")
	require.Equal(t, "export default {}", builder.config.Code)
}

func TestWithCodeEmptyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("tool-id").WithCode("")
	require.NotEmpty(t, builder.errors)
}

func TestWithInputAndOutputAssignSchemas(t *testing.T) {
	t.Parallel()

	inputSchema := engineschema.Schema{"type": "object"}
	outputSchema := engineschema.Schema{"type": "object"}

	builder := New("tool-id").
		WithName("Formatter").
		WithDescription("Formats code").
		WithRuntime("bun").
		WithCode("export default {}").
		WithInput(&inputSchema).
		WithOutput(&outputSchema)

	require.Equal(t, &inputSchema, builder.config.InputSchema)
	require.Equal(t, &outputSchema, builder.config.OutputSchema)
}

func TestBuildReturnsClonedConfig(t *testing.T) {
	t.Parallel()

	inputSchema := engineschema.Schema{"type": "object"}
	outputSchema := engineschema.Schema{"type": "object"}

	builder := New("formatter").
		WithName("Formatter").
		WithDescription("Formats code samples").
		WithRuntime("bun").
		WithCode("export default function() {}").
		WithInput(&inputSchema).
		WithOutput(&outputSchema)

	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotSame(t, builder.config, cfg)
	require.Equal(t, "formatter", cfg.ID)
	require.Equal(t, "Formatter", cfg.Name)
	require.Equal(t, "Formats code samples", cfg.Description)
	require.Equal(t, "bun", cfg.Runtime)
	require.Equal(t, "export default function() {}", cfg.Code)
	require.Equal(t, cfg.InputSchema, &inputSchema)
	require.Equal(t, cfg.OutputSchema, &outputSchema)

	cfg.Name = "modified"
	require.Equal(t, "Formatter", builder.config.Name)
}

func TestBuildAggregatesValidationErrors(t *testing.T) {
	t.Parallel()

	builder := New(" ").
		WithName("").
		WithDescription("").
		WithRuntime("python").
		WithCode("")

	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.True(t, errors.Is(err, buildErr))
	require.GreaterOrEqual(t, len(buildErr.Errors), 4)
}

func TestBuildFailsWhenCodeMissing(t *testing.T) {
	t.Parallel()

	builder := New("formatter").
		WithName("Formatter").
		WithDescription("Formats code").
		WithRuntime("bun")

	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestBuildFailsWithNilContext(t *testing.T) {
	t.Parallel()

	builder := New("formatter").
		WithName("Formatter").
		WithDescription("Formats code").
		WithRuntime("bun").
		WithCode("export default {}")

	cfg, err := builder.Build(nil)
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestBuildUsesLoggerFromContext(t *testing.T) {
	t.Parallel()

	recLogger := &recordingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), recLogger)

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
