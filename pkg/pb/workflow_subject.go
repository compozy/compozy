package pb

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// Command Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdWorkflowTrigger.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_id>.trigger
func (x *CmdWorkflowTrigger) ToSubject() string {
	corrID := GetCorrelationID(x)
	workflowID := GetWorkflowID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.trigger", corrID, workflowID)
}

// ToSubject generates the NATS subject for a CmdWorkflowExecute.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.execute
func (x *CmdWorkflowExecute) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.execute", corrID, wExecID)
}

// ToSubject generates the NATS subject for a CmdWorkflowPause.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.pause
func (x *CmdWorkflowPause) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.pause", corrID, wExecID)
}

// ToSubject generates the NATS subject for a CmdWorkflowResume.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.resume
func (x *CmdWorkflowResume) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.resume", corrID, wExecID)
}

// ToSubject generates the NATS subject for a CmdWorkflowCancel.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.cancel
func (x *CmdWorkflowCancel) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.cancel", corrID, wExecID)
}

// -----------------------------------------------------------------------------
// State Event Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventWorkflowStarted.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.started
func (x *EventWorkflowStarted) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.started", corrID, wExecID)
}

// ToSubject generates the NATS subject for a EventWorkflowPaused.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.paused
func (x *EventWorkflowPaused) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.paused", corrID, wExecID)
}

// ToSubject generates the NATS subject for a EventWorkflowResumed.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.resumed
func (x *EventWorkflowResumed) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.resumed", corrID, wExecID)
}

// ToSubject generates the NATS subject for a EventWorkflowSuccess.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.success
func (x *EventWorkflowSuccess) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.success", corrID, wExecID)
}

// ToSubject generates the NATS subject for a EventWorkflowFailed.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.failed
func (x *EventWorkflowFailed) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.failed", corrID, wExecID)
}

// ToSubject generates the NATS subject for a EventWorkflowCanceled.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.canceled
func (x *EventWorkflowCanceled) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.canceled", corrID, wExecID)
}

// ToSubject generates the NATS subject for a EventWorkflowTimedOut.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.timed_out
func (x *EventWorkflowTimedOut) ToSubject() string {
	corrID := GetCorrelationID(x)
	wExecID := GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.timed_out", corrID, wExecID)
}
