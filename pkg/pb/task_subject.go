package pb

import (
	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// CmdTaskDispatch
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdTaskDispatch.
// Pattern: <version>.compozy.<workflow_exec_id>.task.cmd.<task_id>.dispatch
func (x *CmdTaskDispatch) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildCmdSubject(core.ComponentTask, wExecID, tExecID, core.CmdDispatch)
}

func (x *CmdTaskDispatch) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentTask, workflowExecID, execID, core.CmdDispatch)
}

// -----------------------------------------------------------------------------
// CmdTaskExecute
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdTaskExecute.
// Pattern: <version>.compozy.<workflow_exec_id>.task.cmd.<task_exec_id>.execute
func (x *CmdTaskExecute) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildCmdSubject(core.ComponentTask, wExecID, tExecID, core.CmdExecute)
}

func (x *CmdTaskExecute) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentTask, workflowExecID, execID, core.CmdExecute)
}

// -----------------------------------------------------------------------------
// CmdTaskResume
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdTaskResume.
// Pattern: <version>.compozy.<workflow_exec_id>.task.cmd.<task_exec_id>.resume
func (x *CmdTaskResume) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildCmdSubject(core.ComponentTask, wExecID, tExecID, core.CmdResume)
}

func (x *CmdTaskResume) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildCmdSubject(core.ComponentTask, workflowExecID, execID, core.CmdResume)
}

// -----------------------------------------------------------------------------
// EvtTaskDispatched
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventTaskDispatched.
// Pattern: <version>.compozy.<workflow_exec_id>.task.evt.<task_exec_id>.dispatched
func (x *EventTaskDispatched) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildEvtSubject(core.ComponentTask, wExecID, tExecID, core.EvtDispatched)
}

func (x *EventTaskDispatched) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTask, workflowExecID, execID, core.EvtDispatched)
}

// -----------------------------------------------------------------------------
// EvtTaskStarted
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventTaskStarted.
// Pattern: <version>.compozy.<workflow_exec_id>.task.evt.<task_exec_id>.started
func (x *EventTaskStarted) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildEvtSubject(core.ComponentTask, wExecID, tExecID, core.EvtStarted)
}

func (x *EventTaskStarted) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTask, workflowExecID, execID, core.EvtStarted)
}

// -----------------------------------------------------------------------------
// EvtTaskWaiting
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventTaskWaiting.
// Pattern: <version>.compozy.<workflow_exec_id>.task.evt.<task_exec_id>.waiting_started
func (x *EventTaskWaiting) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildEvtSubject(core.ComponentTask, wExecID, tExecID, core.EvtWaitingStarted)
}

func (x *EventTaskWaiting) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTask, workflowExecID, execID, core.EvtWaitingStarted)
}

// -----------------------------------------------------------------------------
// EvtTaskWaitingEnded
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventTaskWaitingEnded.
// Pattern: <version>.compozy.<workflow_exec_id>.task.evt.<task_exec_id>.waiting_ended
func (x *EventTaskWaitingEnded) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildEvtSubject(core.ComponentTask, wExecID, tExecID, core.EvtWaitingEnded)
}

func (x *EventTaskWaitingEnded) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTask, workflowExecID, execID, core.EvtWaitingEnded)
}

// -----------------------------------------------------------------------------
// EvtTaskWaitingTimedOut
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventTaskWaitingTimedOut.
// Pattern: <version>.compozy.<workflow_exec_id>.task.evt.<task_exec_id>.waiting_timed_out
func (x *EventTaskWaitingTimedOut) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildEvtSubject(core.ComponentTask, wExecID, tExecID, core.EvtWaitingTimedOut)
}

func (x *EventTaskWaitingTimedOut) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTask, workflowExecID, execID, core.EvtWaitingTimedOut)
}

// -----------------------------------------------------------------------------
// EvtTaskSuccess
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventTaskSuccess.
// Pattern: <version>.compozy.<workflow_exec_id>.task.evt.<task_exec_id>.success
func (x *EventTaskSuccess) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildEvtSubject(core.ComponentTask, wExecID, tExecID, core.EvtSuccess)
}

func (x *EventTaskSuccess) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTask, workflowExecID, execID, core.EvtSuccess)
}

// -----------------------------------------------------------------------------
// EvtTaskFailed
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventTaskFailed.
// Pattern: <version>.compozy.<workflow_exec_id>.task.evt.<task_exec_id>.failed
func (x *EventTaskFailed) ToSubject() string {
	wExecID := x.Metadata.WorkflowExecId
	tExecID := GetTaskExecID(x)
	return core.BuildEvtSubject(core.ComponentTask, wExecID, tExecID, core.EvtFailed)
}

func (x *EventTaskFailed) ToSubjectParams(workflowExecID string, execID string) string {
	return core.BuildEvtSubject(core.ComponentTask, workflowExecID, execID, core.EvtFailed)
}
