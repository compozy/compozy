package tool

import (
	"fmt"

	"github.com/compozy/compozy/pkg/pb"
)

// -----------------------------------------------------------------------------
// Command Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a ToolExecuteCommand.
// Pattern: compozy.<correlation_id>.tool.cmds.<tool_exec_id>.execute
func (x *ToolExecuteCommand) ToSubject() string {
	correlationID := pb.GetCorrelationID(x)
	toolID := pb.GetToolID(x)
	return fmt.Sprintf("compozy.%s.tool.cmds.%s.execute", correlationID, toolID)
}

// -----------------------------------------------------------------------------
// State Event Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a ToolExecutionStartedEvent.
// Pattern: compozy.<correlation_id>.tool.events.<tool_exec_id>.started
func (x *ToolExecutionStartedEvent) ToSubject() string {
	correlationID := pb.GetCorrelationID(x)
	toolExecID := pb.GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.events.%s.started", correlationID, toolExecID)
}

// ToSubject generates the NATS subject for a ToolExecutionSuccessEvent.
// Pattern: compozy.<correlation_id>.tool.events.<tool_exec_id>.success
func (x *ToolExecutionSuccessEvent) ToSubject() string {
	correlationID := pb.GetCorrelationID(x)
	toolExecID := pb.GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.events.%s.success", correlationID, toolExecID)
}

// ToSubject generates the NATS subject for a ToolExecutionFailedEvent.
// Pattern: compozy.<correlation_id>.tool.events.<tool_exec_id>.failed
func (x *ToolExecutionFailedEvent) ToSubject() string {
	correlationID := pb.GetCorrelationID(x)
	toolExecID := pb.GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.events.%s.failed", correlationID, toolExecID)
}
