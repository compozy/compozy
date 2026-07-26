package model

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

// NormalizeDirectTaskParticipation returns canonical authored participation
// for an Automation job that materializes a Task directly.
func NormalizeDirectTaskParticipation(
	request *participation.Request,
) (*participation.Request, error) {
	if request == nil {
		return nil, nil
	}
	normalized, err := participation.NormalizeIntent(*request)
	if err != nil {
		return nil, err
	}
	if normalized.ChannelStrategy != nil &&
		*normalized.ChannelStrategy == participation.StrategyLoopRun {
		return nil, fmt.Errorf(
			"%w: direct Automation tasks cannot derive a Loop run channel",
			participation.ErrStrategyInvalid,
		)
	}
	if normalized == (participation.Request{}) {
		return nil, nil
	}
	return &normalized, nil
}

// ValidateDirectTaskParticipationScope rejects Live participation when a
// direct Automation task has no workspace binding.
func ValidateDirectTaskParticipationScope(
	scope Scope,
	request *participation.Request,
	participationPath string,
	scopePath string,
) error {
	if request == nil || Scope(strings.TrimSpace(string(scope))) != AutomationScopeGlobal {
		return nil
	}
	normalized, err := NormalizeDirectTaskParticipation(request)
	if err != nil || normalized == nil || normalized.Mode == nil || *normalized.Mode != participation.ModeLive {
		return err
	}
	return fmt.Errorf(
		"%s cannot be live when %s is %q because a global direct task has no workspace binding",
		participationPath,
		scopePath,
		strings.TrimSpace(string(scope)),
	)
}
