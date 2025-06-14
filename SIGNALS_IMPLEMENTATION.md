# Signal-Based Workflow Triggers - Technical Implementation Guide

## Overview

This document provides the technical implementation details for adding signal-based workflow triggers to Compozy. The design leverages Temporal's native signal capabilities to enable event-driven workflow execution without over-engineering custom infrastructure.

## Architecture

### Core Design: Dispatcher Workflow Pattern

```
External System → REST API → Temporal Signal → Dispatcher Workflow → Child Workflows
```

The architecture centers around a long-running "Dispatcher Workflow" that:

1. Listens for incoming event signals
2. Routes events to appropriate workflows based on configuration
3. Starts target workflows as child workflows with event payload as input

### Key Components

1. **Event REST API** (`/api/v1/events`)

    - Receives external events
    - Sends signals to dispatcher workflow
    - Returns immediate acknowledgment

2. **Dispatcher Workflow**

    - Singleton per project
    - Maintains event→workflow routing map
    - Handles signal reception and child workflow dispatch

3. **Workflow Configuration**
    - Extended with `triggers` section
    - Maps event names to workflows
    - Optional payload validation

## Implementation Steps

### Step 1: Extend Workflow Configuration

**File: `engine/workflow/config.go`**

```go
// Add trigger types
type TriggerType string

const (
    TriggerTypeSignal TriggerType = "signal"
)

// Define trigger structure
type Trigger struct {
    Type   TriggerType    `json:"type"   yaml:"type"   mapstructure:"type"`
    Name   string         `json:"name"   yaml:"name"   mapstructure:"name"`
    Schema *schema.Schema `json:"schema,omitempty" yaml:"schema,omitempty" mapstructure:"schema,omitempty"`
}

// Add to Config struct
type Config struct {
    // ... existing fields ...
    Triggers []Trigger `json:"triggers,omitempty" yaml:"triggers,omitempty" mapstructure:"triggers,omitempty"`
    // ... rest of struct ...
}
```

**Validation:** Add trigger validation in `Config.Validate()`:

```go
for i := range w.Triggers {
    trigger := &w.Triggers[i]
    if trigger.Type != TriggerTypeSignal {
        return fmt.Errorf("unsupported trigger type: %s", trigger.Type)
    }
    if trigger.Name == "" {
        return fmt.Errorf("trigger name is required")
    }
}
```

### Step 2: Create Dispatcher Workflow

**File: `engine/worker/dispatcher_workflow.go`**

```go
package worker

import (
    "time"
    "go.temporal.io/sdk/workflow"
    "github.com/compozy/compozy/engine/core"
    wf "github.com/compozy/compozy/engine/workflow"
    wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

// EventSignal represents an incoming event
type EventSignal struct {
    Name    string      `json:"name"`
    Payload core.Input  `json:"payload"`
}

// DispatcherWorkflow handles event routing
func DispatcherWorkflow(ctx workflow.Context, projectName string) error {
    logger := workflow.GetLogger(ctx)
    logger.Info("DispatcherWorkflow started", "project", projectName)

    // Load workflow configurations
    var data *wfacts.GetData
    lao := workflow.LocalActivityOptions{StartToCloseTimeout: 10 * time.Second}
    ctx = workflow.WithLocalActivityOptions(ctx, lao)
    err := workflow.ExecuteLocalActivity(ctx, wfacts.GetDataLabel, &wfacts.GetDataInput{}).Get(ctx, &data)
    if err != nil {
        return err
    }

    // Build signal routing map
    signalMap := make(map[string]*wf.Config)
    for _, wcfg := range data.Workflows {
        for _, trigger := range wcfg.Triggers {
            if trigger.Type == wf.TriggerTypeSignal {
                signalMap[trigger.Name] = wcfg
                logger.Info("Registered signal trigger", "signal", trigger.Name, "workflow", wcfg.ID)
            }
        }
    }

    // Listen for signals
    signalChan := workflow.GetSignalChannel(ctx, "event_channel")
    for {
        var signal EventSignal
        signalChan.Receive(ctx, &signal)

        logger.Info("Received signal", "name", signal.Name)

        // Find target workflow
        targetWf, ok := signalMap[signal.Name]
        if !ok {
            logger.Warn("Unknown signal", "name", signal.Name)
            continue
        }

        // Start child workflow
        cwo := workflow.ChildWorkflowOptions{
            WorkflowID: targetWf.ID + "-" + core.MustNewID().String(),
        }
        ctx := workflow.WithChildOptions(ctx, cwo)

        childInput := WorkflowInput{
            WorkflowID:     targetWf.ID,
            WorkflowExecID: core.MustNewID(),
            Input:          &signal.Payload,
        }

        err := workflow.ExecuteChildWorkflow(ctx, CompozyWorkflow, childInput).Get(ctx, nil)
        if err != nil {
            logger.Error("Failed to start child workflow", "error", err)
        }
    }
}
```

