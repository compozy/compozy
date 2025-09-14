package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/gosimple/slug"
	"github.com/kaptinlin/jsonschema"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task/services"
	wkacts "github.com/compozy/compozy/engine/worker/activities"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	"github.com/compozy/compozy/pkg/config"
)

// Constants for dispatcher workflow
const (
	maxSignalsPerRun         = 1000
	maxConsecutiveErrors     = 10
	circuitBreakerDelay      = 5 * time.Second
	heartbeatActivityTimeout = 10 * time.Second
	localActivityTimeout     = 10 * time.Second
)

// EventSignal represents an incoming event
type EventSignal struct {
	Name          string     `json:"name"`
	Payload       core.Input `json:"payload"`
	CorrelationID string     `json:"correlation_id,omitempty"`
}

// CompiledTrigger holds workflow config and pre-compiled schema for efficient validation
type CompiledTrigger struct {
	Config         *wf.Config
	Trigger        *wf.Trigger
	CompiledSchema *jsonschema.Schema
}

// GetRegisteredSignalNames returns a list of currently registered signal names for logging
func GetRegisteredSignalNames(signalMap map[string]*CompiledTrigger) []string {
	names := make([]string, 0, len(signalMap))
	for name := range signalMap {
		names = append(names, name)
	}
	return names
}

// BuildSignalRoutingMap creates a map of signal names to compiled triggers with pre-compiled schemas
// It aggregates both explicit signal triggers and webhook event names so that
// webhook events can be dispatched through the same dispatcher without requiring
// duplicate "signal" triggers in the workflow config.
func BuildSignalRoutingMap(ctx workflow.Context, data *wfacts.GetData) (map[string]*CompiledTrigger, error) {
	log := workflow.GetLogger(ctx)
	signalMap := make(map[string]*CompiledTrigger)

	for _, wcfg := range data.Workflows {
		for i := range wcfg.Triggers {
			trigger := &wcfg.Triggers[i]
			switch trigger.Type {
			case wf.TriggerTypeSignal:
				if existing, exists := signalMap[trigger.Name]; exists {
					return nil, fmt.Errorf(
						"duplicate signal name %q registered by both %q and %q",
						trigger.Name,
						existing.Config.ID,
						wcfg.ID,
					)
				}

				target := &CompiledTrigger{Config: wcfg, Trigger: trigger}
				if trigger.Schema != nil {
					compiled, err := trigger.Schema.Compile()
					if err != nil {
						log.Error("Failed to compile schema for trigger",
							"signal", trigger.Name, "workflow", wcfg.ID, "error", err)
						return nil, fmt.Errorf("failed to compile schema for %s: %w", trigger.Name, err)
					}
					target.CompiledSchema = compiled
				}
				signalMap[trigger.Name] = target
				log.Debug("Registered signal trigger", "signal", trigger.Name, "workflow", wcfg.ID)

			case wf.TriggerTypeWebhook:
				// Register each webhook event name as a routable signal
				if trigger.Webhook == nil {
					continue
				}
				for _, ev := range trigger.Webhook.Events {
					if ev.Name == "" {
						continue
					}
					if existing, exists := signalMap[ev.Name]; exists {
						return nil, fmt.Errorf(
							"duplicate event/signal name %q registered by both %q and %q",
							ev.Name,
							existing.Config.ID,
							wcfg.ID,
						)
					}

					target := &CompiledTrigger{Config: wcfg, Trigger: trigger}
					if ev.Schema != nil {
						compiled, err := ev.Schema.Compile()
						if err != nil {
							log.Error("Failed to compile schema for webhook event",
								"event", ev.Name, "workflow", wcfg.ID, "error", err)
							return nil, fmt.Errorf("failed to compile schema for %s: %w", ev.Name, err)
						}
						target.CompiledSchema = compiled
					}
					signalMap[ev.Name] = target
					log.Debug(
						"Registered webhook event as signal",
						"event",
						ev.Name,
						"slug",
						trigger.Webhook.Slug,
						"workflow",
						wcfg.ID,
					)
				}
			default:
				// ignore other trigger types here
			}
		}
	}
	return signalMap, nil
}

