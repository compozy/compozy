package worker

import (
	"fmt"
	"time"

	"github.com/kaptinlin/jsonschema"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

// EventSignal represents an incoming event
type EventSignal struct {
	Name          string     `json:"name"`
	Payload       core.Input `json:"payload"`
	CorrelationID string     `json:"correlation_id,omitempty"`
}

// compiledTrigger holds workflow config and pre-compiled schema for efficient validation
type compiledTrigger struct {
	config         *wf.Config
	trigger        *wf.Trigger
	compiledSchema *jsonschema.Schema
}

// getRegisteredSignalNames returns a list of currently registered signal names for logging
func getRegisteredSignalNames(signalMap map[string]*compiledTrigger) []string {
	names := make([]string, 0, len(signalMap))
	for name := range signalMap {
		names = append(names, name)
	}
	return names
}

// buildSignalRoutingMap creates a map of signal names to compiled triggers with pre-compiled schemas
func buildSignalRoutingMap(ctx workflow.Context, data *wfacts.GetData) (map[string]*compiledTrigger, error) {
	logger := workflow.GetLogger(ctx)
	signalMap := make(map[string]*compiledTrigger)

	for _, wcfg := range data.Workflows {
		for i := range wcfg.Triggers {
			trigger := &wcfg.Triggers[i]
			if trigger.Type == wf.TriggerTypeSignal {
				if existing, exists := signalMap[trigger.Name]; exists {
					return nil, fmt.Errorf(
						"duplicate signal name %q registered by both %q and %q",
						trigger.Name,
						existing.config.ID,
						wcfg.ID,
					)
				}

				target := &compiledTrigger{
					config:  wcfg,
					trigger: trigger,
				}

				// Pre-compile schema if defined
				if trigger.Schema != nil {
					compiled, err := trigger.Schema.Compile()
					if err != nil {
						logger.Error("Failed to compile schema for trigger",
							"signal", trigger.Name, "workflow", wcfg.ID, "error", err)
						return nil, fmt.Errorf("failed to compile schema for %s: %w", trigger.Name, err)
					}
					target.compiledSchema = compiled
				}

				signalMap[trigger.Name] = target
				logger.Info("Registered signal trigger", "signal", trigger.Name, "workflow", wcfg.ID)
			}
		}
	}
	return signalMap, nil
}

// generateCorrelationID generates or uses existing correlation ID for event tracking
func generateCorrelationID(ctx workflow.Context, existingID string) string {
	logger := workflow.GetLogger(ctx)

	if existingID != "" {
		return existingID
	}

	var newID string
	// Use versioning to handle backward compatibility during replay
	version := workflow.GetVersion(ctx, "correlation-id-generation", workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		// Old behavior for existing workflows
		return "generated-id"
	}

	// New behavior with SideEffect
	if err := workflow.SideEffect(ctx, func(_ workflow.Context) any {
		return core.MustNewID().String()
	}).Get(&newID); err != nil {
		logger.Error("Failed to generate correlation ID", "error", err)
		return "generated-id-fallback"
	}
	return newID
}

// validateSignalPayload validates the signal payload against the target's schema
func validateSignalPayload(
	ctx workflow.Context,
	signal EventSignal,
	target *compiledTrigger,
	correlationID string,
) bool {
	logger := workflow.GetLogger(ctx)

	if target.compiledSchema == nil {
		return true // No schema to validate against
	}

	isValid, validationErrors := validatePayloadAgainstCompiledSchema(
		signal.Payload,
		target.compiledSchema,
	)
	if !isValid {
		logger.Error("Payload validation failed",
			"signalName", signal.Name,
			"correlationId", correlationID,
			"targetWorkflow", target.config.ID,
			"validationErrors", validationErrors)
		return false
	}

	logger.Debug("Payload validation passed",
		"signalName", signal.Name,
		"correlationId", correlationID,
		"targetWorkflow", target.config.ID)
	return true
}

// generateWorkflowExecID generates a unique workflow execution ID
func generateWorkflowExecID(ctx workflow.Context) core.ID {
	logger := workflow.GetLogger(ctx)

	var workflowExecID core.ID
	// Use versioning to handle backward compatibility during replay
	version := workflow.GetVersion(ctx, "workflow-id-generation", workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		// Old behavior for existing workflows - use deterministic fallback
		return core.ID("fallback-id")
	}

	// New behavior with SideEffect
	if err := workflow.SideEffect(ctx, func(_ workflow.Context) any {
		return core.MustNewID()
	}).Get(&workflowExecID); err != nil {
		logger.Error("Failed to generate workflow execution ID", "error", err)
		return core.ID("fallback-exec-id")
	}
	return workflowExecID
}

// executeChildWorkflow starts and tracks a child workflow execution
func executeChildWorkflow(
	ctx workflow.Context,
	signal EventSignal,
	target *compiledTrigger,
	correlationID string,
) bool {
	logger := workflow.GetLogger(ctx)

	workflowExecID := generateWorkflowExecID(ctx)

	cwo := workflow.ChildWorkflowOptions{
		WorkflowID:        target.config.ID + "-" + workflowExecID.String(),
		ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON, // Let child continue if parent restarts
	}
	childCtx := workflow.WithChildOptions(ctx, cwo)
	childInput := WorkflowInput{
		WorkflowID:     target.config.ID,
		WorkflowExecID: workflowExecID,
		Input:          &signal.Payload,
	}

	// Execute child workflow with comprehensive error handling
	logger.Info("Starting child workflow",
		"targetWorkflow", target.config.ID,
		"workflowId", cwo.WorkflowID,
		"signalName", signal.Name,
		"correlationId", correlationID)

	childFuture := workflow.ExecuteChildWorkflow(childCtx, CompozyWorkflow, childInput)

	// Get the execution to ensure deterministic behavior during replay
	var childExecution workflow.Execution
	err := childFuture.GetChildWorkflowExecution().Get(ctx, &childExecution)
	if err != nil {
		logger.Error("Failed to get child workflow execution",
			"workflowId", cwo.WorkflowID,
			"targetWorkflow", target.config.ID,
			"signalName", signal.Name,
			"correlationId", correlationID,
			"error", err)
		return false
	}

	logger.Info("Successfully started child workflow",
		"workflowId", cwo.WorkflowID,
		"targetWorkflow", target.config.ID,
		"signalName", signal.Name,
		"correlationId", correlationID,
		"childRunId", childExecution.RunID)

	return true
}

// processEventSignal handles a single event signal with validation and child workflow execution
func processEventSignal(ctx workflow.Context, signal EventSignal, signalMap map[string]*compiledTrigger) bool {
	logger := workflow.GetLogger(ctx)

	// Use provided correlation ID or generate one for tracking this event
	correlationID := generateCorrelationID(ctx, signal.CorrelationID)
	logger.Info("Received signal", "name", signal.Name, "correlationId", correlationID)

	// Find target workflow with enhanced error handling
	target, ok := signalMap[signal.Name]
	if !ok {
		logger.Warn("Unknown signal - no workflow configured",
			"signalName", signal.Name,
			"correlationId", correlationID,
			"availableSignals", getRegisteredSignalNames(signalMap))
		return false // Not a fatal error, just unknown signal
	}

	// Validate payload against pre-compiled schema if defined
	if !validateSignalPayload(ctx, signal, target, correlationID) {
		return false // Not a fatal error, just validation failure
	}

	// Start child workflow with enhanced error handling and retry options
	return executeChildWorkflow(ctx, signal, target, correlationID)
}

// DispatcherWorkflow handles event routing
func DispatcherWorkflow(ctx workflow.Context, projectName string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("DispatcherWorkflow started", "project", projectName)

	// Load workflow configurations
	var data *wfacts.GetData
	lao := workflow.LocalActivityOptions{StartToCloseTimeout: 10 * time.Second}
	ctx = workflow.WithLocalActivityOptions(ctx, lao)
	err := workflow.ExecuteLocalActivity(ctx, wfacts.GetDataLabel, &wfacts.GetDataInput{WorkflowID: "order-processor"}).
		Get(ctx, &data)
	if err != nil {
		logger.Error("Failed to load workflow configuration", "error", err)
		return fmt.Errorf("configuration load failed: %w", err)
	}

	// Build signal routing map with pre-compiled schemas
	signalMap, err := buildSignalRoutingMap(ctx, data)
	if err != nil {
		return err
	}
	// Listen for signals with Continue-As-New protection
	const maxSignalsPerRun = 1000               // Protect from unbounded history growth
	const maxConsecutiveErrors = 10             // Circuit breaker threshold
	const circuitBreakerDelay = 5 * time.Second // Backoff period

	signalChan := workflow.GetSignalChannel(ctx, DispatcherEventChannel)
	processed := 0
	consecutiveErrors := 0

	logger.Info("Dispatcher ready to process events", "maxSignalsPerRun", maxSignalsPerRun)

	for {
		// Check for cancellation before blocking on Receive
		if ctx.Err() != nil {
			logger.Info("DispatcherWorkflow canceled", "processedEvents", processed)
			return ctx.Err()
		}
		// Protect from unbounded history growth
		if processed >= maxSignalsPerRun {
			logger.Info("Reaching max signals per run, continuing as new",
				"processed", processed, "maxSignalsPerRun", maxSignalsPerRun)
			return workflow.NewContinueAsNewError(ctx, DispatcherWorkflow, projectName)
		}
		// Circuit breaker: if too many consecutive errors, pause briefly
		if consecutiveErrors >= maxConsecutiveErrors {
			logger.Warn("Too many consecutive errors, implementing backoff",
				"consecutiveErrors", consecutiveErrors, "maxConsecutiveErrors", maxConsecutiveErrors,
				"backoffDelay", circuitBreakerDelay)
			if err := workflow.Sleep(ctx, circuitBreakerDelay); err != nil {
				logger.Error("Circuit breaker sleep interrupted", "error", err)
			}
			// Note: consecutiveErrors will only reset on successful processing
		}
		var signal EventSignal
		ok := signalChan.Receive(ctx, &signal)
		if !ok {
			// Channel closed, restart
			return workflow.NewContinueAsNewError(ctx, DispatcherWorkflow, projectName)
		}
		if ctx.Err() != nil {
			logger.Info("DispatcherWorkflow canceled while receiving signal")
			return ctx.Err()
		}

		// Skip empty signals (initialization signals)
		if signal.Name == "" {
			logger.Debug("Skipping empty signal (likely initialization signal)")
			processed++
			continue
		}
		// Process the event signal
		success := processEventSignal(ctx, signal, signalMap)

		if success {
			// Reset consecutive errors on successful processing
			consecutiveErrors = 0
		} else {
			// Count failures (unknown signals, validation errors) toward error count
			consecutiveErrors++
		}
		processed++
	}
}
