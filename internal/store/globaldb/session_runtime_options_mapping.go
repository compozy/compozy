package globaldb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func encodeSessionACPOptions(
	selections []store.SessionACPOptionSelection,
	label string,
) (string, error) {
	normalized := store.NormalizeSessionACPOptionSelections(selections)
	if err := store.ValidateSessionACPOptionSelections(normalized); err != nil {
		return "", fmt.Errorf("store: validate %s: %w", label, err)
	}
	if len(normalized) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("store: encode %s: %w", label, err)
	}
	return string(encoded), nil
}

func decodeSessionACPOptions(raw string, label string) ([]store.SessionACPOptionSelection, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var selections []store.SessionACPOptionSelection
	if err := json.Unmarshal([]byte(raw), &selections); err != nil {
		return nil, fmt.Errorf("store: decode %s: %w", label, err)
	}
	normalized := store.NormalizeSessionACPOptionSelections(selections)
	if err := store.ValidateSessionACPOptionSelections(normalized); err != nil {
		return nil, fmt.Errorf("store: validate %s: %w", label, err)
	}
	return normalized, nil
}

func encodeCanonicalSelectedRuntimeACPOptions(
	selection *store.SessionRuntimeSelection,
) (string, error) {
	if selection == nil {
		return "[]", nil
	}
	if err := store.ValidateSessionACPOptionSelections(selection.ACPOptions); err != nil {
		return "", fmt.Errorf("store: validate selected runtime ACP options: %w", err)
	}
	if len(selection.ACPOptions) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(selection.ACPOptions)
	if err != nil {
		return "", fmt.Errorf("store: encode selected runtime ACP options: %w", err)
	}
	return string(encoded), nil
}

func applySessionACPOptionsScan(
	session *store.SessionInfo,
	row *sessionInfoRow,
) error {
	acpOptions, err := decodeSessionACPOptions(row.acpOptionsJSON, "session ACP options")
	if err != nil {
		return err
	}
	selectedOptions, err := decodeSessionACPOptions(
		row.selectedACPOptionsJSON,
		"selected runtime ACP options",
	)
	if err != nil {
		return err
	}
	session.SetACPOptions(acpOptions)
	session.SelectedRuntime = decodeSelectedRuntime(
		row.selectedProvider,
		row.selectedModel,
		row.selectedReasoning,
		row.selectedSpeed,
		selectedOptions,
	)
	return nil
}
