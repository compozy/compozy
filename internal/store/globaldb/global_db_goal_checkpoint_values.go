package globaldb

import "github.com/compozy/agh/internal/loop/goal"

func goalGrantConsumed(grant *goal.ControlGrant) int {
	if grant == nil || grant.Consumed {
		return 1
	}
	return 0
}
