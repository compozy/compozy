package task

import "strings"

// IsNetworkWake reports whether this run is taskless durable Network work.
func (r Run) IsNetworkWake() bool {
	return r.RunKind.Normalize() == RunKindNetworkWake
}

// IsCallActivation reports whether this run owns durable call activation work.
func (r Run) IsCallActivation() bool {
	return r.RunKind.Normalize() == RunKindCallActivation
}

// IsTaskless reports whether this run is durable daemon work without a task anchor.
func (r Run) IsTaskless() bool {
	return r.IsNetworkWake() || r.IsCallActivation()
}

// IsTaskAnchored reports whether task state and projections own this run.
func (r Run) IsTaskAnchored() bool {
	return !r.IsTaskless()
}

// IsLoopWorker reports whether this run is a loop-correlated worker node.
func (r Run) IsLoopWorker() bool {
	return strings.TrimSpace(r.LoopRunID) != "" && r.RunKind.Normalize() != RunKindCoordinator
}
