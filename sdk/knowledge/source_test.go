package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

type sourceRecordingLogger struct {
	debugMessages []string
}

func (l *sourceRecordingLogger) Debug(msg string, _ ...any) {
	l.debugMessages = append(l.debugMessages, msg)
}

func (l *sourceRecordingLogger) Info(string, ...any)  {}
func (l *sourceRecordingLogger) Warn(string, ...any)  {}
func (l *sourceRecordingLogger) Error(string, ...any) {}
func (l *sourceRecordingLogger) With(...any) logger.Logger {
	return l
}

func TestFileSourceBuildSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(file, []byte("# Title"), 0o644))

	builder := NewFileSource(file)
	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, engineknowledge.SourceTypeMarkdownGlob, cfg.Type)
	assert.Equal(t, filepath.Clean(file), cfg.Path)
	assert.Nil(t, cfg.Paths)
	assert.Nil(t, cfg.URLs)
	require.NotSame(t, builder.config, cfg)
}

func TestFileSourceEmptyPathFails(t *testing.T) {
	t.Parallel()

	builder := NewFileSource("   ")
	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "path")
}

func TestDirectorySourceBuildsWithMultiplePaths(t *testing.T) {
	t.Parallel()

	dir1 := t.TempDir()
	dir2 := filepath.Join(t.TempDir(), "nested")
	require.NoError(t, os.MkdirAll(dir2, 0o755))

	builder := NewDirectorySource(dir1, dir2)
	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, engineknowledge.SourceTypeMarkdownGlob, cfg.Type)
	assert.Equal(t, filepath.Clean(dir1), cfg.Path)
	require.Len(t, cfg.Paths, 1)
	assert.Equal(t, filepath.Clean(dir2), cfg.Paths[0])
}

func TestDirectorySourceEmptyFails(t *testing.T) {
	t.Parallel()

	builder := NewDirectorySource()
	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "paths")
}

func TestURLSourceBuildSuccess(t *testing.T) {
	t.Parallel()

	builder := NewURLSource("https://example.com/docs", "http://example.com/faq")
	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, engineknowledge.SourceTypeURL, cfg.Type)
	assert.Equal(t, "https://example.com/docs", cfg.Path)
	require.Len(t, cfg.URLs, 1)
	assert.Equal(t, "http://example.com/faq", cfg.URLs[0])
}

func TestURLSourceInvalidFails(t *testing.T) {
	t.Parallel()

	builder := NewURLSource("://bad-url")
	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid")
}

func TestAPISourceBuildSuccess(t *testing.T) {
	t.Parallel()

	builder := NewAPISource("Confluence")
	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, engineknowledge.SourceTypeURL, cfg.Type)
	assert.Equal(t, "confluence", cfg.Provider)
	assert.Empty(t, cfg.Path)
}

func TestAPISourceUnsupportedProviderFails(t *testing.T) {
	t.Parallel()

	builder := NewAPISource("sharepoint")
	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "not supported")
}

func TestSourceBuildUsesLoggerFromContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(file, []byte("content"), 0o644))

	recLogger := &sourceRecordingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), recLogger)

	builder := NewFileSource(file)
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotEmpty(t, recLogger.debugMessages)
	assert.Contains(t, recLogger.debugMessages[0], "building knowledge source configuration")
}

func TestSourceBuildFailsWithNilContext(t *testing.T) {
	t.Parallel()

	builder := NewURLSource("https://example.com")
	cfg, err := builder.Build(nil)
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "context is required")
}
