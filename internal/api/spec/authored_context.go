package spec

const (
	authoredContextAPIAgentSoulPath                      = "/api/agent/soul"
	authoredContextAPIAgentsAgentNameHeartbeatPath       = "/api/agents/{name}/heartbeat"
	authoredContextAPIAgentsAgentNameHeartbeatStatusPath = "/api/agents/{name}/heartbeat/status"
	authoredContextAPIAgentsAgentNameSoulPath            = "/api/agents/{name}/soul"
	authoredContextSessionInspectPath                    = "/api/workspaces/{workspace_id}/sessions/" +
		"{session_id}/inspect"
	authoredContextAgentCallerIdentityIsMissingDescription           = "Agent caller identity is missing"
	authoredContextAgentOrWorkspaceNotFoundDescription               = "Agent or workspace not found"
	authoredContextForbiddenWorkspaceOrPermissionMismatchDescription = "Forbidden - workspace or permission mismatch"
	authoredContextHeartbeatAuthoringConflictDescription             = "Heartbeat authoring conflict"
	authoredContextHeartbeatValidationFailedDescription              = "Heartbeat validation failed"
	authoredContextInternalServerErrorDescription                    = "Internal server error"
	authoredContextSessionNotFoundDescription                        = "Session not found"
	authoredContextSoulAuthoringConflictDescription                  = "Soul authoring conflict"
	authoredContextSoulValidationFailedDescription                   = "Soul validation failed"
	authoredContextAgentKey                                          = "agent"
	authoredContextAgentsKey                                         = "agents"
	authoredContextSessionsKey                                       = "sessions"
)

func authoredContextOperations() []OperationSpec {
	return cloneOperationSpecs(authoredContextOperationRegistry)
}
