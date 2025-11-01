package compozy_test

import (
	"os"
	"path/filepath"
	"testing"

	enginecore "github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	"github.com/stretchr/testify/require"
)

func TestLoadToolsFromDirPropagatesFilePathOnError(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	service := appconfig.NewService()
	manager := appconfig.NewManager(ctx, service)
	_, err := manager.Load(ctx, appconfig.NewDefaultProvider())
	require.NoError(t, err)
	ctx = appconfig.ContextWithManager(ctx, manager)
	wf := &engineworkflow.Config{ID: "loader"}
	wf.Tasks = []enginetask.Config{
		{
			BaseConfig: enginetask.BaseConfig{
				ID:        "only",
				OnSuccess: &enginecore.SuccessTransition{},
			},
		},
	}
	engine, err := compozy.New(ctx, compozy.WithWorkflow(wf))
	require.NoError(t, err)
	dir := t.TempDir()
	file := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(file, []byte("::invalid::"), 0o600))
	err = engine.LoadToolsFromDir(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad.yaml")
}
