//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestDevelopmentExtensionNetworkConsentLifecycle(t *testing.T) {
	// This ordered lifecycle assertion owns one mutable workspace-local instance.
	const (
		extensionName = "network-dev-only"
		workspaceID   = "workspace-network-dev"
	)
	db := openDaemonTestGlobalDB(t)
	registry := extensionpkg.NewRegistry(db.DB())
	workspaceRoot := t.TempDir()
	workspaceTwoRoot := t.TempDir()
	workspaceOne := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID: workspaceID, Name: "network-dev", RootDir: workspaceRoot,
		},
		WorkspaceID: workspaceID,
	}
	workspaceTwo := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID: "workspace-network-dev-2", Name: "network-dev-two", RootDir: workspaceTwoRoot,
		},
		WorkspaceID: "workspace-network-dev-2",
	}
	workspaceResolver := newNetworkLifecycleWorkspaceResolver(workspaceOne, workspaceTwo)
	manager := extensionpkg.NewManager(
		registry,
		extensionpkg.WithLogger(discardLogger()),
		extensionpkg.WithWorkspaceResolver(workspaceResolver),
	)
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Errorf("Manager.Stop() error = %v", err)
		}
	})
	confirmedAt := time.Date(2026, 8, 2, 21, 0, 0, 0, time.UTC)
	service, ok := newDaemonExtensionService(daemonExtensionServiceDeps{
		Registry:  registry,
		Runtime:   manager,
		HomePaths: testHomePaths(t),
		Logger:    discardLogger(),
		Now:       func() time.Time { return confirmedAt },
	},
		withDaemonExtensionEventWriter(db),
		withDaemonExtensionWorkspaceResolver(workspaceResolver),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		workspaceID,
		taskpkg.OriginKindHTTP,
		"development network consent",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	httpEngine := newExtensionTransportEngine(service, actor, "http")
	udsEngine := newExtensionTransportEngine(service, actor, "uds")
	origin := filepath.Join(workspaceRoot, "network-dev-extension")
	var initialGeneration string
	t.Run("Should link a development extension without a network requirement", func(t *testing.T) {
		var initialDigest string
		initialGeneration, initialDigest = writeDevelopmentNetworkGeneration(
			t,
			origin,
			extensionName,
			"1.0.0",
			nil,
		)
		if initialDigest != "" {
			t.Fatalf("initial network requirement digest = %q, want empty", initialDigest)
		}
		devResponse := performExtensionTransportRequest(
			t,
			httpEngine,
			http.MethodPost,
			"/extensions/dev",
			mustExtensionTransportJSON(t, contract.DevLinkExtensionRequest{
				OriginPath: origin, GenerationHash: initialGeneration,
			}),
		)
		if devResponse.Code != http.StatusCreated {
			t.Fatalf(
				"dev link status = %d, want %d; body=%s",
				devResponse.Code,
				http.StatusCreated,
				devResponse.Body,
			)
		}
		if _, err := registry.Get(extensionName); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("registry.Get(dev-only) error = %v, want no global extension row", err)
		}
	})

	var networkGeneration, networkDigest string
	t.Run("Should refuse a reload that introduces unconfirmed network participation", func(t *testing.T) {
		networkGeneration, networkDigest = writeDevelopmentNetworkGeneration(
			t,
			origin,
			extensionName,
			"1.1.0",
			[]string{"builders"},
		)
		refused := performExtensionTransportRequest(
			t,
			httpEngine,
			http.MethodPost,
			"/extensions/"+extensionName+"/reload",
			mustExtensionTransportJSON(t, contract.ReloadExtensionRequest{GenerationHash: networkGeneration}),
		)
		if refused.Code != http.StatusConflict || !strings.Contains(refused.Body.String(), networkDigest) {
			t.Fatalf(
				"unconfirmed reload = status %d body %s, want 409 with current digest",
				refused.Code,
				refused.Body,
			)
		}
		unchanged, getErr := registry.GetDevLink(extensionName, workspaceID)
		if getErr != nil {
			t.Fatalf("GetDevLink(after refusal) error = %v", getErr)
		}
		if unchanged.BundleGeneration != initialGeneration || unchanged.NetworkConfirmedBy != "" {
			t.Fatalf("dev link after refusal = %#v, want unchanged initial generation", unchanged)
		}
	})

	t.Run("Should persist exact network consent on reload", func(t *testing.T) {
		confirmed := performExtensionTransportRequest(
			t,
			httpEngine,
			http.MethodPost,
			"/extensions/"+extensionName+"/reload",
			mustExtensionTransportJSON(t, contract.ReloadExtensionRequest{
				GenerationHash: networkGeneration, ConfirmNetworkDigest: networkDigest,
			}),
		)
		if confirmed.Code != http.StatusOK {
			t.Fatalf(
				"confirmed reload status = %d, want %d; body=%s",
				confirmed.Code,
				http.StatusOK,
				confirmed.Body,
			)
		}
		var response contract.ExtensionResponse
		if err := json.Unmarshal(confirmed.Body.Bytes(), &response); err != nil {
			t.Fatalf("json.Unmarshal(confirmed reload) error = %v", err)
		}
		payload := response.Extension
		if payload.GenerationHash != networkGeneration || payload.WorkspaceID != workspaceID {
			t.Fatalf("confirmed reload payload = %#v", payload)
		}
		link, getErr := registry.GetDevLink(extensionName, workspaceID)
		if getErr != nil {
			t.Fatalf("GetDevLink(confirmed) error = %v", getErr)
		}
		if link.BundleGeneration != networkGeneration || link.NetworkRequirementDigest != networkDigest ||
			link.NetworkConfirmedBy != "operator" || !link.NetworkConfirmedAt.Equal(confirmedAt) {
			t.Fatalf("confirmed development link = %#v", link)
		}
	})

	t.Run("Should preserve confirmed state when a stale digest is supplied", func(t *testing.T) {
		changedGeneration, changedDigest := writeDevelopmentNetworkGeneration(
			t,
			origin,
			extensionName,
			"1.2.0",
			[]string{"builders", "reviewers"},
		)
		stale := performExtensionTransportRequest(
			t,
			udsEngine,
			http.MethodPost,
			"/extensions/"+extensionName+"/reload",
			mustExtensionTransportJSON(t, contract.ReloadExtensionRequest{
				GenerationHash: changedGeneration, ConfirmNetworkDigest: networkDigest,
			}),
		)
		if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), changedDigest) {
			t.Fatalf(
				"stale UDS reload = status %d body %s, want 409 with changed digest",
				stale.Code,
				stale.Body,
			)
		}
		afterStale, getErr := registry.GetDevLink(extensionName, workspaceID)
		if getErr != nil {
			t.Fatalf("GetDevLink(after stale digest) error = %v", getErr)
		}
		if afterStale.BundleGeneration != networkGeneration ||
			afterStale.NetworkRequirementDigest != networkDigest ||
			afterStale.NetworkConfirmedBy != "operator" ||
			!afterStale.NetworkConfirmedAt.Equal(confirmedAt) {
			t.Fatalf("dev link after stale digest = %#v, want prior confirmed tuple", afterStale)
		}
	})

	t.Run("Should isolate confirmations between global and workspace instances", func(t *testing.T) {
		globalRegistry, globalManifest := installNetworkLifecycleExtension(t, db, extensionName)
		globalDigest, err := extensionpkg.NetworkParticipationRequirementDigest(globalManifest.NetworkParticipation)
		if err != nil {
			t.Fatalf("NetworkParticipationRequirementDigest(global) error = %v", err)
		}
		globalConfirmedAt := confirmedAt.Add(time.Minute)
		if err := globalRegistry.ConfirmNetworkRequirement(
			extensionpkg.GlobalInstanceKey(extensionName),
			globalDigest,
			"operator",
			globalConfirmedAt,
		); err != nil {
			t.Fatalf("ConfirmNetworkRequirement(global) error = %v", err)
		}

		const workspaceTwoID = "workspace-network-dev-2"
		workspaceTwoOrigin := filepath.Join(workspaceTwoRoot, "network-dev-extension")
		workspaceTwoGeneration, workspaceTwoDigest := writeDevelopmentNetworkGeneration(
			t,
			workspaceTwoOrigin,
			extensionName,
			"2.0.0",
			[]string{"builders"},
		)
		workspaceTwoService := newDaemonExtensionService(daemonExtensionServiceDeps{
			Registry:  registry,
			Runtime:   manager,
			HomePaths: testHomePaths(t),
			Logger:    discardLogger(),
			Now:       func() time.Time { return confirmedAt.Add(2 * time.Minute) },
		},
			withDaemonExtensionEventWriter(db),
			withDaemonExtensionWorkspaceResolver(workspaceResolver),
		)
		workspaceTwoActor, err := taskpkg.DeriveHumanActorContextForWorkspace(
			"operator",
			workspaceTwoID,
			taskpkg.OriginKindUDS,
			"workspace two network isolation",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContextForWorkspace(workspace two) error = %v", err)
		}
		workspaceTwoEngine := newExtensionTransportEngine(workspaceTwoService, workspaceTwoActor, "uds")
		workspaceTwoDevRefusal := performExtensionTransportRequest(
			t,
			workspaceTwoEngine,
			http.MethodPost,
			"/extensions/dev",
			mustExtensionTransportJSON(t, contract.DevLinkExtensionRequest{
				OriginPath: workspaceTwoOrigin, GenerationHash: workspaceTwoGeneration,
			}),
		)
		if workspaceTwoDevRefusal.Code != http.StatusConflict ||
			!strings.Contains(workspaceTwoDevRefusal.Body.String(), workspaceTwoDigest) {
			t.Fatalf(
				"workspace two initial dev = status %d body %s, want 409 with digest",
				workspaceTwoDevRefusal.Code,
				workspaceTwoDevRefusal.Body,
			)
		}
		if _, err := registry.GetDevLink(extensionName, workspaceTwoID); !errors.Is(
			err,
			extensionpkg.ErrExtensionNotDevLinked,
		) {
			t.Fatalf("GetDevLink(after initial refusal) error = %v, want no staged link", err)
		}
		workspaceTwoConfirmed := performExtensionTransportRequest(
			t,
			workspaceTwoEngine,
			http.MethodPost,
			"/extensions/dev",
			mustExtensionTransportJSON(t, contract.DevLinkExtensionRequest{
				OriginPath: workspaceTwoOrigin, GenerationHash: workspaceTwoGeneration,
				ConfirmNetworkDigest: workspaceTwoDigest,
			}),
		)
		if workspaceTwoConfirmed.Code != http.StatusCreated {
			t.Fatalf(
				"workspace two confirmed dev = status %d body %s, want 201",
				workspaceTwoConfirmed.Code,
				workspaceTwoConfirmed.Body,
			)
		}
		workspaceTwoConfirmation, err := registry.NetworkConfirmation(extensionpkg.InstanceKey{
			Name: extensionName, WorkspaceID: workspaceTwoID,
		})
		if err != nil {
			t.Fatalf("NetworkConfirmation(workspace two) error = %v", err)
		}
		workspaceTwoConfirmedAt := confirmedAt.Add(2 * time.Minute)
		if workspaceTwoConfirmation.Digest != workspaceTwoDigest ||
			workspaceTwoConfirmation.ConfirmedBy != "operator" ||
			!workspaceTwoConfirmation.ConfirmedAt.Equal(workspaceTwoConfirmedAt) {
			t.Fatalf("workspace two confirmation = %#v, want isolated confirmed tuple", workspaceTwoConfirmation)
		}
		workspaceTwoChangedGeneration, workspaceTwoChangedDigest := writeDevelopmentNetworkGeneration(
			t,
			workspaceTwoOrigin,
			extensionName,
			"2.1.0",
			[]string{"builders", "reviewers"},
		)
		workspaceTwoRefusal := performExtensionTransportRequest(
			t,
			workspaceTwoEngine,
			http.MethodPost,
			"/extensions/"+extensionName+"/reload",
			mustExtensionTransportJSON(
				t,
				contract.ReloadExtensionRequest{GenerationHash: workspaceTwoChangedGeneration},
			),
		)
		if workspaceTwoRefusal.Code != http.StatusConflict ||
			!strings.Contains(workspaceTwoRefusal.Body.String(), workspaceTwoChangedDigest) {
			t.Fatalf(
				"workspace two reload = status %d body %s, want isolated 409 with digest",
				workspaceTwoRefusal.Code,
				workspaceTwoRefusal.Body,
			)
		}
		workspaceOneAfterIsolation, err := registry.NetworkConfirmation(extensionpkg.InstanceKey{
			Name: extensionName, WorkspaceID: workspaceID,
		})
		if err != nil {
			t.Fatalf("NetworkConfirmation(workspace one after isolation) error = %v", err)
		}
		globalAfterIsolation, err := registry.NetworkConfirmation(extensionpkg.GlobalInstanceKey(extensionName))
		if err != nil {
			t.Fatalf("NetworkConfirmation(global after isolation) error = %v", err)
		}
		if workspaceOneAfterIsolation.Digest != networkDigest || workspaceOneAfterIsolation.ConfirmedBy != "operator" ||
			!workspaceOneAfterIsolation.ConfirmedAt.Equal(confirmedAt) {
			t.Fatalf("workspace one confirmation after isolation = %#v", workspaceOneAfterIsolation)
		}
		if globalAfterIsolation.Digest != globalDigest || globalAfterIsolation.ConfirmedBy != "operator" ||
			!globalAfterIsolation.ConfirmedAt.Equal(globalConfirmedAt) {
			t.Fatalf("global confirmation after isolation = %#v", globalAfterIsolation)
		}
	})
}

