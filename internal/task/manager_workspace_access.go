package task

import "github.com/compozy/compozy/internal/workspaceaccess"

// WithWorkspaceAccessPolicy injects the cross-workspace decision policy.
func WithWorkspaceAccessPolicy(policy workspaceaccess.Policy) Option {
	return func(opts *managerOptions) {
		opts.workspaceAccess = policy
	}
}
