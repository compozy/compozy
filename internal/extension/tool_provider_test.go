package extensionpkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestExtensionToolProviderAvailability(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*toolspkg.ExtensionToolRuntimeDescriptor)
		want   toolspkg.ReasonCode
	}{
		{
			name: "Should report handler mismatch from runtime descriptors",
			mutate: func(descriptor *toolspkg.ExtensionToolRuntimeDescriptor) {
				descriptor.Handler = "lookup"
			},
			want: toolspkg.ReasonExtensionRuntimeMismatch,
		},
		{
			name: "Should report input schema digest mismatch from runtime descriptors",
			mutate: func(descriptor *toolspkg.ExtensionToolRuntimeDescriptor) {
				descriptor.InputSchemaDigest = "bad-digest"
			},
			want: toolspkg.ReasonRuntimeDescriptorMismatch,
		},
		{
			name: "Should report risk mismatch from runtime descriptors",
			mutate: func(descriptor *toolspkg.ExtensionToolRuntimeDescriptor) {
				descriptor.ReadOnly = false
				descriptor.Risk = toolspkg.RiskMutating
			},
			want: toolspkg.ReasonExtensionRuntimeMismatch,
		},
		{
			name: "Should report missing runtime handler",
			mutate: func(descriptor *toolspkg.ExtensionToolRuntimeDescriptor) {
				descriptor.Handler = ""
			},
			want: toolspkg.ReasonHandlerMissing,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-tool", true)
			runtimeDescriptor := descriptor.RuntimeDescriptor
			tc.mutate(&runtimeDescriptor)
			runtime := newFakeExtensionToolRuntime(
				t,
				env.registry,
				fixture.manifest.Name,
				[]toolspkg.ExtensionToolRuntimeDescriptor{
					runtimeDescriptor,
				},
			)
			registry := newExtensionToolRegistry(t, env.registry, runtime, extensionToolPolicyAllowAll())

			view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
			if err != nil {
				t.Fatalf("Registry.Get() error = %v", err)
			}
			if view.Availability.Executable {
				t.Fatalf("Availability.Executable = true, want false with reasons %#v", view.Availability.ReasonCodes)
			}
			if !slices.Contains(view.Availability.ReasonCodes, tc.want) {
				t.Fatalf("Availability reasons = %#v, want %q", view.Availability.ReasonCodes, tc.want)
			}
		})
	}
}