type networkLifecycleWorkspaceResolver struct {
	workspaces []workspacepkg.ResolvedWorkspace
}

func newNetworkLifecycleWorkspaceResolver(
	workspaces ...workspacepkg.ResolvedWorkspace,
) *networkLifecycleWorkspaceResolver {
	return &networkLifecycleWorkspaceResolver{
		workspaces: append([]workspacepkg.ResolvedWorkspace(nil), workspaces...),
	}
}

func (r *networkLifecycleWorkspaceResolver) Resolve(
	ctx context.Context,
	ref string,
) (workspacepkg.ResolvedWorkspace, error) {
	if err := ctx.Err(); err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	ref = strings.TrimSpace(ref)
	for _, resolved := range r.workspaces {
		if ref == strings.TrimSpace(resolved.ID) || ref == strings.TrimSpace(resolved.WorkspaceID) {
			return resolved, nil
		}
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r *networkLifecycleWorkspaceResolver) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	if err := ctx.Err(); err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	path = strings.TrimSpace(path)
	for _, resolved := range r.workspaces {
		if path == strings.TrimSpace(resolved.RootDir) {
			return resolved, nil
		}
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func TestInstalledExtensionNetworkConsentTransportLifecycle(t *testing.T) {
	t.Parallel()

	for _, transport := range []struct {
		name   string
		origin taskpkg.OriginKind
	}{
		{name: "http", origin: taskpkg.OriginKindHTTP},
		{name: "uds", origin: taskpkg.OriginKindUDS},
	} {
		t.Run("Should enforce digest-exact consent over "+strings.ToUpper(transport.name), func(t *testing.T) {
			t.Parallel()
			// Nested phases share one installed extension and remain ordered.

			extensionName := "network-kit-" + transport.name
			db := openDaemonTestGlobalDB(t)
			registry, manifest := installNetworkLifecycleExtension(t, db, extensionName)
			runtime := &networkLifecycleRuntime{registry: registry, manifest: manifest}
			writer := &extensionEventRecorder{}
			confirmedAt := time.Date(2026, 8, 2, 22, 0, 0, 0, time.UTC)
			service, ok := newDaemonExtensionService(daemonExtensionServiceDeps{
				Registry:  registry,
				Runtime:   runtime,
				HomePaths: testHomePaths(t),
				Logger:    discardLogger(),
				Now:       func() time.Time { return confirmedAt },
			},
				withDaemonExtensionEventWriter(writer),
				withDaemonExtensionAutomation(&fakeAutomationManager{}),
			).(*daemonExtensionService)
			if !ok {
				t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
			}
			actor, err := taskpkg.DeriveHumanActorContext(
				"operator-transport",
				transport.origin,
				transport.name+" network consent",
			)
			if err != nil {
				t.Fatalf("DeriveHumanActorContext() error = %v", err)
			}
			engine := newExtensionTransportEngine(service, actor, transport.name)
			digest, err := extensionpkg.NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
			if err != nil {
				t.Fatalf("NetworkParticipationRequirementDigest() error = %v", err)
			}

			for _, testCase := range []struct {
				name    string
				request contract.EnableExtensionRequest
			}{
				{
					name:    "Should refuse enable without a confirmation digest",
					request: contract.EnableExtensionRequest{},
				},
				{
					name:    "Should refuse enable with a stale confirmation digest",
					request: contract.EnableExtensionRequest{ConfirmNetworkDigest: strings.Repeat("0", 64)},
				},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					response := performExtensionTransportRequest(
						t,
						engine,
						http.MethodPost,
						"/extensions/"+extensionName+"/enable",
						mustExtensionTransportJSON(t, testCase.request),
					)
					if response.Code != http.StatusConflict {
						t.Fatalf(
							"enable refusal status = %d, want %d; body=%s",
							response.Code,
							http.StatusConflict,
							response.Body,
						)
					}
					var operationError contract.ExtensionOperationErrorPayload
					if err := json.Unmarshal(response.Body.Bytes(), &operationError); err != nil {
						t.Fatalf("json.Unmarshal(enable refusal) error = %v", err)
					}
					if operationError.Code != "extension_network_confirmation_required" ||
						operationError.CurrentDigest != digest {
						t.Fatalf("enable refusal payload = %#v, want current digest %q", operationError, digest)
					}
					info, getErr := registry.Get(extensionName)
					if getErr != nil {
						t.Fatalf("registry.Get(after refusal) error = %v", getErr)
					}
					if info.Enabled || info.NetworkConfirmedBy != "" || !info.NetworkConfirmedAt.IsZero() {
						t.Fatalf("registry after refusal = %#v, want disabled unconfirmed state", info)
					}
					if events := writer.snapshot(); len(events) != 0 {
						t.Fatalf("consent refusal events = %#v, want none", events)
					}
				})
			}

			t.Run("Should persist exact consent and enable the extension", func(t *testing.T) {
				confirmed := performExtensionTransportRequest(
					t,
					engine,
					http.MethodPost,
					"/extensions/"+extensionName+"/enable",
					mustExtensionTransportJSON(t, contract.EnableExtensionRequest{ConfirmNetworkDigest: digest}),
				)
				if confirmed.Code != http.StatusOK {
					t.Fatalf(
						"confirmed enable status = %d, want %d; body=%s",
						confirmed.Code,
						http.StatusOK,
						confirmed.Body,
					)
				}
				var result contract.ExtensionEnableResult
				if err := json.Unmarshal(confirmed.Body.Bytes(), &result); err != nil {
					t.Fatalf("json.Unmarshal(confirmed enable) error = %v", err)
				}
				if !result.Extension.Enabled || result.Extension.Name != extensionName {
					t.Fatalf("confirmed enable result = %#v", result)
				}
				confirmation, err := registry.NetworkConfirmation(extensionpkg.GlobalInstanceKey(extensionName))
				if err != nil {
					t.Fatalf("NetworkConfirmation() error = %v", err)
				}
				if confirmation.Digest != digest || confirmation.ConfirmedBy != "operator" ||
					!confirmation.ConfirmedAt.Equal(confirmedAt) {
					t.Fatalf("persisted confirmation = %#v", confirmation)
				}
				if writer.batchCalls != 1 || len(writer.snapshot()) != 2 {
					t.Fatalf(
						"enable events = batches:%d events:%#v, want one confirmation+enable batch",
						writer.batchCalls,
						writer.snapshot(),
					)
				}
			})

			t.Run("Should reuse persisted consent after disable", func(t *testing.T) {
				disabled := performExtensionTransportRequest(
					t,
					engine,
					http.MethodPost,
					"/extensions/"+extensionName+"/disable",
					nil,
				)
				if disabled.Code != http.StatusOK {
					t.Fatalf("disable status = %d, want %d; body=%s", disabled.Code, http.StatusOK, disabled.Body)
				}
				reenabled := performExtensionTransportRequest(
					t,
					engine,
					http.MethodPost,
					"/extensions/"+extensionName+"/enable",
					nil,
				)
				if reenabled.Code != http.StatusOK {
					t.Fatalf("re-enable status = %d, want %d; body=%s", reenabled.Code, http.StatusOK, reenabled.Body)
				}
			})
		})
	}
}

