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
				`{"extension":{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1}}}`,
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
				`{"extension":{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1}},"enable_error":{"code":"failed","message":"enable failed"}}`,
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
					"enable_error",
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
				`{"extension":{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1}}}`,
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
				`{"extension":{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1}}}`,
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
				`[{"name":"sample","version":"1.0.0","profile":"default","type":"resource","format":"compozy","source":"local","enabled":true,"state":"failed","health":"failed","health_message":"Needs input","last_error":"enable failed","failure_code":"missing_input","consecutive_failures":2,"restart_backoff_ms":1000,"generation_hash":"sha256:abc","origin_path":"/tmp/sample","gateway_confirmation_required":true,"gateway_requirement_digest":"sha256:def","missing_inputs":["api_key"],"missing_env":["TOKEN"],"description":"large static definition","permissions":["*"],"diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}],"contents":{"skills":1}}]`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__extensions_list",
				Name:     "failure",
				MustKeep: []string{"extensions.0.name", "extensions.0.last_error"},
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
