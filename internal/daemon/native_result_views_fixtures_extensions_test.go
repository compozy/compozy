package daemon

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesExtensions(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload *contract.InstallExtensionResponse
		if err := json.Unmarshal(
			[]byte(),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__extensions_info",
				Name:   "failure",
				MustKeep: []string{
					"extension.name",
					"extension.state",
					"extension.last_error",
					"extension.gateway_requirement_digest",
					"extension.diagnostics.0.severity",
					"extension.installation_profile",
					"extension.origin.entry_id",
					"extension.overrides_published",
					"extension.inputs.0.active",
					"extension.mcp_servers.0.status",
					"extension.mcp_servers.0.runtime_name",
					"extension.mcp_servers.0.auth.method",
					"extension.declared_profiles.0.needs_setup",
					"extension.declared_profiles.0.credential_requirements.0.slot",
					"extension.placements.0.dormant",
					"extension.dormant_placements.0.create_action",
					"extension.trust.decision",
					"extension.trust.warnings.0.message",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*contract.InstallExtensionResponse]{Compact: compactExtensionRecord}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload *contract.InstallExtensionResponse
		if err := json.Unmarshal(
			[]byte(),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__extensions_install",
				Name:   "failure",
				MustKeep: []string{
					"extension.name",
					"extension.state",
					"extension.last_error",
					"extension.gateway_requirement_digest",
					"extension.diagnostics.0.severity",
					"extension.installation_profile",
					"extension.origin.entry_id",
					"extension.overrides_published",
					"extension.inputs.0.active",
					"extension.mcp_servers.0.status",
					"extension.mcp_servers.0.runtime_name",
					"extension.mcp_servers.0.auth.method",
					"extension.declared_profiles.0.needs_setup",
					"extension.declared_profiles.0.credential_requirements.0.slot",
					"extension.placements.0.dormant",
					"extension.dormant_placements.0.create_action",
					"extension.trust.decision",
					"extension.trust.warnings.0.message",

					"enable_error.code", "enable_error.error",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*contract.InstallExtensionResponse]{Compact: compactExtensionRecord}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload *contract.InstallExtensionResponse
		if err := json.Unmarshal(
			[]byte(),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__extensions_dev",
				Name:   "failure",
				MustKeep: []string{
					"extension.name",
					"extension.state",
					"extension.last_error",
					"extension.gateway_requirement_digest",
					"extension.diagnostics.0.severity",
					"extension.installation_profile",
					"extension.origin.entry_id",
					"extension.overrides_published",
					"extension.inputs.0.active",
					"extension.mcp_servers.0.status",
					"extension.mcp_servers.0.runtime_name",
					"extension.mcp_servers.0.auth.method",
					"extension.declared_profiles.0.needs_setup",
					"extension.declared_profiles.0.credential_requirements.0.slot",
					"extension.placements.0.dormant",
					"extension.dormant_placements.0.create_action",
					"extension.trust.decision",
					"extension.trust.warnings.0.message",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*contract.InstallExtensionResponse]{Compact: compactExtensionRecord}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload *contract.InstallExtensionResponse
		if err := json.Unmarshal(
			[]byte(),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__extensions_reload",
				Name:   "failure",
				MustKeep: []string{
					"extension.name",
					"extension.state",
					"extension.last_error",
					"extension.gateway_requirement_digest",
					"extension.diagnostics.0.severity",
					"extension.installation_profile",
					"extension.origin.entry_id",
					"extension.overrides_published",
					"extension.inputs.0.active",
					"extension.mcp_servers.0.status",
					"extension.mcp_servers.0.runtime_name",
					"extension.mcp_servers.0.auth.method",
					"extension.declared_profiles.0.needs_setup",
					"extension.declared_profiles.0.credential_requirements.0.slot",
					"extension.placements.0.dormant",
					"extension.dormant_placements.0.create_action",
					"extension.trust.decision",
					"extension.trust.warnings.0.message",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*contract.InstallExtensionResponse]{Compact: compactExtensionRecord}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload []contract.ExtensionPayload
		if err := json.Unmarshal(
			[]byte(),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__extensions_list",
				Name:   "failure",
				MustKeep: []string{
					"extensions.0.name",
					"extensions.0.last_error",
					"extensions.0.installation_profile",
					"extensions.0.origin.entry_id",
					"extensions.0.overrides_published",
					"extensions.0.inputs.0.active",
					"extensions.0.mcp_servers.0.status",
					"extensions.0.mcp_servers.0.runtime_name",
					"extensions.0.mcp_servers.0.auth.method",
					"extensions.0.declared_profiles.0.needs_setup",
					"extensions.0.declared_profiles.0.credential_requirements.0.slot",
					"extensions.0.placements.0.dormant",
					"extensions.0.dormant_placements.0.create_action",
					"extensions.0.trust.decision",
					"extensions.0.trust.warnings.0.message",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[[]contract.ExtensionPayload]{Compact: compactExtensionList, Full: func(p []contract.ExtensionPayload) any { return map[string]any{"extensions": p} }}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload []contract.ExtensionPayload
		if err := json.Unmarshal([]byte(`[]`), &payload); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__extensions_list",
				Name:     "empty",
				MustKeep: []string{"extensions"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[[]contract.ExtensionPayload]{Compact: compactExtensionList, Full: func(p []contract.ExtensionPayload) any { return map[string]any{"extensions": p} }}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload extensionpkg.BuildResult
		if err := json.Unmarshal(
			[]byte(
				`{"generation_dir":"/tmp/generation","generation_hash":"hash","manifest_path":"/tmp/generation/extension.json","manifest":{"name":"sample","version":"1.0.0","permissions":{}}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__extensions_build",
				Name:     "failure",
				MustKeep: []string{"build.generation_hash", "build.manifest.name"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[extensionpkg.BuildResult]{Compact: compactExtensionBuild, Full: func(p extensionpkg.BuildResult) any { return map[string]any{"build": p} }}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	return rows
}
