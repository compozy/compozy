package pb

import (
	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// CmdToolExecute
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdToolExecute.
// Pattern: compozy.<workflow_exec_id>.tool.cmd.<tool_exec_id>.execute
func (x *CmdToolExecute) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetToolExecID(x)
	return core.BuildCmdSubject(core.ComponentTool, wExecID, tExecID, core.CmdExecute)
}

func (x *CmdToolExecute) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentTool, workflowExecID, execID, core.CmdExecute)
}

// -----------------------------------------------------------------------------
// EvtToolStarted
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an EventToolStarted.
// Pattern: compozy.<workflow_exec_id>.tool.evt.<tool_exec_id>.started
func (x *EventToolStarted) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetToolExecID(x)
	return core.BuildEvtSubject(core.ComponentTool, wExecID, tExecID, core.EvtStarted)
}

func (x *EventToolStarted) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTool, workflowExecID, execID, core.EvtStarted)
}

// -----------------------------------------------------------------------------
// EvtToolSuccess
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an EventToolSuccess.
// Pattern: compozy.<workflow_exec_id>.tool.evt.<tool_exec_id>.success
func (x *EventToolSuccess) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetToolExecID(x)
	return core.BuildEvtSubject(core.ComponentTool, wExecID, tExecID, core.EvtSuccess)
}

func (x *EventToolSuccess) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTool, workflowExecID, execID, core.EvtSuccess)
}

// -----------------------------------------------------------------------------
// EvtToolFailed
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an EventToolFailed.
// Pattern: compozy.<workflow_exec_id>.tool.evt.<tool_exec_id>.failed
func (x *EventToolFailed) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetToolExecID(x)
	return core.BuildEvtSubject(core.ComponentTool, wExecID, tExecID, core.EvtFailed)
}

func (x *EventToolFailed) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTool, workflowExecID, execID, core.EvtFailed)
}