// GenerateCorrelationID generates or uses existing correlation ID for event tracking
func GenerateCorrelationID(ctx workflow.Context, existingID string) string {
	log := workflow.GetLogger(ctx)

	if existingID != "" {
		return existingID
	}

	var newID string
	// Use versioning to handle backward compatibility during replay
	version := workflow.GetVersion(ctx, "correlation-id-generation", workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		// Old behavior for existing workflows - use deterministic but unique fallback
		var fallbackID string
		if err := workflow.SideEffect(ctx, func(_ workflow.Context) any {
			return fmt.Sprintf("generated-%d", workflow.Now(ctx).UnixNano())
		}).Get(&fallbackID); err != nil {
			log.Error("Failed to generate fallback correlation ID", "error", err)
			return "generated-id-fallback"
		}
		return fallbackID
	}

	// New behavior with SideEffect
	if err := workflow.SideEffect(ctx, func(_ workflow.Context) any {
		return core.MustNewID().String()
	}).Get(&newID); err != nil {
		log.Error("Failed to generate correlation ID", "error", err)
		return "generated-id-fallback"
	}
	return newID
}

// validateSignalPayload validates the signal payload against the target's schema
func validateSignalPayload(
	ctx workflow.Context,
	signal EventSignal,
	target *CompiledTrigger,
	correlationID string,
) bool {
	log := workflow.GetLogger(ctx)

	if target.CompiledSchema == nil {
		return true // No schema to validate against
	}

	isValid, validationErrors := ValidatePayloadAgainstCompiledSchema(
		signal.Payload,
		target.CompiledSchema,
	)
	if !isValid {
		log.Error("Payload validation failed",
			"signalName", signal.Name,
			"correlationId", correlationID,
			"targetWorkflow", target.Config.ID,
			"validationErrors", validationErrors)
		return false
	}

	log.Debug("Payload validation passed",
		"signalName", signal.Name,
		"correlationId", correlationID,
		"targetWorkflow", target.Config.ID)
	return true
}

// generateWorkflowExecID generates a unique workflow execution ID
func generateWorkflowExecID(ctx workflow.Context) core.ID {
	log := workflow.GetLogger(ctx)
	var workflowExecID core.ID
	// Use versioning to handle backward compatibility during replay
	version := workflow.GetVersion(ctx, "workflow-id-generation", workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		// Old behavior for existing workflows - use deterministic but unique fallback
		var fallbackID string
		if err := workflow.SideEffect(ctx, func(_ workflow.Context) any {
			return fmt.Sprintf("fallback-%d", workflow.Now(ctx).UnixNano())
		}).Get(&fallbackID); err != nil {
			log.Error("Failed to generate fallback workflow execution ID", "error", err)
			return core.ID("fallback-exec-id")
		}
		return core.ID(fallbackID)
	}
	// New behavior with SideEffect
	if err := workflow.SideEffect(ctx, func(_ workflow.Context) any {
		return core.MustNewID()
	}).Get(&workflowExecID); err != nil {
		log.Error("Failed to generate workflow execution ID", "error", err)
		return core.ID("fallback-exec-id")
	}
	return workflowExecID
}

// executeChildWorkflow starts and tracks a child workflow execution
func executeChildWorkflow(
	ctx workflow.Context,
	signal EventSignal,
	target *CompiledTrigger,
	correlationID string,
	appConfig *config.Config,
) bool {
	log := workflow.GetLogger(ctx)
	workflowExecID := generateWorkflowExecID(ctx)
	cwo := workflow.ChildWorkflowOptions{
		WorkflowID:        buildWorkflowID(target.Config.ID, workflowExecID),
		ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON, // Let child continue if parent restarts
	}
	childCtx := workflow.WithChildOptions(ctx, cwo)
	childInput := WorkflowInput{
		WorkflowID:     target.Config.ID,
		WorkflowExecID: workflowExecID,
		Input:          &signal.Payload,
	}

	// Execute child workflow with comprehensive error handling
	log.Info("Starting child workflow",
		"targetWorkflow", target.Config.ID,
		"workflowId", cwo.WorkflowID,
		"signalName", signal.Name,
		"correlationId", correlationID)

	childFuture := workflow.ExecuteChildWorkflow(childCtx, CompozyWorkflow, childInput, appConfig)

	// Get the execution to ensure deterministic behavior during replay
	var childExecution workflow.Execution
	err := childFuture.GetChildWorkflowExecution().Get(ctx, &childExecution)
	if err != nil {
		log.Error("Failed to get child workflow execution",
			"workflowId", cwo.WorkflowID,
			"targetWorkflow", target.Config.ID,
			"signalName", signal.Name,
			"correlationId", correlationID,
			"error", err)
		return false
	}

	log.Debug("Successfully started child workflow",
		"workflowId", cwo.WorkflowID,
		"targetWorkflow", target.Config.ID,
		"signalName", signal.Name,
		"correlationId", correlationID,
		"childRunId", childExecution.RunID)

	return true
}

