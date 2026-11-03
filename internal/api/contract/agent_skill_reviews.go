package contract

import "time"

// Agent skill review surfaces are experimental.
type AgentSkillDraftFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type AgentSkillProposeRequest struct {
	Name               string                `json:"name"`
	Description        string                `json:"description"`
	Body               string                `json:"body"`
	Files              []AgentSkillDraftFile `json:"files,omitempty"`
	EvidenceSessionIDs []string              `json:"evidence_session_ids,omitempty"`
}

type AgentSkillAutomaticContents struct {
	Hooks      int `json:"hooks"`
	MCPServers int `json:"mcp_servers"`
	Scripts    int `json:"scripts"`
}

type AgentSkillReviewPayload struct {
	ID                string                      `json:"id"`
	InstanceID        string                      `json:"instance_id"`
	ProfileID         string                      `json:"profile_id"`
	WorkspaceID       string                      `json:"workspace_id"`
	Agent             string                      `json:"agent"`
	Name              string                      `json:"name"`
	State             string                      `json:"state"`
	Digest            string                      `json:"digest"`
	RunsAutomatically AgentSkillAutomaticContents `json:"runs_automatically"`
	Files             []AgentSkillDraftFile       `json:"files,omitempty"`
	CreatedAt         time.Time                   `json:"created_at"`
	DecidedAt         *time.Time                  `json:"decided_at,omitempty"`
	DecidedBy         string                      `json:"decided_by,omitempty"`
	// Problem is set when the stored draft no longer verifies: `skill_review_invalid`. The review then
	// carries no files and no automatic contents, and it can only be rejected.
	Problem string `json:"problem,omitempty"`
}

type AgentSkillReviewResponse struct {
	Review AgentSkillReviewPayload `json:"review"`
}
type AgentSkillReviewsResponse struct {
	Reviews    []AgentSkillReviewPayload `json:"reviews"`
	NextCursor string                    `json:"next_cursor,omitempty"`
}

type AgentSkillReviewDecisionRequest struct {
	Digest string `json:"digest"`
}
