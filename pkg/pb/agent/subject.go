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
	correlationID := pb.GetCorrelationID(x)
	agentID := pb.GetAgentID(x)
	return fmt.Sprintf("compozy.%s.agent.cmds.%s.execute", correlationID, agentID)
}

// -----------------------------------------------------------------------------
// State Events
// -----------------------------------------------------------------------------

// ToSubject generates the NATS subject for an AgentExecutionStartedEvent.
// Pattern: compozy.<correlation_id>.agent.events.<agent_exec_id>.started
func (x *AgentExecutionStartedEvent) ToSubject() string {
	correlationID := pb.GetCorrelationID(x)
	agentExecID := pb.GetAgentExecID(x)
	return fmt.Sprintf("compozy.%s.agent.events.%s.started", correlationID, agentExecID)
}

// ToSubject generates the NATS subject for an AgentExecutionSuccessEvent.
// Pattern: compozy.<correlation_id>.agent.events.<agent_exec_id>.success
func (x *AgentExecutionSuccessEvent) ToSubject() string {
	correlationID := pb.GetCorrelationID(x)
	agentExecID := pb.GetAgentExecID(x)
	return fmt.Sprintf("compozy.%s.agent.events.%s.success", correlationID, agentExecID)
}

// ToSubject generates the NATS subject for an AgentExecutionFailedEvent.
// Pattern: compozy.<correlation_id>.agent.events.<agent_exec_id>.failed
func (x *AgentExecutionFailedEvent) ToSubject() string {
	correlationID := pb.GetCorrelationID(x)
	agentExecID := pb.GetAgentExecID(x)
	return fmt.Sprintf("compozy.%s.agent.events.%s.failed", correlationID, agentExecID)
}
