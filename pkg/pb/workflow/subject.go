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
	correlationID := pb.GetCorrelationId(x)
	workflowID := pb.GetWorkflowId(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.trigger", correlationID, workflowID)
}

// ToSubject generates the NATS subject for a WorkflowTriggerAsyncCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_id>.trigger_async
func (x *WorkflowTriggerAsyncCommand) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowID := pb.GetWorkflowId(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.trigger_async", correlationID, workflowID)
}

// ToSubject generates the NATS subject for a WorkflowExecuteCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.execute
func (x *WorkflowExecuteCommand) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.execute", correlationID, workflowExecID)
}

// ToSubject generates the NATS subject for a WorkflowPauseCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.pause
func (x *WorkflowPauseCommand) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.pause", correlationID, workflowExecID)
}

// ToSubject generates the NATS subject for a WorkflowResumeCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.resume
func (x *WorkflowResumeCommand) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.resume", correlationID, workflowExecID)
}

// ToSubject generates the NATS subject for a WorkflowCancelCommand.
// Pattern: compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.cancel
func (x *WorkflowCancelCommand) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.cmds.%s.cancel", correlationID, workflowExecID)
}

// -----------------------------------------------------------------------------
// State Event Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a WorkflowExecutionStartedEvent.
// Pattern: compozy.<correlation_id>.workflow.events.<workflow_exec_id>.started
func (x *WorkflowExecutionStartedEvent) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.events.%s.started", correlationID, workflowExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionPausedEvent.
// Pattern: compozy.<correlation_id>.workflow.events.<workflow_exec_id>.paused
func (x *WorkflowExecutionPausedEvent) ToSubject() string {
	correlationID := "unknown_correlation_id"
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.events.%s.paused", correlationID, workflowExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionResumedEvent.
// Pattern: compozy.<correlation_id>.workflow.events.<workflow_exec_id>.resumed
func (x *WorkflowExecutionResumedEvent) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.events.%s.resumed", correlationID, workflowExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionSuccessEvent.
// Pattern: compozy.<correlation_id>.workflow.events.<workflow_exec_id>.success
func (x *WorkflowExecutionSuccessEvent) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.events.%s.success", correlationID, workflowExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionFailedEvent.
// Pattern: compozy.<correlation_id>.workflow.events.<workflow_exec_id>.failed
func (x *WorkflowExecutionFailedEvent) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.events.%s.failed", correlationID, workflowExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionCancelledEvent.
// Pattern: compozy.<correlation_id>.workflow.events.<workflow_exec_id>.cancelled
func (x *WorkflowExecutionCancelledEvent) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.events.%s.cancelled", correlationID, workflowExecID)
}

// ToSubject generates the NATS subject for a WorkflowExecutionTimedOutEvent.
// Pattern: compozy.<correlation_id>.workflow.events.<workflow_exec_id>.timed_out
func (x *WorkflowExecutionTimedOutEvent) ToSubject() string {
	correlationID := pb.GetCorrelationId(x)
	workflowExecID := pb.GetWorkflowExecId(x)
	return fmt.Sprintf("compozy.%s.workflow.events.%s.timed_out", correlationID, workflowExecID)
}
