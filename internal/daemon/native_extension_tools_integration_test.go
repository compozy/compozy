//go:build integration

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	registrypkg "github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func nativeNetworkExtensionDownloadResult(
	t *testing.T,
	version string,
	channelScope string,
) *registrypkg.DownloadResult {
	t.Helper()

	return &registrypkg.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader(nativeExtensionTarGzWithNetwork(t, version, channelScope))),
		Slug:        "acme/tool-ext",
		Version:     version,
		ContentSize: -1,
		ContentType: "application/gzip",
	}
}

func TestNativeExtensionToolsIntegrationLifecycleParity(t *testing.T) {
	t.Run("Should match lifecycle parity through native extension tools", func(t *testing.T) {
		t.Parallel()

		deps, extRegistry, source, runtime := newNativeExtensionToolDeps(t)
		inspectionRuntime := &nativeExtensionInspectionRuntime{
			fakeExtensionRuntime: runtime,
			manager:              extensionpkg.NewManager(extRegistry),
		}
		deps.ExtensionRuntime = func() extensionRuntime { return inspectionRuntime }
		source.downloads["1.0.0"] = nativeNetworkExtensionDownloadResult(t, "1.0.0", "builders")
		source.downloads["2.0.0"] = nativeNetworkExtensionDownloadResult(t, "2.0.0", "reviewers")
		source.latestVersion = "1.0.0"
		automation, ok := deps.Automation.(extensionAutomationPreviewer)
		if !ok {
			t.Fatalf("native extension automation = %T, want extensionAutomationPreviewer", deps.Automation)
		}
		service, ok := newDaemonExtensionService(daemonExtensionServiceDeps{
			Registry:  extRegistry,
			Runtime:   inspectionRuntime,
			HomePaths: deps.HomePaths,
		},
			withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources),
			withDaemonExtensionEventWriter(deps.ExtensionEvents),
			withDaemonExtensionAutomation(automation),
			withDaemonExtensionResources(nil, resources.MutationActor{}, inventoryTestResourceCodecs(t)),
		).(*daemonExtensionService)
		if !ok {
			t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
		}
		deps.Extensions = func() core.ExtensionService { return service }
		registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
		agentScope := toolspkg.Scope{
			SessionID:   "native-agent-session",
			WorkspaceID: "workspace-native-agent",
			ActorKind:   string(taskpkg.ActorKindAgentSession),
		}

		if _, err := registry.Call(
			t.Context(),
			agentScope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsInstall,
				Input: json.RawMessage(
					"{\"source\":\"github\",\"ref\":\"acme/tool-ext\",\"allow_unverified\":true}",
				),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_install) error = %v; cause = %v", err, errors.Unwrap(err))
		}
		installed, err := extRegistry.Get("tool-ext")
		if err != nil {
			t.Fatalf("extension registry Get(installed) error = %v", err)
		}
		if installed.Enabled {
			t.Fatal("installed extension enabled = true, want inert install")
		}
		inspected, err := inspectionRuntime.InspectPackageResources(t.Context(), "tool-ext")
		if err != nil {
			t.Fatalf("InspectPackageResources(tool-ext) error = %v", err)
		}
		if len(inspected.Skills) != 1 {
			t.Fatalf(
				"inspected extension skills = %#v from manifest %#v, want one shipped skill",
				inspected.Skills,
				inspected.Manifest.Resources.Skills,
			)
		}
		assertNativeExtensionInventoryPreviewParity(t, registry, agentScope, service, "tool-ext")

		firstDigest := nativeIntegrationNetworkDigest(t, "builders")
		_, err = registry.Call(t.Context(), agentScope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsEnable,
			Input:  json.RawMessage(`{"name":"tool-ext"}`),
		})
		if !errors.Is(err, extensionpkg.ErrExtensionNetworkConfirmationRequired) {
			t.Fatalf("Registry.Call(extensions_enable without confirmation) error = %v", err)
		}
		if _, err := registry.Call(t.Context(), agentScope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsEnable,
			Input: json.RawMessage(fmt.Sprintf(
				`{"name":"tool-ext","confirm_network_digest":%q}`,
				firstDigest,
			)),
		}); err != nil {
			t.Fatalf("Registry.Call(extensions_enable confirmed) error = %v", err)
		}
		source.latestVersion = "2.0.0"
		_, err = registry.Call(
			t.Context(),
			agentScope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsUpdate,
				Input:  json.RawMessage("{\"name\":\"tool-ext\",\"allow_unverified\":true}"),
			},
		)
		if !errors.Is(err, extensionpkg.ErrExtensionNetworkConfirmationRequired) {
			t.Fatalf("Registry.Call(extensions_update without confirmation) error = %v", err)
		}
		unchanged, err := extRegistry.Get("tool-ext")
		if err != nil || unchanged.Version != "1.0.0" {
			t.Fatalf("extension after refused update = %#v, %v, want version 1.0.0", unchanged, err)
		}
		source.downloads["2.0.0"] = nativeNetworkExtensionDownloadResult(t, "2.0.0", "reviewers")
		secondDigest := nativeIntegrationNetworkDigest(t, "reviewers")
		if _, err := registry.Call(t.Context(), agentScope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsUpdate,
			Input: json.RawMessage(fmt.Sprintf(
				`{"name":"tool-ext","allow_unverified":true,"confirm_network_digest":%q}`,
				secondDigest,
			)),
		}); err != nil {
			t.Fatalf("Registry.Call(extensions_update confirmed) error = %v; cause = %v", err, errors.Unwrap(err))
		}
		updated, err := extRegistry.Get("tool-ext")
		if err != nil {
			t.Fatalf("extension registry Get(updated) error = %v", err)
		}
		if updated.Version != "2.0.0" {
			t.Fatalf("updated version = %q, want 2.0.0", updated.Version)
		}
		confirmation, err := extRegistry.NetworkConfirmation(extensionpkg.GlobalInstanceKey("tool-ext"))
		if err != nil || confirmation.Digest != secondDigest ||
			confirmation.ConfirmedBy != "agent:native-agent-session" {
			t.Fatalf("updated network confirmation = %#v, %v", confirmation, err)
		}

		if _, err := registry.Call(
			t.Context(),
			agentScope,
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
			agentScope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsEnable,
				Input:  json.RawMessage(`{"name":"tool-ext"}`),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_enable) error = %v", err)
		}
		if _, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsRemove,
				Input:  json.RawMessage(`{"name":"tool-ext"}`),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_remove) error = %v; cause = %v", err, errors.Unwrap(err))
		}
		if _, err := extRegistry.Get("tool-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("extension registry Get(after remove) error = %v, want ErrExtensionNotFound", err)
		}
		if runtime.reloadCount < 4 {
			t.Fatalf("reload count = %d, want install/update/enable/disable/remove reloads", runtime.reloadCount)
		}
	})

	t.Run("Should run the workspace-bound authoring and development tools with structured results", func(t *testing.T) {
		deps, extRegistry, _, _ := newNativeExtensionToolDeps(t)
		workspaceRoot := t.TempDir()
		workspaceRegistrationID := "workspace-native-extension"
		workspaceLookupIdentity := "01KYYQSM30GYWR3KY485HKB9QT"
		resolver := &daemonExtensionWorkspaceResolverStub{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      workspaceRegistrationID,
					Name:    "native-extension-workspace",
					RootDir: workspaceRoot,
				},
				WorkspaceID: workspaceLookupIdentity,
			},
		}
		installNativeGlobalResourceExtension(t, extRegistry, "global-native")
		manager := extensionpkg.NewManager(extRegistry, extensionpkg.WithWorkspaceResolver(resolver))
		if err := manager.Start(t.Context()); err != nil {
			t.Fatalf("extension manager Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(context.Background()); err != nil {
				t.Errorf("extension manager Stop() error = %v", err)
			}
		})
		const publishCredential = "native-publish-secret-value"
		publishServer, publishCapture := newNativeExtensionPublishServer(t, publishCredential)
		defer publishServer.Close()
		deps.ExtensionRuntime = func() extensionRuntime { return manager }
		deps.WorkspaceResolver = resolver
		deps.ExtensionConfig.Sources.GitHub.BaseURL = publishServer.URL
		deps.ExtensionSecrets = nativeExtensionSecretResolver{
			values: map[string]string{"env:GITHUB_TOKEN": publishCredential},
		}
		registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{WorkspaceID: workspaceRegistrationID, Operator: true}

		initDir := filepath.Join(workspaceRoot, "scaffolded")
		initResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsInit,
			Input: json.RawMessage(fmt.Sprintf(
				`{"name":"native-scaffold","template":"tool-provider-go","directory":%q}`,
				initDir,
			)),
		})
		if err != nil {
			t.Fatalf("Registry.Call(extensions_init) error = %v", err)
		}
		requireNativeStructuredContains(t, initResult, []byte(`"native-scaffold"`))

		sourceDir := filepath.Join(workspaceRoot, "native-dev")
		writeNativeDevBuildFixture(t, sourceDir, "0.1.0", "first")
		firstBuild := callNativeExtensionBuild(t, registry, scope, sourceDir)
		validateResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsValidate,
			Input:  json.RawMessage(fmt.Sprintf(`{"path":%q}`, firstBuild.GenerationDir)),
		})
		if err != nil {
			t.Fatalf("Registry.Call(extensions_validate) error = %v", err)
		}
		requireNativeStructuredContains(t, validateResult, []byte(`"issues":[]`))

		devResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID:      toolspkg.ToolIDExtensionsDev,
			WorkspaceID: workspaceRegistrationID,
			Input: json.RawMessage(fmt.Sprintf(
				`{"origin_path":%q,"generation_hash":%q}`,
				sourceDir,
				firstBuild.GenerationHash,
			)),
		})
		if err != nil {
			t.Fatalf("Registry.Call(extensions_dev) error = %v", err)
		}
		requireNativeStructuredContains(t, devResult, []byte(`"workspace_id":"`+workspaceRegistrationID+`"`))

		writeNativeDevBuildFixture(t, sourceDir, "0.2.0", "second")
		secondBuild := callNativeExtensionBuild(t, registry, scope, sourceDir)
		reloadResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID:      toolspkg.ToolIDExtensionsReload,
			WorkspaceID: workspaceRegistrationID,
			Input: json.RawMessage(fmt.Sprintf(
				`{"name":"native-dev","generation_hash":%q}`,
				secondBuild.GenerationHash,
			)),
		})
		if err != nil {
			t.Fatalf("Registry.Call(extensions_reload) error = %v", err)
		}
		requireNativeStructuredContains(t, reloadResult, []byte(secondBuild.GenerationHash))
		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID:      toolspkg.ToolIDExtensionsReload,
			WorkspaceID: workspaceRegistrationID,
			Input: json.RawMessage(fmt.Sprintf(
				`{"name":"native-dev","generation_hash":%q}`,
				strings.Repeat("0", 64),
			)),
		})
		if err == nil {
			t.Fatal("Registry.Call(extensions_reload missing generation) error = nil, want failure")
		}

		searchResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsSearch,
			Input:  json.RawMessage(`{"query":"tool","sources":["github"],"limit":1}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(extensions_search) error = %v", err)
		}
		requireNativeStructuredContains(t, searchResult, []byte(`"source":"github"`))

		provenanceResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsProvenance,
			Input:  json.RawMessage(`{"name":"global-native"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(extensions_provenance) error = %v", err)
		}
		requireNativeStructuredContains(t, provenanceResult, []byte(`"installed_from":"local_path"`))

		publishResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsPublish,
			Input: json.RawMessage(fmt.Sprintf(
				`{"generation_dir":%q,"repository":"acme/native-dev","tag_name":"v0.2.0"}`,
				secondBuild.GenerationDir,
			)),
		})
		if err != nil {
			t.Fatalf("Registry.Call(extensions_publish) error = %v", err)
		}
		requireNativeStructuredContains(t, publishResult, []byte(`"digest_sha256"`))
		if strings.Contains(string(publishResult.Structured), publishCredential) ||
			strings.Contains(publishResult.Preview, publishCredential) {
			t.Fatal("extensions_publish result contains the bound credential")
		}
		publishCapture.requireComplete(t, publishCredential)
		deps.ExtensionSecrets = nativeExtensionSecretResolver{values: map[string]string{}}
		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsPublish,
			Input: json.RawMessage(fmt.Sprintf(
				`{"generation_dir":%q,"repository":"acme/native-dev","tag_name":"v0.2.1"}`,
				secondBuild.GenerationDir,
			)),
		})
		if err == nil {
			t.Fatal("Registry.Call(extensions_publish without credential) error = nil, want failure")
		}

		logsResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID:      toolspkg.ToolIDExtensionsLogs,
			WorkspaceID: workspaceRegistrationID,
			Input:       json.RawMessage(`{"name":"native-dev","after":0}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(extensions_logs) error = %v", err)
		}
		var logsSnapshot contract.ExtensionLogsResponse
		if err := json.Unmarshal(logsResult.Structured, &logsSnapshot); err != nil {
			t.Fatalf("json.Unmarshal(extensions_logs) error = %v", err)
		}
		if strings.TrimSpace(logsSnapshot.StreamEpoch) == "" {
			t.Fatal("extensions_logs stream_epoch is empty")
		}
		if len(logsSnapshot.Logs) != 0 {
			t.Fatalf("extensions_logs entries = %#v, want empty", logsSnapshot.Logs)
		}

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID:      toolspkg.ToolIDExtensionsLogs,
			WorkspaceID: workspaceRegistrationID,
			Input:       json.RawMessage(`{"name":"native-dev","after":1}`),
		})
		requireToolReason(
			t,
			err,
			toolspkg.ErrToolInvalidInput,
			toolspkg.ReasonExtensionValidationFailed,
		)
		globalLogs, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsLogs,
			Input:  json.RawMessage(`{"name":"global-native"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(global extensions_logs as operator) error = %v", err)
		}
		requireNativeStructuredContains(t, globalLogs, []byte(`"stream_epoch":"`))

		_, err = registry.Call(t.Context(), toolspkg.Scope{
			SessionID: "session-a",
			ActorKind: string(taskpkg.ActorKindAgentSession),
		}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsLogs,
			Input:  json.RawMessage(`{"name":"global-native"}`),
		})
		if err == nil {
			t.Fatal("Registry.Call(global extensions_logs as agent) error = nil, want denial")
		}
		if reason, ok := toolspkg.ReasonOf(err); !ok || reason != toolspkg.ReasonExtensionSourceForbidden {
			t.Fatalf(
				"ReasonOf(global agent logs error) = %q/%t, want %q",
				reason,
				ok,
				toolspkg.ReasonExtensionSourceForbidden,
			)
		}

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID:      toolspkg.ToolIDExtensionsLogs,
			WorkspaceID: "workspace-forged",
			Input:       json.RawMessage(`{"name":"native-dev"}`),
		})
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID:      toolspkg.ToolIDExtensionsRemove,
			WorkspaceID: workspaceRegistrationID,
			Input:       json.RawMessage(`{"name":"native-dev"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(extensions_remove dev) error = %v", err)
		}

		assertNativeExtensionLifecycleEvents(t, deps.ExtensionEvents, workspaceRegistrationID)
		assertNativeExtensionAuthoringRisk(t)
	})
}

func TestNativeExtensionToolsPortableContract(t *testing.T) {
	t.Parallel()

	deps, extRegistry, _, runtime := newNativeExtensionToolDeps(t)
	deps.ExtensionRuntime = func() extensionRuntime { return runtime }
	registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
	portablePath := filepath.Join("..", "extension", "testdata", "agent-plugin-conformant")

	before, err := extRegistry.List()
	if err != nil {
		t.Fatalf("extension registry List(before validate) error = %v", err)
	}
	validateResult, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsValidate,
		Input:  json.RawMessage(fmt.Sprintf(`{"path":%q}`, portablePath)),
	})
	if err != nil {
		t.Fatalf("Registry.Call(extensions_validate portable) error = %v", err)
	}
	for _, expected := range [][]byte{
		[]byte(`"status":"valid"`), []byte(`"format":"agent-plugin"`),
		[]byte(`"would_ingest"`), []byte(`"issues"`),
	} {
		requireNativeStructuredContains(t, validateResult, expected)
	}
	after, err := extRegistry.List()
	if err != nil {
		t.Fatalf("extension registry List(after validate) error = %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("portable validate mutated registry: before=%#v after=%#v", before, after)
	}

	installResult, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsInstall,
		Input: json.RawMessage(fmt.Sprintf(
			`{"source":"local_path","ref":%q,"allow_unverified":true}`,
			portablePath,
		)),
	})
	if err != nil {
		t.Fatalf("Registry.Call(extensions_install portable) error = %v", err)
	}
	requireNativeStructuredContains(t, installResult, []byte(`"format":"agent-plugin"`))
	requireNativeStructuredContains(t, installResult, []byte(`"extension_agent_plugin_component_skipped"`))

	unrelated := t.TempDir()
	if err := os.WriteFile(filepath.Join(unrelated, "plugin.json"), []byte(`{"name":"npm-package"}`), 0o600); err != nil {
		t.Fatalf("os.WriteFile(unrelated plugin.json) error = %v", err)
	}
	_, err = registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsInstall,
		Input: json.RawMessage(fmt.Sprintf(
			`{"source":"local_path","ref":%q,"allow_unverified":true}`,
			unrelated,
		)),
	})
	toolErr, ok := errors.AsType[*toolspkg.ToolError](err)
	if !ok || string(toolErr.Code) != "extension_agent_plugin_not_manifest" {
		t.Fatalf("portable install error = %#v, want branchable not-manifest code", err)
	}
}

type nativeExtensionInspectionRuntime struct {
	*fakeExtensionRuntime
	manager *extensionpkg.Manager
}

func (r *nativeExtensionInspectionRuntime) InspectPackageResources(
	ctx context.Context,
	name string,
) (*extensionpkg.Extension, error) {
	return r.manager.InspectPackageResources(ctx, name)
}

func assertNativeExtensionInventoryPreviewParity(
	t *testing.T,
	registry toolspkg.Registry,
	scope toolspkg.Scope,
	service *daemonExtensionService,
	name string,
) {
	t.Helper()

	inventoryResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsInventory,
		Input:  json.RawMessage(fmt.Sprintf(`{"name":%q}`, name)),
	})
	if err != nil {
		t.Fatalf("Registry.Call(extensions_inventory) error = %v", err)
	}
	var nativeInventory contract.ExtensionInventoryPayload
	if err := json.Unmarshal(inventoryResult.Structured, &nativeInventory); err != nil {
		t.Fatalf("json.Unmarshal(extensions_inventory) error = %v", err)
	}
	directInventory, err := service.Inventory(t.Context(), name)
	if err != nil {
		t.Fatalf("service.Inventory() error = %v", err)
	}
	if !reflect.DeepEqual(nativeInventory, directInventory) {
		t.Fatalf("native inventory = %#v, want core service payload %#v", nativeInventory, directInventory)
	}
	if len(nativeInventory.Items) != 1 || nativeInventory.Items[0].Live {
		t.Fatalf("native inventory = %#v, want one shipped inactive item", nativeInventory)
	}

	previewResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsPreview,
		Input:  json.RawMessage(fmt.Sprintf(`{"name":%q}`, name)),
	})
	if err != nil {
		t.Fatalf("Registry.Call(extensions_preview) error = %v", err)
	}
	var nativePreview contract.ExtensionEnablePreviewPayload
	if err := json.Unmarshal(previewResult.Structured, &nativePreview); err != nil {
		t.Fatalf("json.Unmarshal(extensions_preview) error = %v", err)
	}
	directPreview, err := service.Preview(t.Context(), name)
	if err != nil {
		t.Fatalf("service.Preview() error = %v", err)
	}
	if !reflect.DeepEqual(nativePreview, directPreview) {
		t.Fatalf("native preview = %#v, want core service payload %#v", nativePreview, directPreview)
	}
	if len(nativePreview.Changes) != 1 || !nativePreview.NetworkConfirmationRequired {
		t.Fatalf("native preview = %#v, want one item and network confirmation", nativePreview)
	}
}

func nativeIntegrationNetworkDigest(t *testing.T, channelScope string) string {
	t.Helper()
	digest, err := extensionpkg.NetworkParticipationRequirementDigest(
		&extensionpkg.NetworkParticipationRequirement{
			Required: true, Mode: "live", ChannelScopes: []string{channelScope},
		},
	)
	if err != nil {
		t.Fatalf("NetworkParticipationRequirementDigest() error = %v", err)
	}
	return digest
}

func callNativeExtensionBuild(
	t *testing.T,
	registry toolspkg.Registry,
	scope toolspkg.Scope,
	sourceDir string,
) extensionpkg.BuildResult {
	t.Helper()
	result, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsBuild,
		Input:  json.RawMessage(fmt.Sprintf(`{"source_dir":%q}`, sourceDir)),
	})
	if err != nil {
		t.Fatalf("Registry.Call(extensions_build) error = %v", err)
	}
	var envelope struct {
		Build extensionpkg.BuildResult `json:"build"`
	}
	if err := json.Unmarshal(result.Structured, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(extensions_build) error = %v", err)
	}
	if len(envelope.Build.GenerationHash) != 64 || envelope.Build.GenerationDir == "" {
		t.Fatalf("extensions_build result = %#v", envelope.Build)
	}
	return envelope.Build
}

func writeNativeDevBuildFixture(t *testing.T, dir, version, behavior string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "go.mod"),
		[]byte("module example.com/native-dev-extension\n\ngo 1.26.4\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(go.mod) error = %v", err)
	}
	describe := fmt.Sprintf(
		`{"name":"native-dev","version":%q,"description":%q,"provides":[],"permissions":[],"subprocess":{"command":""},"sdk":{"name":"go","version":"0.1.0","protocol_version":"1","min_compozy_version":"0.3.0-beta.1"}}`,
		version,
		behavior,
	)
	source := fmt.Sprintf(`package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__describe" {
		fmt.Print(%q)
	}
}
`, describe)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("os.WriteFile(main.go) error = %v", err)
	}
}

func installNativeGlobalResourceExtension(t *testing.T, registry *extensionpkg.Registry, name string) {
	t.Helper()
	dir := t.TempDir()
	manifestContent := fmt.Sprintf(`[extension]