### Step 3: Create Events REST API

**File: `engine/workflow/router/events.go`**

```go
package wfrouter

import (
    "github.com/gin-gonic/gin"
    "github.com/gosimple/slug"
    "go.temporal.io/sdk/client"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/infra/server/router"
)

type EventRequest struct {
    Name    string      `json:"name" binding:"required"`
    Payload core.Input  `json:"payload"`
}

type EventResponse struct {
    Message string `json:"message"`
    EventID string `json:"event_id"`
}

// handleEvent processes incoming events
// @Summary     Send event
// @Description Trigger workflows by sending events
// @Tags        events
// @Accept      json
// @Produce     json
// @Param       event body EventRequest true "Event data"
// @Success     202 {object} router.Response{data=EventResponse}
// @Failure     400 {object} router.Response{error=router.ErrorInfo}
// @Router      /events [post]
func handleEvent(c *gin.Context) {
    var req EventRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        router.RespondWithError(c, 400, router.ValidationError("Invalid event format", err))
        return
    }

    state := router.GetAppState(c)
    worker := state.Worker

    // Deterministic dispatcher workflow ID
    dispatcherID := "dispatcher-" + slug.Make(state.ProjectConfig.Name)
    taskQueue := slug.Make(state.ProjectConfig.Name)

    // Send signal with start
    _, err := worker.GetClient().SignalWithStartWorkflow(
        c.Request.Context(),
        dispatcherID,
        "event_channel",
        req,
        client.StartWorkflowOptions{
            ID:        dispatcherID,
            TaskQueue: taskQueue,
        },
        DispatcherWorkflow,
        state.ProjectConfig.Name,
    )

    if err != nil {
        router.RespondWithError(c, 500, router.InternalError("Failed to send event", err))
        return
    }

    eventID := core.MustNewID().String()
    router.RespondAccepted(c, "event received", EventResponse{
        Message: "event received",
        EventID: eventID,
    })
}
```

### Step 4: Register Dispatcher and Routes

**File: `engine/worker/mod.go`**

Add dispatcher registration and startup:

```go
func (o *Worker) Setup(ctx context.Context) error {
    o.worker.RegisterWorkflow(CompozyWorkflow)
    o.worker.RegisterWorkflow(DispatcherWorkflow) // Register dispatcher

    // ... register activities ...

    err := o.worker.Start()
    if err != nil {
        return err
    }

    // Ensure dispatcher is running
    go o.ensureDispatcherRunning(context.Background())

    return nil
}

func (o *Worker) ensureDispatcherRunning(ctx context.Context) {
    dispatcherID := "dispatcher-" + slug.Make(o.projectConfig.Name)
    _, err := o.client.SignalWithStartWorkflow(
        ctx,
        dispatcherID,
        "noop_channel",
        nil,
        client.StartWorkflowOptions{
            ID:        dispatcherID,
            TaskQueue: o.taskQueue,
        },
        DispatcherWorkflow,
        o.projectConfig.Name,
    )

    if err != nil {
        if !errors.IsWorkflowExecutionAlreadyStartedError(err) {
            logger.Error("Failed to start dispatcher", "error", err)
        }
    }
}

// Add method to expose client
func (o *Worker) GetClient() client.Client {
    return o.client
}
```

