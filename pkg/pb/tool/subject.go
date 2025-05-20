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
	corrID := pb.GetCorrID(x)
	toolID := pb.GetToolID(x)
	return fmt.Sprintf("compozy.%s.tool.cmds.%s.execute", corrID, toolID)
}

// -----------------------------------------------------------------------------
// State Event Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a ToolExecutionStartedEvent.
// Pattern: compozy.<correlation_id>.tool.events.<tool_exec_id>.started
func (x *ToolExecutionStartedEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	toolExecID := pb.GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.events.%s.started", corrID, toolExecID)
}

// ToSubject generates the NATS subject for a ToolExecutionSuccessEvent.
// Pattern: compozy.<correlation_id>.tool.events.<tool_exec_id>.success
func (x *ToolExecutionSuccessEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	toolExecID := pb.GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.events.%s.success", corrID, toolExecID)
}

// ToSubject generates the NATS subject for a ToolExecutionFailedEvent.
// Pattern: compozy.<correlation_id>.tool.events.<tool_exec_id>.failed
func (x *ToolExecutionFailedEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	toolExecID := pb.GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.events.%s.failed", corrID, toolExecID)
}
