package cmdpalette

import (
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
)

func resolveViewCapabilities(
	payload ViewPayload,
	capabilities map[string]string,
	reporter CapabilityGapReporter,
) (ViewPayload, error) {
	for sectionIndex := range payload.Sections {
		rows := payload.Sections[sectionIndex].Rows
		resolved := make([]Row, 0, len(rows))
		for rowIndex, row := range rows {
			path := fmt.Sprintf("sections[%d].rows[%d]", sectionIndex, rowIndex)
			value, keep, err := resolveRequiredElement(path, row, row.Requires, row.Fallback, capabilities, reporter)
			if err != nil {
				return ViewPayload{}, err
			}
			if !keep {
				continue
			}
			value.Actions, err = resolveActions(path+".actions", value.Actions, capabilities, reporter)
			if err != nil {
				return ViewPayload{}, err
			}
			if value.Detail != nil {
				value.Detail, err = resolveDetail(path+".detail", value.Detail, capabilities, reporter)
				if err != nil {
					return ViewPayload{}, err
				}
			}
			resolved = append(resolved, value)
		}
		payload.Sections[sectionIndex].Rows = resolved
	}

	chips := make([]Chip, 0, len(payload.Chips))
	for index, chip := range payload.Chips {
		value, keep, err := resolveRequiredElement(
			fmt.Sprintf("chips[%d]", index), chip, chip.Requires, chip.Fallback, capabilities, reporter,
		)
		if err != nil {
			return ViewPayload{}, err
		}
		if keep {
			chips = append(chips, value)
		}
	}
	payload.Chips = chips

	var err error
	if payload.Detail != nil {
		payload.Detail, err = resolveDetail("detail", payload.Detail, capabilities, reporter)
		if err != nil {
			return ViewPayload{}, err
		}
	}
	if payload.Form != nil {
		payload.Form.Fields, err = resolveFormFields(payload.Form.Fields, capabilities, reporter)
		if err != nil {
			return ViewPayload{}, err
		}
		if payload.Form.Submit != nil {
			actions, actionErr := resolveActions(
				"form.submit",
				[]RowAction{*payload.Form.Submit},
				capabilities,
				reporter,
			)
			if actionErr != nil {
				return ViewPayload{}, actionErr
			}
			payload.Form.Submit = nil
			if len(actions) == 1 {
				payload.Form.Submit = &actions[0]
			}
		}
	}
	if payload.Grid != nil {
		if err := resolveGrid(payload.Grid, capabilities, reporter); err != nil {
			return ViewPayload{}, err
		}
	}
	return payload, nil
}

func resolveDetail(
	path string,
	detail *DetailBody,
	capabilities map[string]string,
	reporter CapabilityGapReporter,
) (*DetailBody, error) {
	resolved := *detail
	resolved.Metadata = make([]MetaField, 0, len(detail.Metadata))
	for index, field := range detail.Metadata {
		value, keep, err := resolveRequiredElement(
			fmt.Sprintf("%s.metadata[%d]", path, index),
			field,
			field.Requires,
			field.Fallback,
			capabilities,
			reporter,
		)
		if err != nil {
			return nil, err
		}
		if keep {
			resolved.Metadata = append(resolved.Metadata, value)
		}
	}
	var err error
	resolved.Actions, err = resolveActions(path+".actions", detail.Actions, capabilities, reporter)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func resolveFormFields(
	fields []FormField,
	capabilities map[string]string,
	reporter CapabilityGapReporter,
) ([]FormField, error) {
	resolved := make([]FormField, 0, len(fields))
	for index, field := range fields {
		value, keep, err := resolveRequiredElement(
			fmt.Sprintf("form.fields[%d]", index),
			field,
			field.Requires,
			field.Fallback,
			capabilities,
			reporter,
		)
		if err != nil {
			return nil, err
		}
		if keep {
			resolved = append(resolved, value)
		}
	}
	return resolved, nil
}

func resolveGrid(
	grid *GridBody,
	capabilities map[string]string,
	reporter CapabilityGapReporter,
) error {
	for sectionIndex := range grid.Sections {
		tiles := grid.Sections[sectionIndex].Tiles
		resolved := make([]GridTile, 0, len(tiles))
		for tileIndex, tile := range tiles {
			path := fmt.Sprintf("grid.sections[%d].tiles[%d]", sectionIndex, tileIndex)
			value, keep, err := resolveRequiredElement(
				path, tile, tile.Requires, tile.Fallback, capabilities, reporter,
			)
			if err != nil {
				return err
			}
			if !keep {
				continue
			}
			value.Actions, err = resolveActions(path+".actions", value.Actions, capabilities, reporter)
			if err != nil {
				return err
			}
			resolved = append(resolved, value)
		}
		grid.Sections[sectionIndex].Tiles = resolved
	}
	return nil
}

func resolveActions(
	path string,
	actions []RowAction,
	capabilities map[string]string,
	reporter CapabilityGapReporter,
) ([]RowAction, error) {
	resolved := make([]RowAction, 0, len(actions))
	for index, action := range actions {
		value, keep, err := resolveRequiredElement(
			fmt.Sprintf("%s[%d]", path, index),
			action,
			action.Requires,
			action.Fallback,
			capabilities,
			reporter,
		)
		if err != nil {
			return nil, err
		}
		if keep {
			resolved = append(resolved, value)
		}
	}
	return resolved, nil
}

func resolveRequiredElement[T any](
	path string,
	element T,
	requires map[string]string,
	fallback string,
	capabilities map[string]string,
	reporter CapabilityGapReporter,
) (T, bool, error) {
	if requirementsMet(requires, capabilities) {
		return element, true, nil
	}
	if reporter != nil {
		reporter.RecordCapabilityGap(path, maps.Clone(requires))
	}
	var zero T
	if fallback == "" || fallback == "drop" {
		return zero, false, nil
	}
	if err := json.Unmarshal([]byte(fallback), &zero); err != nil {
		return zero, false, viewValidationError(path+".fallback", "must be drop or a valid fallback element: %v", err)
	}
	return zero, true, nil
}

func requirementsMet(requires, capabilities map[string]string) bool {
	for name, required := range requires {
		actual, exists := capabilities[name]
		if !exists || compareCapabilityVersions(actual, required) < 0 {
			return false
		}
	}
	return true
}

func compareCapabilityVersions(actual, required string) int {
	actualParts := strings.Split(actual, ".")
	requiredParts := strings.Split(required, ".")
	length := max(len(actualParts), len(requiredParts))
	for index := range length {
		actualPart := capabilityVersionPart(actualParts, index)
		requiredPart := capabilityVersionPart(requiredParts, index)
		if actualPart < requiredPart {
			return -1
		}
		if actualPart > requiredPart {
			return 1
		}
	}
	return 0
}

func capabilityVersionPart(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	value, err := strconv.Atoi(parts[index])
	if err != nil {
		return -1
	}
	return value
}
