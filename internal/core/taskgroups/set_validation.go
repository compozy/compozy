package taskgroups

import "slices"

const (
	rejectionDependsOnSelected = "depends_on_selected"
	rejectionUnmetDependency   = "unmet_dependency"
	rejectionAlreadyCompleted  = "already_completed"
	rejectionUnknown           = "unknown"
)

// SetValidationResult classifies a selected set for parallel task group execution.
type SetValidationResult struct {
	Eligible     []string
	Rejected     map[string]Rejection
	PlanChecksum string
}

// Rejection describes why one selected task group cannot run in parallel.
type Rejection struct {
	Reason   string
	Blockers []string
}

// ValidateIndependentSet validates that selected task groups are mutually independent.
func ValidateIndependentSet(plan Plan, selected []string) (SetValidationResult, error) {
	result := SetValidationResult{
		Eligible:     make([]string, 0, len(selected)),
		Rejected:     make(map[string]Rejection),
		PlanChecksum: plan.Checksum,
	}
	selectedIDs := sortedUnique(selected)
	if len(selectedIDs) == 0 {
		return result, newError(ErrSelectionRequired, plan.Initiative, "", plan.Path, nil)
	}

	activeIDs := make([]string, 0, len(selectedIDs))
	for _, taskGroupID := range selectedIDs {
		if _, found := plan.TaskGroup(taskGroupID); !found {
			result.Rejected[taskGroupID] = Rejection{Reason: rejectionUnknown}
			continue
		}
		if plan.IsComplete(taskGroupID) {
			result.Rejected[taskGroupID] = Rejection{Reason: rejectionAlreadyCompleted}
			continue
		}
		activeIDs = append(activeIDs, taskGroupID)
	}

	for _, taskGroupID := range activeIDs {
		readiness, err := EvaluateReadiness(plan, taskGroupID)
		if err != nil {
			return result, err
		}
		if blockers := selectedPeerBlockers(activeIDs, taskGroupID, readiness.IndependentPeers); len(blockers) > 0 {
			result.Rejected[taskGroupID] = Rejection{
				Reason:   rejectionDependsOnSelected,
				Blockers: blockers,
			}
			continue
		}
		if !readiness.Eligible {
			result.Rejected[taskGroupID] = Rejection{
				Reason:   rejectionUnmetDependency,
				Blockers: unmetDependencyBlockers(plan, readiness),
			}
			continue
		}
		result.Eligible = append(result.Eligible, taskGroupID)
	}

	return result, nil
}

func sortedUnique(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		unique[value] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func selectedPeerBlockers(selected []string, taskGroupID string, independentPeers []string) []string {
	peers := make(map[string]struct{}, len(independentPeers))
	for _, peer := range independentPeers {
		peers[peer] = struct{}{}
	}
	blockers := make([]string, 0)
	for _, selectedID := range selected {
		if selectedID == taskGroupID {
			continue
		}
		if _, independent := peers[selectedID]; !independent {
			blockers = append(blockers, selectedID)
		}
	}
	return blockers
}

func unmetDependencyBlockers(plan Plan, readiness Readiness) []string {
	blockers := make(map[string]struct{})
	for _, dependency := range readiness.DirectUnmet {
		blockers[dependency.From] = struct{}{}
	}
	for _, path := range readiness.TransitiveUnmet {
		for _, taskGroupID := range path.TaskGroupIDs {
			if !plan.IsComplete(taskGroupID) {
				blockers[taskGroupID] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(blockers))
	for taskGroupID := range blockers {
		result = append(result, taskGroupID)
	}
	slices.Sort(result)
	return result
}
