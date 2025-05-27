package pb

import (
	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// CmdAgentExecute
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an CmdAgentExecute.
// Pattern: <version>.compozy.<workflow_exec_id>.agent.cmd.<agent_id>.execute
func (x *CmdAgentExecute) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	aExecID := GetAgentExecID(x)
	return core.BuildCmdSubject(core.ComponentAgent, wExecID, aExecID, core.CmdExecute)
}

func (x *CmdAgentExecute) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentAgent, workflowExecID, execID, core.CmdExecute)
}

// -----------------------------------------------------------------------------
// EvtAgentStarted
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an EventAgentStarted.
// Pattern: <version>.compozy.<workflow_exec_id>.agent.evt.<agent_exec_id>.started
func (x *EventAgentStarted) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	aExecID := GetAgentExecID(x)
	return core.BuildEvtSubject(core.ComponentAgent, wExecID, aExecID, core.EvtStarted)
}

func (x *EventAgentStarted) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentAgent, workflowExecID, execID, core.EvtStarted)
}

// -----------------------------------------------------------------------------
// EvtAgentSuccess
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an EventAgentSuccess.
// Pattern: <version>.compozy.<workflow_exec_id>.agent.evt.<agent_exec_id>.success
func (x *EventAgentSuccess) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	aExecID := GetAgentExecID(x)
	return core.BuildEvtSubject(core.ComponentAgent, wExecID, aExecID, core.EvtSuccess)
}

func (x *EventAgentSuccess) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentAgent, workflowExecID, execID, core.EvtSuccess)
}

// -----------------------------------------------------------------------------
// EvtAgentFailed
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an EventAgentFailed.
// Pattern: <version>.compozy.<workflow_exec_id>.agent.evt.<agent_exec_id>.failed
func (x *EventAgentFailed) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	aExecID := GetAgentExecID(x)
	return core.BuildEvtSubject(core.ComponentAgent, wExecID, aExecID, core.EvtFailed)
}

func (x *EventAgentFailed) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentAgent, workflowExecID, execID, core.EvtFailed)
}
