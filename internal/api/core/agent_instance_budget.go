package core

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
)

// AgentInstanceDailyBudgetStore is an optional capability for operator-owned
// budget mutations and public budget status, without widening InstanceStore.
type AgentInstanceDailyBudgetStore interface {
	SetDailyBudgetLimits(context.Context, string, agentinstance.DailyBudgetLimits) error
	DailyBudgetLimits(context.Context, string) (agentinstance.DailyBudgetLimits, error)
	ReadDailyBudgetUsage(context.Context, string, time.Time) (agentinstance.DailyBudgetUsage, error)
}

// decimalBudgetUSD preserves every decimal place of exact canonical receipts.
// Receipt amounts are finite base-ten decimals, so their reduced denominators
// contain only factors two and five.
func decimalBudgetUSD(value *big.Rat) string {
	if value == nil || value.Sign() == 0 {
		return "0"
	}
	denominator := new(big.Int).Set(value.Denom())
	var twoCount, fiveCount int
	for new(big.Int).Mod(denominator, big.NewInt(2)).Sign() == 0 {
		denominator.Quo(denominator, big.NewInt(2))
		twoCount++
	}
	for new(big.Int).Mod(denominator, big.NewInt(5)).Sign() == 0 {
		denominator.Quo(denominator, big.NewInt(5))
		fiveCount++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return value.RatString()
	}
	digits := max(twoCount, fiveCount)
	decimal := value.FloatString(digits)
	if digits > 0 {
		decimal = strings.TrimRight(strings.TrimRight(decimal, "0"), ".")
	}
	return decimal
}