func writeDevelopmentNetworkGeneration(
	t *testing.T,
	origin string,
	name string,
	version string,
	scopes []string,
) (string, string) {
	t.Helper()
	manifestText := daemonTestExtensionManifest(name, daemonTestExtensionOptions{
		version:        version,
		runtimeCommand: daemonExtensionHelperCommand(t),
		runtimeArgs:    daemonExtensionHelperArgs(),
		runtimeEnv:     daemonExtensionHelperScenarioEnv("network_dev", version),
		capabilities:   []string{},
		permissions:    []string{},
	})
	if len(scopes) > 0 {
		manifestText += fmt.Sprintf(`
[network_participation]
required = true
mode = "live"
channel_scopes = [%s]
`, quotedSecretEnvNames(scopes))
	}
	dist := filepath.Join(origin, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dist, err)
	}
	staging, err := os.MkdirTemp(dist, ".network-generation-")
	if err != nil {
		t.Fatalf("os.MkdirTemp(%q) error = %v", dist, err)
	}
	if err := os.WriteFile(filepath.Join(staging, "extension.toml"), []byte(manifestText), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	manifest, err := extensionpkg.LoadManifest(staging)
	if err != nil {
		t.Fatalf("LoadManifest(staging) error = %v", err)
	}
	digest, err := extensionpkg.NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
	if err != nil {
		t.Fatalf("NetworkParticipationRequirementDigest() error = %v", err)
	}
	generation, err := extensionpkg.ComputeDirectoryChecksum(staging)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(staging) error = %v", err)
	}
	generationDir := filepath.Join(dist, "gen-"+generation)
	if err := os.Rename(staging, generationDir); err != nil {
		t.Fatalf("os.Rename(%q, %q) error = %v", staging, generationDir, err)
	}
	return generation, digest
}
