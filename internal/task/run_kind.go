package task

import "strings"

// IsLoopWorker reports whether this run is a loop-correlated worker node.
func (r Run) IsLoopWorker() bool {
	return strings.TrimSpace(r.LoopRunID) != "" && r.RunKind.Normalize() != RunKindCoordinator
}
