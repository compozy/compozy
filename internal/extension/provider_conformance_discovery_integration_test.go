//go:build integration

package extensionpkg_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionpkg "github.com/compozy/agh/internal/extension"
	protocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

type discoveredBridgeProvider struct {
	Name     string
	Dir      string
	Manifest *extensionpkg.Manifest
}

type providerSchemaContract struct {
	Schema        string
	Version       string
	RequiredSlots []string
}

func discoverBridgeProviders(root string) ([]discoveredBridgeProvider, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read provider root: %w", err)
	}
	providers := make([]discoveredBridgeProvider, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "extension.toml")); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("stat provider manifest %q: %w", entry.Name(), err)
		}
		manifest, err := extensionpkg.LoadManifest(dir)
		if err != nil {
			return nil, fmt.Errorf("load provider manifest %q: %w", entry.Name(), err)
		}
		providers = append(providers, discoveredBridgeProvider{Name: entry.Name(), Dir: dir, Manifest: manifest})
	}
	sort.Slice(providers, func(left int, right int) bool { return providers[left].Name < providers[right].Name })
	return providers, nil
}

func validateDiscoveredProviderManifest(provider discoveredBridgeProvider) (providerSchemaContract, error) {
	if provider.Manifest == nil {
		return providerSchemaContract{}, fmt.Errorf("provider %q manifest is required", provider.Name)
	}
	if err := provider.Manifest.Validate(); err != nil {
		return providerSchemaContract{}, fmt.Errorf("provider %q manifest validation: %w", provider.Name, err)
	}
	if provider.Manifest.Name != provider.Name {
		return providerSchemaContract{}, fmt.Errorf(
			"provider directory %q declares extension name %q",
			provider.Name,
			provider.Manifest.Name,
		)
	}
	if provider.Manifest.Bridge.Platform != provider.Name {
		return providerSchemaContract{}, fmt.Errorf(
			"provider %q declares bridge platform %q",
			provider.Name,
			provider.Manifest.Bridge.Platform,
		)
	}
	if !containsString(provider.Manifest.Capabilities.Provides, protocol.CapabilityProvideBridgeAdapter) {
		return providerSchemaContract{}, fmt.Errorf("provider %q does not declare bridge.adapter", provider.Name)
	}
	if provider.Manifest.Bridge.ConfigSchema == nil {
		return providerSchemaContract{}, fmt.Errorf("provider %q omits bridge.config_schema", provider.Name)
	}
	contract := providerSchemaContract{
		Schema:  "agh.bridge." + provider.Name,
		Version: strings.TrimSpace(provider.Manifest.Bridge.ConfigSchema.Version),
	}
	for _, slot := range provider.Manifest.Bridge.SecretSlots {
		if slot.Required {
			contract.RequiredSlots = append(contract.RequiredSlots, strings.TrimSpace(slot.Name))
		}
	}
	sort.Strings(contract.RequiredSlots)
	if len(contract.RequiredSlots) == 0 {
		return providerSchemaContract{}, fmt.Errorf("provider %q declares no required secret slots", provider.Name)
	}
	if err := validateProviderSchemaContract(provider.Manifest, contract); err != nil {
		return providerSchemaContract{}, err
	}
	return contract, nil
}

func validateProviderSchemaContract(manifest *extensionpkg.Manifest, contract providerSchemaContract) error {
	if manifest == nil || manifest.Bridge.ConfigSchema == nil {
		return errors.New("provider manifest config schema is required")
	}
	if got := strings.TrimSpace(manifest.Bridge.ConfigSchema.Schema); got != contract.Schema {
		return fmt.Errorf("provider config schema = %q, want %q", got, contract.Schema)
	}
	if got := strings.TrimSpace(manifest.Bridge.ConfigSchema.Version); got == "" || got != contract.Version {
		return fmt.Errorf("provider config schema version = %q, want %q", got, contract.Version)
	}
	seen := make(map[string]bool, len(manifest.Bridge.SecretSlots))
	for _, slot := range manifest.Bridge.SecretSlots {
		name := strings.TrimSpace(slot.Name)
		if seen[name] {
			return fmt.Errorf("provider secret slot %q is duplicated", name)
		}
		seen[name] = slot.Required
	}
	for _, required := range contract.RequiredSlots {
		if !seen[required] {
			return fmt.Errorf("provider config schema requires missing secret slot %q", required)
		}
	}
	return nil
}

