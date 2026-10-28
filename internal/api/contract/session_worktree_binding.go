package contract

type BindSessionWorktreeRequest struct {
	Worktree           string  `json:"worktree"                       binding:"required"`
	ExpectedWorktreeID *string `json:"expected_worktree_id,omitempty"`
}
