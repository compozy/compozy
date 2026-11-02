package contract

type AgentDraftRuntime struct {
	Provider        string `json:"provider"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type DraftAgentInstructionsRequest struct {
	Goal    string            `json:"goal"`
	Runtime AgentDraftRuntime `json:"runtime"`
}

type DraftAgentInstructionsResponse struct {
	Instructions string `json:"instructions"`
}
