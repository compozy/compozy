package compozy_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadWorkflowsPerformanceBudget(t *testing.T) {
	t.Run("Should load workflows within 100ms per file", func(t *testing.T) {
		ctx := loaderTestContext(t)
		engine, err := compozy.New(
			ctx,
			compozy.WithAgent(&engineagent.Config{
				ID:           "seed-agent",
				Instructions: "Seed agent",
				Model: engineagent.Model{
					Config: enginecore.ProviderConfig{
						Provider: enginecore.ProviderName("openai"),
						Model:    "gpt-4o-mini",
					},
				},
			}),
			compozy.WithWorkflow(&engineworkflow.Config{
				ID: "seed-workflow",
				Tasks: []enginetask.Config{
					{
						BaseConfig: enginetask.BaseConfig{ID: "seed-task"},
					},
				},
			}),
		)
		require.NoError(t, err)
		dir := t.TempDir()
		fileCount := 10
		for i := 0; i < fileCount; i++ {
			path := filepath.Join(dir, fmt.Sprintf("workflow_%02d.yaml", i))
			content := fmt.Sprintf(
				"id: wf-%02d\n"+
					"tasks:\n"+
					"  - id: task-%02d\n"+
					"    type: basic\n"+
					"    agent:\n"+
					"      id: inline-agent-%02d\n"+
					"      instructions: \"Respond to loader benchmarks\"\n"+
					"      model:\n"+
					"        config:\n"+
					"          provider: openai\n"+
					"          model: gpt-4o-mini\n"+
					"    action: respond\n"+
					"    final: true\n",
				i,
				i,
				i,
			)
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		}
		start := time.Now()
		err = engine.LoadWorkflowsFromDir(ctx, dir)
		require.NoError(t, err)
		perFile := time.Since(start) / time.Duration(fileCount)
		assert.LessOrEqual(t, perFile, 100*time.Millisecond, "per-file load budget exceeded: %v", perFile)
		report, err := engine.ValidateReferences()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, report.ResourceCount, fileCount+2)
	})
}

func loaderTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	service := appconfig.NewService()
	manager := appconfig.NewManager(ctx, service)
	_, err := manager.Load(ctx, appconfig.NewDefaultProvider())
	require.NoError(t, err)
	ctx = appconfig.ContextWithManager(ctx, manager)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	listenCfg := net.ListenConfig{}
	for {
		ln, err := listenCfg.Listen(context.WithoutCancel(ctx), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := ln.Addr().(*net.TCPAddr)
		require.NoError(t, ln.Close())
		if addr.Port <= 64535 {
			cfg.Temporal.Standalone.FrontendPort = addr.Port
			break
		}
	}
	return ctx
}
