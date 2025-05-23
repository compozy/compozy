package task

import (
	"fmt"

	"github.com/compozy/compozy/pkg/pb"
)

// -----------------------------------------------------------------------------
// Command Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a CmdTaskDispatch.
// Pattern: compozy.<correlation_id>.task.cmds.<task_id>.dispatch
func (x *CmdTaskDispatch) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	taskID := pb.GetTaskID(x)
	return fmt.Sprintf("compozy.%s.task.cmds.%s.dispatch", corrID, taskID)
}

// ToSubject generates the NATS subject for a CmdTaskExecute.
// Pattern: compozy.<correlation_id>.task.cmds.<task_exec_id>.execute
func (x *CmdTaskExecute) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.cmds.%s.execute", corrID, tExecID)
}

// ToSubject generates the NATS subject for a CmdTaskResume.
// Pattern: compozy.<correlation_id>.task.cmds.<task_exec_id>.resume
func (x *CmdTaskResume) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.cmds.%s.resume", corrID, tExecID)
}

// -----------------------------------------------------------------------------
// State Event Subjects
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for a EventTaskDispatched.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.dispatched
func (x *EventTaskDispatched) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.dispatched", corrID, tExecID)
}

// ToSubject generates the NATS subject for a EventTaskStarted.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.started
func (x *EventTaskStarted) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.started", corrID, tExecID)
}

// ToSubject generates the NATS subject for a EventTaskWaiting.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_started
func (x *EventTaskWaiting) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.waiting_started", corrID, tExecID)
}

// ToSubject generates the NATS subject for a EventTaskWaitingEnded.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_ended
func (x *EventTaskWaitingEnded) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.waiting_ended", corrID, tExecID)
}

// ToSubject generates the NATS subject for a EventTaskWaitingTimedOut.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_timed_out
func (x *EventTaskWaitingTimedOut) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.waiting_timed_out", corrID, tExecID)
}

// ToSubject generates the NATS subject for a EventTaskSuccess.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.success
func (x *EventTaskSuccess) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.success", corrID, tExecID)
}

// ToSubject generates the NATS subject for a EventTaskFailed.
// Pattern: compozy.<correlation_id>.task.evts.<task_exec_id>.failed
func (x *EventTaskFailed) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	tExecID := pb.GetTaskExecID(x)
	return fmt.Sprintf("compozy.%s.task.evts.%s.failed", corrID, tExecID)
}
