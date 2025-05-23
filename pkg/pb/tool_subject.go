package pb

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// Commands
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdToolExecute.
// Pattern: compozy.<correlation_id>.tool.cmds.<tool_id>.execute
func (x *CmdToolExecute) ToSubject() string {
	corrID := GetCorrelationID(x)
	toolID := GetToolID(x)
	return fmt.Sprintf("compozy.%s.tool.cmds.%s.execute", corrID, toolID)
}

// -----------------------------------------------------------------------------
// State Events
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an EventToolStarted.
// Pattern: compozy.<correlation_id>.tool.evts.<tool_exec_id>.started
func (x *EventToolStarted) ToSubject() string {
	corrID := GetCorrelationID(x)
	toolExecID := GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.evts.%s.started", corrID, toolExecID)
}

// ToSubject generates the NATS subject for an EventToolSuccess.
// Pattern: compozy.<correlation_id>.tool.evts.<tool_exec_id>.success
func (x *EventToolSuccess) ToSubject() string {
	corrID := GetCorrelationID(x)
	toolExecID := GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.evts.%s.success", corrID, toolExecID)
}

// ToSubject generates the NATS subject for an EventToolFailed.
// Pattern: compozy.<correlation_id>.tool.evts.<tool_exec_id>.failed
func (x *EventToolFailed) ToSubject() string {
	corrID := GetCorrelationID(x)
	toolExecID := GetToolExecID(x)
	return fmt.Sprintf("compozy.%s.tool.evts.%s.failed", corrID, toolExecID)
}
