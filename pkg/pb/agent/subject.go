package agent

import (
	"fmt"

	"github.com/compozy/compozy/pkg/pb"
)

// -----------------------------------------------------------------------------
// Commands
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an AgentExecuteCommand.
// Pattern: compozy.<correlation_id>.agent.cmds.<agent_id>.execute
func (x *AgentExecuteCommand) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	agID := pb.GetAgentID(x)
	return fmt.Sprintf("compozy.%s.agent.cmds.%s.execute", corrID, agID)
}

// -----------------------------------------------------------------------------
// State Events
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an AgentExecutionStartedEvent.
// Pattern: compozy.<correlation_id>.agent.evts.<agent_exec_id>.started
func (x *AgentExecutionStartedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	agentExecID := pb.GetAgentExecID(x)
	return fmt.Sprintf("compozy.%s.agent.evts.%s.started", corrID, agentExecID)
}

// ToSubject generates the NATS subject for an AgentExecutionSuccessEvent.
// Pattern: compozy.<correlation_id>.agent.evts.<agent_exec_id>.success
func (x *AgentExecutionSuccessEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	agentExecID := pb.GetAgentExecID(x)
	return fmt.Sprintf("compozy.%s.agent.evts.%s.success", corrID, agentExecID)
}

// ToSubject generates the NATS subject for an AgentExecutionFailedEvent.
// Pattern: compozy.<correlation_id>.agent.evts.<agent_exec_id>.failed
func (x *AgentExecutionFailedEvent) ToSubject() string {
	corrID := pb.GetCorrelationID(x)
	agentExecID := pb.GetAgentExecID(x)
	return fmt.Sprintf("compozy.%s.agent.evts.%s.failed", corrID, agentExecID)
}
