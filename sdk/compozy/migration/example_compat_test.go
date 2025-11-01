package compozy_test

import (
	"context"
	"net"
	"testing"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/v2/agent"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	"github.com/compozy/compozy/sdk/v2/task"
	"github.com/compozy/compozy/sdk/v2/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// maxTemporalPort matches Temporal's documented upper port bound for development clusters.
	maxTemporalPort = 64535
	// maxTemporalPortAttempts prevents infinite retries when searching for an available port.
	maxTemporalPortAttempts = 50
)

func TestMigrationGuideExampleCompatibility(t *testing.T) {
	t.Run("Should assemble engine from migrated sdk resources", func(t *testing.T) {
		ctx := migrationTestContext(t)
		model := engineagent.Model{
			Config: enginecore.ProviderConfig{
				Provider: enginecore.ProviderName("openai"),
				Model:    "gpt-4o-mini",
			},
		}
		agentCfg, err := agent.New(
			ctx,
			"migration-assistant",
			agent.WithInstructions("Guide users through the migration workflow."),
			agent.WithModel(model),
		)
		require.NoError(t, err)
		withParams := enginecore.Input{
			"name": "{{ .workflow.input.name }}",
		}
		taskCfg, err := task.New(
			ctx,
			"welcome",
			task.WithAction("prepare-migration"),
			task.WithAgent(agentCfg),
			task.WithWith(&withParams),
			task.WithFinal(true),
		)
		require.NoError(t, err)
		outputs := enginecore.Output{
			"message": "{{ .tasks.welcome.output.message }}",
		}
		workflowCfg, err := workflow.New(
			ctx,
			"migration-demo",
			workflow.WithTasks([]enginetask.Config{*taskCfg}),
			workflow.WithOutputs(&outputs),
		)
		require.NoError(t, err)
		engine, err := compozy.New(
			ctx,
			compozy.WithProject(&engineproject.Config{Name: "migration"}),
			compozy.WithAgent(agentCfg),
			compozy.WithWorkflow(workflowCfg),
		)
		require.NoError(t, err)
		report, err := engine.ValidateReferences()
		require.NoError(t, err)
		assert.True(t, report.Valid)
		assert.GreaterOrEqual(t, report.ResourceCount, 3)
	})
}

func migrationTestContext(t *testing.T) context.Context {
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
	for attempt := 0; attempt < maxTemporalPortAttempts; attempt++ {
		ln, err := listenCfg.Listen(context.WithoutCancel(t.Context()), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := ln.Addr().(*net.TCPAddr)
		require.NoError(t, ln.Close())
		if addr.Port <= maxTemporalPort {
			cfg.Temporal.Standalone.FrontendPort = addr.Port
			return ctx
		}
	}
	t.Fatalf("failed to allocate Temporal frontend port within %d attempts", maxTemporalPortAttempts)
	return nil
}
