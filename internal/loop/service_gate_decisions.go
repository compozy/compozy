package loop

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/task"
)

type activeHumanCriterion struct {
	ID string `json:"id"`
}

func gateDecisionRecords(
	run Run,
	gateID NodeID,
	decision GateDecision,
	actor task.ActorContext,
	decidedAt time.Time,
) ([]GateDecisionRecord, error) {
	criterionIDs, err := activeHumanCriterionIDs(run.ActiveHumanCriteria)
	if err != nil {
		return nil, err
	}
	if len(criterionIDs) == 0 {
		criterionID := strings.TrimSpace(string(gateID))
		if criterionID == "" {
			return nil, fmt.Errorf("%w: gate id is required", ErrValidation)
		}
		criterionIDs = []string{criterionID}
	}
	records := make([]GateDecisionRecord, 0, len(criterionIDs))
	for _, criterionID := range criterionIDs {
		records = append(records, GateDecisionRecord{
			WorkspaceID: run.WorkspaceID,
			RunID:       run.ID,
			Generation:  run.Generation,
			GateID:      gateID,
			CriterionID: criterionID,
			Decision:    decision,
			Actor:       actor,
			DecidedAt:   decidedAt,
		})
	}
	return records, nil
}

func activeHumanCriterionIDs(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var criteria []activeHumanCriterion
	if err := json.Unmarshal(raw, &criteria); err != nil {
		return nil, fmt.Errorf("%w: decode active human criteria: %w", ErrValidation, err)
	}
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(criteria))
	for _, criterion := range criteria {
		id := strings.TrimSpace(criterion.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
