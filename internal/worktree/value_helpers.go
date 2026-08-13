package worktree

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
