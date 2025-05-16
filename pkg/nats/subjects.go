package nats

import "fmt"

// Subject constants define the base structure for NATS subjects
const (
	SubjectPrefix   = "compozy"
	SubjectAgent    = "agent"
	SubjectTool     = "tool"
	SubjectError    = "error"
	SubjectRequest  = "request"
	SubjectResponse = "response"
	SubjectLog      = "log"
)

// GenAgentRequestSubject creates a subject for an AgentRequest
// compozy.<execID>.agent.<agentID>.request
func GenAgentRequestSubject(execID, agentID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", SubjectPrefix, execID, SubjectAgent, agentID, SubjectRequest)
}

// GenAgentResponseSubject creates a subject for an AgentResponse (for subscriptions, not replies)
// compozy.<execID>.agent.<agentID>.response
func GenAgentResponseSubject(execID, agentID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", SubjectPrefix, execID, SubjectAgent, agentID, SubjectResponse)
}

// GenToolRequestSubject creates a subject for a ToolRequest
// compozy.<execID>.tool.<toolID>.request
func GenToolRequestSubject(execID, toolID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", SubjectPrefix, execID, SubjectTool, toolID, SubjectRequest)
}

// GenToolResponseSubject creates a subject for a ToolResponse (for subscriptions, not replies)
// compozy.<execID>.tool.<toolID>.response
func GenToolResponseSubject(execID, toolID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", SubjectPrefix, execID, SubjectTool, toolID, SubjectResponse)
}

// GenErrorSubject creates a subject for an ErrorMessage
// compozy.<execID>.error
func GenErrorSubject(execID string) string {
	return fmt.Sprintf("%s.%s.%s", SubjectPrefix, execID, SubjectError)
}

// GenAgentRequestWildcard creates a wildcard subject for subscribing to all AgentRequests for an agent
// compozy.<execID>.agent.<agentID>.request.>
func GenAgentRequestWildcard(execID, agentID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s.>", SubjectPrefix, execID, SubjectAgent, agentID, SubjectRequest)
}

// GenToolRequestWildcard creates a wildcard subject for subscribing to all ToolRequests for a tool
// compozy.<execID>.tool.<toolID>.request.>
func GenToolRequestWildcard(execID, toolID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s.>", SubjectPrefix, execID, SubjectTool, toolID, SubjectRequest)
}

// GenLogSubject creates a subject for log messages
// compozy.<execID>.log.<level>
func GenLogSubject(execID string, level LogLevel) string {
	return fmt.Sprintf("%s.%s.%s.%s", SubjectPrefix, execID, SubjectLog, level)
}

// GenLogWildcard creates a wildcard subject for subscribing to all log messages
// compozy.<execID>.log.*
func GenLogWildcard(execID string) string {
	return fmt.Sprintf("%s.%s.%s.*", SubjectPrefix, execID, SubjectLog)
}

// GenLogLevelWildcard creates a wildcard subject for subscribing to all log messages of a specific level
// compozy.<execID>.log.<level>
func GenLogLevelWildcard(execID string, level LogLevel) string {
	return fmt.Sprintf("%s.%s.%s.%s", SubjectPrefix, execID, SubjectLog, level)
}
