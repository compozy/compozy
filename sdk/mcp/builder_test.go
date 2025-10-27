package mcp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
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

func TestNewTrimsIDAndSetsResource(t *testing.T) {
	t.Parallel()

	builder := New("  filesystem  ")
	require.NotNil(t, builder)
	require.Equal(t, "filesystem", builder.config.ID)
	require.Equal(t, string(core.ConfigMCP), builder.config.Resource)
}

func TestWithCommandStoresCommandAndArgs(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithCommand("  mcp-server-filesystem  ", "  --root", " /data ")
	require.Equal(t, "mcp-server-filesystem", builder.config.Command)
	require.Equal(t, []string{"--root", "/data"}, builder.config.Args)
	require.Equal(t, mcpproxy.TransportStdio, builder.config.Transport)
}

func TestWithCommandEmptyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithCommand(" ")
	require.NotEmpty(t, builder.errors)
}

func TestWithURLStoresURL(t *testing.T) {
	t.Parallel()

	builder := New("github").WithURL("  https://api.github.com/mcp?v=1 ")
	require.Equal(t, "https://api.github.com/mcp?v=1", builder.config.URL)
	require.Equal(t, enginemcp.DefaultTransport, builder.config.Transport)
}

func TestWithURLEmptyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("github").WithURL(" ")
	require.NotEmpty(t, builder.errors)
}

func TestBuildReturnsCommandConfig(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithCommand("mcp-server", "--root", "/data")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotSame(t, builder.config, cfg)
	require.Equal(t, "filesystem", cfg.ID)
	require.Equal(t, "mcp-server", cfg.Command)
	require.Equal(t, []string{"--root", "/data"}, cfg.Args)
	require.Equal(t, mcpproxy.TransportStdio, cfg.Transport)
}

func TestBuildReturnsURLConfig(t *testing.T) {
	t.Parallel()

	builder := New("github").WithURL("https://example.com/mcp?v=2")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "https://example.com/mcp?v=2", cfg.URL)
	require.Equal(t, enginemcp.DefaultTransport, cfg.Transport)
}

func TestBuildFailsWithoutCommandOrURL(t *testing.T) {
	t.Parallel()

	builder := New("filesystem")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "either command or url must be configured")
}

func TestBuildFailsWhenBothConfigured(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithCommand("mcp-server").WithURL("https://example.com/mcp")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "configure either command or url")
}

func TestBuildAggregatesErrors(t *testing.T) {
	t.Parallel()

	builder := New("bad id").WithCommand(" ").WithURL(" ")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.True(t, errors.Is(err, buildErr))
	require.GreaterOrEqual(t, len(buildErr.Errors), 3)
}

func TestBuildFailsWithNilContext(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithCommand("mcp-server")
	cfg, err := builder.Build(nil)
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestBuildUsesLoggerFromContext(t *testing.T) {
	t.Parallel()

	recLogger := &recordingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), recLogger)

	builder := New("filesystem").WithCommand("mcp-server")
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotEmpty(t, recLogger.debugMessages)
	require.Contains(t, recLogger.debugMessages[0], "building MCP configuration")
}
