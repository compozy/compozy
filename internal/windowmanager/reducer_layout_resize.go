package windowmanager

import (
	"fmt"
	"math"
)

func (r *reducer) resize(snapshot *Snapshot, command ResizeLayoutCommand) (bool, error) {
	if !finite(command.Delta) {
		return false, fmt.Errorf("resize delta is not finite: %w", ErrInvalidCommand)
	}
	node, exists := findNodeInSnapshot(snapshot, command.SplitID)
	if !exists || node.Kind != NodeKindSplit {
		return false, fmt.Errorf("split %q: %w", command.SplitID, ErrInvalidCommand)
	}
	if command.BoundaryIndex < 0 || command.BoundaryIndex >= len(node.Children)-1 {
		return false, fmt.Errorf("boundary %d: %w", command.BoundaryIndex, ErrInvalidCommand)
	}
	left := command.BoundaryIndex
	right := left + 1
	leftWeight := node.Weights[left] + command.Delta
	rightWeight := node.Weights[right] - command.Delta
	if leftWeight < 0.01 || rightWeight < 0.01 {
		return false, fmt.Errorf("resize exceeds boundary: %w", ErrInvalidCommand)
	}
	if math.Abs(command.Delta) <= weightTolerance {
		return false, nil
	}
	node.Weights[left] = leftWeight
	node.Weights[right] = rightWeight
	normalizeWeights(node.Weights)
	r.changes.node(command.SplitID)
	return true, nil
}
