package config

import (
	"fmt"

	"sort"
	"strings"
)

func normalizeCapabilityCatalog(catalog *CapabilityCatalog, source string) (*CapabilityCatalog, error) {
	if catalog == nil {
		return nil, nil
	}

	normalized := &CapabilityCatalog{
		Capabilities: make([]CapabilityDef, 0, len(catalog.Capabilities)),
	}
	seen := make(map[string]int, len(catalog.Capabilities))

	for idx, capability := range catalog.Capabilities {
		next := normalizeCapabilityDef(capability)
		if err := validateCapabilityDef(capability, next, source, idx); err != nil {
			return nil, err
		}
		next.Requirements = canonicalizeCapabilityRequirements(next.Requirements)
		if priorIdx, ok := seen[next.ID]; ok {
			return nil, fmt.Errorf(
				"config: validate capability catalog %q: duplicate capability id %q after normalization at indexes %d and %d",
				source,
				next.ID,
				priorIdx,
				idx,
			)
		}
		digest, err := computeCapabilityDigest(next)
		if err != nil {
			return nil, fmt.Errorf(
				"config: compute capability digest for %q in %q: %w",
				next.ID,
				source,
				err,
			)
		}
		next.Digest = digest
		seen[next.ID] = idx
		normalized.Capabilities = append(normalized.Capabilities, next)
	}

	return normalized, nil
}

func normalizeCapabilityCatalogRecords(
	records []capabilityCatalogRecord,
	source string,
) (*CapabilityCatalog, error) {
	normalized := &CapabilityCatalog{
		Capabilities: make([]CapabilityDef, 0, len(records)),
	}
	seen := make(map[string]string, len(records))

	for _, record := range records {
		next := normalizeCapabilityDef(record.capability)
		if err := validateDirectoryCapabilityDef(record.capability, next, record); err != nil {
			return nil, err
		}
		next.Requirements = canonicalizeCapabilityRequirements(next.Requirements)
		if priorSource, ok := seen[next.ID]; ok {
			return nil, fmt.Errorf(
				"config: validate capability catalog %q: duplicate capability id %q after normalization in %q and %q",
				source,
				next.ID,
				priorSource,
				record.source,
			)
		}
		digest, err := computeCapabilityDigest(next)
		if err != nil {
			return nil, fmt.Errorf(
				"config: compute capability digest for %q in %q: %w",
				next.ID,
				record.source,
				err,
			)
		}
		next.Digest = digest
		seen[next.ID] = record.source
		normalized.Capabilities = append(normalized.Capabilities, next)
	}

	return normalized, nil
}

func normalizeCapabilityDef(capability CapabilityDef) CapabilityDef {
	return CapabilityDef{
		ID:                strings.TrimSpace(capability.ID),
		Summary:           strings.TrimSpace(capability.Summary),
		Outcome:           strings.TrimSpace(capability.Outcome),
		Version:           strings.TrimSpace(capability.Version),
		ContextNeeded:     normalizeCapabilityStringList(capability.ContextNeeded),
		ArtifactsExpected: normalizeCapabilityStringList(capability.ArtifactsExpected),
		ExecutionOutline:  normalizeCapabilityStringList(capability.ExecutionOutline),
		Constraints:       normalizeCapabilityStringList(capability.Constraints),
		Examples:          normalizeCapabilityStringList(capability.Examples),
		Requirements:      normalizeCapabilityRequirementList(capability.Requirements),
	}
}

func validateCapabilityDef(raw CapabilityDef, capability CapabilityDef, source string, idx int) error {
	fieldPrefix := fmt.Sprintf("config: validate capability catalog %q: capabilities[%d]", source, idx)

	switch {
	case capability.ID == "":
		return fmt.Errorf("%s.id is required", fieldPrefix)
	case capability.Summary == "":
		return fmt.Errorf("%s.summary is required", fieldPrefix)
	case capability.Outcome == "":
		return fmt.Errorf("%s.outcome is required", fieldPrefix)
	case raw.Version != "" && capability.Version == "":
		return fmt.Errorf("%s.version must not be blank when provided", fieldPrefix)
	}

	if err := validateCapabilityRequirements(capability.Requirements, fieldPrefix+".requirements"); err != nil {
		return err
	}
	return nil
}

func validateDirectoryCapabilityDef(raw CapabilityDef, capability CapabilityDef, record capabilityCatalogRecord) error {
	normalizedBasename := strings.TrimSpace(record.basename)

	switch {
	case capability.ID == "":
		return fmt.Errorf("config: validate capability %q: id is required", record.source)
	case capability.Summary == "":
		return fmt.Errorf("config: validate capability %q: summary is required", record.source)
	case capability.Outcome == "":
		return fmt.Errorf("config: validate capability %q: outcome is required", record.source)
	case normalizedBasename != capability.ID:
		return fmt.Errorf(
			"config: validate capability %q: basename %q must match id %q",
			record.source,
			record.basename,
			capability.ID,
		)
	case raw.Version != "" && capability.Version == "":
		return fmt.Errorf("config: validate capability %q: version must not be blank when provided", record.source)
	}

	if err := validateCapabilityRequirements(
		capability.Requirements,
		fmt.Sprintf("config: validate capability %q: requirements", record.source),
	); err != nil {
		return err
	}
	return nil
}

func normalizeCapabilityStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

func normalizeCapabilityRequirementList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, len(values))
	for idx, value := range values {
		normalized[idx] = strings.TrimSpace(value)
	}

	return normalized
}

func validateCapabilityRequirements(requirements []string, fieldPrefix string) error {
	if len(requirements) == 0 {
		return nil
	}

	seen := make(map[string]int, len(requirements))
	for idx, requirement := range requirements {
		if requirement == "" {
			return fmt.Errorf("%s[%d] is required", fieldPrefix, idx)
		}
		if priorIdx, ok := seen[requirement]; ok {
			return fmt.Errorf(
				"%s duplicate value %q after normalization at indexes %d and %d",
				fieldPrefix,
				requirement,
				priorIdx,
				idx,
			)
		}
		seen[requirement] = idx
	}

	return nil
}

func canonicalizeCapabilityRequirements(requirements []string) []string {
	if len(requirements) == 0 {
		return nil
	}

	canonical := append([]string(nil), requirements...)
	sort.Strings(canonical)
	return canonical
}
