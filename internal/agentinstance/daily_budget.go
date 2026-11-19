package agentinstance

import (
	"errors"
	"math"
	"math/big"
	"strconv"
)

// DailyBudgetLimits holds optional operator caps. A nil cap leaves its dimension
// unrestricted, including when an observed provider has no USD pricing.
type DailyBudgetLimits struct {
	Tokens *int64
	USD    *float64
}

// Validate rejects caps that cannot be enforced with monotonic usage totals.
func (limits DailyBudgetLimits) Validate() error {
	if limits.Tokens != nil && *limits.Tokens < 0 {
		return errors.New("agentinstance: daily token cap cannot be negative")
	}
	if limits.USD != nil && (*limits.USD < 0 || math.IsNaN(*limits.USD) || math.IsInf(*limits.USD, 0)) {
		return errors.New("agentinstance: daily USD cap must be finite and non-negative")
	}
	return nil
}

// DailyBudgetUsage represents consumption in one instance's local calendar day.
// USD retains the exact decimal cost subtotal even if some receipts have no price.
// HasUnpricedUsage records incomplete pricing separately from that subtotal.
type DailyBudgetUsage struct {
	Tokens           int64
	USD              *big.Rat
	HasUnpricedUsage bool
}

// Validate rejects invalid restored totals before they can affect admission or
// be combined with new receipts.
func (usage DailyBudgetUsage) Validate() error {
	if usage.Tokens < 0 || (usage.USD != nil && usage.USD.Sign() < 0) {
		return errors.New("agentinstance: daily usage cannot be negative")
	}
	return nil
}

// AddReceipt accumulates one priced or unpriced receipt. Invalid receipts fail
// without changing totals; unknown pricing never erases the known lower bound.
func (usage *DailyBudgetUsage) AddReceipt(tokens int64, usd *float64) error {
	if err := usage.Validate(); err != nil {
		return err
	}
	if tokens < 0 || usage.Tokens > math.MaxInt64-tokens {
		return errors.New("agentinstance: daily token receipt is invalid or overflows")
	}
	if usd != nil && (*usd < 0 || math.IsNaN(*usd) || math.IsInf(*usd, 0)) {
		return errors.New("agentinstance: daily USD receipt must be finite and non-negative")
	}
	usage.Tokens += tokens
	if usd == nil {
		usage.HasUnpricedUsage = true
		return nil
	}
	receipt := decimalUSD(*usd)
	if usage.USD == nil {
		usage.USD = receipt
		return nil
	}
	usage.USD = new(big.Rat).Add(usage.USD, receipt)
	return nil
}

// decimalUSD interprets the shortest round-trippable decimal representation
// of a finite provider amount, avoiding binary float errors in accumulation.
func decimalUSD(amount float64) *big.Rat {
	decimal := strconv.FormatFloat(amount, 'f', -1, 64)
	ratio, ok := new(big.Rat).SetString(decimal)
	if !ok {
		panic("invariant: finite float formatted in decimal must parse as rational")
	}
	return ratio
}

// DailyBudgetAssessment describes whether the next turn is held and why.
type DailyBudgetAssessment struct {
	Warn            bool
	Exhausted       bool
	TokensExhausted bool
	USDExhausted    bool
	USDUnpriced     bool
}

// EvaluateDailyBudget calculates one budget decision without changing notice or
// pause state. The caller must persist warning deduplication and pause atomically
// with its authoritative usage and admission state.
func EvaluateDailyBudget(
	limits DailyBudgetLimits,
	used DailyBudgetUsage,
	previousWarning bool,
) (DailyBudgetAssessment, error) {
	if err := limits.Validate(); err != nil {
		return DailyBudgetAssessment{}, err
	}
	if err := used.Validate(); err != nil {
		return DailyBudgetAssessment{}, err
	}
	var assessment DailyBudgetAssessment
	if limits.Tokens != nil {
		tokenLimit := *limits.Tokens
		assessment.TokensExhausted = used.Tokens >= tokenLimit
		if tokenLimit > 0 && used.Tokens >= tokenLimit-tokenLimit/5 {
			assessment.Warn = true
		}
	}
	if limits.USD != nil {
		usdLimit := *limits.USD
		if usdLimit == 0 {
			// An explicit zero-dollar cap disables admission regardless of pricing.
			assessment.USDExhausted = true
		} else if used.USD != nil {
			capUSD := decimalUSD(usdLimit)
			assessment.USDExhausted = used.USD.Cmp(capUSD) >= 0
			warningUSD := new(big.Rat).Mul(capUSD, big.NewRat(4, 5))
			if used.USD.Cmp(warningUSD) >= 0 {
				assessment.Warn = true
			}
		}
	}
	assessment.USDUnpriced = limits.USD != nil && used.HasUnpricedUsage
	assessment.Exhausted = assessment.TokensExhausted || assessment.USDExhausted || assessment.USDUnpriced
	assessment.Warn = assessment.Warn && !previousWarning
	return assessment, nil
}