name = %q
version = "1.0.0"
min_compozy_version = "0.3.0-beta.1"

[capabilities]
provides = []

[permissions]
requires = []
`, name)
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("os.WriteFile(global extension manifest) error = %v", err)
	}
	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(global extension) error = %v", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(global extension) error = %v", err)
	}
	provenance := extensionpkg.LocalPathProvenance(manifest, dir, checksum, time.Now().UTC(), true)
	if err := registry.Install(
		manifest,
		dir,
		checksum,
		extensionpkg.WithInstallProvenance(provenance),
	); err != nil {
		t.Fatalf("Registry.Install(global extension) error = %v", err)
	}
}

func assertNativeExtensionLifecycleEvents(
	t *testing.T,
	storeReader store.EventSummaryStore,
	workspaceID string,
) {
	t.Helper()
	for _, eventType := range []string{
		eventspkg.ExtensionDevLinked,
		eventspkg.ExtensionDevUnlinked,
		eventspkg.ExtensionReloadCompleted,
	} {
		events, err := storeReader.ListEventSummaries(t.Context(), store.EventSummaryQuery{Type: eventType})
		if err != nil {
			t.Fatalf("ListEventSummaries(%s) error = %v", eventType, err)
		}
		if len(events) != 1 {
			t.Fatalf("ListEventSummaries(%s) count = %d, want 1", eventType, len(events))
		}
		var fields map[string]any
		if err := json.Unmarshal(events[0].Content, &fields); err != nil {
			t.Fatalf("json.Unmarshal(%s event) error = %v", eventType, err)
		}
		if len(fields) != 3 || fields["workspace_id"] != workspaceID || fields["extension_generation"] == "" {
			t.Fatalf("%s fields = %#v, want exact extension/workspace/generation keys", eventType, fields)
		}
	}
	failedReloads, err := storeReader.ListEventSummaries(
		t.Context(),
		store.EventSummaryQuery{Type: eventspkg.ExtensionReloadFailed},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries(%s) error = %v", eventspkg.ExtensionReloadFailed, err)
	}
	if len(failedReloads) != 0 {
		t.Fatalf(
			"ListEventSummaries(%s) = %#v, want no event for validation refusal",
			eventspkg.ExtensionReloadFailed,
			failedReloads,
		)
	}
	for _, eventType := range []string{eventspkg.ExtensionPublishCompleted, eventspkg.ExtensionPublishFailed} {
		events, err := storeReader.ListEventSummaries(t.Context(), store.EventSummaryQuery{Type: eventType})
		if err != nil {
			t.Fatalf("ListEventSummaries(%s) error = %v", eventType, err)
		}
		if len(events) != 1 || string(events[0].Content) != `{"extension_name":"native-dev"}` {
			t.Fatalf("ListEventSummaries(%s) = %#v, want exact native-dev payload", eventType, events)
		}
	}
}

func assertNativeExtensionAuthoringRisk(t *testing.T) {
	t.Helper()
	descriptors := make(map[toolspkg.ToolID]toolspkg.Descriptor)
	for _, descriptor := range builtintools.NativeDescriptors() {
		descriptors[descriptor.ID] = descriptor
	}
	for _, id := range []toolspkg.ToolID{
		toolspkg.ToolIDExtensionsBuild,
		toolspkg.ToolIDExtensionsDev,
		toolspkg.ToolIDExtensionsReload,
	} {
		descriptor := descriptors[id]
		if !descriptor.RequiresInteraction || descriptor.ReadOnly {
			t.Fatalf("descriptor %s = %#v, want interaction-gated mutation", id, descriptor)
		}
	}
	validate := descriptors[toolspkg.ToolIDExtensionsValidate]
	if !validate.ReadOnly || validate.RequiresInteraction || validate.Risk != toolspkg.RiskRead {
		t.Fatalf("validate descriptor = %#v, want read-only non-interactive", validate)
	}
	for _, id := range []toolspkg.ToolID{
		toolspkg.ToolIDExtensionsSearch,
		toolspkg.ToolIDExtensionsProvenance,
	} {
		descriptor := descriptors[id]
		if !descriptor.ReadOnly || descriptor.RequiresInteraction || descriptor.Risk != toolspkg.RiskRead {
			t.Fatalf("descriptor %s = %#v, want read-only non-interactive", id, descriptor)
		}
	}
	publish := descriptors[toolspkg.ToolIDExtensionsPublish]
	if publish.ReadOnly || !publish.RequiresInteraction || publish.Risk != toolspkg.RiskOpenWorld {
		t.Fatalf("publish descriptor = %#v, want interaction-gated open-world mutation", publish)
	}
}

type nativeExtensionSecretResolver struct {
	values map[string]string
}

func (r nativeExtensionSecretResolver) ResolveRef(_ context.Context, ref string) (string, error) {
	value, ok := r.values[ref]
	if !ok {
		return "", errors.New("test secret binding unavailable")
	}
	return value, nil
}

type nativeExtensionPublishCapture struct {
	mu            sync.Mutex
	authorization []string
	assetNames    []string
	payloads      [][]byte
	err           error
}

func newNativeExtensionPublishServer(
	t *testing.T,
	credential string,
) (*httptest.Server, *nativeExtensionPublishCapture) {
	t.Helper()
	capture := &nativeExtensionPublishCapture{}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		payload, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "read request", http.StatusInternalServerError)
			capture.recordError(err)
			return
		}
		capture.recordRequest(request.Header.Get("Authorization"), request.URL.Query().Get("name"), payload)
		if request.Header.Get("Authorization") != "Bearer "+credential {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet &&
			request.URL.Path == "/repos/acme/native-dev/releases/tags/v0.2.0":
			writer.WriteHeader(http.StatusNotFound)
		case request.Method == http.MethodPost && request.URL.Path == "/repos/acme/native-dev/releases":
			writer.WriteHeader(http.StatusCreated)
			capture.encode(writer, map[string]any{
				"id": 1, "html_url": server.URL + "/release/v0.2.0",
				"upload_url": server.URL + "/uploads/assets{?name,label}", "assets": []any{},
			})
		case request.Method == http.MethodPost && request.URL.Path == "/uploads/assets":
			writer.WriteHeader(http.StatusCreated)
			capture.encode(writer, map[string]any{
				"id": 2, "browser_download_url": server.URL + "/downloads/" + request.URL.Query().Get("name"),
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	return server, capture
}

func (c *nativeExtensionPublishCapture) recordRequest(authorization string, assetName string, payload []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authorization = append(c.authorization, authorization)
	if strings.TrimSpace(assetName) != "" {
		c.assetNames = append(c.assetNames, assetName)
		c.payloads = append(c.payloads, append([]byte(nil), payload...))
	}
}

func (c *nativeExtensionPublishCapture) recordError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = errors.Join(c.err, err)
}

func (c *nativeExtensionPublishCapture) encode(writer http.ResponseWriter, value any) {
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		c.recordError(err)
	}
}

func (c *nativeExtensionPublishCapture) requireComplete(t *testing.T, credential string) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		t.Fatalf("publish server error = %v", c.err)
	}
	if len(c.assetNames) != 2 || !strings.HasSuffix(c.assetNames[1], ".sha256") {
		t.Fatalf("published assets = %#v, want archive and digest sidecar", c.assetNames)
	}
	for _, authorization := range c.authorization {
		if authorization != "Bearer "+credential {
			t.Fatalf("publish authorization = %q, want bound credential", authorization)
		}
	}
	for index, payload := range c.payloads {
		if len(payload) == 0 {
			t.Fatalf("published payload %d is empty", index)
		}
		if strings.Contains(string(payload), credential) {
			t.Fatalf("published payload %d contains credential", index)
		}
	}
}
