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
func GenAgentRequestSubject(agentID, requestID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", SubjectPrefix, SubjectAgent, agentID, SubjectRequest, requestID)
}

// GenAgentResponseSubject creates a subject for an AgentResponse (for subscriptions, not replies)
func GenAgentResponseSubject(agentID, requestID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", SubjectPrefix, SubjectAgent, agentID, SubjectResponse, requestID)
}

// GenToolRequestSubject creates a subject for a ToolRequest
func GenToolRequestSubject(toolID, requestID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", SubjectPrefix, SubjectTool, toolID, SubjectRequest, requestID)
}

// GenToolResponseSubject creates a subject for a ToolResponse (for subscriptions, not replies)
func GenToolResponseSubject(toolID, requestID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", SubjectPrefix, SubjectTool, toolID, SubjectResponse, requestID)
}

// GenErrorSubject creates a subject for an ErrorMessage
func GenErrorSubject(requestID string) string {
	return fmt.Sprintf("%s.%s.%s", SubjectPrefix, SubjectError, requestID)
}

// GenAgentRequestWildcard creates a wildcard subject for subscribing to all AgentRequests for an agent
func GenAgentRequestWildcard(agentID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.>", SubjectPrefix, SubjectAgent, agentID, SubjectRequest)
}

// GenToolRequestWildcard creates a wildcard subject for subscribing to all ToolRequests for a tool
func GenToolRequestWildcard(toolID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.>", SubjectPrefix, SubjectTool, toolID, SubjectRequest)
}

// GenLogSubject creates a subject for log messages
func GenLogSubject(level LogLevel) string {
	return fmt.Sprintf("%s.%s.%s", SubjectPrefix, SubjectLog, level)
}

// GenLogWildcard creates a wildcard subject for subscribing to all log messages
func GenLogWildcard() string {
	return fmt.Sprintf("%s.%s.>", SubjectPrefix, SubjectLog)
}
