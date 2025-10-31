package compozy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	enginetool "github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadYAMLSuccess(t *testing.T) {
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
	cfg, abs, err := loadYAML[*enginetool.Config](engine, file)
	require.NoError(t, err)
	assert.Equal(t, "yaml-tool", cfg.ID)
	assert.Equal(t, "tool", cfg.Resource)
	assert.Equal(t, filepath.Clean(file), abs)
}

func TestLoadFromDirAccumulatesErrors(t *testing.T) {
	t.Parallel()
	ctx := lifecycleTestContext(t)
	engine := &Engine{ctx: ctx}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	bad := filepath.Join(dir, "bad.yml")
	require.NoError(t, os.WriteFile(good, []byte("kind: ok"), 0o600))
	require.NoError(t, os.WriteFile(bad, []byte("kind: bad"), 0o600))
	seen := make([]string, 0)
	loader := func(path string) error {
		seen = append(seen, filepath.Base(path))
		if strings.Contains(path, "bad") {
			return fmt.Errorf("failed")
		}
		return nil
	}
	err := engine.loadFromDir(dir, loader)
	require.Error(t, err)
	assert.Len(t, seen, 2)
	assert.Contains(t, err.Error(), "bad.yml")
}
