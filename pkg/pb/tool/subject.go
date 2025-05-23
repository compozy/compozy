package tool

import (
	"fmt"

	"github.com/compozy/compozy/pkg/pb"
)

// -----------------------------------------------------------------------------
// Command Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdToolExecute.
// Pattern: compozy.<correlation_id>.tool.cmds.<tool_exec_id>.execute
func (x *CmdToolExecute) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	toolID := pb.GetToolID(x)
	return fmt.Sprintf("compozy.%s.tool.cmds.%s.execute", corrID, toolID)
}

// -----------------------------------------------------------------------------
// State Event Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventToolStarted.
// Pattern: compozy.<correlation_id>.tool.evts.<tool_exec_id>.started
func (x *EventToolStarted) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	toolExecID := pb.GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.evts.%s.started", corrID, toolExecID)
}

// ToSubject generates the NATS subject for a EventToolSuccess.
// Pattern: compozy.<correlation_id>.tool.evts.<tool_exec_id>.success
func (x *EventToolSuccess) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	toolExecID := pb.GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.evts.%s.success", corrID, toolExecID)
}

// ToSubject generates the NATS subject for a EventToolFailed.
// Pattern: compozy.<correlation_id>.tool.evts.<tool_exec_id>.failed
func (x *EventToolFailed) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	toolExecID := pb.GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.evts.%s.failed", corrID, toolExecID)
}
