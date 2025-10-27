package mcp

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestWithProtoStoresVersion(t *testing.T) {
	t.Parallel()

	builder := New("github").WithProto(" 2025-03-26 ")
	require.Equal(t, "2025-03-26", builder.config.Proto)
}

func TestWithProtoEmptyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("github").WithProto(" ")
	require.NotEmpty(t, builder.errors)
}

func TestWithTransportSetsTransport(t *testing.T) {
	t.Parallel()

	builder := New("github").WithTransport(mcpproxy.TransportStreamableHTTP)
	require.Equal(t, mcpproxy.TransportStreamableHTTP, builder.config.Transport)
}

func TestWithTransportInvalidAddsError(t *testing.T) {
	t.Parallel()

	builder := New("github").WithTransport(mcpproxy.TransportType("invalid"))
	require.NotEmpty(t, builder.errors)
}

func TestWithHeadersStoresHeaders(t *testing.T) {
	t.Parallel()

	headers := map[string]string{
		"Authorization": "Bearer {{ .env.GITHUB_TOKEN }}",
		"X-Trace":       "abc123",
	}
	builder := New("github").WithHeaders(headers)
	require.Equal(t, headers, builder.config.Headers)
	headers["Authorization"] = "mutated"
	require.NotEqual(t, headers["Authorization"], builder.config.Headers["Authorization"])
}

func TestWithHeaderAddsSingleHeader(t *testing.T) {
	t.Parallel()

	builder := New("github").WithHeader("Authorization", "Bearer token")
	require.Equal(t, "Bearer token", builder.config.Headers["Authorization"])
	builder = builder.WithHeader("Content-Type", "application/json")
	require.Equal(t, "application/json", builder.config.Headers["Content-Type"])
}

func TestWithHeaderEmptyKeyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("github").WithHeader(" ", "value")
	require.NotEmpty(t, builder.errors)
}

func TestWithEnvStoresEnv(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"ROOT_DIR": "/data",
		"TOKEN":    "{{ .env.API_TOKEN }}",
	}
	builder := New("filesystem").WithEnv(env)
	require.Equal(t, env, builder.config.Env)
	env["ROOT_DIR"] = "/other"
	require.NotEqual(t, env["ROOT_DIR"], builder.config.Env["ROOT_DIR"])
}

func TestWithEnvVarAddsSingleVar(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithEnvVar("ROOT_DIR", "/data")
	require.Equal(t, "/data", builder.config.Env["ROOT_DIR"])
	builder = builder.WithEnvVar("LOG_LEVEL", "debug")
	require.Equal(t, "debug", builder.config.Env["LOG_LEVEL"])
}

func TestWithEnvVarEmptyKeyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithEnvVar(" ", "value")
	require.NotEmpty(t, builder.errors)
}

func TestWithMaxSessionsStoresValue(t *testing.T) {
	t.Parallel()

	builder := New("github").WithMaxSessions(5)
	require.Equal(t, 5, builder.config.MaxSessions)
}

func TestWithMaxSessionsNegativeAddsError(t *testing.T) {
	t.Parallel()

	builder := New("github").WithMaxSessions(-1)
	require.NotEmpty(t, builder.errors)
}

func TestWithStartTimeoutSetsValue(t *testing.T) {
	t.Parallel()

	timeout := 15 * time.Second
	builder := New("filesystem").WithStartTimeout(timeout)
	require.Equal(t, timeout, builder.config.StartTimeout)
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

func TestBuildReturnsExplicitHTTPTransport(t *testing.T) {
	t.Parallel()

	builder := New("github").WithTransport(mcpproxy.TransportStreamableHTTP).WithURL("https://example.com/mcp")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, mcpproxy.TransportStreamableHTTP, cfg.Transport)
}

func TestBuildIncludesHeadersForURLBasedMCP(t *testing.T) {
	t.Parallel()

	builder := New("github").
		WithURL("https://example.com/mcp").
		WithHeader("Authorization", "Bearer {{ .env.GITHUB_TOKEN }}").
		WithProto("2025-03-26").
		WithMaxSessions(10)
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "Bearer {{ .env.GITHUB_TOKEN }}", cfg.Headers["Authorization"])
	require.Equal(t, "2025-03-26", cfg.Proto)
	require.Equal(t, 10, cfg.MaxSessions)
}

func TestBuildIncludesEnvForCommandBasedMCP(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").
		WithCommand("mcp-server-filesystem").
		WithEnvVar("ROOT_DIR", "/data").
		WithStartTimeout(20 * time.Second)
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "/data", cfg.Env["ROOT_DIR"])
	require.Equal(t, 20*time.Second, cfg.StartTimeout)
}

func TestBuildFailsWithInvalidProtoFormat(t *testing.T) {
	t.Parallel()

	builder := New("github").
		WithURL("https://example.com/mcp").
		WithProto("2025/03/26")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "protocol version must follow YYYY-MM-DD format")
}

func TestBuildFailsWithNegativeMaxSessions(t *testing.T) {
	t.Parallel()

	builder := New("github").
		WithURL("https://example.com/mcp").
		WithMaxSessions(-1)
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "max sessions cannot be negative")
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

func TestBuildFailsWhenStdioTransportWithURL(t *testing.T) {
	t.Parallel()

	builder := New("github").WithURL("https://example.com/mcp").WithTransport(mcpproxy.TransportStdio)
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "stdio transport")
}

func TestBuildFailsWhenHTTPTransportWithCommand(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithCommand("mcp-server").WithTransport(mcpproxy.TransportSSE)
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "transport cannot be used with command configuration")
}

func TestBuildFailsWhenHeadersWithoutURL(t *testing.T) {
	t.Parallel()

	builder := New("github").WithHeader("Authorization", "token")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "headers require a configured url")
}

func TestBuildFailsWhenHeadersWithCommand(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").
		WithCommand("mcp-server").
		WithHeader("Authorization", "token")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "headers are only supported for url-based MCP servers")
}

func TestBuildFailsWhenEnvWithoutCommand(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithEnvVar("ROOT_DIR", "/data")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "environment variables require a configured command")
}

func TestBuildFailsWhenEnvWithURL(t *testing.T) {
	t.Parallel()

	builder := New("github").
		WithURL("https://example.com").
		WithEnvVar("ROOT_DIR", "/data")
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "environment variables are only supported for command-based MCP servers")
}

func TestBuildFailsWhenStartTimeoutWithoutCommand(t *testing.T) {
	t.Parallel()

	builder := New("filesystem").WithStartTimeout(10 * time.Second)
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "start timeout requires a configured command")
}

func TestBuildFailsWhenStartTimeoutWithURL(t *testing.T) {
	t.Parallel()

	builder := New("github").
		WithURL("https://example.com").
		WithStartTimeout(10 * time.Second)
	ctx := t.Context()
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "start timeout is only supported for command-based MCP servers")
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
	var missingCtx context.Context
	cfg, err := builder.Build(missingCtx)
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
