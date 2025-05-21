package workflow

import (
	"fmt"

	"github.com/compozy/compozy/pkg/pb"
)

// -----------------------------------------------------------------------------
// Command Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a WorkflowTriggerCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_id>.trigger
func (x *WorkflowTriggerCommand) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfID := pb.GetWorkflowID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.trigger", corrID, wfID)
}

// ToSubject generates the NATS subject for a WorkflowExecuteCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.execute
func (x *WorkflowExecuteCommand) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.execute", corrID, wfExecID)
}

// ToSubject generates the NATS subject for a WorkflowPauseCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.pause
func (x *WorkflowPauseCommand) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.pause", corrID, wfExecID)
}

// ToSubject generates the NATS subject for a WorkflowResumeCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.resume
func (x *WorkflowResumeCommand) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.resume", corrID, wfExecID)
}

// ToSubject generates the NATS subject for a WorkflowCancelCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.cancel
func (x *WorkflowCancelCommand) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.cancel", corrID, wfExecID)
}

// -----------------------------------------------------------------------------
// State Event Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a WorkflowExecutionStartedEvent.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.started
func (x *WorkflowExecutionStartedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.started", corrID, wfExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionPausedEvent.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.paused
func (x *WorkflowExecutionPausedEvent) ToSubject() string {
	corrID := "unknown_correlation_id"
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.paused", corrID, wfExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionResumedEvent.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.resumed
func (x *WorkflowExecutionResumedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.resumed", corrID, wfExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionSuccessEvent.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.success
func (x *WorkflowExecutionSuccessEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.success", corrID, wfExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionFailedEvent.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.failed
func (x *WorkflowExecutionFailedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.failed", corrID, wfExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionCancelledEvent.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.canceled
func (x *WorkflowExecutionCancelledEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.canceled", corrID, wfExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionTimedOutEvent.
// Pattern: compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.timed_out
func (x *WorkflowExecutionTimedOutEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	wfExecID := pb.GetWorkflowExecID(x)
	return fmt.Sprintf("compozy.%s.workflow.evts.%s.timed_out", corrID, wfExecID)
}
