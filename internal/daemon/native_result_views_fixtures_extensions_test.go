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
			[]byte(
				`{"extension":{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1},"installation_profile":"default","origin":{"source":"catalog","source_ref":"acme/sample","entry_id":"sample"},"overrides_published":true,"inputs":[{"id":"token","type":"secret","set":false,"active":false}],"mcp_servers":[{"name":"remote","owner":"sample","transport":"http","launch":"managed","status":"failed","runtime_name":"sample.remote","auth":{"method":"oauth","registration":"dynamic"}}],"declared_profiles":[{"name":"growth","exists":true,"created_by_extension":true,"needs_setup":true,"credential_requirements":[{"provider":"codex","slot":"api_key","source_extension":"sample","missing":true}]}],"placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"dormant_placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"trust":{"decision":"allow_unverified","registry_tier":"community","checksum_verified":false,"allow_unverified":true,"warnings":[{"id":"trust-a","code":"unverified","severity":"warn","category":"trust","title":"Unverified","message":"Checksum unavailable","data_freshness":"fresh","evidence":{"checksum":"missing"},"suggested_command":"compozy extensions provenance sample"}]}}}`,
			),
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
			[]byte(
				`{"extension":{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1},"installation_profile":"default","origin":{"source":"catalog","source_ref":"acme/sample","entry_id":"sample"},"overrides_published":true,"inputs":[{"id":"token","type":"secret","set":false,"active":false}],"mcp_servers":[{"name":"remote","owner":"sample","transport":"http","launch":"managed","status":"failed","runtime_name":"sample.remote","auth":{"method":"oauth","registration":"dynamic"}}],"declared_profiles":[{"name":"growth","exists":true,"created_by_extension":true,"needs_setup":true,"credential_requirements":[{"provider":"codex","slot":"api_key","source_extension":"sample","missing":true}]}],"placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"dormant_placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"trust":{"decision":"allow_unverified","registry_tier":"community","checksum_verified":false,"allow_unverified":true,"warnings":[{"id":"trust-a","code":"unverified","severity":"warn","category":"trust","title":"Unverified","message":"Checksum unavailable","data_freshness":"fresh","evidence":{"checksum":"missing"},"suggested_command":"compozy extensions provenance sample"}]}},"enable_error":{"code":"failed","error":"enable failed"}}`,
			),
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
			[]byte(
				`{"extension":{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1},"installation_profile":"default","origin":{"source":"catalog","source_ref":"acme/sample","entry_id":"sample"},"overrides_published":true,"inputs":[{"id":"token","type":"secret","set":false,"active":false}],"mcp_servers":[{"name":"remote","owner":"sample","transport":"http","launch":"managed","status":"failed","runtime_name":"sample.remote","auth":{"method":"oauth","registration":"dynamic"}}],"declared_profiles":[{"name":"growth","exists":true,"created_by_extension":true,"needs_setup":true,"credential_requirements":[{"provider":"codex","slot":"api_key","source_extension":"sample","missing":true}]}],"placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"dormant_placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"trust":{"decision":"allow_unverified","registry_tier":"community","checksum_verified":false,"allow_unverified":true,"warnings":[{"id":"trust-a","code":"unverified","severity":"warn","category":"trust","title":"Unverified","message":"Checksum unavailable","data_freshness":"fresh","evidence":{"checksum":"missing"},"suggested_command":"compozy extensions provenance sample"}]}}}`,
			),
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
			[]byte(
				`{"extension":{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1},"installation_profile":"default","origin":{"source":"catalog","source_ref":"acme/sample","entry_id":"sample"},"overrides_published":true,"inputs":[{"id":"token","type":"secret","set":false,"active":false}],"mcp_servers":[{"name":"remote","owner":"sample","transport":"http","launch":"managed","status":"failed","runtime_name":"sample.remote","auth":{"method":"oauth","registration":"dynamic"}}],"declared_profiles":[{"name":"growth","exists":true,"created_by_extension":true,"needs_setup":true,"credential_requirements":[{"provider":"codex","slot":"api_key","source_extension":"sample","missing":true}]}],"placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"dormant_placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"trust":{"decision":"allow_unverified","registry_tier":"community","checksum_verified":false,"allow_unverified":true,"warnings":[{"id":"trust-a","code":"unverified","severity":"warn","category":"trust","title":"Unverified","message":"Checksum unavailable","data_freshness":"fresh","evidence":{"checksum":"missing"},"suggested_command":"compozy extensions provenance sample"}]}}}`,
			),
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
			[]byte(
				`[{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1},"installation_profile":"default","origin":{"source":"catalog","source_ref":"acme/sample","entry_id":"sample"},"overrides_published":true,"inputs":[{"id":"token","type":"secret","set":false,"active":false}],"mcp_servers":[{"name":"remote","owner":"sample","transport":"http","launch":"managed","status":"failed","runtime_name":"sample.remote","auth":{"method":"oauth","registration":"dynamic"}}],"declared_profiles":[{"name":"growth","exists":true,"created_by_extension":true,"needs_setup":true,"credential_requirements":[{"provider":"codex","slot":"api_key","source_extension":"sample","missing":true}]}],"placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"dormant_placements":[{"kind":"agent","resource":"worker","profile":"missing","dormant":true,"create_action":"compozy profile create missing"}],"trust":{"decision":"allow_unverified","registry_tier":"community","checksum_verified":false,"allow_unverified":true,"warnings":[{"id":"trust-a","code":"unverified","severity":"warn","category":"trust","title":"Unverified","message":"Checksum unavailable","data_freshness":"fresh","evidence":{"checksum":"missing"},"suggested_command":"compozy extensions provenance sample"}]}}]`,
			),
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
