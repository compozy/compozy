package clientstate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
)

var (
	domainPattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	keyPattern    = regexp.MustCompile(`^[a-zA-Z0-9:._-]{1,128}$`)
)

func validateDomain(domain string) error {
	if !domainPattern.MatchString(domain) {
		return fmt.Errorf("clientstate: domain %q: %w", domain, ErrInvalidDomain)
	}
	return nil
}

func validateKey(key string) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("clientstate: key %q: %w", key, ErrInvalidKey)
	}
	return nil
}

func validateOps(ops []Op, limits Limits) error {
	if len(ops) == 0 {
		return ErrEmptyApply
	}
	for index, op := range ops {
		if err := validateKey(op.Key); err != nil {
			return fmt.Errorf("clientstate: op %d: %w", index, err)
		}
		switch op.Kind {
		case OpPut:
			if len(op.Value) > limits.MaxValueBytes {
				return fmt.Errorf(
					"clientstate: op %d key %q has %d bytes: %w",
					index,
					op.Key,
					len(op.Value),
					ErrValueTooLarge,
				)
			}
			if !isJSONObject(op.Value) {
				return fmt.Errorf("clientstate: op %d key %q: %w", index, op.Key, ErrInvalidValue)
			}
		case OpDelete:
			if len(op.Value) != 0 {
				return fmt.Errorf(
					"clientstate: delete op %d key %q carries a value: %w",
					index,
					op.Key,
					ErrInvalidValue,
				)
			}
		default:
			return fmt.Errorf("clientstate: op %d has unknown kind %d", index, op.Kind)
		}
	}
	return nil
}

func isJSONObject(value []byte) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}' && json.Valid(trimmed)
}

func normalizeDomains(domains []string) (map[string]struct{}, error) {
	if len(domains) == 0 {
		return nil, nil
	}
	selected := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		if err := validateDomain(domain); err != nil {
			return nil, err
		}
		selected[domain] = struct{}{}
	}
	return selected, nil
}
