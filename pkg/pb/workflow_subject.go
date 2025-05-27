package pb

import (
	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// CmdWorkflowTrigger
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdWorkflowTrigger.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.cmd.<workflow_exec_id>.trigger
func (x *CmdWorkflowTrigger) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildCmdSubject(core.ComponentWorkflow, wExecID, wExecID, core.CmdTrigger)
}

func (x *CmdWorkflowTrigger) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentWorkflow, workflowExecID, execID, core.CmdTrigger)
}

// -----------------------------------------------------------------------------
// CmdWorkflowExecute
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdWorkflowExecute.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.cmd.<workflow_exec_id>.execute
func (x *CmdWorkflowExecute) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	return core.BuildCmdSubject(core.ComponentWorkflow, wExecID, wExecID, core.CmdExecute)
}

func (x *CmdWorkflowExecute) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentWorkflow, workflowExecID, execID, core.CmdExecute)
}

// -----------------------------------------------------------------------------
// CmdWorkflowPause
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdWorkflowPause.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.cmd.<workflow_exec_id>.pause
func (x *CmdWorkflowPause) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildCmdSubject(core.ComponentWorkflow, wExecID, wExecID, core.CmdPause)
}

func (x *CmdWorkflowPause) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentWorkflow, workflowExecID, execID, core.CmdPause)
}

// -----------------------------------------------------------------------------
// CmdWorkflowResume
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdWorkflowResume.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.cmd.<workflow_exec_id>.resume
func (x *CmdWorkflowResume) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildCmdSubject(core.ComponentWorkflow, wExecID, wExecID, core.CmdResume)
}

func (x *CmdWorkflowResume) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentWorkflow, workflowExecID, execID, core.CmdResume)
}

// -----------------------------------------------------------------------------
// CmdWorkflowCancel
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdWorkflowCancel.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.cmd.<workflow_exec_id>.cancel
func (x *CmdWorkflowCancel) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildCmdSubject(core.ComponentWorkflow, wExecID, wExecID, core.CmdCancel)
}

func (x *CmdWorkflowCancel) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentWorkflow, workflowExecID, execID, core.CmdCancel)
}

// -----------------------------------------------------------------------------
// EvtWorkflowStarted
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventWorkflowStarted.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.evt.<workflow_exec_id>.started
func (x *EventWorkflowStarted) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildEvtSubject(core.ComponentWorkflow, wExecID, wExecID, core.EvtStarted)
}

func (x *EventWorkflowStarted) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentWorkflow, workflowExecID, execID, core.EvtStarted)
}

// -----------------------------------------------------------------------------
// EvtWorkflowPaused
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventWorkflowPaused.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.evt.<workflow_exec_id>.paused
func (x *EventWorkflowPaused) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildEvtSubject(core.ComponentWorkflow, wExecID, wExecID, core.EvtPaused)
}

func (x *EventWorkflowPaused) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentWorkflow, workflowExecID, execID, core.EvtPaused)
}

// -----------------------------------------------------------------------------
// EvtWorkflowResumed
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventWorkflowResumed.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.evt.<workflow_exec_id>.resumed
func (x *EventWorkflowResumed) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildEvtSubject(core.ComponentWorkflow, wExecID, wExecID, core.EvtResumed)
}

func (x *EventWorkflowResumed) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentWorkflow, workflowExecID, execID, core.EvtResumed)
}

// -----------------------------------------------------------------------------
// EvtWorkflowSuccess
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventWorkflowSuccess.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.evt.<workflow_exec_id>.success
func (x *EventWorkflowSuccess) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildEvtSubject(core.ComponentWorkflow, wExecID, wExecID, core.EvtSuccess)
}

func (x *EventWorkflowSuccess) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentWorkflow, workflowExecID, execID, core.EvtSuccess)
}

// -----------------------------------------------------------------------------
// EvtWorkflowFailed
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventWorkflowFailed.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.evt.<workflow_exec_id>.failed
func (x *EventWorkflowFailed) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildEvtSubject(core.ComponentWorkflow, wExecID, wExecID, core.EvtFailed)
}

func (x *EventWorkflowFailed) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentWorkflow, workflowExecID, execID, core.EvtFailed)
}

// -----------------------------------------------------------------------------
// EvtWorkflowCanceled
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventWorkflowCanceled.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.evt.<workflow_exec_id>.canceled
func (x *EventWorkflowCanceled) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildEvtSubject(core.ComponentWorkflow, wExecID, wExecID, core.EvtCanceled)
}

func (x *EventWorkflowCanceled) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentWorkflow, workflowExecID, execID, core.EvtCanceled)
}

// -----------------------------------------------------------------------------
// EvtWorkflowTimedOut
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventWorkflowTimedOut.
// Pattern: <version>.compozy.<workflow_exec_id>.workflow.evt.<workflow_exec_id>.timed_out
func (x *EventWorkflowTimedOut) ToSubject() string {
	wExecID := GetWorkflowExecID(x)
	return core.BuildEvtSubject(core.ComponentWorkflow, wExecID, wExecID, core.EvtTimedOut)
}

func (x *EventWorkflowTimedOut) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentWorkflow, workflowExecID, execID, core.EvtTimedOut)
}