// ProcessEventSignal handles a single event signal with validation and child workflow execution
func ProcessEventSignal(
	ctx workflow.Context,
	signal EventSignal,
	signalMap map[string]*CompiledTrigger,
	appConfig *config.Config,
) bool {
	log := workflow.GetLogger(ctx)

	// Use provided correlation ID or generate one for tracking this event
	correlationID := GenerateCorrelationID(ctx, signal.CorrelationID)
	log.Debug("Received signal", "name", signal.Name, "correlationId", correlationID)

	// Find target workflow with enhanced error handling
	target, ok := signalMap[signal.Name]
	if !ok {
		log.Warn("Unknown signal - no workflow configured",
			"signalName", signal.Name,
			"correlationId", correlationID,
			"availableSignals", GetRegisteredSignalNames(signalMap))
		return false // Not a fatal error, just unknown signal
	}

	// Validate payload against pre-compiled schema if defined
	if !validateSignalPayload(ctx, signal, target, correlationID) {
		return false // Not a fatal error, just validation failure
	}

	// Start child workflow with enhanced error handling and retry options
	return executeChildWorkflow(ctx, signal, target, correlationID, appConfig)
}

// startDispatcherHeartbeat starts the heartbeat goroutine for the dispatcher
func startDispatcherHeartbeat(
	ctx workflow.Context,
	data *wfacts.GetData,
	dispatcherID, projectName, serverID string,
) (workflow.CancelFunc, time.Duration) {
	log := workflow.GetLogger(ctx)
	heartbeatInterval := data.AppConfig.Runtime.DispatcherHeartbeatInterval
	heartbeatCtx, heartbeatCancel := workflow.WithCancel(ctx)
	workflow.Go(heartbeatCtx, func(ctx workflow.Context) {
		log.Debug("Starting dispatcher heartbeat", "interval", heartbeatInterval)
		for {
			heartbeatInput := &wkacts.DispatcherHeartbeatInput{
				DispatcherID: dispatcherID,
				ProjectName:  projectName,
				ServerID:     serverID,
				TTL:          data.AppConfig.Runtime.DispatcherHeartbeatTTL,
			}
			disconnectedCtx, cancel := workflow.NewDisconnectedContext(ctx)
			ao := workflow.ActivityOptions{StartToCloseTimeout: heartbeatActivityTimeout}
			disconnectedCtx = workflow.WithActivityOptions(disconnectedCtx, ao)
			if err := workflow.ExecuteActivity(
				disconnectedCtx,
				wkacts.DispatcherHeartbeatLabel,
				heartbeatInput,
			).Get(disconnectedCtx, nil); err != nil {
				log.Warn("Failed to send dispatcher heartbeat", "error", err)
			}
			cancel()
			if err := workflow.Sleep(ctx, heartbeatInterval); err != nil {
				log.Debug("Heartbeat goroutine stopped")
				return
			}
		}
	})
	return heartbeatCancel, heartbeatInterval
}