**File: `engine/workflow/router/register.go`**

Add event route registration:

```go
func Register(e *gin.Engine, state *server.AppState) {
    api := e.Group("/api")
    v0 := api.Group("/v0")

    // ... existing routes ...

    // Event routes
    v1 := api.Group("/v1")
    v1.POST("/events", middleware.Auth(), handleEvent)
}
```

### Step 5: Update Workflow Input Context

To make event data accessible in workflows, update the template context:

**File: `engine/core/input.go`**

```go
// Add event context to workflow input
type InputContext struct {
    Input   Input  `json:"input"`
    Trigger *Input `json:"trigger,omitempty"` // Event payload for signal-triggered workflows
}
```

## Testing Strategy

### Unit Tests

1. **Configuration Tests** (`engine/workflow/config_test.go`)

    - Test trigger parsing and validation
    - Test invalid trigger configurations

2. **Dispatcher Tests** (`engine/worker/dispatcher_workflow_test.go`)
    - Test signal routing logic
    - Test child workflow dispatch
    - Test unknown event handling

### Integration Tests

1. **End-to-End Signal Flow** (`test/e2e/signals_test.go`)

    ```go
    func TestSignalTriggeredWorkflow(t *testing.T) {
        // 1. Define workflow with signal trigger
        // 2. Start worker with dispatcher
        // 3. Send event via API
        // 4. Verify workflow execution
        // 5. Check workflow received event payload
    }
    ```

2. **Multiple Workflows per Event**

    - Test multiple workflows listening to same event
    - Verify all workflows execute independently

3. **Error Scenarios**
    - Test invalid event names
    - Test malformed payloads
    - Test dispatcher recovery

## Migration Guide

### For Existing Workflows

No changes required. Existing workflows continue to work with direct API triggers.

### For New Event-Driven Workflows

1. Add `triggers` section to workflow YAML:

    ```yaml
    triggers:
        - type: signal
          name: "order.created"
    ```

2. Access event data in tasks:

    ```yaml
    tasks:
        - id: process-order
          config:
              template: |
                  Processing order {{ .trigger.orderId }}
    ```

3. Send events via API:
    ```bash
    curl -X POST http://localhost:8080/api/v1/events \
        -H "Authorization: Bearer $TOKEN" \
        -d '{"name": "order.created", "payload": {"orderId": "123"}}'
    ```

## Monitoring and Observability

### Logging

Structured logging at key points:

- Event reception
- Signal routing decisions
- Child workflow dispatch
- Error conditions

### Temporal UI

Monitor dispatcher workflow in Temporal UI:

- Signal history
- Child workflow creation
- Error states

## Security Considerations

1. **API Authentication**: Events endpoint requires bearer token
2. **Event Validation**: Optional schema validation for payloads
3. **Rate Limiting**: Consider adding rate limits for event API
4. **Event Sources**: Whitelist trusted event sources if needed

## Performance Optimization

1. **Dispatcher Efficiency**

    - In-memory routing map rebuilt on replay
    - Minimal processing per signal
    - Child workflows handle actual work

2. **Scalability**

    - Single dispatcher per project/queue
    - Horizontal scaling via Temporal workers
    - Event backpressure handled by Temporal

3. **Latency**
    - Sub-second event to workflow dispatch
    - Async acknowledgment to event producers
    - No blocking operations in dispatcher

## Future Enhancements

1. **Event Filtering**: Add conditions to triggers
2. **Event Transformation**: Transform payloads before dispatch
3. **Workflow Signals**: Allow running workflows to receive signals
4. **Event Replay**: Replay historical events for testing
5. **External Integrations**: Direct webhook receivers, message queue consumers