func TestExtensionToolProviderProjectionGeneration(t *testing.T) {
	t.Parallel()

	t.Run("Should keep projection tokens stable for equivalent runtime projection sets", func(t *testing.T) {
		t.Parallel()

		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-generation-ordering", true)
		primary := descriptor.RuntimeDescriptor
		primary.Capabilities = []string{"capability.beta", "capability.alpha", "capability.beta"}
		primary.InputSchema = json.RawMessage(`{
			"type": "object",
			"properties": {"query": {"type": "string"}}
		}`)
		primary.OutputSchema = json.RawMessage(`{
			"properties": {"result": {"type": "string"}},
			"type": "object"
		}`)
		secondary := primary
		secondary.Handler = "secondary-handler"
		secondary.Capabilities = []string{"capability.gamma", "capability.alpha", "capability.gamma"}
		secondary.InputSchema = json.RawMessage(`{
			"type": "object",
			"properties": {"limit": {"type": "integer"}}
		}`)
		secondary.OutputSchema = json.RawMessage(`{
			"properties": {"items": {"type": "array"}},
			"type": "object"
		}`)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{primary, secondary},
		)
		runtime.extension.InitializeResult.AcceptedCapabilities.Provides = []string{
			extensionprotocol.CapabilityToolProvider,
			"capability.bridge",
			extensionprotocol.CapabilityToolProvider,
		}
		provider, err := NewExtensionToolProvider(env.registry, func() ExtensionToolRuntime {
			return runtime
		})
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}

		first, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || first == "" {
			t.Fatalf("ProjectionGeneration(first) = %q, %t, want authoritative token", first, known)
		}

		primary.Capabilities = []string{"capability.alpha", "capability.beta"}
		primary.InputSchema = json.RawMessage(`{"properties":{"query":{"type":"string"}},"type":"object"}`)
		primary.OutputSchema = json.RawMessage(`{"type":"object","properties":{"result":{"type":"string"}}}`)
		secondary.Capabilities = []string{"capability.alpha", "capability.gamma"}
		secondary.InputSchema = json.RawMessage(`{"properties":{"limit":{"type":"integer"}},"type":"object"}`)
		secondary.OutputSchema = json.RawMessage(`{"type":"object","properties":{"items":{"type":"array"}}}`)
		runtime.descriptors = []toolspkg.ExtensionToolRuntimeDescriptor{secondary, primary}
		runtime.extension.InitializeResult.AcceptedCapabilities.Provides = []string{
			"capability.bridge",
			extensionprotocol.CapabilityToolProvider,
		}
		second, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || second != first {
			t.Fatalf(
				"ProjectionGeneration(reordered equivalent runtime state) = %q, %t, want %q",
				second,
				known,
				first,
			)
		}
	})

	t.Run("Should change when runtime descriptors change", func(t *testing.T) {
		t.Parallel()

		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-generation", true)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptor.RuntimeDescriptor},
		)
		provider, err := NewExtensionToolProvider(env.registry, func() ExtensionToolRuntime {
			return runtime
		})
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}

		first, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || first == "" {
			t.Fatalf("ProjectionGeneration(first) = %q, %t, want authoritative token", first, known)
		}
		runtime.descriptors[0].Handler = "changed-handler"
		second, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || second == first {
			t.Fatalf(
				"ProjectionGeneration(runtime changed) = %q, %t, want token different from %q",
				second,
				known,
				first,
			)
		}
	})

	t.Run("Should reflect manifest byte changes with preserved metadata", func(t *testing.T) {
		t.Parallel()

		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-generation-manifest", true)
		info, err := env.registry.Get(fixture.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", fixture.manifest.Name, err)
		}
		original, stat := readManifestContentAndMetadata(t, info.ManifestPath)
		malformed := malformedManifestContent(original)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptor.RuntimeDescriptor},
		)
		provider, err := NewExtensionToolProvider(env.registry, func() ExtensionToolRuntime {
			return runtime
		})
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}

		validGeneration, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || validGeneration == "" {
			t.Fatalf("ProjectionGeneration(valid) = %q, %t, want authoritative token", validGeneration, known)
		}
		replaceManifestContentPreservingMetadata(t, info.ManifestPath, malformed, stat)
		invalidGeneration, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || invalidGeneration == validGeneration {
			t.Fatalf(
				"ProjectionGeneration(valid to invalid) = %q, %t, want token different from %q",
				invalidGeneration,
				known,
				validGeneration,
			)
		}

		replaceManifestContentPreservingMetadata(t, info.ManifestPath, original, stat)
		repairedGeneration, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || repairedGeneration == invalidGeneration {
			t.Fatalf(
				"ProjectionGeneration(invalid to valid) = %q, %t, want token different from %q",
				repairedGeneration,
				known,
				invalidGeneration,
			)
		}
	})

	t.Run("Should keep healthy partial projections for unchanged invalid manifests", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		healthy := createExtensionToolTestExtension(t, "healthy-generation", "fake-extension", nil, nil, true)
		broken := createExtensionToolTestExtension(t, "broken-generation", "fake-extension", nil, nil, true)
		installManagerFixture(t, env.registry, healthy, SourceUser, true)
		installManagerFixture(t, env.registry, broken, SourceUser, true)
		brokenInfo, err := env.registry.Get(broken.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", broken.manifest.Name, err)
		}
		original, stat := readManifestContentAndMetadata(t, brokenInfo.ManifestPath)
		replaceManifestContentPreservingMetadata(
			t,
			brokenInfo.ManifestPath,
			malformedManifestContent(original),
			stat,
		)
		descriptors, err := ResolveManifestToolDescriptors(healthy.manifest)
		if err != nil || len(descriptors) != 1 {
			t.Fatalf("ResolveManifestToolDescriptors(healthy) = %#v, %v, want one descriptor", descriptors, err)
		}
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			healthy.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptors[0].RuntimeDescriptor},
		)
		var logs bytes.Buffer
		provider, err := NewExtensionToolProvider(
			env.registry,
			func() ExtensionToolRuntime { return runtime },
			WithToolProviderLogger(slog.New(slog.NewTextHandler(&logs, nil))),
		)
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}

		first, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || first == "" {
			t.Fatalf("ProjectionGeneration(first) = %q, %t, want authoritative token", first, known)
		}
		second, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || second != first {
			t.Fatalf("ProjectionGeneration(repeated) = %q, %t, want unchanged token %q", second, known, first)
		}
		if got := strings.Count(logs.String(), "extension_name=broken-generation"); got != 1 {
			t.Fatalf("invalid manifest warning count = %d, want 1; logs = %q", got, logs.String())
		}
	})

	t.Run("Should keep deterministic projection tokens stable across workspace alternation", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		fixtureA := createExtensionToolTestExtension(t, "projection-workspace-a", "fake-extension", nil, nil, true)
		fixtureB := createExtensionToolTestExtension(t, "projection-workspace-b", "fake-extension", nil, nil, true)
		installManagerFixture(t, env.registry, fixtureA, SourceUser, true)
		installManagerFixture(t, env.registry, fixtureB, SourceUser, true)
		infoA, err := env.registry.Get(fixtureA.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", fixtureA.manifest.Name, err)
		}
		infoB, err := env.registry.Get(fixtureB.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", fixtureB.manifest.Name, err)
		}
		descriptorsA, err := ResolveManifestToolDescriptors(fixtureA.manifest)
		if err != nil || len(descriptorsA) != 1 {
			t.Fatalf("ResolveManifestToolDescriptors(workspace A) = %#v, %v, want one descriptor", descriptorsA, err)
		}
		descriptorsB, err := ResolveManifestToolDescriptors(fixtureB.manifest)
		if err != nil || len(descriptorsB) != 1 {
			t.Fatalf("ResolveManifestToolDescriptors(workspace B) = %#v, %v, want one descriptor", descriptorsB, err)
		}
		fakeA := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixtureA.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptorsA[0].RuntimeDescriptor},
		)
		fakeB := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixtureB.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptorsB[0].RuntimeDescriptor},
		)
		keyA := GlobalInstanceKey(infoA.Name)
		keyB := GlobalInstanceKey(infoB.Name)
		runtime := &fakeScopedExtensionToolRuntime{
			fakeExtensionToolRuntime: fakeA,
			infosByWorkspace: map[string][]ExtensionInfo{
				"workspace-a": {*infoA},
				"workspace-b": {*infoB},
			},
			extensionsByInstance: map[InstanceKey]*Extension{
				keyA: fakeA.extension,
				keyB: fakeB.extension,
			},
			descriptorsByInstance: map[InstanceKey][]toolspkg.ExtensionToolRuntimeDescriptor{
				keyA: fakeA.descriptors,
				keyB: fakeB.descriptors,
			},
		}
		provider, err := NewExtensionToolProvider(env.registry, func() ExtensionToolRuntime { return runtime })
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}
		generationFor := func(workspaceID string) string {
			t.Helper()
			generation, known := provider.ProjectionGeneration(
				t.Context(),
				toolspkg.Scope{WorkspaceID: workspaceID},
			)
			if !known || generation == "" {
				t.Fatalf("ProjectionGeneration(%q) = %q, %t, want token", workspaceID, generation, known)
			}
			return generation
		}

		aFirst := generationFor("workspace-a")
		bFirst := generationFor("workspace-b")
		if aRepeated := generationFor("workspace-a"); aRepeated != aFirst {
			t.Fatalf("ProjectionGeneration(A/B/A) = %q, want %q", aRepeated, aFirst)
		}
		if bRepeated := generationFor("workspace-b"); bRepeated != bFirst {
			t.Fatalf("ProjectionGeneration(B/A/B) = %q, want %q", bRepeated, bFirst)
		}

		originalA, statA := readManifestContentAndMetadata(t, infoA.ManifestPath)
		replaceManifestContentPreservingMetadata(t, infoA.ManifestPath, malformedManifestContent(originalA), statA)
		if aChanged := generationFor("workspace-a"); aChanged == aFirst {
			t.Fatalf("ProjectionGeneration(A manifest change) = %q, want token different from %q", aChanged, aFirst)
		}
		if bAfterManifestChange := generationFor("workspace-b"); bAfterManifestChange != bFirst {
			t.Fatalf("ProjectionGeneration(B after A manifest change) = %q, want %q", bAfterManifestChange, bFirst)
		}

		replaceManifestContentPreservingMetadata(t, infoA.ManifestPath, originalA, statA)
		if aRestored := generationFor("workspace-a"); aRestored != aFirst {
			t.Fatalf("ProjectionGeneration(A manifest restored) = %q, want %q", aRestored, aFirst)
		}
		fakeB.extension.Status.Healthy = false
		if bRuntimeChanged := generationFor("workspace-b"); bRuntimeChanged == bFirst {
			t.Fatalf(
				"ProjectionGeneration(B runtime change) = %q, want token different from %q",
				bRuntimeChanged,
				bFirst,
			)
		}
		if aAfterRuntimeChange := generationFor("workspace-a"); aAfterRuntimeChange != aFirst {
			t.Fatalf("ProjectionGeneration(A after B runtime change) = %q, want %q", aAfterRuntimeChange, aFirst)
		}
	})

	t.Run("Should serialize manifest refreshes before publishing cached content", func(t *testing.T) {
		t.Parallel()

		env, fixture, _ := createExtensionToolProviderFixture(t, "ext-generation-order", true)
		info, err := env.registry.Get(fixture.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", fixture.manifest.Name, err)
		}
		original, stat := readManifestContentAndMetadata(t, info.ManifestPath)
		releaseWarning := make(chan struct{})
		logger := &blockingExtensionToolLogHandler{
			started: make(chan struct{}),
			release: releaseWarning,
		}
		t.Cleanup(logger.Release)
		provider, err := NewExtensionToolProvider(
			env.registry,
			func() ExtensionToolRuntime { return nil },
			WithToolProviderLogger(slog.New(logger)),
		)
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}

		replaceManifestContentPreservingMetadata(
			t,
			info.ManifestPath,
			malformedManifestContent(original),
			stat,
		)
		firstResult := make(chan extensionToolProviderListResult, 1)
		go func() {
			tools, listErr := provider.List(t.Context(), toolspkg.Scope{})
			firstResult <- extensionToolProviderListResult{tools: tools, err: listErr}
		}()
		select {
		case <-logger.started:
		case first := <-firstResult:
			t.Fatalf("Provider.List(earlier invalid content) completed before blocking warning: %#v", first)
		}

		replaceManifestContentPreservingMetadata(t, info.ManifestPath, original, stat)
		waitBase, cancelWait := context.WithCancel(t.Context())
		t.Cleanup(cancelWait)
		waitContext := &manifestRefreshWaitContext{
			Context: waitBase,
			waiting: make(chan struct{}),
		}
		secondResult := make(chan extensionToolProviderListResult, 1)
		go func() {
			tools, listErr := provider.List(waitContext, toolspkg.Scope{})
			secondResult <- extensionToolProviderListResult{tools: tools, err: listErr}
		}()
		select {
		case <-waitContext.waiting:
			cancelWait()
		case second := <-secondResult:
			t.Fatalf("Provider.List(newer content) completed before waiting for refresh: %#v", second)
		}
		second := <-secondResult
		if !errors.Is(second.err, context.Canceled) {
			t.Fatalf("Provider.List(waiting refresh) error = %v, want context canceled", second.err)
		}

		logger.Release()
		first := <-firstResult
		if first.err != nil {
			t.Fatalf("Provider.List(earlier invalid content) error = %v", first.err)
		}
		if len(first.tools) != 0 {
			t.Fatalf("Provider.List(earlier invalid content) = %#v, want invalid extension isolated", first.tools)
		}

		currentGeneration, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || currentGeneration == "" {
			t.Fatalf(
				"ProjectionGeneration(newer content) = %q, %t, want authoritative token",
				currentGeneration,
				known,
			)
		}
		stableGeneration, known := provider.ProjectionGeneration(t.Context(), toolspkg.Scope{})
		if !known || stableGeneration != currentGeneration {
			t.Fatalf(
				"ProjectionGeneration(unchanged newer content) = %q, %t, want %q",
				stableGeneration,
				known,
				currentGeneration,
			)
		}
	})
}

func TestExtensionToolProviderCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Should remove disabled extension tools from registry projections", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-tool", true)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptor.RuntimeDescriptor},
		)
		registry := newExtensionToolRegistry(t, env.registry, runtime, extensionToolPolicyAllowAll())

		views, err := registry.List(ctx, toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Registry.List(enabled) error = %v", err)
		}
		if !extensionToolViewsContain(views, descriptor.Tool.ID) {
			t.Fatalf("Registry.List(enabled) omitted %q", descriptor.Tool.ID)
		}

		if err := env.registry.Disable(fixture.manifest.Name); err != nil {
			t.Fatalf("Disable(%q) error = %v", fixture.manifest.Name, err)
		}
		views, err = registry.List(ctx, toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Registry.List(disabled) error = %v", err)
		}
		if extensionToolViewsContain(views, descriptor.Tool.ID) {
			t.Fatalf("Registry.List(disabled) contains %q, want removed from catalog", descriptor.Tool.ID)
		}
		searchResults, err := registry.Search(ctx, toolspkg.Scope{Operator: true}, toolspkg.SearchQuery{
			Query: descriptor.Tool.ID.String(),
		})
		if err != nil {
			t.Fatalf("Registry.Search(disabled) error = %v", err)
		}
		if len(searchResults) != 0 {
			t.Fatalf("Registry.Search(disabled) returned %#v, want no disabled extension tools", searchResults)
		}
		_, err = registry.Get(ctx, toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
		if !errors.Is(err, toolspkg.ErrToolNotFound) {
			t.Fatalf("Registry.Get(disabled) error = %v, want ErrToolNotFound", err)
		}

		if err := env.registry.Enable(fixture.manifest.Name); err != nil {
			t.Fatalf("Enable(%q) error = %v", fixture.manifest.Name, err)
		}
		views, err = registry.List(ctx, toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Registry.List(restored) error = %v", err)
		}
		if !extensionToolViewsContain(views, descriptor.Tool.ID) {
			t.Fatalf("Registry.List(restored) omitted %q", descriptor.Tool.ID)
		}
	})

	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string)
		logKey string
	}{
		{
			name: "Should isolate an extension with a missing manifest",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Remove(path); err != nil {
					t.Fatalf("os.Remove(%q) error = %v", path, err)
				}
			},
			logKey: "broken-missing",
		},
		{
			name: "Should isolate an extension with a malformed manifest",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				writeFile(t, path, "{")
			},
			logKey: "broken-malformed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := newRegistryTestEnv(t)
			healthy := createExtensionToolTestExtension(t, "healthy-tool", "fake-extension", nil, nil, true)
			broken := createExtensionToolTestExtension(t, tc.logKey, "fake-extension", nil, nil, true)
			installManagerFixture(t, env.registry, healthy, SourceUser, true)
			installManagerFixture(t, env.registry, broken, SourceUser, true)
			brokenInfo, err := env.registry.Get(broken.manifest.Name)
			if err != nil {
				t.Fatalf("Registry.Get(%q) error = %v", broken.manifest.Name, err)
			}
			tc.mutate(t, brokenInfo.ManifestPath)

			descriptors, err := ResolveManifestToolDescriptors(healthy.manifest)
			if err != nil || len(descriptors) != 1 {
				t.Fatalf("ResolveManifestToolDescriptors(healthy) = %#v, %v, want one descriptor", descriptors, err)
			}
			runtime := newFakeExtensionToolRuntime(
				t,
				env.registry,
				healthy.manifest.Name,
				[]toolspkg.ExtensionToolRuntimeDescriptor{descriptors[0].RuntimeDescriptor},
			)
			var logs bytes.Buffer
			registry := newExtensionToolRegistry(
				t,
				env.registry,
				runtime,
				extensionToolPolicyAllowAll(),
				WithToolProviderLogger(slog.New(slog.NewTextHandler(&logs, nil))),
			)

			views, err := registry.List(testutil.Context(t), toolspkg.Scope{Operator: true})
			if err != nil {
				t.Fatalf("Registry.List() error = %v, want invalid extension isolated", err)
			}
			if !extensionToolViewsContain(views, descriptors[0].Tool.ID) {
				t.Fatalf("Registry.List() omitted healthy tool %q", descriptors[0].Tool.ID)
			}
			_, err = registry.Call(
				testutil.Context(t),
				toolspkg.Scope{SessionID: "session-1"},
				toolspkg.CallRequest{
					ToolID: descriptors[0].Tool.ID,
					Input:  json.RawMessage(`{"query":"alpha"}`),
				},
			)
			if err != nil {
				t.Fatalf("Registry.Call(healthy) error = %v, want invalid extension isolated", err)
			}
			if !strings.Contains(logs.String(), tc.logKey) {
				t.Fatalf("provider logs = %q, want invalid extension name %q", logs.String(), tc.logKey)
			}
		})
	}

	t.Run("Should cache healthy partial catalogs for unchanged invalid manifests", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		healthy := createExtensionToolTestExtension(t, "healthy-catalog", "fake-extension", nil, nil, true)
		broken := createExtensionToolTestExtension(t, "broken-catalog", "fake-extension", nil, nil, true)
		installManagerFixture(t, env.registry, healthy, SourceUser, true)
		installManagerFixture(t, env.registry, broken, SourceUser, true)
		brokenInfo, err := env.registry.Get(broken.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", broken.manifest.Name, err)
		}
		original, stat := readManifestContentAndMetadata(t, brokenInfo.ManifestPath)
		malformed := malformedManifestContent(original)
		replaceManifestContentPreservingMetadata(t, brokenInfo.ManifestPath, malformed, stat)
		healthyDescriptors, err := ResolveManifestToolDescriptors(healthy.manifest)
		if err != nil || len(healthyDescriptors) != 1 {
			t.Fatalf(
				"ResolveManifestToolDescriptors(healthy) = %#v, %v, want one descriptor",
				healthyDescriptors,
				err,
			)
		}
		var logs bytes.Buffer
		provider, err := NewExtensionToolProvider(
			env.registry,
			func() ExtensionToolRuntime { return nil },
			WithToolProviderLogger(slog.New(slog.NewTextHandler(&logs, nil))),
		)
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}

		for _, name := range []string{"first", "repeated"} {
			catalog, listErr := provider.List(testutil.Context(t), toolspkg.Scope{})
			if listErr != nil {
				t.Fatalf("Provider.List(%s) error = %v", name, listErr)
			}
			if len(catalog) != 1 || catalog[0].ID != healthyDescriptors[0].Tool.ID {
				t.Fatalf("Provider.List(%s) = %#v, want healthy tool %q", name, catalog, healthyDescriptors[0].Tool.ID)
			}
		}
		if got := strings.Count(logs.String(), "extension_name=broken-catalog"); got != 1 {
			t.Fatalf("unchanged invalid manifest warning count = %d, want 1; logs = %q", got, logs.String())
		}

		changedMalformed := slices.Clone(malformed)
		changedMalformed[0] = '['
		replaceManifestContentPreservingMetadata(t, brokenInfo.ManifestPath, changedMalformed, stat)
		catalog, err := provider.List(testutil.Context(t), toolspkg.Scope{})
		if err != nil {
			t.Fatalf("Provider.List(changed invalid) error = %v", err)
		}
		if len(catalog) != 1 || catalog[0].ID != healthyDescriptors[0].Tool.ID {
			t.Fatalf(
				"Provider.List(changed invalid) = %#v, want healthy tool %q",
				catalog,
				healthyDescriptors[0].Tool.ID,
			)
		}
		if got := strings.Count(logs.String(), "extension_name=broken-catalog"); got != 2 {
			t.Fatalf("changed invalid manifest warning count = %d, want 2; logs = %q", got, logs.String())
		}
	})

	t.Run("Should recover from a transient manifest read error without metadata changes", func(t *testing.T) {
		t.Parallel()

		env, fixture, descriptor := createExtensionToolProviderFixture(t, "transient-read-error", true)
		info, err := env.registry.Get(fixture.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", fixture.manifest.Name, err)
		}
		var logs bytes.Buffer
		provider, err := NewExtensionToolProvider(
			env.registry,
			func() ExtensionToolRuntime { return nil },
			WithToolProviderLogger(slog.New(slog.NewTextHandler(&logs, nil))),
		)
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}
		readFile := provider.manifestReadFile
		firstRead := true
		provider.manifestReadFile = func(path string) ([]byte, error) {
			if path == info.ManifestPath && firstRead {
				firstRead = false
				return nil, errors.New("transient manifest read error")
			}
			return readFile(path)
		}

		first, err := provider.List(testutil.Context(t), toolspkg.Scope{})
		if err != nil {
			t.Fatalf("Provider.List(transient read error) error = %v", err)
		}
		if len(first) != 0 {
			t.Fatalf("Provider.List(transient read error) = %#v, want invalid extension isolated", first)
		}
		second, err := provider.List(testutil.Context(t), toolspkg.Scope{})
		if err != nil {
			t.Fatalf("Provider.List(recovered read) error = %v", err)
		}
		if len(second) != 1 || second[0].ID != descriptor.Tool.ID {
			t.Fatalf("Provider.List(recovered read) = %#v, want healthy tool %q", second, descriptor.Tool.ID)
		}
		if got := strings.Count(logs.String(), "extension_name=transient-read-error"); got != 1 {
			t.Fatalf("transient read error warning count = %d, want 1; logs = %q", got, logs.String())
		}
	})

	t.Run("Should retain global snapshots behind workspace development overrides", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		healthyA := createExtensionToolTestExtension(t, "workspace-healthy-a", "fake-extension", nil, nil, true)
		healthyB := createExtensionToolTestExtension(t, "workspace-healthy-b", "fake-extension", nil, nil, true)
		broken := createExtensionToolTestExtension(t, "workspace-broken", "fake-extension", nil, nil, true)
		installManagerFixture(t, env.registry, healthyA, SourceUser, true)
		installManagerFixture(t, env.registry, healthyB, SourceUser, true)
		installManagerFixture(t, env.registry, broken, SourceUser, true)
		healthyAInfo, err := env.registry.Get(healthyA.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", healthyA.manifest.Name, err)
		}
		healthyBInfo, err := env.registry.Get(healthyB.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", healthyB.manifest.Name, err)
		}
		brokenInfo, err := env.registry.Get(broken.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", broken.manifest.Name, err)
		}
		workspaceHealthyAInfo := cloneExtensionInfo(*healthyAInfo)
		workspaceHealthyAInfo.Name = brokenInfo.Name
		workspaceHealthyAInfo.Source = SourceWorkspace
		workspaceHealthyBInfo := cloneExtensionInfo(*healthyBInfo)
		workspaceHealthyBInfo.Name = brokenInfo.Name
		workspaceHealthyBInfo.Source = SourceWorkspace
		original, stat := readManifestContentAndMetadata(t, brokenInfo.ManifestPath)
		replaceManifestContentPreservingMetadata(
			t,
			brokenInfo.ManifestPath,
			malformedManifestContent(original),
			stat,
		)
		healthyADescriptors, err := ResolveManifestToolDescriptors(healthyA.manifest)
		if err != nil || len(healthyADescriptors) != 1 {
			t.Fatalf(
				"ResolveManifestToolDescriptors(workspace A) = %#v, %v, want one descriptor",
				healthyADescriptors,
				err,
			)
		}
		healthyBDescriptors, err := ResolveManifestToolDescriptors(healthyB.manifest)
		if err != nil || len(healthyBDescriptors) != 1 {
			t.Fatalf(
				"ResolveManifestToolDescriptors(workspace B) = %#v, %v, want one descriptor",
				healthyBDescriptors,
				err,
			)
		}
		runtime := &fakeScopedExtensionToolRuntime{
			fakeExtensionToolRuntime: newFakeExtensionToolRuntime(
				t,
				env.registry,
				healthyA.manifest.Name,
				nil,
			),
			infosByWorkspace: map[string][]ExtensionInfo{
				"":            {*brokenInfo},
				"workspace-a": {workspaceHealthyAInfo},
				"workspace-b": {workspaceHealthyBInfo},
			},
		}
		var logs bytes.Buffer
		provider, err := NewExtensionToolProvider(
			env.registry,
			func() ExtensionToolRuntime { return runtime },
			WithToolProviderLogger(slog.New(slog.NewTextHandler(&logs, nil))),
		)
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}

		for _, testCase := range []struct {
			workspaceID string
			wantID      toolspkg.ToolID
			wantEmpty   bool
		}{
			{workspaceID: "", wantEmpty: true},
			{workspaceID: "workspace-a", wantID: healthyADescriptors[0].Tool.ID},
			{workspaceID: "workspace-b", wantID: healthyBDescriptors[0].Tool.ID},
			{workspaceID: "", wantEmpty: true},
		} {
			catalog, listErr := provider.List(testutil.Context(t), toolspkg.Scope{WorkspaceID: testCase.workspaceID})
			if listErr != nil {
				t.Fatalf("Provider.List(%q) error = %v", testCase.workspaceID, listErr)
			}
			if testCase.wantEmpty {
				if len(catalog) != 0 {
					t.Fatalf("Provider.List(%q) = %#v, want overridden invalid catalog", testCase.workspaceID, catalog)
				}
				continue
			}
			if len(catalog) != 1 || catalog[0].ID != testCase.wantID {
				t.Fatalf(
					"Provider.List(%q) = %#v, want only healthy tool %q",
					testCase.workspaceID,
					catalog,
					testCase.wantID,
				)
			}
		}
		if got := strings.Count(logs.String(), "extension_name=workspace-broken"); got != 1 {
			t.Fatalf("global invalid manifest warning count = %d, want 1; logs = %q", got, logs.String())
		}
	})

	t.Run("Should name each invalid extension with identical bytes once", func(t *testing.T) {
		t.Parallel()

		env, fixture, _ := createExtensionToolProviderFixture(t, "invalid-json", true)
		jsonInfo, err := env.registry.Get(fixture.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", fixture.manifest.Name, err)
		}
		tomlDir := t.TempDir()
		tomlPath := filepath.Join(tomlDir, manifestTOMLFileName)
		const invalidManifest = "{"
		writeFile(t, jsonInfo.ManifestPath, invalidManifest)
		writeFile(t, tomlPath, invalidManifest)
		_, jsonParseErr := loadManifestContentAtPath(jsonInfo.ManifestPath, []byte(invalidManifest))
		if jsonParseErr == nil {
			t.Fatal("loadManifestContentAtPath(JSON) error = nil, want invalid manifest error")
		}
		_, tomlParseErr := loadManifestContentAtPath(tomlPath, []byte(invalidManifest))
		if tomlParseErr == nil {
			t.Fatal("loadManifestContentAtPath(TOML) error = nil, want invalid manifest error")
		}
		if !strings.Contains(jsonParseErr.Error(), "JSON") || !strings.Contains(tomlParseErr.Error(), "toml:") {
			t.Fatalf("parser errors = %q, %q, want JSON and TOML parser errors", jsonParseErr, tomlParseErr)
		}
		tomlInfo := cloneExtensionInfo(*jsonInfo)
		tomlInfo.Name = "invalid-toml"
		tomlInfo.ManifestPath = tomlPath
		runtime := &fakeScopedExtensionToolRuntime{
			fakeExtensionToolRuntime: newFakeExtensionToolRuntime(
				t,
				env.registry,
				fixture.manifest.Name,
				nil,
			),
			infosByWorkspace: map[string][]ExtensionInfo{
				"workspace-invalid": {*jsonInfo, tomlInfo},
			},
		}
		var logs bytes.Buffer
		provider, err := NewExtensionToolProvider(
			env.registry,
			func() ExtensionToolRuntime { return runtime },
			WithToolProviderLogger(slog.New(slog.NewTextHandler(&logs, nil))),
		)
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}

		for _, name := range []string{"first", "repeated"} {
			catalog, listErr := provider.List(
				testutil.Context(t),
				toolspkg.Scope{WorkspaceID: "workspace-invalid"},
			)
			if listErr != nil {
				t.Fatalf("Provider.List(%s) error = %v", name, listErr)
			}
			if len(catalog) != 0 {
				t.Fatalf("Provider.List(%s) = %#v, want invalid extensions isolated", name, catalog)
			}
		}
		for _, extensionName := range []string{"invalid-json", "invalid-toml"} {
			if got := strings.Count(logs.String(), "extension_name="+extensionName); got != 1 {
				t.Fatalf(
					"warning count for %q = %d, want 1; logs = %q",
					extensionName,
					got,
					logs.String(),
				)
			}
		}
		for _, parserMarker := range []string{"JSON", "toml:"} {
			if !strings.Contains(logs.String(), parserMarker) {
				t.Fatalf("provider logs = %q, want parser marker %q", logs.String(), parserMarker)
			}
		}
	})

	t.Run("Should reflect manifest byte changes with unchanged file metadata", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		fixture := createExtensionToolTestExtension(t, "repaired-tool", "fake-extension", nil, nil, true)
		installManagerFixture(t, env.registry, fixture, SourceUser, true)
		info, err := env.registry.Get(fixture.manifest.Name)
		if err != nil {
			t.Fatalf("Registry.Get(%q) error = %v", fixture.manifest.Name, err)
		}
		original, stat := readManifestContentAndMetadata(t, info.ManifestPath)
		malformed := malformedManifestContent(original)

		provider, err := NewExtensionToolProvider(env.registry, func() ExtensionToolRuntime { return nil })
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}
		first, err := provider.List(testutil.Context(t), toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Provider.List(valid) error = %v", err)
		}
		if len(first) != 1 {
			t.Fatalf("Provider.List(valid) = %#v, want extension tool", first)
		}

		replaceManifestContentPreservingMetadata(t, info.ManifestPath, malformed, stat)
		invalid, err := provider.List(testutil.Context(t), toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Provider.List(valid to invalid) error = %v", err)
		}
		if len(invalid) != 0 {
			t.Fatalf("Provider.List(valid to invalid) = %#v, want invalid extension isolated", invalid)
		}

		replaceManifestContentPreservingMetadata(t, info.ManifestPath, original, stat)
		repaired, err := provider.List(testutil.Context(t), toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Provider.List(repaired) error = %v", err)
		}
		if len(repaired) != 1 {
			t.Fatalf("Provider.List(repaired) = %#v, want repaired extension tool", repaired)
		}
	})
}

