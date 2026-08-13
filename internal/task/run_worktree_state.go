package task

import "strings"

// WorktreeIDValue returns the optional bound worktree identity.
func (r Run) WorktreeIDValue() string {
	if r.RunWorktreeState == nil {
		return ""
	}
	return r.WorktreeID
}

// ResolvedWorktreeModeValue returns the immutable worktree mode snapshot.
func (r Run) ResolvedWorktreeModeValue() WorktreeMode {
	if r.RunWorktreeState == nil {
		return ""
	}
	return r.ResolvedWorktreeMode
}

// ResolvedWorktreeRefValue returns the optional immutable worktree reference.
func (r Run) ResolvedWorktreeRefValue() string {
	if r.RunWorktreeState == nil {
		return ""
	}
	return r.ResolvedWorktreeRef
}

// SetWorktreeState replaces the run's worktree binding snapshot.
func (r *Run) SetWorktreeState(worktreeID string, mode WorktreeMode, ref string) {
	if r == nil {
		return
	}
	r.RunWorktreeState = &RunWorktreeState{
		WorktreeID:           strings.TrimSpace(worktreeID),
		ResolvedWorktreeMode: mode.Normalize(),
		ResolvedWorktreeRef:  strings.TrimSpace(ref),
	}
}

// SetWorktreeID updates the bound worktree identity without changing the snapshot policy.
func (r *Run) SetWorktreeID(worktreeID string) {
	state := r.mutableWorktreeState()
	state.WorktreeID = strings.TrimSpace(worktreeID)
}

// SetResolvedWorktreeMode updates the immutable mode snapshot.
func (r *Run) SetResolvedWorktreeMode(mode WorktreeMode) {
	state := r.mutableWorktreeState()
	state.ResolvedWorktreeMode = mode.Normalize()
}

// SetResolvedWorktreeRef updates the immutable reference snapshot.
func (r *Run) SetResolvedWorktreeRef(ref string) {
	state := r.mutableWorktreeState()
	state.ResolvedWorktreeRef = strings.TrimSpace(ref)
}

func (r *Run) mutableWorktreeState() *RunWorktreeState {
	state := RunWorktreeState{}
	if r.RunWorktreeState != nil {
		state = *r.RunWorktreeState
	}
	r.RunWorktreeState = &state
	return r.RunWorktreeState
}
