package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestExtensionEnableNetworkConsentLifecycle(t *testing.T) {
	// This ordered lifecycle assertion owns one mutable extension instance.
	db := openDaemonTestGlobalDB(t)
	registry, manifest := installNetworkLifecycleExtension(t, db, "network-kit")
	runtime := &networkLifecycleRuntime{registry: registry, manifest: manifest}
	writer := &extensionEventRecorder{}
	service, ok := newDaemonExtensionService(&daemonExtensionServiceDeps{
		Registry:  registry,
		Runtime:   runtime,
		HomePaths: testHomePaths(t),
		Logger:    discardLogger(),
		Now:       func() time.Time { return time.Date(2026, 8, 2, 19, 0, 0, 0, time.UTC) },
	},
		withDaemonExtensionEventWriter(writer),
		withDaemonExtensionAutomation(&fakeAutomationManager{}),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	operator, err := taskpkg.DeriveHumanActorContext("operator-1", taskpkg.OriginKindCLI, "cli")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	digest, err := extensionpkg.NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
	if err != nil {
		t.Fatalf("NetworkParticipationRequirementDigest() error = %v", err)
	}

	for _, testCase := range []struct {
		name     string
		supplied string
	}{
		{name: "Should refuse enable without a confirmation digest"},
		{name: "Should refuse enable with a stale confirmation digest", supplied: strings.Repeat("0", 64)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, enableErr := service.Enable(
				t.Context(),
				manifest.Name,
				contract.EnableExtensionRequest{ConfirmNetworkDigest: testCase.supplied},
				operator,
			)
			if !errors.Is(enableErr, extensionpkg.ErrExtensionNetworkConfirmationRequired) {
				t.Fatalf("Enable(confirm=%q) error = %v, want confirmation required", testCase.supplied, enableErr)
			}
			required, requiredMatched := errors.AsType[*extensionpkg.NetworkConfirmationRequiredError](enableErr)
			if !requiredMatched || required.CurrentDigest != digest {
				t.Fatalf(
					"Enable(confirm=%q) error = %#v, want current digest %q",
					testCase.supplied,
					required,
					digest,
				)
			}
			info, getErr := registry.Get(manifest.Name)
			if getErr != nil {
				t.Fatalf("registry.Get(after refusal) error = %v", getErr)
			}
			if info.Enabled || info.NetworkConfirmedBy != "" || !info.NetworkConfirmedAt.IsZero() {
				t.Fatalf("registry after refusal = %#v, want byte-equivalent disabled unconfirmed state", info)
			}
		})
	}

	t.Run("Should persist operator consent and enable with the exact digest", func(t *testing.T) {
		result, err := service.Enable(
			t.Context(),
			manifest.Name,
			contract.EnableExtensionRequest{ConfirmNetworkDigest: digest},
			operator,
		)
		if err != nil {
			t.Fatalf("Enable(exact confirmation) error = %v", err)
		}
		if !result.Extension.Enabled || runtime.reloadCalls != 1 {
			t.Fatalf("Enable(exact confirmation) = %#v reloads=%d", result, runtime.reloadCalls)
		}
		confirmation, err := registry.NetworkConfirmation(extensionpkg.GlobalInstanceKey(manifest.Name))
		if err != nil {
			t.Fatalf("NetworkConfirmation(operator) error = %v", err)
		}
		if confirmation.Digest != digest ||
			confirmation.ConfirmedBy != "operator" ||
			confirmation.ConfirmedAt.IsZero() {
			t.Fatalf("NetworkConfirmation(operator) = %#v", confirmation)
		}
		if writer.batchCalls != 1 || len(writer.snapshot()) != 2 {
			t.Fatalf("operator confirmation events = batches:%d events:%#v", writer.batchCalls, writer.snapshot())
		}
	})

	t.Run("Should reuse persisted consent when the digest is unchanged", func(t *testing.T) {
		if _, err := service.Disable(t.Context(), manifest.Name, operator); err != nil {
			t.Fatalf("Disable() error = %v", err)
		}
		if _, err := service.Enable(
			t.Context(), manifest.Name, contract.EnableExtensionRequest{}, operator,
		); err != nil {
			t.Fatalf("Enable(after unchanged digest) error = %v", err)
		}
		unchanged, err := registry.NetworkConfirmation(extensionpkg.GlobalInstanceKey(manifest.Name))
		if err != nil || unchanged.ConfirmedBy != "operator" {
			t.Fatalf("NetworkConfirmation(after toggle) = %#v, %v", unchanged, err)
		}
	})

	t.Run("Should record the confirming agent identity", func(t *testing.T) {
		if _, err := service.Disable(t.Context(), manifest.Name, operator); err != nil {
			t.Fatalf("Disable(before agent confirmation) error = %v", err)
		}
		if err := registry.RestoreNetworkConfirmation(
			extensionpkg.GlobalInstanceKey(manifest.Name),
			extensionpkg.NetworkConfirmation{Digest: digest},
		); err != nil {
			t.Fatalf("RestoreNetworkConfirmation(unconfirmed) error = %v", err)
		}
		agent, err := taskpkg.DeriveAgentSessionActorContext("session-7", "workspace-1")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}
		if _, err := service.Enable(
			t.Context(),
			manifest.Name,
			contract.EnableExtensionRequest{ConfirmNetworkDigest: digest},
			agent,
		); err != nil {
			t.Fatalf("Enable(agent confirmation) error = %v", err)
		}
		agentConfirmation, err := registry.NetworkConfirmation(extensionpkg.GlobalInstanceKey(manifest.Name))
		if err != nil {
			t.Fatalf("NetworkConfirmation(agent) error = %v", err)
		}
		if agentConfirmation.ConfirmedBy != "agent:session-7" {
			t.Fatalf("NetworkConfirmation(agent).ConfirmedBy = %q, want agent:session-7", agentConfirmation.ConfirmedBy)
		}
	})

	t.Run("Should require fresh consent when the network requirement changes", func(t *testing.T) {
		if _, err := service.Disable(t.Context(), manifest.Name, operator); err != nil {
			t.Fatalf("Disable(before candidate change) error = %v", err)
		}
		candidate := *manifest
		candidateRequirement := *manifest.NetworkParticipation
		candidateRequirement.ChannelScopes = []string{"reviewers"}
		candidate.NetworkParticipation = &candidateRequirement
		runtime.manifest = &candidate
		candidateDigest, err := extensionpkg.NetworkParticipationRequirementDigest(candidate.NetworkParticipation)
		if err != nil {
			t.Fatalf("NetworkParticipationRequirementDigest(candidate) error = %v", err)
		}
		_, err = service.Enable(t.Context(), manifest.Name, contract.EnableExtensionRequest{}, operator)
		required, requiredMatched := errors.AsType[*extensionpkg.NetworkConfirmationRequiredError](err)
		if !requiredMatched || required.CurrentDigest != candidateDigest {
			t.Fatalf(
				"Enable(changed candidate) error = %#v, want confirmation for candidate digest %q",
				err,
				candidateDigest,
			)
		}
		info, err := registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("registry.Get(after changed candidate refusal) error = %v", err)
		}
		if info.Enabled {
			t.Fatal("changed candidate inherited prior consent and became enabled")
		}
	})
}

