package kernel

import (
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/pkg/compozy/events"
)

// KernelDeps groups shared infrastructure used by kernel handlers.
//
//nolint:revive // KernelDeps is the task- and ADR-defined API name for kernel construction.
type KernelDeps struct {
	Logger        *slog.Logger
	EventBus      *events.Bus[events.Event]
	Workspace     workspace.Context
	AgentRegistry agent.Registry

	ops operations
}

// BuildDefault constructs a dispatcher with all six Phase A command handlers registered.
func BuildDefault(deps KernelDeps) *Dispatcher {
	dispatcher := NewDispatcher()
	ops := deps.resolveOperations()

	Register(dispatcher, newRunStartHandler(deps, ops))
	Register(dispatcher, newWorkflowPrepareHandler(deps, ops))
	Register(dispatcher, newWorkflowSyncHandler(deps, ops))
	Register(dispatcher, newWorkflowArchiveHandler(deps, ops))
	Register(dispatcher, newWorkspaceMigrateHandler(deps, ops))
	Register(dispatcher, newReviewsFetchHandler(deps, ops))

	return dispatcher
}

func (deps KernelDeps) resolveOperations() operations {
	if deps.ops != nil {
		return deps.ops
	}
	return realOperations{
		agentRegistry: deps.AgentRegistry,
	}
}

func expectedDefaultCommandTypes() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[commands.RunStartCommand](),
		reflect.TypeFor[commands.WorkflowPrepareCommand](),
		reflect.TypeFor[commands.WorkflowSyncCommand](),
		reflect.TypeFor[commands.WorkflowArchiveCommand](),
		reflect.TypeFor[commands.WorkspaceMigrateCommand](),
		reflect.TypeFor[commands.ReviewsFetchCommand](),
	}
}

func selfTestDefaultRegistry(d *Dispatcher) error {
	registered := make(map[reflect.Type]struct{}, len(registeredCommandTypes(d)))
	for _, commandType := range registeredCommandTypes(d) {
		registered[commandType] = struct{}{}
	}

	missing := make([]string, 0)
	for _, commandType := range expectedDefaultCommandTypes() {
		if _, ok := registered[commandType]; ok {
			continue
		}
		missing = append(missing, formatType(commandType))
	}
	if len(missing) == 0 {
		return nil
	}

	sort.Strings(missing)
	return fmt.Errorf("kernel: missing default handlers for %s", strings.Join(missing, ", "))
}
