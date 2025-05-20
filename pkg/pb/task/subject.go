package task

import (
	"fmt"

	"github.com/compozy/compozy/pkg/pb"
)

// -----------------------------------------------------------------------------
// Command Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a TaskTriggerCommand.
// Pattern: compozy.<correlation_id>.task.cmds.<task_id>.trigger
func (x *TaskTriggerCommand) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tID := pb.GetTaskID(x)
	return fmt.Sprintf("compozy.%s.task.cmds.%s.trigger", corrID, tID)
}

// ToSubject generates the NATS subject for a TaskExecuteCommand.
// Pattern: compozy.<correlation_id>.task.cmds.<task_exec_id>.execute
func (x *TaskExecuteCommand) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.cmds.%s.execute", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskResumeCommand.
// Pattern: compozy.<correlation_id>.task.cmds.<task_exec_id>.resume
func (x *TaskResumeCommand) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.cmds.%s.resume", corrID, tExecID)
}

// -----------------------------------------------------------------------------
// State Event Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a TaskDispatchedEvent.
// Pattern: compozy.<correlation_id>.task.events.<task_exec_id>.dispatched
func (x *TaskDispatchedEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.events.%s.dispatched", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskExecutionStartedEvent.
// Pattern: compozy.<correlation_id>.task.events.<task_exec_id>.started
func (x *TaskExecutionStartedEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.events.%s.started", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskWaitingStartedEvent.
// Pattern: compozy.<correlation_id>.task.events.<task_exec_id>.waiting_started
func (x *TaskWaitingStartedEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.events.%s.waiting_started", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskWaitingEndedEvent.
// Pattern: compozy.<correlation_id>.task.events.<task_exec_id>.waiting_ended
func (x *TaskWaitingEndedEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.events.%s.waiting_ended", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskWaitingTimedOutEvent.
// Pattern: compozy.<correlation_id>.task.events.<task_exec_id>.waiting_timed_out
func (x *TaskWaitingTimedOutEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.events.%s.waiting_timed_out", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskExecutionSuccessEvent.
// Pattern: compozy.<correlation_id>.task.events.<task_exec_id>.success
func (x *TaskExecutionSuccessEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.events.%s.success", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskExecutionFailedEvent.
// Pattern: compozy.<correlation_id>.task.events.<task_exec_id>.failed
func (x *TaskExecutionFailedEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.events.%s.failed", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskRetryScheduledEvent.
// Pattern: compozy.<correlation_id>.task.events.<original_task_exec_id>.retry_scheduled
func (x *TaskRetryScheduledEvent) ToSubject() string {
	corrID := pb.GetCorrID(x)
	originalTaskExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.events.%s.retry_scheduled", corrID, originalTaskExecID)
}
