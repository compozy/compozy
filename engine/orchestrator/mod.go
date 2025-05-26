package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	tkuc "github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	wfuc "github.com/compozy/compozy/engine/workflow/uc"
	"github.com/compozy/compozy/pkg/logger"
)

type Config struct {
	WorkflowRepoFactory func() workflow.Repository
	TaskRepoFactory     func() task.Repository
	AgentRepoFactory    func() agent.Repository
	ToolRepoFactory     func() tool.Repository
}

type Orchestrator struct {
	ns            *nats.Server
	nc            *nats.Client
	store         core.Store
	config        Config
	publisher     core.EventPublisher
	subscriber    core.EventSubscriber
	projectConfig *project.Config
	workflows     []*workflow.Config
}

func NewOrchestrator(
	ns *nats.Server,
	nc *nats.Client,
	store core.Store,
	config Config,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) *Orchestrator {
	publisher := nats.NewEventPublisher(nc)
	subscriber := nats.NewEventSubscriber(nc)
	return &Orchestrator{
		ns:            ns,
		nc:            nc,
		publisher:     publisher,
		subscriber:    subscriber,
		store:         store,
		config:        config,
		projectConfig: projectConfig,
		workflows:     workflows,
	}
}

func (o *Orchestrator) Config() *Config {
	return &o.config
}

func (o *Orchestrator) Setup(ctx context.Context) error {
	if err := o.registerWorkflow(ctx); err != nil {
		return err
	}
	if err := o.registerTask(ctx); err != nil {
		return err
	}
	return nil
}

func (o *Orchestrator) Stop(ctx context.Context) error {
	logger.Debug("Shutting down Orchestrator")
	if err := o.nc.CloseWithContext(ctx); err != nil {
		return fmt.Errorf("failed to close NATS client: %w", err)
	}
	if err := o.store.CloseWithContext(ctx); err != nil {
		return fmt.Errorf("failed to close store: %w", err)
	}
	logger.Debug("Orchestrator stopped successfully")
	return nil
}

// -----------------------------------------------------------------------------
// Workflow
// -----------------------------------------------------------------------------

func (o *Orchestrator) registerWorkflow(ctx context.Context) error {
	if err := o.registerWorkflowTrigger(ctx); err != nil {
		return err
	}
	if err := o.registerWorkflowExecute(ctx); err != nil {
		return err
	}
	if err := o.registerWorkflowEvents(ctx); err != nil {
		return err
	}
	return nil
}

func (o *Orchestrator) registerWorkflowTrigger(ctx context.Context) error {
	workflowRepo := o.config.WorkflowRepoFactory()
	uc := wfuc.NewHandleTrigger(o.nc, workflowRepo, o.projectConfig, o.workflows)
	name := nats.CmdConsumerName(core.ComponentWorkflow, core.CmdTrigger)
	subjects := []string{uc.CMD.ToSubjectParams("*", "*")}
	consumer, err := o.nc.GetCmdConsumer(ctx, name, subjects)
	if err != nil {
		return err
	}
	return o.subscriber.SubscribeConsumer(ctx, consumer, uc.Handler)
}

func (o *Orchestrator) registerWorkflowExecute(ctx context.Context) error {
	workflowRepo := o.config.WorkflowRepoFactory()
	uc := wfuc.NewHandleExecute(o.nc, workflowRepo, o.workflows)
	name := nats.CmdConsumerName(core.ComponentWorkflow, core.CmdExecute)
	subjects := []string{uc.CMD.ToSubjectParams("*", "*")}
	consumer, err := o.nc.GetCmdConsumer(ctx, name, subjects)
	if err != nil {
		return err
	}
	return o.subscriber.SubscribeConsumer(ctx, consumer, uc.Handler)
}

func (o *Orchestrator) registerWorkflowEvents(ctx context.Context) error {
	workflowRepo := o.config.WorkflowRepoFactory()
	uc := wfuc.NewHandleEvents(o.store, workflowRepo)
	name := nats.EventConsumerName(core.ComponentWorkflow, core.EvtAll)
	subjects := []string{core.BuildEvtSubject("*", "*", "*", "*")}
	consumer, err := o.nc.GetEvtConsumer(ctx, name, subjects)
	if err != nil {
		return err
	}
	return o.subscriber.SubscribeConsumer(ctx, consumer, uc.Handler)
}

// -----------------------------------------------------------------------------
// Task
// -----------------------------------------------------------------------------

func (o *Orchestrator) registerTask(ctx context.Context) error {
	if err := o.registerTaskDispatch(ctx); err != nil {
		return err
	}
	if err := o.registerTaskExecute(ctx); err != nil {
		return err
	}
	if err := o.registerTaskEvents(ctx); err != nil {
		return err
	}
	return nil
}

func (o *Orchestrator) registerTaskDispatch(ctx context.Context) error {
	taskRepo := o.config.TaskRepoFactory()
	uc := tkuc.NewHandleDispatch(o.nc, taskRepo, o.workflows)
	name := nats.CmdConsumerName(core.ComponentTask, core.CmdDispatch)
	subjects := []string{uc.CMD.ToSubjectParams("*", "*")}
	consumer, err := o.nc.GetCmdConsumer(ctx, name, subjects)
	if err != nil {
		return err
	}
	return o.subscriber.SubscribeConsumer(ctx, consumer, uc.Handler)
}

func (o *Orchestrator) registerTaskExecute(ctx context.Context) error {
	taskRepo := o.config.TaskRepoFactory()
	uc := tkuc.NewHandleExecute(o.nc, taskRepo)
	name := nats.CmdConsumerName(core.ComponentTask, core.CmdExecute)
	subjects := []string{uc.CMD.ToSubjectParams("*", "*")}
	consumer, err := o.nc.GetCmdConsumer(ctx, name, subjects)
	if err != nil {
		return err
	}
	return o.subscriber.SubscribeConsumer(ctx, consumer, uc.Handler)
}

func (o *Orchestrator) registerTaskEvents(ctx context.Context) error {
	taskRepo := o.config.TaskRepoFactory()
	uc := tkuc.NewHandleEvents(o.store, taskRepo)
	name := nats.EventConsumerName(core.ComponentTask, core.EvtAll)
	subjects := []string{
		core.BuildEvtSubject(core.ComponentTask, "*", "*", "*"),
		core.BuildEvtSubject(core.ComponentAgent, "*", "*", "*"),
		core.BuildEvtSubject(core.ComponentTool, "*", "*", "*"),
	}
	consumer, err := o.nc.GetEvtConsumer(ctx, name, subjects)
	if err != nil {
		return err
	}
	return o.subscriber.SubscribeConsumer(ctx, consumer, uc.Handler)
}
