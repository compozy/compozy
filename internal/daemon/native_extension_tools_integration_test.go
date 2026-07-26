//go:build integration

package daemon

import (
	"encoding/json"
	"errors"
	"testing"

	extensionpkg "github.com/compozy/agh/internal/extension"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestNativeExtensionToolsIntegrationLifecycleParity(t *testing.T) {
	t.Run("Should match lifecycle parity through native extension tools", func(t *testing.T) {
		t.Parallel()

		deps, extRegistry, source, runtime := newNativeExtensionToolDeps(t)
		source.latestVersion = "1.0.0"
		registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())

		if _, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsInstall,
				Input: json.RawMessage(
					"{\"source\":\"marketplace\",\"slug\":\"acme/tool-ext\",\"registry\":\"github\",\"allow_unverified\":true}",
				),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_install) error = %v", err)
		}
		source.latestVersion = "2.0.0"
		if _, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsUpdate,
				Input:  json.RawMessage("{\"name\":\"tool-ext\",\"allow_unverified\":true}"),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_update) error = %v", err)
		}
		updated, err := extRegistry.Get("tool-ext")
		if err != nil {
			t.Fatalf("extension registry Get(updated) error = %v", err)
		}
		if updated.Version != "2.0.0" {
			t.Fatalf("updated version = %q, want 2.0.0", updated.Version)
		}

		if _, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsDisable,
				Input:  json.RawMessage(`{"name":"tool-ext"}`),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_disable) error = %v", err)
		}
		disabled, err := extRegistry.Get("tool-ext")
		if err != nil {
			t.Fatalf("extension registry Get(disabled) error = %v", err)
		}
		if disabled.Enabled {
			t.Fatal("extension enabled after disable = true, want false")
		}

		if _, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsEnable,
				Input:  json.RawMessage(`{"name":"tool-ext"}`),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_enable) error = %v", err)
		}
		if _, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsRemove,
				Input:  json.RawMessage(`{"name":"tool-ext"}`),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_remove) error = %v", err)
		}
		if _, err := extRegistry.Get("tool-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("extension registry Get(after remove) error = %v, want ErrExtensionNotFound", err)
		}
		if runtime.reloadCount < 4 {
			t.Fatalf("reload count = %d, want install/update/enable/disable/remove reloads", runtime.reloadCount)
		}
	})
}
