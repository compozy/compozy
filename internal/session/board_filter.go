package session

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const childrenExclude = "exclude"

var ErrBoardFilterInvalid = fmt.Errorf("%w: %w", ErrListQueryInvalid, errors.New("invalid board selector"))

func ParseBoardColumns(values []string) ([]string, error) {
	columns := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		switch BoardColumn(value) {
		case BoardNeedsYou, BoardWorking, BoardWaiting, BoardInReview, BoardDone:
		default:
			return nil, fmt.Errorf(
				"%w: board_column %q is not one of needs_you, working, waiting, in_review, done",
				ErrBoardFilterInvalid,
				value,
			)
		}
		if !slices.Contains(columns, value) {
			columns = append(columns, value)
		}
	}
	return columns, nil
}

func ParseChildrenFilter(value string) (bool, error) {
	switch strings.TrimSpace(value) {
	case "", "include":
		return false, nil
	case childrenExclude:
		return true, nil
	default:
		return false, fmt.Errorf("%w: children %q is not one of include, exclude", ErrBoardFilterInvalid, value)
	}
}
