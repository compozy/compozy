package participation

import (
	"errors"
	"fmt"
)

var (
	ErrUnavailable             = errors.New("network_participation_unavailable")
	ErrChannelUnknown          = errors.New("network_channel_unknown")
	ErrStrategyInvalid         = errors.New("network_strategy_invalid")
	ErrStrategyChannelConflict = errors.New("network_strategy_channel_conflict")
	ErrAuthorityDenied         = errors.New("network_participation_permission_denied")
	ErrBoundsExceedCeiling     = errors.New("network_bounds_exceed_ceiling")
	ErrLoopRequiresLive        = errors.New("loop_requires_live")
	ErrLiveUnsupported         = errors.New("network_live_unsupported")
	ErrNotParticipating        = errors.New("not_participating")
)

type contractError struct {
	kind   error
	field  string
	reason string
}

func (e *contractError) Error() string {
	if e.field == "" {
		return fmt.Sprintf("%s: %s", e.kind, e.reason)
	}
	return fmt.Sprintf("%s: %s: %s", e.kind, e.field, e.reason)
}

func (e *contractError) Unwrap() error {
	return e.kind
}

func newContractError(kind error, field, format string, args ...any) error {
	return &contractError{
		kind:   kind,
		field:  field,
		reason: fmt.Sprintf(format, args...),
	}
}
