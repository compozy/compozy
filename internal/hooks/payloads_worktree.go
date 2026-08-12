package hooks

type WorktreeContext struct {
	WorktreeID    string `json:"worktree_id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	Name          string `json:"name"`
	Branch        string `json:"branch"`
	Path          string `json:"path"`
	Origin        string `json:"origin"`
}

type WorktreeRisk struct {
	ChangedFiles    int  `json:"changed_files"`
	Insertions      int  `json:"insertions"`
	Deletions       int  `json:"deletions"`
	UnpushedCommits int  `json:"unpushed_commits"`
	ExistsOnRemote  bool `json:"exists_on_remote"`
}

type WorktreePreCreatePayload struct {
	PayloadBase
	WorktreeContext
	Denied     bool   `json:"denied,omitempty"`
	DenyReason string `json:"deny_reason,omitempty"`
}

type WorktreePreRemovePayload struct {
	PayloadBase
	Worktree   WorktreeContext `json:"worktree"`
	Force      bool            `json:"force"`
	Risk       WorktreeRisk    `json:"risk"`
	Denied     bool            `json:"denied,omitempty"`
	DenyReason string          `json:"deny_reason,omitempty"`
}

type WorktreeObservationPayload struct {
	PayloadBase
	WorktreeContext
}

type WorktreeControlPatch struct {
	ControlPatch
}

type WorktreeObservationPatch struct{}
