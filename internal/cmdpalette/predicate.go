package cmdpalette

import (
	"fmt"
	"reflect"
	"strings"
)

const attachedShellReason = "requires an attached shell"

func validatePredicate(predicate Predicate) error {
	if !knownContextKey(predicate.Key) {
		return fmt.Errorf("unknown context key %q", predicate.Key)
	}
	operator := predicate.Operator
	if operator == "" {
		operator = PredicateEquals
	}
	switch operator {
	case PredicateEquals, PredicateNotEquals:
		return nil
	case PredicateGreaterThanOrEqual:
		if predicate.Key != ContextDesktopWindowCount {
			return fmt.Errorf("operator %q is only valid for %s", operator, ContextDesktopWindowCount)
		}
		if _, ok := numericValue(predicate.Value); !ok {
			return fmt.Errorf("operator %q requires a numeric value", operator)
		}
		return nil
	default:
		return fmt.Errorf("unknown predicate operator %q", operator)
	}
}

func knownContextKey(key ContextKey) bool {
	switch key {
	case ContextWindowFocused, ContextWindowFloating, ContextWindowStacked,
		ContextDesktopWindowCount, ContextScopeGlobal, ContextShellDesktop,
		ContextSessionState, ContextWorkspaceTrusted:
		return true
	default:
		return false
	}
}

func clientContextKey(key ContextKey) bool {
	switch key {
	case ContextWindowFocused, ContextWindowFloating, ContextWindowStacked,
		ContextDesktopWindowCount, ContextShellDesktop, ContextSessionState:
		return true
	default:
		return false
	}
}

func resolveAvailability(descriptor Descriptor, snapshot *ContextSnapshot) (bool, string) {
	if reason := strings.TrimSpace(descriptor.ProviderUnavailableReason); reason != "" {
		return false, reason
	}
	if snapshot == nil && descriptor.Action.Kind != ActionKindTool && !descriptor.AvailabilityExempt {
		return false, attachedShellReason
	}
	for _, predicate := range descriptor.When {
		if snapshot == nil && clientContextKey(predicate.Key) {
			return false, attachedShellReason
		}
		if !predicateMatches(predicate, snapshot) {
			reason := strings.TrimSpace(predicate.Reason)
			if reason == "" {
				reason = fmt.Sprintf("requires %s", predicate.Key)
			}
			return false, reason
		}
	}
	return true, ""
}

func predicateMatches(predicate Predicate, snapshot *ContextSnapshot) bool {
	if snapshot == nil {
		return false
	}
	actual, exists := snapshot.Values[predicate.Key]
	if !exists {
		return false
	}
	operator := predicate.Operator
	if operator == "" {
		operator = PredicateEquals
	}
	switch operator {
	case PredicateEquals:
		return reflect.DeepEqual(actual, predicate.Value)
	case PredicateNotEquals:
		return !reflect.DeepEqual(actual, predicate.Value)
	case PredicateGreaterThanOrEqual:
		actualNumber, actualOK := numericValue(actual)
		wantedNumber, wantedOK := numericValue(predicate.Value)
		return actualOK && wantedOK && actualNumber >= wantedNumber
	default:
		return false
	}
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}