type networkLifecycleRuntime struct {
	registry    *extensionpkg.Registry
	manifest    *extensionpkg.Manifest
	reloadCalls int
}

var _ extensionRuntime = (*networkLifecycleRuntime)(nil)

func (*networkLifecycleRuntime) Start(context.Context) error {
	return nil
}

func (*networkLifecycleRuntime) Stop(context.Context) error {
	return nil
}

func (*networkLifecycleRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}

func (r *networkLifecycleRuntime) Get(name string) (*extensionpkg.Extension, error) {
	info, err := r.registry.Get(name)
	if err != nil {
		return nil, err
	}
	return &extensionpkg.Extension{
		Info: *info, Manifest: r.manifest,
		Status: extensionpkg.ExtensionStatus{
			Name: info.Name, Version: info.Version, Source: info.Source,
			Enabled: info.Enabled, Registered: info.Enabled, Active: info.Enabled,
		},
	}, nil
}

func (r *networkLifecycleRuntime) InspectPackageResources(
	_ context.Context,
	name string,
) (*extensionpkg.Extension, error) {
	return r.Get(name)
}

func (r *networkLifecycleRuntime) Reload(context.Context) error {
	r.reloadCalls++
	return nil
}

func installNetworkLifecycleExtension(
	t *testing.T,
	db *globaldb.GlobalDB,
	name string,
) (*extensionpkg.Registry, *extensionpkg.Manifest) {
	t.Helper()
	dir := t.TempDir()
	manifestText := fmt.Sprintf(`[extension]
name = %q
version = "1.0.0"
description = "Network lifecycle fixture"
min_compozy_version = "0.1.0"

[network_participation]
required = true
mode = "live"
channel_scopes = ["builders"]

[capabilities]
provides = []

[permissions]
requires = []

[subprocess]
command = "fake-extension"
`, name)
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(manifestText), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum() error = %v", err)
	}
	registry := extensionpkg.NewRegistry(db.DB())
	if err := registry.Install(
		manifest,
		dir,
		checksum,
		extensionpkg.WithInstallEnabled(false),
	); err != nil {
		t.Fatalf("Registry.Install() error = %v", err)
	}
	return registry, manifest
}
