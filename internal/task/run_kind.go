package task

import "strings"

// IsNetworkWake reports whether this run is taskless durable Network work.
func (r Run) IsNetworkWake() bool {
	return r.RunKind.Normalize() == RunKindNetworkWake
}

// IsTaskAnchored reports whether task state and projections own this run.
func (r Run) IsTaskAnchored() bool {
	return !r.IsNetworkWake()
}

// IsLoopWorker reports whether this run is a loop-correlated worker node.
func (r Run) IsLoopWorker() bool {
	return strings.TrimSpace(r.LoopRunID) != "" && r.RunKind.Normalize() != RunKindCoordinator
}