func runDiscoveredProviderRuntime(
	t *testing.T,
	repoRoot string,
	provider discoveredBridgeProvider,
	contract providerSchemaContract,
) error {
	t.Helper()
	binary := filepath.Join(t.TempDir(), provider.Name)
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer buildCancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, ".")
	build.Dir = provider.Dir
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build provider %q: %w\n%s", provider.Name, err, output)
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer runCancel()
	cmd := exec.CommandContext(runCtx, binary, "serve")
	cmd.Dir = provider.Dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("provider %q stdin pipe: %w", provider.Name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("provider %q stdout pipe: %w", provider.Name, err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start provider %q: %w", provider.Name, err)
	}

	peerCtx, peerCancel := context.WithCancel(runCtx)
	defer peerCancel()
	peer := bridgesdk.NewPeer(stdout, stdin)
	domainInstance := bridgepkg.BridgeInstance{
		ID:             "brg-conformance-" + provider.Name,
		Platform:       provider.Name,
		ExtensionName:  provider.Name,
		DisplayName:    provider.Name + " conformance",
		Scope:          bridgepkg.ScopeWorkspace,
		WorkspaceID:    "ws-provider-conformance",
		Source:         bridgepkg.BridgeInstanceSourceDynamic,
		Enabled:        true,
		Status:         bridgepkg.BridgeStatusStarting,
		DMPolicy:       bridgepkg.BridgeDMPolicyOpen,
		ProviderConfig: json.RawMessage(`{}`),
	}
	managed := subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstanceToContract(domainInstance),
	}
	if err := peer.Handle(string(protocol.HostAPIMethodBridgesInstancesList), func(
		context.Context,
		json.RawMessage,
	) (any, error) {
		return []bridgecontract.BridgeInstance{managed.Instance}, nil
	}); err != nil {
		return fmt.Errorf("register provider %q instance-list host handler: %w", provider.Name, err)
	}
	getStarted := make(chan struct{})
	releaseGet := make(chan struct{})
	if err := peer.Handle(string(protocol.HostAPIMethodBridgesInstancesGet), func(
		ctx context.Context,
		_ json.RawMessage,
	) (any, error) {
		select {
		case <-getStarted:
		default:
			close(getStarted)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-releaseGet:
			return managed.Instance, nil
		}
	}); err != nil {
		return fmt.Errorf("register provider %q instance-get host handler: %w", provider.Name, err)
	}
	peerErr := make(chan error, 1)
	go func() { peerErr <- peer.Serve(peerCtx) }()

	request := discoveredProviderInitializeRequest(provider.Name, managed)
	var response subprocess.InitializeResponse
	if err := peer.Call(runCtx, "initialize", request, &response); err != nil {
		return fmt.Errorf("initialize provider %q: %w (stderr=%s)", provider.Name, err, stderr.String())
	}
	if response.ExtensionInfo.Name != provider.Name {
		return fmt.Errorf(
			"provider %q initialize name = %q",
			provider.Name,
			response.ExtensionInfo.Name,
		)
	}
	wantMethods := []string{"bridges/deliver", "bridges/targets/snapshot", "health_check", "shutdown"}
	if !sameStringSet(response.ImplementedMethods, wantMethods) {
		return fmt.Errorf(
			"provider %q implemented methods = %v, want %v",
			provider.Name,
			response.ImplementedMethods,
			wantMethods,
		)
	}
	select {
	case <-getStarted:
	case <-runCtx.Done():
		return fmt.Errorf("provider %q did not request its managed instance: %w", provider.Name, runCtx.Err())
	}
	var health subprocess.HealthCheckResponse
	if err := peer.Call(runCtx, "health_check", nil, &health); err != nil {
		return fmt.Errorf("health-check provider %q: %w", provider.Name, err)
	}
	if !health.Healthy {
		return fmt.Errorf("provider %q health is false: %s", provider.Name, health.Message)
	}
	shutdownErr := make(chan error, 1)
	go func() {
		var shutdown subprocess.ShutdownResponse
		shutdownErr <- peer.Call(runCtx, "shutdown", subprocess.ShutdownRequest{
			Reason:     "provider conformance",
			DeadlineMS: int64(time.Second / time.Millisecond),
		}, &shutdown)
	}()
	if err := waitForProviderShutdownAdmission(runCtx, peer); err != nil {
		return fmt.Errorf("provider %q shutdown admission: %w", provider.Name, err)
	}
	close(releaseGet)
	if err := <-shutdownErr; err != nil {
		return fmt.Errorf("shutdown provider %q: %w", provider.Name, err)
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("close provider %q stdin: %w", provider.Name, err)
	}
	peerCancel()
	waitErr := cmd.Wait()
	select {
	case err := <-peerErr:
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) &&
			!strings.Contains(err.Error(), "closed") {
			return fmt.Errorf("serve provider %q peer: %w", provider.Name, err)
		}
	case <-time.After(time.Second):
		return fmt.Errorf("provider %q peer did not stop", provider.Name)
	}
	if waitErr != nil {
		return fmt.Errorf("wait provider %q: %w (stderr=%s)", provider.Name, waitErr, stderr.String())
	}
	if contract.Schema == "" {
		return fmt.Errorf("provider %q schema contract is empty", provider.Name)
	}
	return nil
}

