package session

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	// ErrBadgeFilterInvalid reports an unsupported public badge filter.
	ErrBadgeFilterInvalid = errors.New("session: badge filter is invalid")
)

var publicBadgeFilterOrder = []Badge{
	BadgeWaitingForInput,
	BadgeWaitingForAuth,
	BadgeIdle,
	BadgeDone,
	BadgeRunning,
	BadgeStopped,
	BadgeFailed,
	BadgeHung,
	BadgeUnhealthy,
}

// ParseBadgeFilters validates, deduplicates, and canonically orders public badge filters.
func ParseBadgeFilters(values []string) ([]Badge, error) {
	selected := make(map[Badge]struct{}, len(values))
	for _, value := range values {
		for token := range strings.SplitSeq(value, ",") {
			badge := Badge(strings.TrimSpace(token))
			if badge == "" {
				continue
			}
			if !publicBadgeFilterAllowed(badge) {
				return nil, fmt.Errorf(
					"%w: unknown session badge %q (valid: %s)",
					ErrBadgeFilterInvalid,
					badge,
					strings.Join(publicBadgeFilterStrings(), ", "),
				)
			}
			selected[badge] = struct{}{}
		}
	}
	result := make([]Badge, 0, len(selected))
	for _, badge := range publicBadgeFilterOrder {
		if _, ok := selected[badge]; ok {
			result = append(result, badge)
		}
	}
	return result, nil
}

func publicBadgeFilterAllowed(target Badge) bool {
	return slices.Contains(publicBadgeFilterOrder, target)
}

func publicBadgeFilterStrings() []string {
	result := make([]string, 0, len(publicBadgeFilterOrder))
	for _, badge := range publicBadgeFilterOrder {
		result = append(result, string(badge))
	}
	return result
}
