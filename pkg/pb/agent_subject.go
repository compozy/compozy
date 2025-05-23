package pb

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// Commands
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an CmdAgentExecute.
// Pattern: compozy.<correlation_id>.agent.cmds.<agent_id>.execute
func (x *CmdAgentExecute) ToSubject() string {
	corrID := GetCorrelationID(x)
	agentID := GetAgentID(x)
	return fmt.Sprintf("compozy.%s.agent.cmds.%s.execute", corrID, agentID)
}

// -----------------------------------------------------------------------------
// State Events
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an EventAgentStarted.
// Pattern: compozy.<correlation_id>.agent.evts.<agent_exec_id>.started
func (x *EventAgentStarted) ToSubject() string {
	corrID := GetCorrelationID(x)
	agentExecID := GetAgentExecID(x)
	return fmt.Sprintf("compozy.%s.agent.evts.%s.started", corrID, agentExecID)
}

// ToSubject generates the NATS subject for an EventAgentSuccess.
// Pattern: compozy.<correlation_id>.agent.evts.<agent_exec_id>.success
func (x *EventAgentSuccess) ToSubject() string {
	corrID := GetCorrelationID(x)
	agentExecID := GetAgentExecID(x)
	return fmt.Sprintf("compozy.%s.agent.evts.%s.success", corrID, agentExecID)
}

// ToSubject generates the NATS subject for an EventAgentFailed.
// Pattern: compozy.<correlation_id>.agent.evts.<agent_exec_id>.failed
func (x *EventAgentFailed) ToSubject() string {
	corrID := GetCorrelationID(x)
	agentExecID := GetAgentExecID(x)
	return fmt.Sprintf("compozy.%s.agent.evts.%s.failed", corrID, agentExecID)
}
