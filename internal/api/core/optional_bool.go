package core

import "strings"

func parseOptionalBoolPointer(raw string) (*bool, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := ParseOptionalBool(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
