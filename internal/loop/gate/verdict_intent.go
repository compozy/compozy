package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// VerdictIntent is the sanitized, immutable machine-verdict mutation carried by a completion plan.
type VerdictIntent struct {
	GateID         string          `json:"gate_id"`
	ItemIndex      int             `json:"item_index"`
	Outcome        VerdictOutcome  `json:"outcome"`
	Score          *float64        `json:"score,omitempty"`
	RouteCauseRank *int            `json:"route_cause_rank,omitempty"`
	BlockingIssues json.RawMessage `json:"blocking_issues"`
	Criteria       json.RawMessage `json:"criteria"`
}

// VerdictRecord is the durable read model for one machine gate verdict.
type VerdictRecord struct {
	RunID          string
	Generation     int64
	GateID         string
	ItemIndex      int
	Outcome        VerdictOutcome
	Score          *float64
	RouteCauseRank *int
	BlockingIssues json.RawMessage
	Criteria       json.RawMessage
	DecidedAt      time.Time
}

// VerdictReader provides workspace-scoped machine verdict history.
type VerdictReader interface {
	ListGateVerdicts(
		ctx context.Context,
		workspaceID string,
		runID string,
		generation int64,
	) ([]VerdictRecord, error)
	ListRouteCausingVerdicts(
		ctx context.Context,
		workspaceID string,
		runID string,
		generation int64,
	) ([]VerdictRecord, error)
}

// NewVerdictIntent projects an aggregate verdict through the single diagnostics sanitization boundary.
func NewVerdictIntent(
	gateID string,
	itemIndex int,
	verdict Verdict,
	routeCauseRank *int,
) (VerdictIntent, error) {
	trimmedGateID := strings.TrimSpace(gateID)
	if trimmedGateID == "" {
		return VerdictIntent{}, fmt.Errorf("gate verdict intent gate_id is required")
	}
	if itemIndex < 0 {
		return VerdictIntent{}, fmt.Errorf("gate verdict intent item_index must be non-negative")
	}
	if routeCauseRank != nil && *routeCauseRank < 0 {
		return VerdictIntent{}, fmt.Errorf("gate verdict intent route_cause_rank must be non-negative")
	}
	var projectedRouteCauseRank *int
	if routeCauseRank != nil {
		rank := *routeCauseRank
		projectedRouteCauseRank = &rank
	}
	score, err := verdictScore(verdict.Criteria)
	if err != nil {
		return VerdictIntent{}, err
	}
	blocking, err := sanitizeVerdictDiagnostics(verdict.BlockingIssues)
	if err != nil {
		return VerdictIntent{}, err
	}
	criteria, err := sanitizeVerdictDiagnostics(verdict.Criteria)
	if err != nil {
		return VerdictIntent{}, err
	}
	return VerdictIntent{
		GateID:         trimmedGateID,
		ItemIndex:      itemIndex,
		Outcome:        verdict.Outcome,
		Score:          score,
		RouteCauseRank: projectedRouteCauseRank,
		BlockingIssues: blocking,
		Criteria:       criteria,
	}, nil
}

func verdictScore(criteria []CriterionResult) (*float64, error) {
	var score *float64
	for _, criterion := range criteria {
		if criterion.Score == nil {
			continue
		}
		if err := validateFiniteScore(criterion.Score); err != nil {
			return nil, fmt.Errorf("gate verdict intent: %w", err)
		}
		if score != nil {
			return nil, fmt.Errorf("gate verdict intent: multiple criterion scores are unsupported")
		}
		value := *criterion.Score
		score = &value
	}
	return score, nil
}
