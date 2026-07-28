package windowmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func statesEqual(left, right State) (bool, error) {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false, fmt.Errorf("marshal left window state: %w", err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false, fmt.Errorf("marshal right window state: %w", err)
	}
	return bytes.Equal(leftJSON, rightJSON), nil
}

func routeIntentsEqual(left, right RouteIntent) bool {
	if left.Pathname != right.Pathname || len(left.Search) != len(right.Search) {
		return false
	}
	for key, leftValue := range left.Search {
		rightValue, exists := right.Search[key]
		if !exists || !bytes.Equal(leftValue, rightValue) {
			return false
		}
	}
	return true
}
