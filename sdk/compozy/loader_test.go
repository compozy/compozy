package compozy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	enginetool "github.com/compozy/compozy/engine/tool"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadYAMLSuccess(t *testing.T) {
	t.Run("Should load YAML configuration successfully", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{ctx: ctx}
		dir := t.TempDir()
		file := filepath.Join(dir, "tool.yaml")
		content := strings.TrimSpace(`resource: tool
id: yaml-tool
type: http
`)
		require.NoError(t, os.WriteFile(file, []byte(content), 0o600))
		cfg, abs, err := loadYAML[*enginetool.Config](ctx, engine, file)
		require.NoError(t, err)
		assert.Equal(t, "yaml-tool", cfg.ID)
		assert.Equal(t, "tool", cfg.Resource)
		assert.Equal(t, filepath.Clean(file), abs)
	})
}

func TestLoadFromDirAccumulatesErrors(t *testing.T) {
	t.Run("Should accumulate loader errors and report them", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{ctx: ctx}
		dir := t.TempDir()
		good := filepath.Join(dir, "good.yaml")
		bad := filepath.Join(dir, "bad.yml")
		require.NoError(t, os.WriteFile(good, []byte("kind: ok"), 0o600))
		require.NoError(t, os.WriteFile(bad, []byte("kind: bad"), 0o600))
		seen := make([]string, 0)
		loader := func(_ context.Context, path string) error {
			seen = append(seen, filepath.Base(path))
			if strings.Contains(path, "bad") {
				return fmt.Errorf("failed")
			}
			return nil
		}
		err := engine.loadFromDir(ctx, dir, loader)
		require.Error(t, err)
		assert.Len(t, seen, 2)
		assert.Contains(t, err.Error(), "bad.yml")
	})
}

func TestLoadYAMLErrorConditions(t *testing.T) {
	t.Run("Should return error when engine is nil", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		_, _, err := loadYAML[*enginetool.Config](ctx, nil, "file.yaml")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "engine is nil")
	})
	t.Run("Should validate that path is not blank", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{ctx: ctx}
		_, _, err := loadYAML[*enginetool.Config](ctx, engine, " ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path is required")
	})
	t.Run("Should require non nil context", func(t *testing.T) {
		t.Parallel()
		engine := &Engine{ctx: lifecycleTestContext(t)}
		//lint:ignore SA1012 testing nil context handling
		_, _, err := loadYAML[*enginetool.Config](nil, engine, "file.yaml")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context is required")
	})
	t.Run("Should validate engine context is set", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{}
		_, _, err := loadYAML[*enginetool.Config](ctx, engine, "file.yaml")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "engine context is not set")
	})
}

func TestLoadYAMLHandlesCanceledContext(t *testing.T) {
	t.Run("Should fail when context is canceled", func(t *testing.T) {
		t.Parallel()
		baseCtx := lifecycleTestContext(t)
		engine := &Engine{ctx: baseCtx}
		dir := t.TempDir()
		file := filepath.Join(dir, "tool.yaml")
		require.NoError(t, os.WriteFile(file, []byte("resource: tool\nid: ctx-tool\ntype: http\n"), 0o600))
		cancelCtx, cancel := context.WithCancel(baseCtx)
		cancel()
		_, _, err := loadYAML[*enginetool.Config](cancelCtx, engine, file)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "operation canceled")
	})
}

func TestLoadYAMLStatFailure(t *testing.T) {
	t.Run("Should report stat errors for missing file", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{ctx: ctx}
		_, _, err := loadYAML[*enginetool.Config](ctx, engine, "missing.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stat missing.yaml")
	})
}

func TestLoadYAMLDecodeFailure(t *testing.T) {
	t.Run("Should report decode failure for invalid YAML", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{ctx: ctx}
		cfg := appconfig.FromContext(ctx)
		require.NotNil(t, cfg)
		bad := filepath.Join(t.TempDir(), "invalid.yaml")
		require.NoError(t, os.WriteFile(bad, []byte("{"), 0o600))
		_, _, err := loadYAML[*enginetool.Config](ctx, engine, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})
}

func TestLoadFromDirValidatesInputs(t *testing.T) {
	t.Run("Should require engine instance", func(t *testing.T) {
		t.Parallel()
		var engine *Engine
		ctx := lifecycleTestContext(t)
		err := engine.loadFromDir(ctx, "", nil)
		require.Error(t, err)
	})
	t.Run("Should require directory path and loader", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{ctx: ctx}
		err := engine.loadFromDir(ctx, "", func(context.Context, string) error { return nil })
		require.Error(t, err)
	})
}

func TestLoadFromDirHandlesCanceledContext(t *testing.T) {
	t.Run("Should fail when context is canceled", func(t *testing.T) {
		t.Parallel()
		baseCtx := lifecycleTestContext(t)
		engine := &Engine{ctx: baseCtx}
		dir := t.TempDir()
		cancelCtx, cancel := context.WithCancel(baseCtx)
		cancel()
		err := engine.loadFromDir(cancelCtx, dir, func(context.Context, string) error { return nil })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "operation canceled")
	})
}
