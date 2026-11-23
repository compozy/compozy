package contract

import "time"

// Agent skill review surfaces are experimental.
type AgentSkillDraftFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type AgentSkillProposeRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Body        string                `json:"body"`
	Files       []AgentSkillDraftFile `json:"files,omitempty"`
	// Mode is new (default) or edit. An edit is an overlay on the installed skill: SKILL.md is
	// regenerated, Files add or replace text files, RemovePaths delete files, the rest is kept.
	Mode               string           `json:"mode,omitempty"`
	RemovePaths        []string         `json:"remove_paths,omitempty"`
	Lesson             AgentSkillLesson `json:"lesson"`
	EvidenceSessionIDs []string         `json:"evidence_session_ids"`
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

// AgentSkillBaseFile is one file of an edit's base snapshot: text files carry their (redacted)
// content; non-UTF-8 files carry only binary and size.
type AgentSkillBaseFile struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Binary  bool   `json:"binary,omitempty"`
	Size    int64  `json:"size,omitempty"`
}

// AgentSkillFileChange marks one path of an edit's base or resulting tree as added, removed,
// modified, or unchanged.
type AgentSkillFileChange struct {
	Path   string `json:"path"`
	Change string `json:"change"`
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
	Lesson             *AgentSkillLesson `json:"lesson,omitempty"`
	EvidenceSessionIDs []string          `json:"evidence_session_ids"`
	// RunsAutomatically counts the tree approval installs; for an edit, kept base files count.
	RunsAutomatically AgentSkillAutomaticContents `json:"runs_automatically"`
	// BaseRunsAutomatically counts the base snapshot of an edit.
	BaseRunsAutomatically *AgentSkillAutomaticContents `json:"base_runs_automatically,omitempty"`
	// Changes covers every path of an edit's base and resulting tree (detail only).
	Changes []AgentSkillFileChange `json:"changes,omitempty"`
	// Files lists the proposed text files; for an edit, the regenerated SKILL.md plus added or
	// replaced files (detail only).
	Files []AgentSkillDraftFile `json:"files,omitempty"`
	// BaseFiles is the base snapshot of an edit (detail only); empty when the snapshot fails
	// verification, with Problem `skill_review_snapshot_invalid`.
	BaseFiles []AgentSkillBaseFile `json:"base_files,omitzero"`
	CreatedAt time.Time            `json:"created_at"`
	DecidedAt *time.Time           `json:"decided_at,omitempty"`
	DecidedBy string               `json:"decided_by,omitempty"`
	// DecidedVia is operator or auto_approve once decided.
	DecidedVia string `json:"decided_via,omitempty"`
	// RevertedBy and RevertedAt record the operator revert of an approved review.
	RevertedBy string     `json:"reverted_by,omitempty"`
	RevertedAt *time.Time `json:"reverted_at,omitempty"`
	// Revert says whether the review can be reverted now (detail only).
	Revert *AgentSkillRevertEligibility `json:"revert,omitempty"`
	// Problem is set when the stored draft no longer verifies: `skill_review_invalid`. The review then
	// carries no files and no automatic contents, and it can only be rejected. For an edit whose base
	// snapshot fails verification it is `skill_review_snapshot_invalid`: no base files or changes.
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

// AgentSkillRevertEligibility is the outcome a revert would have now. Reason is one of not_approved,
// legacy, superseded, changed_on_disk, or target_unavailable, and is absent when Allowed.
type AgentSkillRevertEligibility struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// AgentSkillRevertConflictPayload is the 409 body of a refused revert: the stable code, the
// eligibility reason (superseded, changed_on_disk, legacy, target_unavailable) and the message.
type AgentSkillRevertConflictPayload struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Error  string `json:"error"`
}
