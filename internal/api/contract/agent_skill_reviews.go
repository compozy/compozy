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
	Lesson             AgentSkillLesson      `json:"lesson"`
	EvidenceSessionIDs []string              `json:"evidence_session_ids"`
}

// AgentSkillLesson is the structured, evidence-backed claim a skill proposal carries.
type AgentSkillLesson struct {
	Cue      string `json:"cue"`
	Lesson   string `json:"lesson"`
	Evidence string `json:"evidence"`
	// Confidence is low, medium, or high; omitted means medium.
	Confidence string `json:"confidence,omitempty"`
}

type AgentSkillAutomaticContents struct {
	Hooks      int `json:"hooks"`
	MCPServers int `json:"mcp_servers"`
	Scripts    int `json:"scripts"`
}

type AgentSkillReviewPayload struct {
	ID          string `json:"id"`
	InstanceID  string `json:"instance_id"`
	ProfileID   string `json:"profile_id"`
	WorkspaceID string `json:"workspace_id"`
	Agent       string `json:"agent"`
	Name        string `json:"name"`
	State       string `json:"state"`
	// Kind is new or edit; reviews created before lessons report new.
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
	// BaseDigest is the digest of the installed skill an edit was proposed against.
	BaseDigest string `json:"base_digest,omitempty"`
	// Lesson is absent for reviews created before lessons existed.
	Lesson             *AgentSkillLesson           `json:"lesson,omitempty"`
	EvidenceSessionIDs []string                    `json:"evidence_session_ids"`
	RunsAutomatically  AgentSkillAutomaticContents `json:"runs_automatically"`
	Files              []AgentSkillDraftFile       `json:"files,omitempty"`
	CreatedAt          time.Time                   `json:"created_at"`
	DecidedAt          *time.Time                  `json:"decided_at,omitempty"`
	DecidedBy          string                      `json:"decided_by,omitempty"`
	// DecidedVia is operator or auto_approve once decided.
	DecidedVia string `json:"decided_via,omitempty"`
	// Problem is set when the stored draft no longer verifies: `skill_review_invalid`. The review then
	// carries no files and no automatic contents, and it can only be rejected.
	Problem string `json:"problem,omitempty"`
}

type AgentSkillReviewResponse struct {
	Review AgentSkillReviewPayload `json:"review"`
	// DuplicateOfPending is set by a proposal that returned an existing pending review unchanged.
	DuplicateOfPending bool `json:"duplicate_of_pending,omitempty"`
}
type AgentSkillReviewsResponse struct {
	Reviews    []AgentSkillReviewPayload `json:"reviews"`
	NextCursor string                    `json:"next_cursor,omitempty"`
}

type AgentSkillReviewDecisionRequest struct {
	Digest string `json:"digest"`
}
