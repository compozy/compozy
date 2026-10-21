package session

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	ErrWorkspaceSelectionInvalid  = errors.New("workspace_breadth_invalid")
	ErrWorkspaceSelectionReserved = errors.New("workspace_reserved")
)

func NormalizeWorkspaceIDs(values []string, all bool) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if !all {
		return nil, fmt.Errorf("%w: %w: workspace_ids requires all_workspaces", ErrListQueryInvalid, ErrWorkspaceSelectionInvalid)
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "global" {
			return nil, fmt.Errorf("%w: %w: global is not a data workspace", ErrListQueryInvalid, ErrWorkspaceSelectionReserved)
		}
		if value == "" {
			return nil, fmt.Errorf("%w: invalid workspace_ids value %q; use @user for User", errors.Join(ErrListQueryInvalid, ErrWorkspaceSelectionInvalid), value)
		}
		normalized = append(normalized, value)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized), nil
}

func sessionMatchesWorkspaceIDs(id string, values []string) bool {
	if len(values) == 0 {
		return true
	}
	if id == "" {
		id = "@user"
	}
	return slices.Contains(values, id)
}
