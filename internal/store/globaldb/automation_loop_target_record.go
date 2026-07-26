package globaldb

import (
	"database/sql"
	"fmt"
	"strings"

	automation "github.com/compozy/agh/internal/automation/model"
)

type automationLoopTargetEncoded struct {
	workspaceID              string
	loopName                 string
	inputsJSON               any
	inputMappingJSON         any
	networkParticipationJSON any
}

type automationLoopTargetRecord struct {
	workspaceID             sql.NullString
	loopName                sql.NullString
	inputsRaw               sql.NullString
	inputMappingRaw         sql.NullString
	networkParticipationRaw sql.NullString
}

func normalizeAutomationTargetKind(kind automation.TargetKind, target *automation.LoopTarget) automation.TargetKind {
	normalized := automation.TargetKind(strings.TrimSpace(string(kind)))
	if normalized != "" {
		return normalized
	}
	if target != nil {
		return automation.TargetKindLoop
	}
	return automation.TargetKindAgent
}

func normalizeAutomationLoopTarget(target *automation.LoopTarget) *automation.LoopTarget {
	if target == nil {
		return nil
	}
	normalized := *target
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.LoopName = strings.TrimSpace(normalized.LoopName)
	normalized.InputMapping = normalizeStringMap(normalized.InputMapping)
	normalized.NetworkParticipation = normalizeAutomationParticipationIntent(
		normalized.NetworkParticipation,
	)
	return &normalized
}

func normalizeStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(values))
	for rawKey, rawValue := range values {
		normalized[strings.TrimSpace(rawKey)] = strings.TrimSpace(rawValue)
	}
	return normalized
}

func encodeAutomationLoopTarget(
	target *automation.LoopTarget,
	label string,
) (automationLoopTargetEncoded, error) {
	if target == nil {
		return automationLoopTargetEncoded{}, nil
	}
	inputsJSON, err := encodeOptionalAutomationJSON(target.Inputs, len(target.Inputs) == 0, label+".inputs")
	if err != nil {
		return automationLoopTargetEncoded{}, err
	}
	inputMappingJSON, err := encodeOptionalAutomationJSON(
		target.InputMapping,
		len(target.InputMapping) == 0,
		label+".input_mapping",
	)
	if err != nil {
		return automationLoopTargetEncoded{}, err
	}
	networkParticipationJSON, err := encodeOptionalAutomationJSON(
		target.NetworkParticipation,
		target.NetworkParticipation == nil,
		label+".network_participation",
	)
	if err != nil {
		return automationLoopTargetEncoded{}, err
	}
	return automationLoopTargetEncoded{
		workspaceID:              strings.TrimSpace(target.WorkspaceID),
		loopName:                 strings.TrimSpace(target.LoopName),
		inputsJSON:               inputsJSON,
		inputMappingJSON:         inputMappingJSON,
		networkParticipationJSON: networkParticipationJSON,
	}, nil
}

func decodeAutomationLoopTarget(
	record automationLoopTargetRecord,
	targetKind automation.TargetKind,
	target **automation.LoopTarget,
	label string,
) error {
	if targetKind != automation.TargetKindLoop {
		if record.hasAnyValue() {
			return fmt.Errorf("store: %s columns require target_kind %q", label, automation.TargetKindLoop)
		}
		*target = nil
		return nil
	}

	workspaceID := automationNullStringValue(record.workspaceID)
	loopName := automationNullStringValue(record.loopName)
	if workspaceID == "" {
		return fmt.Errorf("store: %s.workspace_id is required", label)
	}
	if loopName == "" {
		return fmt.Errorf("store: %s.loop_name is required", label)
	}

	decoded := automation.LoopTarget{
		WorkspaceID: workspaceID,
		LoopName:    loopName,
	}
	if err := decodeOptionalAutomationJSON(record.inputsRaw, &decoded.Inputs, label+".inputs"); err != nil {
		return err
	}
	if err := decodeOptionalAutomationJSON(
		record.inputMappingRaw,
		&decoded.InputMapping,
		label+".input_mapping",
	); err != nil {
		return err
	}
	if err := decodeOptionalAutomationJSON(
		record.networkParticipationRaw,
		&decoded.NetworkParticipation,
		label+".network_participation",
	); err != nil {
		return err
	}
	*target = &decoded
	return nil
}

func (r automationLoopTargetRecord) hasAnyValue() bool {
	return r.workspaceID.Valid ||
		r.loopName.Valid ||
		r.inputsRaw.Valid ||
		r.inputMappingRaw.Valid ||
		r.networkParticipationRaw.Valid
}

func decodeOptionalAutomationJSON[T any](raw sql.NullString, target *T, label string) error {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil
	}
	if err := decodeAutomationJSON(raw.String, target, label); err != nil {
		return err
	}
	return nil
}
