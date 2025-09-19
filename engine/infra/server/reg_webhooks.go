package server

import (
	"context"
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	prmiddleware "github.com/compozy/compozy/engine/infra/server/middleware/project"
	sizemw "github.com/compozy/compozy/engine/infra/server/middleware/size"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/webhook"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/metric"
)

func setupWebhookSystem(
	ctx context.Context,
	state *appstate.State,
	router *gin.Engine,
	server *Server,
) error {
	if err := attachWebhookRegistry(ctx, state); err != nil {
		return err
	}
	var meter metric.Meter
	if server != nil && server.monitoring != nil && server.monitoring.IsInitialized() {
		meter = server.monitoring.Meter()
	}
	return registerPublicWebhookRoutes(ctx, router, state, server, meter)
}

func registerPublicWebhookRoutes(
	ctx context.Context,
	router *gin.Engine,
	state *appstate.State,
	server *Server,
	meter metric.Meter,
) error {
	cfg := config.FromContext(ctx)
	limiterMax := cfg.Webhooks.DefaultMaxBody
	hooks := router.Group(routes.Hooks())
	hooks.Use(sizemw.BodySizeLimiter(limiterMax))
	if state.ProjectConfig.Name == "" {
		return fmt.Errorf("project name is empty; set project.projectConfig.name")
	}
	hooks.Use(prmiddleware.Middleware(state.ProjectConfig.Name))
	reg := resolveWebhookRegistry(state)
	eval, err := task.NewCELEvaluator()
	if err != nil {
		return fmt.Errorf("failed to init CEL evaluator: %w", err)
	}
	if server != nil {
		if closer, ok := any(eval).(interface{ Close() error }); ok {
			server.RegisterCleanup(func() {
				if err := closer.Close(); err != nil {
					logger.FromContext(ctx).Warn("Failed to close CEL evaluator", "error", err)
				}
			})
		}
	}
	filter := webhook.NewCELAdapter(eval)
	dispatcher := buildWebhookDispatcher(ctx, state)
	idemSvc, err := buildWebhookIdempotencyService(server)
	if err != nil {
		return fmt.Errorf("failed to initialize redis idempotency service: %w", err)
	}
	const (
		DefaultWebhookMaxRetries   = 0
		DefaultWebhookInitialDelay = 0
	)
	orchestrator := webhook.NewOrchestrator(
		cfg,
		reg,
		filter,
		dispatcher,
		idemSvc,
		DefaultWebhookMaxRetries,
		DefaultWebhookInitialDelay,
	)
	if meter != nil {
		metrics, err := webhook.NewMetrics(ctx, meter)
		if err != nil {
			return fmt.Errorf("failed to initialize webhook metrics: %w", err)
		}
		orchestrator.SetMetrics(metrics)
	}
	webhook.RegisterPublic(hooks, orchestrator)
	logger.FromContext(ctx).Info("Public webhook routes registered", "path", routes.Hooks())
	return nil
}

func attachWebhookRegistry(ctx context.Context, state *appstate.State) error {
	reg := webhook.NewRegistry()
	for _, wf := range state.GetWorkflows() {
		for i := range wf.Triggers {
			t := wf.Triggers[i]
			if t.Type == workflow.TriggerTypeWebhook && t.Webhook != nil {
				entry := webhook.RegistryEntry{WorkflowID: wf.ID, Webhook: t.Webhook}
				if err := reg.Add(t.Webhook.Slug, entry); err != nil {
					return fmt.Errorf(
						"webhook registry: failed to add slug '%s' from workflow '%s': %w",
						t.Webhook.Slug,
						wf.ID,
						err,
					)
				}
			}
		}
	}
	state.SetWebhookRegistry(reg)
	slugs := reg.Slugs()
	limit := len(slugs)
	if limit > 5 {
		limit = 5
	}
	log := logger.FromContext(ctx)
	if limit > 0 {
		sort.Strings(slugs)
		log.Info("Webhook registry initialized", "count", len(slugs), "slugs", slugs[:limit])
	} else {
		log.Info("Webhook registry initialized", "count", 0)
	}
	return nil
}

func resolveWebhookRegistry(state *appstate.State) webhook.Lookup {
	if ext, ok := state.WebhookRegistry(); ok {
		if reg, ok := ext.(webhook.Lookup); ok {
			return reg
		}
	}
	return webhook.NewRegistry()
}

func buildWebhookDispatcher(ctx context.Context, state *appstate.State) services.SignalDispatcher {
	if state.Worker == nil {
		return nil
	}
	dispatcherID := state.Worker.GetDispatcherID()
	taskQueue := state.Worker.GetTaskQueue()
	if dispatcherID == "" || taskQueue == "" {
		logger.FromContext(ctx).Warn(
			"Worker missing dispatcher metadata; webhook dispatch disabled",
			"dispatcher_id", dispatcherID,
			"task_queue", taskQueue,
		)
		return nil
	}
	return worker.NewSignalDispatcher(
		state.Worker.GetClient(),
		dispatcherID,
		taskQueue,
		state.Worker.GetServerID(),
	)
}

func buildWebhookIdempotencyService(server *Server) (webhook.Service, error) {
	if server != nil && server.redisClient != nil {
		svc, err := webhook.NewServiceFromCache(server.redisClient)
		if err != nil {
			return nil, err
		}
		return svc, nil
	}
	return webhook.NewInMemoryService(), nil
}