func TestExtensionToolProviderDispatch(t *testing.T) {
	t.Parallel()

	t.Run("Should call extension tool handlers through Registry.Call", func(t *testing.T) {
		t.Parallel()

		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-tool", true)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptor.RuntimeDescriptor},
		)
		runtime.callResult = toolspkg.ToolResult{
			Content: []toolspkg.ToolContent{{Type: "text", Text: "ok"}},
		}
		registry := newExtensionToolRegistry(t, env.registry, runtime, extensionToolPolicyAllowAll())

		result, err := registry.Call(testutil.Context(t), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call() error = %v", err)
		}
		if got := result.Content[0].Text; got != "ok" {
			t.Fatalf("Result content = %q, want ok", got)
		}
		if len(runtime.calls) != 1 {
			t.Fatalf("runtime calls = %d, want 1", len(runtime.calls))
		}
		if got := runtime.calls[0].Handler; got != "search" {
			t.Fatalf("Call handler = %q, want search", got)
		}
	})

	t.Run("Should forward daemon authenticated workspace context outside tool input", func(t *testing.T) {
		t.Parallel()

		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-workspace", true)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptor.RuntimeDescriptor},
		)
		registry := newExtensionToolRegistry(t, env.registry, runtime, extensionToolPolicyAllowAll())
		scope := toolspkg.Scope{SessionID: "session-1", WorkspaceID: "workspace-identity"}
		_, err := registry.Call(testutil.Context(t), scope, toolspkg.CallRequest{
			ToolID:               descriptor.Tool.ID,
			WorkspaceID:          scope.WorkspaceID,
			TrustedWorkspaceRoot: "/trusted/workspace",
			Input:                json.RawMessage(`{"query":"alpha"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call() error = %v", err)
		}
		if len(runtime.calls) != 1 || runtime.calls[0].TrustedWorkspace == nil {
			t.Fatalf("runtime calls = %#v, want trusted workspace context", runtime.calls)
		}
		if got, want := runtime.calls[0].TrustedWorkspace.ID, "workspace-identity"; got != want {
			t.Fatalf("TrustedWorkspace.ID = %q, want %q", got, want)
		}
		if got, want := runtime.calls[0].TrustedWorkspace.Root, "/trusted/workspace"; got != want {
			t.Fatalf("TrustedWorkspace.Root = %q, want %q", got, want)
		}
		if strings.Contains(string(runtime.calls[0].Input), "trusted") ||
			strings.Contains(string(runtime.calls[0].Input), "workspace-identity") {
			t.Fatalf("tool input leaked trusted workspace context: %s", runtime.calls[0].Input)
		}
	})

	t.Run("Should gate mutating extension tools on approval policy", func(t *testing.T) {
		t.Parallel()

		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-mutating", false)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptor.RuntimeDescriptor},
		)
		registry := newExtensionToolRegistry(t, env.registry, runtime, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ExternalDefault:      toolspkg.ExternalDefaultEnabled,
			ApprovalAvailable:    true,
		})

		view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
		if err != nil {
			t.Fatalf("Registry.Get() error = %v", err)
		}
		if !view.Decision.ApprovalRequired {
			t.Fatalf("Decision.ApprovalRequired = false, want true")
		}
		if !slices.Contains(view.Decision.ReasonCodes, toolspkg.ReasonApprovalRequired) {
			t.Fatalf("Decision reasons = %#v, want approval_required", view.Decision.ReasonCodes)
		}
	})
}

func TestExtensionToolProviderSubprocessIntegration(t *testing.T) {
	t.Run("Should dispatch a read-only tool through a real subprocess", func(t *testing.T) {
		env, fixture, descriptor, manager := startExtensionToolSubprocess(t, "ext-tool", "tool_provider", true)
		registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())

		result, err := registry.Call(testutil.Context(t), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call() error = %v", err)
		}
		if len(result.Content) != 1 || result.Content[0].Type != "text" {
			t.Fatalf("Result.Content = %#v, want text content", result.Content)
		}
		if _, err := manager.Get(fixture.manifest.Name); err != nil {
			t.Fatalf("Manager.Get(%q) error = %v", fixture.manifest.Name, err)
		}
	})

	t.Run("Should mark a mutating subprocess tool as requiring approval", func(t *testing.T) {
		env, _, descriptor, manager := startExtensionToolSubprocess(t, "ext-mutating", "tool_provider", false)
		registry := newExtensionToolRegistry(t, env.registry, manager, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ExternalDefault:      toolspkg.ExternalDefaultEnabled,
			ApprovalAvailable:    true,
		})

		view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
		if err != nil {
			t.Fatalf("Registry.Get() error = %v", err)
		}
		if !view.Decision.ApprovalRequired {
			t.Fatalf("Decision.ApprovalRequired = false, want true")
		}
	})

	testCases := []struct {
		name     string
		scenario string
		want     toolspkg.ReasonCode
	}{
		{
			name:     "Should surface subprocess schema mismatch availability",
			scenario: "tool_runtime_schema_mismatch",
			want:     toolspkg.ReasonRuntimeDescriptorMismatch,
		},
		{
			name:     "Should surface subprocess handler mismatch availability",
			scenario: "tool_runtime_handler_mismatch",
			want:     toolspkg.ReasonExtensionRuntimeMismatch,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env, _, descriptor, manager := startExtensionToolSubprocess(t, "ext-tool", tc.scenario, true)
			registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())

			view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
			if err != nil {
				t.Fatalf("Registry.Get() error = %v", err)
			}
			if !slices.Contains(view.Availability.ReasonCodes, tc.want) {
				t.Fatalf("Availability reasons = %#v, want %q", view.Availability.ReasonCodes, tc.want)
			}
		})
	}

	t.Run("Should surface subprocess handler errors through Registry.Call", func(t *testing.T) {
		env, _, descriptor, manager := startExtensionToolSubprocess(t, "ext-tool", "tool_call_error", true)
		registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())

		_, err := registry.Call(testutil.Context(t), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if !errors.Is(err, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("Registry.Call() error = %v, want ErrToolBackendFailed", err)
		}
	})

	t.Run("Should propagate cancellation to subprocess tool calls", func(t *testing.T) {
		env, _, descriptor, manager := startExtensionToolSubprocess(t, "ext-tool", "tool_call_slow", true)
		registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())
		ctx, cancel := context.WithTimeout(testutil.Context(t), 20*time.Millisecond)
		defer cancel()

		_, err := registry.Call(ctx, toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if !errors.Is(err, toolspkg.ErrToolTimedOut) {
			t.Fatalf("Registry.Call() error = %v, want ErrToolTimedOut", err)
		}
	})
}

func TestExtensionToolProviderGoSDKSubprocessIntegration(t *testing.T) {
	t.Run("Should dispatch a read-only Go SDK tool through Registry.Call", func(t *testing.T) {
		binary := buildGoSDKToolProviderBinary(t, "go-sdk-tool", true)
		env, fixture, descriptor, manager := startCompiledGoSDKToolSubprocess(t, "go-sdk-tool", binary, true)
		registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())

		result, err := registry.Call(testutil.Context(t), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call() error = %v", err)
		}
		if len(result.Content) != 1 || result.Content[0].Text != "go-sdk:alpha" {
			t.Fatalf("Result.Content = %#v, want go-sdk:alpha", result.Content)
		}
		if _, err := manager.Get(fixture.manifest.Name); err != nil {
			t.Fatalf("Manager.Get(%q) error = %v", fixture.manifest.Name, err)
		}
	})

	t.Run("Should gate a mutating Go SDK tool on approval policy", func(t *testing.T) {
		binary := buildGoSDKToolProviderBinary(t, "go-sdk-mutating", false)
		env, _, descriptor, manager := startCompiledGoSDKToolSubprocess(t, "go-sdk-mutating", binary, false)
		registry := newExtensionToolRegistry(t, env.registry, manager, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ExternalDefault:      toolspkg.ExternalDefaultEnabled,
			ApprovalAvailable:    true,
		})

		view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
		if err != nil {
			t.Fatalf("Registry.Get() error = %v", err)
		}
		if !view.Decision.ApprovalRequired {
			t.Fatalf("Decision.ApprovalRequired = false, want true")
		}
	})
}

type fakeExtensionToolRuntime struct {
	extension   *Extension
	descriptors []toolspkg.ExtensionToolRuntimeDescriptor
	calls       []toolspkg.ExtensionToolCallRequest
	callResult  toolspkg.ToolResult
	callErr     error
}

type fakeScopedExtensionToolRuntime struct {
	*fakeExtensionToolRuntime
	infosByWorkspace      map[string][]ExtensionInfo
	extensionsByInstance  map[InstanceKey]*Extension
	descriptorsByInstance map[InstanceKey][]toolspkg.ExtensionToolRuntimeDescriptor
}

type extensionToolProviderListResult struct {
	tools []toolspkg.Descriptor
	err   error
}

type manifestRefreshWaitContext struct {
	context.Context
	waiting  chan struct{}
	waitOnce sync.Once
}

func (c *manifestRefreshWaitContext) Done() <-chan struct{} {
	c.waitOnce.Do(func() {
		close(c.waiting)
	})
	return c.Context.Done()
}

type blockingExtensionToolLogHandler struct {
	started     chan struct{}
	release     chan struct{}
	startedOnce sync.Once
	releaseOnce sync.Once
}

var _ slog.Handler = (*blockingExtensionToolLogHandler)(nil)

func (h *blockingExtensionToolLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *blockingExtensionToolLogHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Message != "extension: skip invalid extension tool manifest" {
		return nil
	}
	h.startedOnce.Do(func() {
		close(h.started)
	})
	select {
	case <-h.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *blockingExtensionToolLogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *blockingExtensionToolLogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *blockingExtensionToolLogHandler) Release() {
	h.releaseOnce.Do(func() {
		close(h.release)
	})
}

var _ ExtensionToolRuntime = (*fakeExtensionToolRuntime)(nil)
var _ extensionScopedToolRuntime = (*fakeScopedExtensionToolRuntime)(nil)

func (f *fakeScopedExtensionToolRuntime) GetForInstance(key InstanceKey) (*Extension, error) {
	if extension := f.extensionsByInstance[key.Normalize()]; extension != nil {
		return extension, nil
	}
	return f.extension, nil
}

func (f *fakeScopedExtensionToolRuntime) ListForWorkspace(workspaceID string) []ExtensionInfo {
	infos := f.infosByWorkspace[workspaceID]
	cloned := make([]ExtensionInfo, len(infos))
	for i := range infos {
		cloned[i] = cloneExtensionInfo(infos[i])
	}
	return cloned
}

func (f *fakeScopedExtensionToolRuntime) ProvideToolsForInstance(
	ctx context.Context,
	key InstanceKey,
) ([]toolspkg.ExtensionToolRuntimeDescriptor, error) {
	if descriptors, ok := f.descriptorsByInstance[key.Normalize()]; ok {
		return cloneRuntimeToolDescriptors(descriptors), nil
	}
	return f.ProvideTools(ctx, "")
}

func (f *fakeScopedExtensionToolRuntime) CallToolForInstance(
	ctx context.Context,
	_ InstanceKey,
	req toolspkg.ExtensionToolCallRequest,
) (toolspkg.ToolResult, error) {
	return f.CallTool(ctx, "", req)
}

func (f *fakeExtensionToolRuntime) Get(string) (*Extension, error) {
	return f.extension, nil
}

func (f *fakeExtensionToolRuntime) ProvideTools(
	context.Context,
	string,
) ([]toolspkg.ExtensionToolRuntimeDescriptor, error) {
	return cloneRuntimeToolDescriptors(f.descriptors), nil
}

func (f *fakeExtensionToolRuntime) CallTool(
	_ context.Context,
	_ string,
	req toolspkg.ExtensionToolCallRequest,
) (toolspkg.ToolResult, error) {
	f.calls = append(f.calls, req)
	if f.callErr != nil {
		return toolspkg.ToolResult{}, f.callErr
	}
	return f.callResult, nil
}

func createExtensionToolProviderFixture(
	t *testing.T,
	name string,
	readOnly bool,
) (registryTestEnv, managerFixture, ManifestToolDescriptor) {
	t.Helper()

	env := newRegistryTestEnv(t)
	fixture := createExtensionToolTestExtension(t, name, "fake-extension", nil, nil, readOnly)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)
	descriptors, err := ResolveManifestToolDescriptors(fixture.manifest)
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("manifest tool descriptors = %d, want 1", len(descriptors))
	}
	return env, fixture, descriptors[0]
}

func newFakeExtensionToolRuntime(
	t *testing.T,
	registry *Registry,
	name string,
	descriptors []toolspkg.ExtensionToolRuntimeDescriptor,
) *fakeExtensionToolRuntime {
	t.Helper()

	info, err := registry.Get(name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", name, err)
	}
	return &fakeExtensionToolRuntime{
		extension: &Extension{
			Info: *info,
			InitializeResult: &subprocess.InitializeResponse{
				AcceptedCapabilities: subprocess.AcceptedCapabilities{
					Provides: []string{extensionprotocol.CapabilityToolProvider},
				},
				ImplementedMethods: []string{
					string(extensionprotocol.ExtensionServiceMethodProvideTools),
					string(extensionprotocol.ExtensionServiceMethodToolsCall),
				},
			},
			Status: ExtensionStatus{
				Name:    name,
				Enabled: true,
				Active:  true,
				Healthy: true,
			},
		},
		descriptors: descriptors,
	}
}

func newExtensionToolRegistry(
	t *testing.T,
	extensionRegistry *Registry,
	runtime ExtensionToolRuntime,
	policyInputs toolspkg.PolicyInputs,
	providerOpts ...ExtensionToolProviderOption,
) *toolspkg.RuntimeRegistry {
	t.Helper()

	provider, err := NewExtensionToolProvider(extensionRegistry, func() ExtensionToolRuntime {
		return runtime
	}, providerOpts...)
	if err != nil {
		t.Fatalf("NewExtensionToolProvider() error = %v", err)
	}
	registry, err := toolspkg.NewRegistry(
		toolspkg.WithProviders(provider),
		toolspkg.WithPolicyInputs(policyInputs, toolspkg.ToolsetCatalog{}),
	)
	if err != nil {
		t.Fatalf("toolspkg.NewRegistry() error = %v", err)
	}
	return registry
}

func extensionToolPolicyAllowAll() toolspkg.PolicyInputs {
	return toolspkg.PolicyInputs{
		SystemPermissionMode: toolspkg.PermissionModeApproveAll,
		ExternalDefault:      toolspkg.ExternalDefaultEnabled,
		ApprovalAvailable:    true,
	}
}

func extensionToolViewsContain(views []toolspkg.ToolView, id toolspkg.ToolID) bool {
	for i := range views {
		if views[i].Descriptor.ID == id {
			return true
		}
	}
	return false
}

func startExtensionToolSubprocess(
	t *testing.T,
	name string,
	scenario string,
	readOnly bool,
) (registryTestEnv, managerFixture, ManifestToolDescriptor, *Manager) {
	t.Helper()

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "tool-helper.log")
	fixture := createExtensionToolTestExtension(
		t,
		name,
		helperCommand(t),
		helperArgs(),
		helperEnv(scenario, markerPath),
		readOnly,
	)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)
	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Stop() cleanup error = %v", err)
		}
	})
	waitForManagerCondition(t, time.Second, func() bool {
		extension, err := manager.Get(name)
		return err == nil && extension.Status.Active && extension.Status.Healthy
	})

	descriptors, err := ResolveManifestToolDescriptors(fixture.manifest)
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("manifest tool descriptors = %d, want 1", len(descriptors))
	}
	return env, fixture, descriptors[0], manager
}

func startCompiledGoSDKToolSubprocess(
	t *testing.T,
	name string,
	command string,
	readOnly bool,
) (registryTestEnv, managerFixture, ManifestToolDescriptor, *Manager) {
	t.Helper()

	env := newRegistryTestEnv(t)
	fixture := createExtensionToolTestExtension(t, name, command, nil, nil, readOnly)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)
	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Stop() cleanup error = %v", err)
		}
	})
	waitForManagerCondition(t, time.Second, func() bool {
		extension, err := manager.Get(name)
		return err == nil && extension.Status.Active && extension.Status.Healthy
	})

	descriptors, err := ResolveManifestToolDescriptors(fixture.manifest)
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("manifest tool descriptors = %d, want 1", len(descriptors))
	}
	return env, fixture, descriptors[0], manager
}

func buildGoSDKToolProviderBinary(t *testing.T, name string, readOnly bool) string {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs(repo root) error = %v", err)
	}
	dir := t.TempDir()
	writeFile(
		t,
		filepath.Join(dir, "go.mod"),
		"module example.com/"+name+"\n\ngo 1.26.4\n\nrequire github.com/compozy/compozy/sdk/go v0.0.0\n",
	)
	writeFile(t, filepath.Join(dir, "main.go"), goSDKToolProviderSource(name, readOnly))

	edit := exec.CommandContext(
		testutil.Context(t),
		"go",
		"mod",
		"edit",
		"-replace",
		"github.com/compozy/compozy/sdk/go="+filepath.Join(repoRoot, "sdk", "go"),
	)
	edit.Dir = dir
	if output, err := edit.CombinedOutput(); err != nil {
		t.Fatalf("go mod edit -replace error = %v\n%s", err, string(output))
	}

	binary := filepath.Join(dir, "extension-bin")
	build := exec.CommandContext(testutil.Context(t), "go", "build", "-o", binary, ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build Go SDK extension error = %v\n%s", err, string(output))
	}
	return binary
}

func goSDKToolProviderSource(name string, readOnly bool) string {
	return fmt.Sprintf(`package main

import (
	"context"
	"fmt"
	"os"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

type searchInput struct {
	Query string %[1]q
}

func main() {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name:    %[2]q,
		Version: "0.1.0",
		Capabilities: compozysdk.CapabilitiesConfig{
			Provides: []string{"tool.provider"},
		},
	})
	if err := compozysdk.Tool[searchInput](
		extension,
		"search",
		compozysdk.ToolOptions{
			ReadOnly:    %[3]t,
			InputSchema: map[string]any{
				"type": "object",
				"required": []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
			},
		},
		func(_ context.Context, req compozysdk.ToolRequest[searchInput]) (compozysdk.ToolResult, error) {
			return compozysdk.TextResult("go-sdk:" + req.Input.Query), nil
		},
	); err != nil {
		fmt.Fprintf(os.Stderr, "register tool: %%v\n", err)
		os.Exit(1)
	}
	if err := extension.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "run extension: %%v\n", err)
		os.Exit(1)
	}
}
`, `json:"query"`, name, readOnly)
}

func createExtensionToolTestExtension(
	t *testing.T,
	name string,
	command string,
	args []string,
	env map[string]string,
	readOnly bool,
) managerFixture {
	t.Helper()

	dir := t.TempDir()
	writeFile(
		t,
		filepath.Join(dir, manifestJSONFileName),
		extensionToolManifestJSON(name, command, args, env, readOnly),
	)
	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(%q) error = %v", dir, err)
	}
	checksum, err := ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", dir, err)
	}
	return managerFixture{
		dir:      dir,
		manifest: manifest,
		checksum: checksum,
	}
}

func extensionToolManifestJSON(
	name string,
	command string,
	args []string,
	env map[string]string,
	readOnly bool,
) string {
	risk := toolspkg.RiskMutating
	if readOnly {
		risk = toolspkg.RiskRead
	}
	subprocessConfig := map[string]any{
		"command": command,
	}
	if len(args) > 0 {
		subprocessConfig["args"] = args
	}
	if len(env) > 0 {
		subprocessConfig["env"] = env
	}
	payload := map[string]any{
		"extension": map[string]any{
			"name":                name,
			"version":             "0.1.0",
			"description":         "Extension tool provider test fixture",
			"min_compozy_version": "0.5.0",
		},
		"capabilities": map[string]any{
			"provides": []string{extensionprotocol.CapabilityToolProvider},
		},
		"subprocess": subprocessConfig,
		"resources": map[string]any{
			"tools": map[string]any{
				"search": map[string]any{
					"description": "Search extension data",
					"read_only":   readOnly,
					"visibility":  toolspkg.VisibilitySession,
					"risk":        risk,
					"backend": map[string]any{
						"kind":    toolspkg.BackendExtensionHost,
						"handler": "search",
					},
					"input_schema": map[string]any{
						"type":     "object",
						"required": []string{"query"},
						"properties": map[string]any{
							"query": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal extension tool manifest fixture: %v", err))
	}
	return string(data)
}

func readManifestContentAndMetadata(t *testing.T, path string) ([]byte, os.FileInfo) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}
	return content, info
}

func malformedManifestContent(original []byte) []byte {
	malformed := bytes.Repeat([]byte{' '}, len(original))
	malformed[0] = '{'
	return malformed
}

func replaceManifestContentPreservingMetadata(
	t *testing.T,
	path string,
	content []byte,
	info os.FileInfo,
) {
	t.Helper()

	if err := os.WriteFile(path, content, info.Mode()); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v", path, err)
	}
}
