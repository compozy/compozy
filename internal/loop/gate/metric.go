package gate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/compozy/compozy/internal/loop/dsl"
)

var errMetricScoreInvalid = errors.New("metric score is invalid")

type metricScoreOutput struct {
	Score *float64 `json:"score"`
}

// BestUpdateIntent records a score that is eligible to replace the current best generation.
type BestUpdateIntent struct {
	Generation int64   `json:"generation"`
	Score      float64 `json:"score"`
}

// ParseCommandMetricScore parses the exact score-only JSON contract emitted by command criteria.
func ParseCommandMetricScore(stdout string) (*float64, error) {
	payload := bytes.TrimSpace([]byte(stdout))
	if len(payload) < 2 || payload[0] != '{' || payload[len(payload)-1] != '}' {
		return nil, fmt.Errorf("%w: command metric stdout must be exactly one JSON object", errMetricScoreInvalid)
	}
	if !json.Valid(payload) {
		return nil, fmt.Errorf("%w: command metric stdout must be valid JSON", errMetricScoreInvalid)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var decoded metricScoreOutput
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("%w: decode command metric stdout: %w", errMetricScoreInvalid, err)
	}
	if err := validateFiniteScore(decoded.Score); err != nil {
		return nil, fmt.Errorf("%w: %w", errMetricScoreInvalid, err)
	}
	return decoded.Score, nil
}

// BestUpdateForVerdict returns an update only for an approved, finite metric score that strictly
// improves the baseline by the declared directional delta.
func BestUpdateForVerdict(
	gate Gate,
	verdict Verdict,
	generation int64,
	current *BestUpdateIntent,
) (*BestUpdateIntent, error) {
	if generation < 1 {
		return nil, fmt.Errorf("%w: generation must be positive", errMetricScoreInvalid)
	}
	metric, result := metricResult(gate, verdict)
	if metric == nil || verdict.Outcome != VerdictOutcomeApproved {
		return nil, nil
	}
	if result == nil {
		return nil, fmt.Errorf("%w: metric criterion result is missing", errMetricScoreInvalid)
	}
	if err := validateFiniteScore(result.Score); err != nil {
		return nil, fmt.Errorf("%w: %w", errMetricScoreInvalid, err)
	}
	if current == nil {
		return &BestUpdateIntent{Generation: generation, Score: *result.Score}, nil
	}
	if !isFinite(current.Score) {
		return nil, fmt.Errorf("%w: current best score must be finite", errMetricScoreInvalid)
	}
	improved, err := metricImproves(metric, *result.Score, current.Score)
	if err != nil {
		return nil, err
	}
	if !improved {
		return nil, nil
	}
	return &BestUpdateIntent{Generation: generation, Score: *result.Score}, nil
}

func applyMetricBaseline(
	criteria []dsl.GateCriterion,
	results []CriterionResult,
	bestScore *float64,
) ([]CriterionResult, error) {
	if bestScore == nil || !isFinite(*bestScore) {
		return results, nil
	}
	// The loop contract permits exactly one metric criterion; verdictScore enforces the same invariant.
	for _, criterion := range criteria {
		if criterion.Metric == nil {
			continue
		}
		for index := range results {
			if results[index].ID != criterion.ID || results[index].Outcome != VerdictOutcomeApproved ||
				results[index].Score == nil {
				continue
			}
			improved, err := metricImproves(criterion.Metric, *results[index].Score, *bestScore)
			if err != nil {
				return nil, err
			}
			if improved {
				return results, nil
			}
			results[index].Outcome = VerdictOutcomeRejected
			results[index].Passed = false
			results[index].BlockingIssues = append(results[index].BlockingIssues, BlockingIssue{
				ID:   "metric_regression",
				Note: "metric score did not improve the best baseline",
			})
			return results, nil
		}
		return results, nil
	}
	return results, nil
}

func metricResult(gate Gate, verdict Verdict) (*dsl.MetricSpec, *CriterionResult) {
	// The loop contract permits exactly one metric criterion; verdictScore enforces the same invariant.
	for _, criterion := range gate.Criteria {
		if criterion.Metric == nil {
			continue
		}
		for index := range verdict.Criteria {
			if verdict.Criteria[index].ID == criterion.ID {
				return criterion.Metric, &verdict.Criteria[index]
			}
		}
		return criterion.Metric, nil
	}
	return nil, nil
}

func metricImproves(metric *dsl.MetricSpec, candidate float64, current float64) (bool, error) {
	if metric == nil || !isFinite(candidate) || !isFinite(current) {
		return false, nil
	}
	minimum := 0.0
	if metric.MinDelta != nil {
		minimum = *metric.MinDelta
	}
	if !isFinite(minimum) || minimum < 0 {
		return false, nil
	}
	switch metric.Direction {
	case dsl.MetricMaximize:
		return candidate > current && metricDeltaMeetsMinimum(candidate-current, minimum, candidate, current), nil
	case dsl.MetricMinimize:
		return candidate < current && metricDeltaMeetsMinimum(current-candidate, minimum, candidate, current), nil
	default:
		return false, fmt.Errorf("%w: unsupported metric direction %q", errMetricScoreInvalid, metric.Direction)
	}
}

func metricDeltaMeetsMinimum(delta, minimum, candidate, current float64) bool {
	// Authored decimal thresholds arrive as float64. Scale a tiny tolerance to the operands so
	// exact decimal improvements such as 0.3 - 0.2 >= 0.1 are not rejected by representation error.
	scale := math.Max(1, math.Max(math.Abs(candidate), math.Abs(current)))
	tolerance := scale * 1e-12
	return delta+tolerance >= minimum
}

func validateFiniteScore(score *float64) error {
	if score == nil {
		return errors.New("metric criterion output missing score")
	}
	if !isFinite(*score) {
		return errors.New("metric criterion score must be finite")
	}
	return nil
}

func isFinite(score float64) bool {
	return !math.IsNaN(score) && !math.IsInf(score, 0)
}
