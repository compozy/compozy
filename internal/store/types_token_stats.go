package store

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// TokenStats is the aggregated usage record for a session in the global database.
type TokenStats struct {
	ID           string
	SessionID    string
	AgentName    string
	InputTokens  *int64
	OutputTokens *int64
	TotalTokens  *int64
	TotalCost    *float64
	CostCurrency *string
	CostStatus   string
	CostSource   string
	TurnCount    int64
	UpdatedAt    time.Time
}

// TokenStatsUpdate adds one or more turns of usage into a session aggregate.
type TokenStatsUpdate struct {
	SessionID    string
	AgentName    string
	InputTokens  *int64
	OutputTokens *int64
	TotalTokens  *int64
	CostAmount   *float64
	CostCurrency *string
	CostStatus   string
	CostSource   string
	Turns        int64
	UpdatedAt    time.Time
}

// Validate ensures the aggregate update contains the required identifying fields.
func (u TokenStatsUpdate) Validate() error {
	if err := requireField(u.SessionID, "token stats session id"); err != nil {
		return err
	}
	if err := requireField(u.AgentName, "token stats agent name"); err != nil {
		return err
	}
	return validateTokenCostShape(u.CostStatus, u.CostSource, u.CostAmount, u.CostCurrency)
}

func validateTokenCostShape(costStatus string, costSource string, amount *float64, currency *string) error {
	status := strings.TrimSpace(costStatus)
	source := strings.TrimSpace(costSource)
	if status != costStatus || source != costSource {
		return errors.New("store: token cost status and source must not carry surrounding whitespace")
	}
	switch status {
	case tokenCostStatusActual:
		if source != tokenCostSourceAgentReported || !validTokenStatsMoney(amount, currency) {
			return errors.New(
				"store: actual token cost requires agent_reported amount and currency; " +
					"amount must be finite and non-negative",
			)
		}
	case tokenCostStatusEstimated:
		if source != tokenCostSourceCatalogConfig &&
			source != tokenCostSourceModelsDev &&
			source != tokenCostSourceBuiltin {
			return errors.New("store: estimated token cost requires a catalog source")
		}
		if !validTokenStatsMoney(amount, currency) {
			return errors.New(
				"store: estimated token cost requires amount and currency; " +
					"amount must be finite and non-negative",
			)
		}
	case tokenCostStatusIncluded:
		if source != tokenCostSourceNone || amount != nil || currency != nil {
			return errors.New("store: included token cost cannot carry amount or currency")
		}
	case tokenCostStatusUnknown:
		if source != tokenCostSourceNone || amount != nil || currency != nil {
			return errors.New("store: unknown token cost cannot carry amount or currency")
		}
	default:
		return fmt.Errorf("store: invalid token cost status %q", costStatus)
	}
	return nil
}

func validTokenStatsMoney(amount *float64, currency *string) bool {
	return amount != nil && *amount >= 0 && !math.IsNaN(*amount) && !math.IsInf(*amount, 0) &&
		currency != nil && strings.TrimSpace(*currency) != ""
}

// TokenStatsQuery filters token aggregation lookups.
type TokenStatsQuery struct {
	SessionID string
	AgentName string
	Limit     int
}

// Validate ensures the query uses sane bounds.
func (q TokenStatsQuery) Validate() error {
	return requirePositiveLimit(q.Limit, "token stats limit")
}
