package runtime

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	engineruntime "github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/testutil"
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
	builder := NewBun()
	require.NotNil(t, builder)
	require.Equal(t, engineruntime.RuntimeTypeBun, builder.config.RuntimeType)
	require.NotEmpty(t, builder.config.BunPermissions)
}

func TestNilBuilderFluentMethods(t *testing.T) {
	var builder *Builder
	require.Nil(t, builder.WithEntrypoint("./main.ts"))
	require.Nil(t, builder.WithBunPermissions("--allow-read"))
	require.Nil(t, builder.WithToolTimeout(time.Second))
	require.Nil(t, builder.WithNativeTools(&engineruntime.NativeToolsConfig{}))
	require.Nil(t, builder.WithMaxMemoryMB(128))
	cfg, err := builder.Build(testutil.NewTestContext(t))
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "runtime builder is required")
}

func TestWithEntrypointTrimsValue(t *testing.T) {
	builder := NewBun().WithEntrypoint("  ./tools/main.ts  ")
	require.Equal(t, "./tools/main.ts", builder.config.EntrypointPath)
}

func TestWithEntrypointEmptyAddsError(t *testing.T) {
	builder := NewBun().WithEntrypoint("  ")
	require.NotEmpty(t, builder.errors)
}

func TestWithBunPermissionsStoresNormalizedValues(t *testing.T) {
	builder := NewBun().WithBunPermissions(" --allow-read ", "--ALLOW-NET", "--allow-read")
	require.Equal(t, []string{"--allow-read", "--allow-net"}, builder.config.BunPermissions)
}

func TestWithBunPermissionsPreservesScopeValues(t *testing.T) {
	builder := NewBun().WithBunPermissions("--allow-env=API_KEY,API_SECRET")
	require.Equal(t, []string{"--allow-env=API_KEY,API_SECRET"}, builder.config.BunPermissions)
}

func TestWithBunPermissionsInvalidAddsError(t *testing.T) {
	builder := NewBun().WithBunPermissions("--allow-read", "invalid")
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "invalid bun permission")
}

func TestWithBunPermissionsRejectsEmptyScopes(t *testing.T) {
	builder := NewBun().WithBunPermissions("--allow-net=")
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "invalid bun permission")
}

func TestWithBunPermissionsValidatesRuntimeType(t *testing.T) {
	builder := NewBun()
	builder.config.RuntimeType = engineruntime.RuntimeTypeNode
	builder = builder.WithBunPermissions("--allow-read")
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[0].Error(), "bun permissions can only be used")
}

func TestWithBunPermissionsAggregatesErrors(t *testing.T) {
	builder := NewBun().WithBunPermissions(" ", "--allow-net=", "--allow-env")
	require.GreaterOrEqual(t, len(builder.errors), 2)
}

func TestWithBunPermissionsRequiresValues(t *testing.T) {
	builder := NewBun().WithBunPermissions()
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "at least one bun permission must be provided")
}

func TestWithMaxMemoryMBStoresValue(t *testing.T) {
	builder := NewBun().WithMaxMemoryMB(512)
	require.Equal(t, 512, builder.config.MaxMemoryMB)
}

func TestWithMaxMemoryMBInvalidAddsError(t *testing.T) {
	builder := NewBun().WithMaxMemoryMB(0)
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "max memory must be positive")
}

func TestWithToolTimeoutStoresValue(t *testing.T) {
	builder := NewBun().WithToolTimeout(45 * time.Second)
	require.Equal(t, 45*time.Second, builder.config.ToolExecutionTimeout)
}

func TestWithToolTimeoutInvalidAddsError(t *testing.T) {
	builder := NewBun().WithToolTimeout(0)
	require.NotEmpty(t, builder.errors)
	require.Contains(t, builder.errors[len(builder.errors)-1].Error(), "tool timeout must be positive")
	require.NotZero(t, builder.config.ToolExecutionTimeout)
}

func TestWithNativeToolsNilClearsConfig(t *testing.T) {
	builder := NewBun().WithNativeTools(&engineruntime.NativeToolsConfig{CallAgents: true})
	builder.WithNativeTools(nil)
	require.Nil(t, builder.config.NativeTools)
}

func TestBuildReturnsClonedConfig(t *testing.T) {
	recLogger := &recordingLogger{}
	ctx := logger.ContextWithLogger(testutil.NewTestContext(t), recLogger)
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
	ctx := testutil.NewTestContext(t)
	builder := NewBun().WithMaxMemoryMB(128)
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.True(t, errors.Is(err, buildErr))
}

func TestBuildFailsForInvalidPermissions(t *testing.T) {
	ctx := testutil.NewTestContext(t)
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
	ctx := testutil.NewTestContext(t)
	builder := NewBun().
		WithEntrypoint("./tools/main.ts").
		WithMaxMemoryMB(-32)
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
}

func TestBuildFailsForInvalidToolTimeout(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	builder := NewBun().WithEntrypoint("./tools/main.ts")
	builder.config.ToolExecutionTimeout = 0
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
}

func TestBuildFailsWhenPermissionsMissing(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	builder := NewBun().WithEntrypoint("./tools/main.ts")
	builder.config.BunPermissions = nil
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
	require.Contains(t, err.Error(), "at least one bun permission is required")
}

func TestBuildFailsWhenPermissionsMismatchRuntimeType(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	builder := NewBun().WithEntrypoint("./tools/main.ts")
	builder.config.RuntimeType = engineruntime.RuntimeTypeNode
	builder.config.BunPermissions = []string{"--allow-read"}
	config, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, config)
}

func TestWithNativeToolsConfiguresRuntime(t *testing.T) {
	ctx := testutil.NewTestContext(t)
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
	ctx := logger.ContextWithLogger(testutil.NewTestContext(t), logger.NewForTests())
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

func TestBuildCloneFailure(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	builder := NewBun().WithEntrypoint("./tools/main.ts")
	original := cloneRuntimeConfig
	cloneRuntimeConfig = func(*engineruntime.Config) (*engineruntime.Config, error) {
		return nil, fmt.Errorf("clone failed")
	}
	t.Cleanup(func() {
		cloneRuntimeConfig = original
	})
	cfg, err := builder.Build(ctx)
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to clone runtime config")
}

func TestBuildRequiresContext(t *testing.T) {
	builder := NewBun().WithEntrypoint("./tools/main.ts")
	var missingCtx context.Context
	cfg, err := builder.Build(missingCtx)
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context is required")
}
