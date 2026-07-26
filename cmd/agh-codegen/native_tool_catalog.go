package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
	builtintools "github.com/compozy/agh/internal/tools/builtin"
)

type nativeToolCatalogEntry struct {
	ToolID               toolspkg.ToolID      `json:"tool_id"`
	NativeName           string               `json:"native_name"`
	Toolsets             []toolspkg.ToolsetID `json:"toolsets,omitempty"`
	Risk                 toolspkg.RiskClass   `json:"risk"`
	ReadOnly             bool                 `json:"read_only"`
	Destructive          bool                 `json:"destructive"`
	OpenWorld            bool                 `json:"open_world"`
	InputSchemaDigest    string               `json:"input_schema_digest"`
	OutputSchemaDigest   string               `json:"output_schema_digest,omitempty"`
	RequiredCapabilities []string             `json:"required_capabilities,omitempty"`
}

func generateNativeToolCatalog() ([]byte, error) {
	descriptors := builtintools.NativeDescriptors()
	entries := make([]nativeToolCatalogEntry, 0, len(descriptors))
	for _, descriptor := range descriptors {
		descriptor, err := toolspkg.DescriptorWithSchemaDigests(descriptor)
		if err != nil {
			return nil, fmt.Errorf("generate native tool catalog %q: %w", descriptor.ID, err)
		}
		nativeName := strings.TrimSpace(descriptor.Backend.NativeName)
		if nativeName == "" {
			return nil, fmt.Errorf("generate native tool catalog %q: native_name is required", descriptor.ID)
		}
		entry := nativeToolCatalogEntry{
			ToolID:               descriptor.ID,
			NativeName:           nativeName,
			Toolsets:             append([]toolspkg.ToolsetID(nil), descriptor.Toolsets...),
			Risk:                 descriptor.Risk,
			ReadOnly:             descriptor.ReadOnly,
			Destructive:          descriptor.Destructive,
			OpenWorld:            descriptor.OpenWorld,
			InputSchemaDigest:    descriptor.InputSchemaDigest,
			OutputSchemaDigest:   descriptor.OutputSchemaDigest,
			RequiredCapabilities: append([]string(nil), descriptor.Backend.RequiresCapabilities...),
		}
		slices.Sort(entry.Toolsets)
		slices.Sort(entry.RequiredCapabilities)
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(left nativeToolCatalogEntry, right nativeToolCatalogEntry) int {
		return strings.Compare(left.ToolID.String(), right.ToolID.String())
	})
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal native tool catalog: %w", err)
	}
	data = append(data, byte('\n'))
	return data, nil
}
