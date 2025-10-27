package runtime

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	engineruntime "github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

type recordingLogger struct {
	messages []string
}

func (l *recordingLogger) Debug(msg string, _ ...any) {
	l.messages = append(l.messages, msg)
}

func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(string, ...any)  {}
func (l *recordingLogger) Error(string, ...any) {}
func (l *recordingLogger) With(...any) logger.Logger {
	return l
}

func TestNewBunInitializesConfig(t *testing.T) {
	t.Parallel()

	builder := NewBun()
	require.NotNil(t, builder)
	require.Equal(t, engineruntime.RuntimeTypeBun, builder.config.RuntimeType)
	require.NotEmpty(t, builder.config.BunPermissions)
}

func TestWithEntrypointTrimsValue(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithEntrypoint("  ./tools/main.ts  ")
	require.Equal(t, "./tools/main.ts", builder.config.EntrypointPath)
}

func TestWithEntrypointEmptyAddsError(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithEntrypoint("  ")
	require.NotEmpty(t, builder.errors)
}

func TestWithBunPermissionsStoresNormalizedValues(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithBunPermissions(" --allow-read ", "--ALLOW-NET", "--allow-read")
	require.Equal(t, []string{"--allow-read", "--allow-net"}, builder.config.BunPermissions)
}

func TestWithBunPermissionsPreservesScopeValues(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithBunPermissions("--allow-env=API_KEY,API_SECRET")
	require.Equal(t, []string{"--allow-env=API_KEY,API_SECRET"}, builder.config.BunPermissions)
}

func TestWithBunPermissionsInvalidAddsError(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithBunPermissions("--allow-read", "invalid")
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "invalid bun permission")
}

func TestWithBunPermissionsRejectsEmptyScopes(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithBunPermissions("--allow-net=")
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "invalid bun permission")
}

func TestWithBunPermissionsValidatesRuntimeType(t *testing.T) {
	t.Parallel()

	builder := NewBun()
	builder.config.RuntimeType = engineruntime.RuntimeTypeNode
	builder = builder.WithBunPermissions("--allow-read")
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[0].Error(), "bun permissions can only be used")
}

func TestWithBunPermissionsAggregatesErrors(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithBunPermissions(" ", "--allow-net=", "--allow-env")
	require.GreaterOrEqual(t, len(builder.errors), 2)
}

func TestWithMaxMemoryMBStoresValue(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithMaxMemoryMB(512)
	require.Equal(t, 512, builder.config.MaxMemoryMB)
}

func TestWithMaxMemoryMBInvalidAddsError(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithMaxMemoryMB(0)
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "max memory must be positive")
}

func TestWithToolTimeoutStoresValue(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithToolTimeout(45 * time.Second)
	require.Equal(t, 45*time.Second, builder.config.ToolExecutionTimeout)
}

func TestWithToolTimeoutInvalidAddsError(t *testing.T) {
	t.Parallel()

	builder := NewBun().WithToolTimeout(0)
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "tool timeout must be positive")
	require.NotZero(t, builder.config.ToolExecutionTimeout)
}

func TestBuildReturnsClonedConfig(t *testing.T) {
	t.Parallel()

	recLogger := &recordingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), recLogger)
	builder := NewBun().
		WithEntrypoint("./tools/main.ts").
		WithBunPermissions("--allow-read", "--allow-env").
		WithMaxMemoryMB(256)

	config, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotSame(t, builder.config, config)
	require.Equal(t, "./tools/main.ts", config.EntrypointPath)
	require.Equal(t, 256, config.MaxMemoryMB)
	require.Equal(t, []string{"--allow-read", "--allow-env"}, config.BunPermissions)
	require.NotEmpty(t, recLogger.messages)
}

func TestBuildFailsWhenEntrypointMissing(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := NewBun().WithMaxMemoryMB(128)
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.True(t, errors.Is(err, buildErr))
}

func TestBuildFailsForInvalidPermissions(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := NewBun().
		WithEntrypoint("./tools/main.ts").
		WithBunPermissions("invalid")
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.GreaterOrEqual(t, len(buildErr.Errors), 1)
}

func TestBuildFailsForInvalidMemory(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := NewBun().
		WithEntrypoint("./tools/main.ts").
		WithMaxMemoryMB(-32)
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
}

func TestBuildFailsForInvalidToolTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := NewBun().WithEntrypoint("./tools/main.ts")
	builder.config.ToolExecutionTimeout = 0
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
}

func TestBuildFailsWhenPermissionsMismatchRuntimeType(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := NewBun().WithEntrypoint("./tools/main.ts")
	builder.config.RuntimeType = engineruntime.RuntimeTypeNode
	builder.config.BunPermissions = []string{"--allow-read"}
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
}

func TestWithNativeToolsConfiguresRuntime(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tools := &engineruntime.NativeToolsConfig{CallAgents: true, CallWorkflows: true}
	builder := NewBun().
		WithEntrypoint("./tools/main.ts").
		WithNativeTools(tools)
	config, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotNil(t, config.NativeTools)
	require.True(t, config.NativeTools.CallAgents)
	require.True(t, config.NativeTools.CallWorkflows)
	require.NotSame(t, tools, config.NativeTools)
}

func TestRuntimeBuilderIntegrationWithEngineConfig(t *testing.T) {
	t.Parallel()

	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	builder := NewBun().
		WithEntrypoint("./tools/main.ts").
		WithBunPermissions("--allow-read", "--allow-net=api.example.com").
		WithToolTimeout(30 * time.Second)

	config, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, config)

	merged := engineruntime.MergeWithDefaults(config)
	require.Equal(t, config.BunPermissions, merged.BunPermissions)
	require.Equal(t, config.ToolExecutionTimeout, merged.ToolExecutionTimeout)
	require.Equal(t, config.EntrypointPath, merged.EntrypointPath)
}