// handleSignalProcessing handles the main signal processing loop
func handleSignalProcessing(
	ctx workflow.Context,
	signalMap map[string]*CompiledTrigger,
	heartbeatCancel workflow.CancelFunc,
	projectName, serverID string,
	appConfig *config.Config,
) error {
	log := workflow.GetLogger(ctx)
	// Using constants defined at package level
	signalChan := workflow.GetSignalChannel(ctx, DispatcherEventChannel)
	processed := 0
	consecutiveErrors := 0
	log.Debug("Dispatcher ready to process events", "maxSignalsPerRun", maxSignalsPerRun)
	for {
		if ctx.Err() != nil {
			log.Info("DispatcherWorkflow canceled", "processedEvents", processed)
			return ctx.Err()
		}
		if processed >= maxSignalsPerRun {
			log.Info("Reaching max signals per run, continuing as new",
				"processed", processed, "maxSignalsPerRun", maxSignalsPerRun)
			heartbeatCancel()
			return workflow.NewContinueAsNewError(ctx, DispatcherWorkflow, projectName, serverID)
		}
		if consecutiveErrors >= maxConsecutiveErrors {
			log.Warn("Too many consecutive errors, implementing backoff",
				"consecutiveErrors", consecutiveErrors, "maxConsecutiveErrors", maxConsecutiveErrors,
				"backoffDelay", circuitBreakerDelay)
			if err := workflow.Sleep(ctx, circuitBreakerDelay); err != nil {
				log.Debug("Circuit breaker sleep interrupted", "error", err)
			}
		}
		var signal EventSignal
		ok := signalChan.Receive(ctx, &signal)
		if !ok {
			heartbeatCancel()
			return workflow.NewContinueAsNewError(ctx, DispatcherWorkflow, projectName, serverID)
		}
		if ctx.Err() != nil {
			log.Debug("DispatcherWorkflow canceled while receiving signal")
			return ctx.Err()
		}
		if signal.Name == "" {
			log.Debug("Skipping empty signal (likely initialization signal)")
			processed++
			continue
		}
		success := ProcessEventSignal(ctx, signal, signalMap, appConfig)
		if success {
			consecutiveErrors = 0
		} else {
			consecutiveErrors++
		}
		processed++
	}
}

// DispatcherWorkflow handles event routing
func DispatcherWorkflow(ctx workflow.Context, projectName string, serverID string) error {
	log := workflow.GetLogger(ctx)
	log.Info("DispatcherWorkflow started", "project", projectName)
	workflowInfo := workflow.GetInfo(ctx)
	dispatcherID := workflowInfo.WorkflowExecution.ID
	var data *wfacts.GetData
	lao := workflow.LocalActivityOptions{StartToCloseTimeout: localActivityTimeout}
	ctx = workflow.WithLocalActivityOptions(ctx, lao)
	err := workflow.ExecuteLocalActivity(ctx, wfacts.GetDataLabel, &wfacts.GetDataInput{WorkflowID: projectName}).
		Get(ctx, &data)
	if err != nil {
		return fmt.Errorf("configuration load failed: %w", err)
	}
	signalMap, err := BuildSignalRoutingMap(ctx, data)
	if err != nil {
		return err
	}
	heartbeatCancel, _ := startDispatcherHeartbeat(ctx, data, dispatcherID, projectName, serverID)
	defer heartbeatCancel()
	return handleSignalProcessing(ctx, signalMap, heartbeatCancel, projectName, serverID, data.AppConfig)
}

// -----------------------------------------------------------------------------
// Signal Dispatcher
// -----------------------------------------------------------------------------

// SignalDispatcher implements SignalDispatcher using Temporal client
type SignalDispatcher struct {
	client       client.Client
	dispatcherID string
	taskQueue    string
	serverID     string
}

// NewSignalDispatcher creates a new TemporalSignalDispatcher
func NewSignalDispatcher(
	client client.Client,
	dispatcherID string,
	taskQueue string,
	serverID string,
) services.SignalDispatcher {
	return &SignalDispatcher{
		client:       client,
		dispatcherID: dispatcherID,
		taskQueue:    taskQueue,
		serverID:     serverID,
	}
}

// DispatchSignal sends a signal using Temporal's SignalWithStartWorkflow
func (t *SignalDispatcher) DispatchSignal(
	ctx context.Context,
	signalName string,
	payload map[string]any,
	correlationID string,
) error {
	projectName, err := core.GetProjectName(ctx)
	if err != nil {
		return fmt.Errorf("failed to get project name: %w", err)
	}

	_, err = t.client.SignalWithStartWorkflow(
		ctx,
		t.dispatcherID,
		DispatcherEventChannel,
		EventSignal{
			Name:          signalName,
			Payload:       core.Input(payload),
			CorrelationID: correlationID,
		},
		client.StartWorkflowOptions{
			ID:        t.dispatcherID,
			TaskQueue: t.taskQueue,
		},
		DispatcherWorkflow,
		projectName,
		t.serverID,
	)
	return err
}

func GetTaskQueue(projectName string) string {
	return slug.Make(projectName)
}
