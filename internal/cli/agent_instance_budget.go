package cli

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

// updateBudget is an explicit full replacement. Reading an old pair of limits
// and writing it back after changing one field would silently discard another
// operator's concurrent change. Require both flags, including 'off' for a
// dimension that must remain unlimited; --clear removes both.
func (options agentInstanceOptions) updateBudget(
	ctx context.Context, cmd *cobra.Command, base any, id string,
) error {
	setTokens, setUSD := cmd.Flags().Changed("tokens"), cmd.Flags().Changed("usd")
	if options.clearBudget && (setTokens || setUSD) {
		return errors.New("cli: cannot combine --clear with --tokens or --usd")
	}
	if !options.clearBudget && (!setTokens || !setUSD) {
		return errors.New("cli: specify both --tokens and --usd (use 'off' for an unlimited dimension), or --clear")
	}
	budgetClient, ok := base.(agentInstanceBudgetClient)
	if !ok {
		return errors.New("cli: instance daily budget settings unavailable")
	}
	var input contract.AgentInstanceDailyBudgetInput
	if !options.clearBudget {
		if setTokens {
			value := strings.TrimSpace(options.budgetTokens)
			if strings.EqualFold(value, "off") {
				input.Tokens = nil
			} else {
				limit, err := strconv.ParseInt(value, 10, 64)
				if err != nil || limit < 0 {
					return fmt.Errorf("cli: --tokens must be a non-negative integer or 'off'")
				}
				input.Tokens = &limit
			}
		}
		if setUSD {
			value := strings.TrimSpace(options.budgetUSD)
			if strings.EqualFold(value, "off") {
				input.USD = nil
			} else {
				limit, err := strconv.ParseFloat(value, 64)
				if err != nil || limit < 0 || math.IsNaN(limit) || math.IsInf(limit, 0) {
					return errors.New("cli: invalid USD cap; use a finite non-negative number or 'off'")
				}
				input.USD = &limit
			}
		}
	}
	result, err := budgetClient.SetAgentInstanceDailyBudget(ctx, id, input)
	if err != nil {
		return err
	}
	return writeInstanceOutput(cmd, result, instanceText(result.Instance))
}
