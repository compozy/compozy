package acp

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

// SessionConfigOptionSelection identifies one typed ACP config option update.
type SessionConfigOptionSelection struct {
	ID        string
	ValueID   string
	BoolValue *bool
}

type setSessionConfigOptionWireRequest struct {
	SessionID acpsdk.SessionId       `json:"sessionId"`
	ConfigID  acpsdk.SessionConfigId `json:"configId"`
	Type      string                 `json:"type"`
	Value     any                    `json:"value"`
}

func normalizeSessionConfigOptionSelections(
	selections []SessionConfigOptionSelection,
) ([]SessionConfigOptionSelection, error) {
	if err := validateSessionConfigOptionSelections(selections); err != nil {
		return nil, err
	}
	if len(selections) == 0 {
		return nil, nil
	}
	normalized := make([]SessionConfigOptionSelection, 0, len(selections))
	for _, selection := range selections {
		candidate, err := selection.normalize()
		if err != nil {
			return nil, err
		}
		if candidate.BoolValue != nil {
			candidate.BoolValue = new(*candidate.BoolValue)
		}
		normalized = append(normalized, candidate)
	}
	slices.SortFunc(normalized, func(left SessionConfigOptionSelection, right SessionConfigOptionSelection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return normalized, nil
}

func validateSessionConfigOptionSelections(selections []SessionConfigOptionSelection) error {
	seen := make(map[string]struct{}, len(selections))
	for _, selection := range selections {
		candidate, err := selection.normalize()
		if err != nil {
			return err
		}
		if _, exists := seen[candidate.ID]; exists {
			return fmt.Errorf("acp: config option %q is selected more than once", candidate.ID)
		}
		seen[candidate.ID] = struct{}{}
	}
	return nil
}

// NormalizeSessionConfigOptionSelections validates and canonicalizes typed ACP selections.
func NormalizeSessionConfigOptionSelections(
	selections []SessionConfigOptionSelection,
) ([]SessionConfigOptionSelection, error) {
	return normalizeSessionConfigOptionSelections(selections)
}

// CloneSessionConfigOptionSelections returns an ownership-safe copy of ACP selections.
func CloneSessionConfigOptionSelections(
	selections []SessionConfigOptionSelection,
) []SessionConfigOptionSelection {
	if len(selections) == 0 {
		return nil
	}
	cloned := make([]SessionConfigOptionSelection, 0, len(selections))
	for _, selection := range selections {
		candidate := SessionConfigOptionSelection{
			ID:      strings.TrimSpace(selection.ID),
			ValueID: strings.TrimSpace(selection.ValueID),
		}
		if selection.BoolValue != nil {
			candidate.BoolValue = new(*selection.BoolValue)
		}
		cloned = append(cloned, candidate)
	}
	return cloned
}

func (s SessionConfigOptionSelection) normalize() (SessionConfigOptionSelection, error) {
	normalized := SessionConfigOptionSelection{
		ID:        strings.TrimSpace(s.ID),
		ValueID:   strings.TrimSpace(s.ValueID),
		BoolValue: s.BoolValue,
	}
	if normalized.ID == "" {
		return SessionConfigOptionSelection{}, errors.New("acp: config option ID is required")
	}
	if (normalized.ValueID == "") == (normalized.BoolValue == nil) {
		return SessionConfigOptionSelection{}, errors.New(
			"acp: config option selection requires exactly one value ID or boolean value",
		)
	}
	return normalized, nil
}

func (s SessionConfigOptionSelection) request(
	sessionID acpsdk.SessionId,
) (setSessionConfigOptionWireRequest, error) {
	normalized, err := s.normalize()
	if err != nil {
		return setSessionConfigOptionWireRequest{}, err
	}
	if strings.TrimSpace(string(sessionID)) == "" {
		return setSessionConfigOptionWireRequest{}, errors.New("acp: session ID is required")
	}
	if normalized.BoolValue != nil {
		return setSessionConfigOptionWireRequest{
			SessionID: sessionID,
			ConfigID:  acpsdk.SessionConfigId(normalized.ID),
			Type:      string(SessionConfigOptionKindBoolean),
			Value:     *normalized.BoolValue,
		}, nil
	}
	return setSessionConfigOptionWireRequest{
		SessionID: sessionID,
		ConfigID:  acpsdk.SessionConfigId(normalized.ID),
		Type:      "id",
		Value:     acpsdk.SessionConfigValueId(normalized.ValueID),
	}, nil
}

func (s SessionConfigOptionSelection) matches(option SessionConfigOption) bool {
	normalized, err := s.normalize()
	if err != nil || strings.TrimSpace(option.ID) != normalized.ID {
		return false
	}
	switch option.Kind {
	case SessionConfigOptionKindSelect:
		return normalized.BoolValue == nil && option.CurrentValueID == normalized.ValueID
	case SessionConfigOptionKindBoolean:
		return normalized.BoolValue != nil && option.CurrentBool != nil &&
			*option.CurrentBool == *normalized.BoolValue
	default:
		return false
	}
}

func validateSessionConfigOptionSelection(
	selection SessionConfigOptionSelection,
	option SessionConfigOption,
) error {
	normalized, err := selection.normalize()
	if err != nil {
		return err
	}
	if normalized.ID != strings.TrimSpace(option.ID) {
		return fmt.Errorf("acp: config option %q is not advertised", normalized.ID)
	}
	if option.ReadOnly {
		return fmt.Errorf("acp: config option %q is read-only", normalized.ID)
	}
	switch option.Kind {
	case SessionConfigOptionKindSelect:
		if normalized.BoolValue != nil || !configOptionAllowsValue(option, normalized.ValueID) {
			return fmt.Errorf("acp: config option %q does not allow value %q", normalized.ID, normalized.ValueID)
		}
	case SessionConfigOptionKindBoolean:
		if normalized.BoolValue == nil {
			return fmt.Errorf("acp: config option %q requires a boolean value", normalized.ID)
		}
	default:
		return fmt.Errorf("acp: config option %q is read-only", normalized.ID)
	}
	return nil
}
