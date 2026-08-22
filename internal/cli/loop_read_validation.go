package cli

import "fmt"

const (
	loopInventoryPageLimitMax = 200
	loopRunReadPageLimitMax   = 500
)

func validateLoopPageLimit(limit int, limitSet bool, maximum int) error {
	if (limitSet && limit < 1) || limit > maximum {
		return withCommandExitCode(2, fmt.Errorf("--limit must be between 1 and %d", maximum))
	}
	return nil
}
