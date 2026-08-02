package loop

import (
	"slices"
	"strings"
)

const genericRepairGuidance = "review the failure and adjust"

// RepairFailure is one classified node failure considered at a generation boundary.
type RepairFailure struct {
	NodeID   string
	Failure  ClassifiedFailure
	Absorbed bool
}

// RepairContextEntry keeps cause and guidance scoped to one failed node.
type RepairContextEntry struct {
	NodeID   string `json:"node_id"`
	Cause    string `json:"cause"`
	Guidance string `json:"guidance"`
}

// BuildRepairContext projects unabsorbed failures into deterministic per-node guidance.
func BuildRepairContext(failures []RepairFailure) []RepairContextEntry {
	entries := make([]RepairContextEntry, 0, len(failures))
	for _, failure := range failures {
		if failure.Absorbed {
			continue
		}
		guidance := strings.TrimSpace(failure.Failure.Hint)
		if guidance == "" {
			guidance = genericRepairGuidance
		}
		entries = append(entries, RepairContextEntry{
			NodeID:   strings.TrimSpace(failure.NodeID),
			Cause:    failure.Failure.Cause,
			Guidance: guidance,
		})
	}
	slices.SortFunc(entries, func(left, right RepairContextEntry) int {
		return strings.Compare(left.NodeID, right.NodeID)
	})
	return entries
}
