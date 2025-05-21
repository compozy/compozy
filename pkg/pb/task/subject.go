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
	corrID := pb.GetCorrelationID(x)
	tID := pb.GetTaskID(x)
	return fmt.Sprintf("compozy.%s.task.cmds.%s.trigger", corrID, tID)
}

// ToSubject generates the NATS subject for a TaskExecuteCommand.
// Pattern: compozy.<correlation_id>.task.cmds.<task_exec_id>.execute
func (x *TaskExecuteCommand) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.cmds.%s.execute", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskResumeCommand.
// Pattern: compozy.<correlation_id>.task.cmds.<task_exec_id>.resume
func (x *TaskResumeCommand) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.cmds.%s.resume", corrID, tExecID)
}

// -----------------------------------------------------------------------------
// State Event Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a TaskDispatchedEvent.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.dispatched
func (x *TaskDispatchedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.dispatched", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskExecutionStartedEvent.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.started
func (x *TaskExecutionStartedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.started", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskExecutionWaitingStartedEvent.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_started
func (x *TaskExecutionWaitingStartedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.waiting_started", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskExecutionWaitingEndedEvent.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_ended
func (x *TaskExecutionWaitingEndedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.waiting_ended", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskExecutionWaitingTimedOutEvent.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_timed_out
func (x *TaskExecutionWaitingTimedOutEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.waiting_timed_out", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskExecutionSuccessEvent.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.success
func (x *TaskExecutionSuccessEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.success", corrID, tExecID)
}

// ToSubject generates the NATS subject for a TaskExecutionFailedEvent.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.failed
func (x *TaskExecutionFailedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.failed", corrID, tExecID)
}
