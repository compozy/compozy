package task

// WithWorkspaceActiveRunCap injects the trusted concurrent-run limit applied
// to workspace-scoped task execution. Zero disables the limit.
func WithWorkspaceActiveRunCap(limit int) Option {
	return func(opts *managerOptions) {
		opts.workspaceActiveRunCap = limit
	}
}

// WithGovernedRootActiveRunCap injects the trusted concurrent call-activation limit.
// Zero disables the limit.
func WithGovernedRootActiveRunCap(limit int) Option {
	return func(opts *managerOptions) {
		opts.governedRootActiveRunCap = limit
	}
}