func discoveredProviderInitializeRequest(
	name string,
	managed subprocess.InitializeBridgeManagedInstance,
) subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          "1",
		SupportedProtocolVersion: []string{"1"},
		AGHVersion:               "test",
		SessionNonce:             "provider-conformance-" + name,
		Extension: subprocess.InitializeExtension{
			Name:       name,
			Version:    "0.1.0",
			SourceTier: "workspace",
		},
		Capabilities: subprocess.InitializeCapabilities{
			Provides: []string{protocol.CapabilityProvideBridgeAdapter},
			GrantedActions: []protocol.HostAPIMethod{
				protocol.HostAPIMethodBridgesInstancesList,
				protocol.HostAPIMethodBridgesInstancesGet,
				protocol.HostAPIMethodBridgesInstancesReportState,
				protocol.HostAPIMethodBridgesMessagesIngest,
			},
			GrantedSecurity: []string{"bridge.read", "bridge.write"},
		},
		Methods: subprocess.InitializeMethods{ExtensionServices: []string{
			"bridges/deliver",
			"bridges/targets/snapshot",
			"health_check",
			"shutdown",
		}},
		Runtime: subprocess.InitializeRuntime{
			HealthCheckIntervalMS: 5000,
			HealthCheckTimeoutMS:  1000,
			ShutdownTimeoutMS:     1000,
			DefaultHookTimeoutMS:  1000,
			Bridge: &subprocess.InitializeBridgeRuntime{
				RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
				Purpose:          subprocess.BridgeRuntimePurposeService,
				Provider:         name,
				Platform:         name,
				ManagedInstances: []subprocess.InitializeBridgeManagedInstance{managed},
			},
		},
	}
}

func waitForProviderShutdownAdmission(ctx context.Context, peer *bridgesdk.Peer) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		var health subprocess.HealthCheckResponse
		if err := peer.Call(ctx, "health_check", nil, &health); err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func createSyntheticProviderRoot(t *testing.T, repoRoot string) string {
	t.Helper()
	root, err := os.MkdirTemp(repoRoot, ".provider-conformance-")
	if err != nil {
		t.Fatalf("os.MkdirTemp(repoRoot) error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove synthetic provider root error = %v", err)
		}
	})
	providerDir := filepath.Join(root, "synthetic")
	if err := os.MkdirAll(providerDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(synthetic provider) error = %v", err)
	}
	writeTestFile(t, filepath.Join(providerDir, "extension.toml"), syntheticProviderManifest)
	writeTestFile(t, filepath.Join(providerDir, "main.go"), syntheticProviderSource)
	return root
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func cloneProviderManifest(source *extensionpkg.Manifest) *extensionpkg.Manifest {
	if source == nil {
		return nil
	}
	clone := *source
	clone.Bridge.SecretSlots = append([]bridgepkg.BridgeSecretSlot(nil), source.Bridge.SecretSlots...)
	if source.Bridge.ConfigSchema != nil {
		schema := *source.Bridge.ConfigSchema
		clone.Bridge.ConfigSchema = &schema
	}
	return &clone
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}

func sameStringSet(left []string, right []string) bool {
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

const syntheticProviderManifest = `
[extension]
name = "synthetic"
version = "0.1.0"
description = "Synthetic conformance provider"
min_agh_version = "0.5.0"

[capabilities]
provides = ["bridge.adapter"]

[bridge]
platform = "synthetic"
display_name = "Synthetic"

[[bridge.secret_slots]]
name = "token"
description = "Synthetic provider token"
required = true

[bridge.config_schema]
schema = "agh.bridge.synthetic"
version = "1"

[actions]
requires = [
  "bridges/instances/list",
  "bridges/instances/get",
  "bridges/instances/report_state",
  "bridges/messages/ingest",
]

[subprocess]
command = "./synthetic"
args = ["serve"]

[security]
capabilities = ["bridge.read", "bridge.write"]
`

const syntheticProviderSource = `
package main

import (
	"context"
	"io"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func main() { bridgesdk.Main("synthetic", serve) }

func serve(stdin io.Reader, stdout io.Writer, _ io.Writer) error {
	lifecycle, err := bridgesdk.NewProviderLifecycle(bridgesdk.ProviderLifecycleConfig{
		ProviderName: "synthetic",
	})
	if err != nil { return err }
	runtime, err := bridgesdk.NewRuntime(bridgesdk.RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "synthetic", Version: "0.1.0", SDKName: "bridgesdk"},
		Initialize: lifecycle.Initialize,
		Deliver: func(_ context.Context, session *bridgesdk.Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
			return session.AckDelivery(request, "synthetic-message", "")
		},
		Check: func(context.Context, *bridgesdk.Session, bridgepkg.BridgeCheckRequest) (bridgepkg.BridgeCheckResponse, error) {
			return bridgepkg.BridgeCheckResponse{}, nil
		},
		Shutdown: lifecycle.Shutdown,
	})
	if err != nil { return err }
	return lifecycle.Serve(context.Background(), runtime, stdin, stdout)
}
`
