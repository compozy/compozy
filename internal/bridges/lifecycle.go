package bridges

import (
	"fmt"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

// ValidateInstanceStateTransition reports whether the next enabled/status pair is
// a valid lifecycle transition from the current instance state.
func ValidateInstanceStateTransition(current BridgeInstance, nextEnabled bool, nextStatus BridgeStatus) error {
	normalizedCurrent := current.normalize()
	if err := normalizedCurrent.Validate(); err != nil {
		return err
	}

	normalizedNextStatus := nextStatus.Normalize()
	if err := validateInstanceLifecycle(nextEnabled, normalizedNextStatus); err != nil {
		return err
	}

	if normalizedCurrent.Enabled == nextEnabled && normalizedCurrent.Status == normalizedNextStatus {
		return nil
	}

	if !canTransitionInstanceState(
		normalizedCurrent.Enabled,
		normalizedCurrent.Status,
		nextEnabled,
		normalizedNextStatus,
	) {
		return fmt.Errorf(
			"%w: enabled=%t,status=%s -> enabled=%t,status=%s",
			ErrInvalidBridgeStateTransition,
			normalizedCurrent.Enabled,
			normalizedCurrent.Status,
			nextEnabled,
			normalizedNextStatus,
		)
	}

	return nil
}

func validateInstanceLifecycle(enabled bool, status BridgeStatus) error {
	return bridgecontract.ValidateBridgeInstanceLifecycle(enabled, bridgecontract.BridgeStatus(status))
}

func canTransitionInstanceState(
	currentEnabled bool,
	currentStatus BridgeStatus,
	nextEnabled bool,
	nextStatus BridgeStatus,
) bool {
	normalizedCurrent := currentStatus.Normalize()
	normalizedNext := nextStatus.Normalize()

	switch normalizedCurrent {
	case BridgeStatusDisabled:
		return !currentEnabled && nextEnabled && normalizedNext == BridgeStatusStarting
	case BridgeStatusStarting:
		if !currentEnabled {
			return false
		}
		return transitionFromStarting(nextEnabled, normalizedNext)
	case BridgeStatusReady:
		if !currentEnabled {
			return false
		}
		return transitionFromReady(nextEnabled, normalizedNext)
	case BridgeStatusDegraded:
		if !currentEnabled {
			return false
		}
		return transitionFromDegraded(nextEnabled, normalizedNext)
	case BridgeStatusAuthRequired:
		if !currentEnabled {
			return false
		}
		return transitionFromAuthRequired(nextEnabled, normalizedNext)
	case BridgeStatusError:
		if !currentEnabled {
			return false
		}
		return transitionFromError(nextEnabled, normalizedNext)
	default:
		return false
	}
}

func transitionFromStarting(nextEnabled bool, nextStatus BridgeStatus) bool {
	if !nextEnabled {
		return nextStatus == BridgeStatusDisabled
	}

	switch nextStatus {
	case BridgeStatusStarting, BridgeStatusReady, BridgeStatusDegraded, BridgeStatusAuthRequired, BridgeStatusError:
		return true
	default:
		return false
	}
}

func transitionFromReady(nextEnabled bool, nextStatus BridgeStatus) bool {
	if !nextEnabled {
		return nextStatus == BridgeStatusDisabled
	}

	switch nextStatus {
	case BridgeStatusReady, BridgeStatusStarting, BridgeStatusDegraded, BridgeStatusAuthRequired, BridgeStatusError:
		return true
	default:
		return false
	}
}

func transitionFromDegraded(nextEnabled bool, nextStatus BridgeStatus) bool {
	if !nextEnabled {
		return nextStatus == BridgeStatusDisabled
	}

	switch nextStatus {
	case BridgeStatusDegraded, BridgeStatusStarting, BridgeStatusReady, BridgeStatusAuthRequired, BridgeStatusError:
		return true
	default:
		return false
	}
}

func transitionFromAuthRequired(nextEnabled bool, nextStatus BridgeStatus) bool {
	if !nextEnabled {
		return nextStatus == BridgeStatusDisabled
	}

	switch nextStatus {
	case BridgeStatusAuthRequired, BridgeStatusStarting, BridgeStatusError:
		return true
	default:
		return false
	}
}

func transitionFromError(nextEnabled bool, nextStatus BridgeStatus) bool {
	if !nextEnabled {
		return nextStatus == BridgeStatusDisabled
	}

	switch nextStatus {
	case BridgeStatusError, BridgeStatusStarting:
		return true
	default:
		return false
	}
}
