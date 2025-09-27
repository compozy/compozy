package fetch

import (
	"encoding/json"
	"fmt"
)

func decodeArgs(payload map[string]any) (Args, error) {
	var args Args
	if payload == nil {
		return args, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return args, fmt.Errorf("failed to marshal fetch args: %w", err)
	}
	if err := json.Unmarshal(data, &args); err != nil {
		return args, fmt.Errorf("failed to unmarshal fetch args: %w", err)
	}
	return args, nil
}
