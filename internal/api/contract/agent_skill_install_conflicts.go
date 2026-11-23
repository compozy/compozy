package contract

import "time"

// Agent skill install-conflict surfaces are experimental.

// AgentSkillInstallConflictPayload is a skill left with two versions by an external edit during an
// install, revert, or resolution.
type AgentSkillInstallConflictPayload struct {
	OpID     string `json:"op_id"`
	Skill    string `json:"skill"`
	ReviewID string `json:"review_id,omitempty"`
	// Action is install, revert, or resolve.
	Action string `json:"action"`
	// State is conflict, or resolving while an interrupted resolution waits to finish.
	State string `json:"state"`
	// Keep is the recorded choice of a resolving conflict: current or previous.
	Keep  string     `json:"keep,omitempty"`
	Since *time.Time `json:"since,omitempty"`
}

type AgentSkillInstallConflictsResponse struct {
	Conflicts []AgentSkillInstallConflictPayload `json:"conflicts"`
}

type AgentSkillInstallConflictResolveRequest struct {
	// Keep is current (the edited tree) or previous (the tree before the operation).
	Keep string `json:"keep"`
}

type AgentSkillInstallConflictResolveResponse struct {
	OpID  string `json:"op_id"`
	Skill string `json:"skill"`
	Keep  string `json:"keep"`
	// KeptCopy is the directory holding the edited copy after keep=previous.
	KeptCopy        string `json:"kept_copy,omitempty"`
	AlreadyResolved bool   `json:"already_resolved,omitempty"`
}
