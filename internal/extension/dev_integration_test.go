//go:build integration && !windows

package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestManagerDevelopmentLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should invoke the active subprocess generation and restore the published extension after unlink", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		published := createManagerTestExtension(t, devManifest("dev-tool", "0.1.0", ""), nil)
		installManagerFixture(t, env.registry, published, SourceUser, true)

		workspace := newDevTestWorkspace(t, "workspace-tool-runtime")
		origin := filepath.Join(workspace.RootDir, "tool-extension")
		firstHash := writeDevTestGenerationFiles(t, origin, map[string]string{
			manifestJSONFileName: extensionToolManifestJSON(
				"dev-tool",
				helperCommand(t),
				helperArgs(),
				helperEnv("tool_call_generation_marker", "first"),
				true,
			),
		})
		secondHash := writeDevTestGenerationFiles(t, origin, map[string]string{
			manifestJSONFileName: extensionToolManifestJSON(
				"dev-tool",
				helperCommand(t),
				helperArgs(),
				helperEnv("tool_call_generation_marker", "second"),
				true,
			),
		})
		manager := NewManager(
			env.registry,
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
		)
		startDevTestManager(t, manager)

		if _, err := manager.LinkDevelopmentFromOrigin(
			testutil.Context(t), workspace.WorkspaceID, origin, firstHash,
		); err != nil {
			t.Fatalf("LinkDevelopmentFromOrigin() error = %v", err)
		}
		key := InstanceKey{Name: "dev-tool", WorkspaceID: workspace.WorkspaceID}
		assertDevToolGeneration(t, manager, key, "first")

		if _, err := manager.ReloadExtension(testutil.Context(t), key, secondHash); err != nil {
			t.Fatalf("ReloadExtension() error = %v", err)
		}
		assertDevToolGeneration(t, manager, key, "second")

		if err := manager.UnlinkDevelopment(testutil.Context(t), key); err != nil {
			t.Fatalf("UnlinkDevelopment() error = %v", err)
		}
		active, err := env.registry.ResolveActive(key.Name, key.WorkspaceID)
		if err != nil {
			t.Fatalf("ResolveActive() error = %v", err)
		}
		if active.DevLink != nil || active.Published == nil || active.Published.Version != "0.1.0" {
			t.Fatalf("ResolveActive() = %#v, want published fallback", active)
		}
	})

	t.Run("Should override a published extension and restore it after unlink", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		published := createManagerTestExtension(t, devManifest("dev-clock", "0.1.0", ""), nil)
		installManagerFixture(t, env.registry, published, SourceUser, true)

		workspace := newDevTestWorkspace(t, "workspace-one")
		origin := filepath.Join(workspace.RootDir, "clock-extension")
		firstHash := writeDevTestGeneration(t, origin, devManifest("dev-clock", "0.2.0", ""))
		secondHash := writeDevTestGeneration(t, origin, devManifest("dev-clock", "0.3.0", ""))
		manager := NewManager(
			env.registry,
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
		)
		startDevTestManager(t, manager)

		linked, err := manager.LinkDevelopmentFromOrigin(
			testutil.Context(t),
			workspace.WorkspaceID,
			origin,
			firstHash,
		)
		if err != nil {
			t.Fatalf("LinkDevelopmentFromOrigin() error = %v", err)
		}
		if !linked.OverridesPublished || linked.Status.WorkspaceID != workspace.WorkspaceID {
			t.Fatalf("linked extension = %#v, want workspace override", linked)
		}

		reloaded, err := manager.ReloadExtension(
			testutil.Context(t),
			InstanceKey{Name: "dev-clock", WorkspaceID: workspace.WorkspaceID},
			secondHash,
		)
		if err != nil {
			t.Fatalf("ReloadExtension() error = %v", err)
		}
		if reloaded.Status.GenerationHash != secondHash || reloaded.Status.LastGoodGeneration != secondHash {
			t.Fatalf("reloaded status = %#v, want generation %q", reloaded.Status, secondHash)
		}

		key := InstanceKey{Name: "dev-clock", WorkspaceID: workspace.WorkspaceID}
		if err := manager.UnlinkDevelopment(testutil.Context(t), key); err != nil {
			t.Fatalf("UnlinkDevelopment() error = %v", err)
		}
		active, err := env.registry.ResolveActive("dev-clock", workspace.WorkspaceID)
		if err != nil {
			t.Fatalf("ResolveActive() error = %v", err)
		}
		if active.DevLink != nil || active.Published == nil || active.Published.Version != "0.1.0" {
			t.Fatalf("ResolveActive() = %#v, want published fallback", active)
		}
	})

	t.Run("Should isolate the same extension name across workspaces", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		firstWorkspace := newDevTestWorkspace(t, "workspace-alpha")
		secondWorkspace := newDevTestWorkspace(t, "workspace-beta")
		resolver := newHostAPIFakeWorkspaceResolver(firstWorkspace)
		resolver.upsert(secondWorkspace)
		manager := NewManager(env.registry, WithWorkspaceResolver(resolver))
		startDevTestManager(t, manager)

		firstOrigin := filepath.Join(firstWorkspace.RootDir, "same-extension")
		secondOrigin := filepath.Join(secondWorkspace.RootDir, "same-extension")
		firstHash := writeDevTestGeneration(t, firstOrigin, devManifest("same-name", "1.0.0", ""))
		secondHash := writeDevTestGeneration(t, secondOrigin, devManifest("same-name", "2.0.0", ""))
		if _, err := manager.LinkDevelopmentFromOrigin(
			testutil.Context(t), firstWorkspace.WorkspaceID, firstOrigin, firstHash,
		); err != nil {
			t.Fatalf("LinkDevelopmentFromOrigin(first) error = %v", err)
		}
		if _, err := manager.LinkDevelopmentFromOrigin(
			testutil.Context(t), secondWorkspace.WorkspaceID, secondOrigin, secondHash,
		); err != nil {
			t.Fatalf("LinkDevelopmentFromOrigin(second) error = %v", err)
		}

		first, err := manager.GetForInstance(InstanceKey{
			Name: "same-name", WorkspaceID: firstWorkspace.WorkspaceID,
		})
		if err != nil {
			t.Fatalf("GetForInstance(first) error = %v", err)
		}
		second, err := manager.GetForInstance(InstanceKey{
			Name: "same-name", WorkspaceID: secondWorkspace.WorkspaceID,
		})
		if err != nil {
			t.Fatalf("GetForInstance(second) error = %v", err)
		}
		if first.Status.GenerationHash != firstHash || second.Status.GenerationHash != secondHash {
			t.Fatalf("workspace generations = %q/%q, want %q/%q", first.Status.GenerationHash, second.Status.GenerationHash, firstHash, secondHash)
		}
		if first.RootDir == second.RootDir {
			t.Fatalf("workspace roots both = %q, want isolated generations", first.RootDir)
		}
	})

	t.Run("Should restore the last good generation after activation failure", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		workspace := newDevTestWorkspace(t, "workspace-last-good")
		origin := filepath.Join(workspace.RootDir, "resilient-extension")
		goodHash := writeDevTestGeneration(t, origin, devManifest("resilient", "1.0.0", ""))
		badHash := writeDevTestGeneration(t, origin, devManifest("resilient", "2.0.0", "./missing-extension-binary"))
		manager := NewManager(
			env.registry,
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
		)
		startDevTestManager(t, manager)
		if _, err := manager.LinkDevelopmentFromOrigin(
			testutil.Context(t), workspace.WorkspaceID, origin, goodHash,
		); err != nil {
			t.Fatalf("LinkDevelopmentFromOrigin() error = %v", err)
		}

		key := InstanceKey{Name: "resilient", WorkspaceID: workspace.WorkspaceID}
		if _, err := manager.ReloadExtension(testutil.Context(t), key, badHash); err == nil {
			t.Fatal("ReloadExtension(bad generation) error = nil, want activation failure")
		}
		current, err := manager.GetForInstance(key)
		if err != nil {
			t.Fatalf("GetForInstance() error = %v", err)
		}
		if current.Status.GenerationHash != goodHash || current.Status.LastGoodGeneration != goodHash {
			t.Fatalf("current status = %#v, want last-good generation %q", current.Status, goodHash)
		}
		if current.Status.FailureCode != extensionFailureActivationFailed ||
			!strings.Contains(current.Status.LastError, extensionFailureActivationFailed) {
			t.Fatalf("current failure = %#v, want honest activation failure", current.Status)
		}
		link, err := env.registry.GetDevLink(key.Name, key.WorkspaceID)
		if err != nil {
			t.Fatalf("GetDevLink() error = %v", err)
		}
		if link.BundleGeneration != goodHash {
			t.Fatalf("persisted generation = %q, want %q", link.BundleGeneration, goodHash)
		}
	})
}

func TestManagerDevelopmentReloadConcurrency(t *testing.T) {
	t.Parallel()

	env := newRegistryTestEnv(t)
	firstWorkspace := newDevTestWorkspace(t, "workspace-concurrent")
	secondWorkspace := newDevTestWorkspace(t, "workspace-independent")
	resolver := newHostAPIFakeWorkspaceResolver(firstWorkspace)
	resolver.upsert(secondWorkspace)
	manager := NewManager(env.registry, WithWorkspaceResolver(resolver))
	startDevTestManager(t, manager)

	firstOrigin := filepath.Join(firstWorkspace.RootDir, "concurrent-extension")
	secondOrigin := filepath.Join(secondWorkspace.RootDir, "concurrent-extension")
	firstInitial := writeDevTestGeneration(t, firstOrigin, devManifest("concurrent", "0.1.0", ""))
	secondInitial := writeDevTestGeneration(t, secondOrigin, devManifest("concurrent", "0.2.0", ""))
	if _, err := manager.LinkDevelopmentFromOrigin(
		testutil.Context(t), firstWorkspace.WorkspaceID, firstOrigin, firstInitial,
	); err != nil {
		t.Fatalf("LinkDevelopmentFromOrigin(first) error = %v", err)
	}
	if _, err := manager.LinkDevelopmentFromOrigin(
		testutil.Context(t), secondWorkspace.WorkspaceID, secondOrigin, secondInitial,
	); err != nil {
		t.Fatalf("LinkDevelopmentFromOrigin(second) error = %v", err)
	}

	firstHashes := make([]string, 0, 10)
	for index := range 10 {
		firstHashes = append(firstHashes, writeDevTestGeneration(
			t,
			firstOrigin,
			devManifest("concurrent", fmt.Sprintf("1.%d.0", index), ""),
		))
	}
	secondHash := writeDevTestGeneration(t, secondOrigin, devManifest("concurrent", "3.0.0", ""))

	errorsByReload := make(chan error, len(firstHashes)+1)
	var reloads sync.WaitGroup
	for _, generationHash := range firstHashes {
		reloads.Add(1)
		go func(hash string) {
			defer reloads.Done()
			_, err := manager.ReloadExtension(testutil.Context(t), InstanceKey{
				Name: "concurrent", WorkspaceID: firstWorkspace.WorkspaceID,
			}, hash)
			errorsByReload <- err
		}(generationHash)
	}
	reloads.Add(1)
	go func() {
		defer reloads.Done()
		_, err := manager.ReloadExtension(testutil.Context(t), InstanceKey{
			Name: "concurrent", WorkspaceID: secondWorkspace.WorkspaceID,
		}, secondHash)
		errorsByReload <- err
	}()
	reloads.Wait()
	close(errorsByReload)
	for err := range errorsByReload {
		if err != nil {
			t.Fatalf("ReloadExtension() error = %v", err)
		}
	}

	assertDevRuntimeMatchesLink(t, manager, env.registry, InstanceKey{
		Name: "concurrent", WorkspaceID: firstWorkspace.WorkspaceID,
	})
	assertDevRuntimeMatchesLink(t, manager, env.registry, InstanceKey{
		Name: "concurrent", WorkspaceID: secondWorkspace.WorkspaceID,
	})
}

func TestManagerDevelopmentCoordinatorBarrier(t *testing.T) {
	t.Parallel()

	env := newRegistryTestEnv(t)
	workspace := newDevTestWorkspace(t, "workspace-barrier")
	origin := filepath.Join(workspace.RootDir, "barrier-extension")
	firstHash := writeDevTestGenerationFiles(t, origin, map[string]string{
		manifestJSONFileName: extensionToolManifestJSON(
			"barrier-tool",
			helperCommand(t),
			helperArgs(),
			helperEnv("tool_call_generation_marker", "first"),
			true,
		),
	})
	manager := NewManager(
		env.registry,
		WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
	)
	startDevTestManager(t, manager)
	if _, err := manager.LinkDevelopmentFromOrigin(
		testutil.Context(t), workspace.WorkspaceID, origin, firstHash,
	); err != nil {
		t.Fatalf("LinkDevelopmentFromOrigin() error = %v", err)
	}
	key := InstanceKey{Name: "barrier-tool", WorkspaceID: workspace.WorkspaceID}
	assertDevToolGeneration(t, manager, key, "first")

	staging, secondHash := stageDevTestGenerationFiles(t, origin, map[string]string{
		manifestJSONFileName: extensionToolManifestJSON(
			"barrier-tool",
			helperCommand(t),
			helperArgs(),
			helperEnv("tool_call_generation_marker", "second"),
			true,
		),
	})
	publish := make(chan struct{})
	publishErr := make(chan error, 1)
	go func() {
		<-publish
		publishErr <- os.Rename(
			staging,
			filepath.Join(origin, "dist", generationPrefix+secondHash),
		)
	}()

	if _, err := manager.ReloadExtension(testutil.Context(t), key, secondHash); !errors.Is(
		err,
		ErrExtensionGenerationInvalid,
	) {
		t.Fatalf("ReloadExtension(staged) error = %v, want ErrExtensionGenerationInvalid", err)
	}
	assertDevToolGeneration(t, manager, key, "first")
	close(publish)
	if err := <-publishErr; err != nil {
		t.Fatalf("publish staged generation error = %v", err)
	}
	if _, err := manager.ReloadExtension(testutil.Context(t), key, secondHash); err != nil {
		t.Fatalf("ReloadExtension(second) error = %v", err)
	}
	assertDevToolGeneration(t, manager, key, "second")

	badHash := writeDevTestGenerationFiles(t, origin, map[string]string{
		manifestJSONFileName: extensionToolManifestJSON(
			"barrier-tool",
			"./missing-extension-binary",
			nil,
			nil,
			true,
		),
	})
	fallback, err := manager.ReloadExtension(testutil.Context(t), key, badHash)
	if err == nil {
		t.Fatal("ReloadExtension(bad) error = nil, want activation failure")
	}
	if fallback == nil || !fallback.Status.Active {
		t.Fatalf("ReloadExtension(bad) = (%#v, %v), want active last-good generation", fallback, err)
	}
	assertDevToolGeneration(t, manager, key, "second")
	current, err := manager.GetForInstance(key)
	if err != nil {
		t.Fatalf("GetForInstance() error = %v", err)
	}
	if current.Status.LastGoodGeneration != secondHash ||
		current.Status.FailureCode != extensionFailureActivationFailed {
		t.Fatalf("post-failure status = %#v, want running second generation", current.Status)
	}

	thirdHash := writeDevTestGenerationFiles(t, origin, map[string]string{
		manifestJSONFileName: extensionToolManifestJSON(
			"barrier-tool",
			helperCommand(t),
			helperArgs(),
			helperEnv("tool_call_generation_marker", "third"),
			true,
		),
	})
	if _, err := manager.ReloadExtension(testutil.Context(t), key, thirdHash); err != nil {
		t.Fatalf("ReloadExtension(third) error = %v", err)
	}
	assertDevToolGeneration(t, manager, key, "third")
}

func TestManagerStartDevelopmentLinks(t *testing.T) {
	t.Parallel()

	t.Run("Should report a missing origin without blocking daemon startup", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		workspace := newDevTestWorkspace(t, "workspace-missing")
		if _, err := env.registry.LinkDev(DevLinkRequest{
			Name:           "missing-origin",
			WorkspaceID:    workspace.WorkspaceID,
			OriginPath:     filepath.Join(workspace.RootDir, "gone"),
			GenerationHash: strings.Repeat("a", 64),
		}); err != nil {
			t.Fatalf("LinkDev() error = %v", err)
		}
		manager := NewManager(
			env.registry,
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
		)
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v, want non-blocking missing origin", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t)); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		extension, err := manager.GetForInstance(InstanceKey{
			Name: "missing-origin", WorkspaceID: workspace.WorkspaceID,
		})
		if err != nil {
			t.Fatalf("GetForInstance() error = %v", err)
		}
		if extension.Status.FailureCode != extensionFailureMissingOrigin ||
			extension.Status.Phase != ExtensionPhase("errored") {
			t.Fatalf("missing-origin status = %#v", extension.Status)
		}
	})
}

func TestVerifyDevGeneration(t *testing.T) {
	t.Parallel()

	t.Run("Should reject malformed hashes and content mutation", func(t *testing.T) {
		t.Parallel()

		origin := filepath.Join(t.TempDir(), "origin")
		validHash := writeDevTestGeneration(t, origin, devManifest("hash-check", "1.0.0", ""))
		for _, hostile := range []string{
			"",
			"../" + validHash,
			"gen-" + validHash,
			strings.ToUpper(validHash),
			validHash[:63],
			validHash + "0",
		} {
			if _, err := verifyDevGeneration(origin, hostile); !errors.Is(err, ErrExtensionGenerationInvalid) {
				t.Fatalf("verifyDevGeneration(%q) error = %v, want ErrExtensionGenerationInvalid", hostile, err)
			}
		}
		writeFile(t, filepath.Join(origin, "dist", generationPrefix+validHash, "tampered.txt"), "tampered")
		if _, err := verifyDevGeneration(origin, validHash); !errors.Is(err, ErrExtensionGenerationInvalid) {
			t.Fatalf("verifyDevGeneration(tampered) error = %v, want ErrExtensionGenerationInvalid", err)
		}
	})

	t.Run("Should reject generation symlinks escaping the dist root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		origin := filepath.Join(root, "origin")
		dist := filepath.Join(origin, "dist")
		outside := filepath.Join(root, "outside")
		if err := os.MkdirAll(dist, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(dist) error = %v", err)
		}
		if err := os.MkdirAll(outside, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(outside) error = %v", err)
		}
		writeFile(t, filepath.Join(outside, manifestTOMLFileName), devManifest("escape", "1.0.0", ""))
		hash, err := ComputeDirectoryChecksum(outside)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(outside) error = %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(dist, generationPrefix+hash)); err != nil {
			t.Fatalf("os.Symlink() error = %v", err)
		}
		if _, err := verifyDevGeneration(origin, hash); !errors.Is(err, ErrExtensionGenerationInvalid) {
			t.Fatalf("verifyDevGeneration(escape) error = %v, want ErrExtensionGenerationInvalid", err)
		}
	})

	t.Run("Should reject an origin outside the workspace root", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		outside := t.TempDir()
		if _, err := canonicalizeDevOrigin(workspaceRoot, outside); err == nil ||
			!strings.Contains(err.Error(), "escapes workspace root") {
			t.Fatalf("canonicalizeDevOrigin(outside) error = %v, want containment rejection", err)
		}
	})

	t.Run("Should reject an origin symlink that resolves outside the workspace root", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		outside := t.TempDir()
		originLink := filepath.Join(workspaceRoot, "linked-origin")
		if err := os.Symlink(outside, originLink); err != nil {
			t.Fatalf("os.Symlink() error = %v", err)
		}
		if _, err := canonicalizeDevOrigin(workspaceRoot, originLink); err == nil ||
			!strings.Contains(err.Error(), "escapes workspace root") {
			t.Fatalf("canonicalizeDevOrigin(symlink escape) error = %v, want containment rejection", err)
		}
	})
}

func newDevTestWorkspace(t *testing.T, id string) *workspacepkg.ResolvedWorkspace {
	t.Helper()
	return &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      id,
			Name:    id,
			RootDir: t.TempDir(),
		},
		WorkspaceID: id,
	}
}

func startDevTestManager(t *testing.T, manager *Manager) {
	t.Helper()
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Errorf("Manager.Stop() error = %v", err)
		}
	})
}

func writeDevTestGeneration(t *testing.T, origin, manifest string) string {
	t.Helper()
	return writeDevTestGenerationFiles(t, origin, map[string]string{
		manifestTOMLFileName: manifest,
	})
}

func writeDevTestGenerationFiles(t *testing.T, origin string, files map[string]string) string {
	t.Helper()
	staging, hash := stageDevTestGenerationFiles(t, origin, files)
	generationDir := filepath.Join(origin, "dist", generationPrefix+hash)
	if err := os.Rename(staging, generationDir); err != nil {
		if removeErr := os.RemoveAll(staging); removeErr != nil {
			t.Errorf("os.RemoveAll(%q) error = %v", staging, removeErr)
		}
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("os.Rename(%q, %q) error = %v", staging, generationDir, err)
		}
	}
	return hash
}

func stageDevTestGenerationFiles(t *testing.T, origin string, files map[string]string) (string, string) {
	t.Helper()
	dist := filepath.Join(origin, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dist, err)
	}
	staging, err := os.MkdirTemp(dist, ".dev-test-generation-")
	if err != nil {
		t.Fatalf("os.MkdirTemp(%q) error = %v", dist, err)
	}
	for relativePath, content := range files {
		writeFile(t, filepath.Join(staging, relativePath), content)
	}
	hash, err := ComputeDirectoryChecksum(staging)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", staging, err)
	}
	return staging, hash
}

func assertDevToolGeneration(t *testing.T, manager *Manager, key InstanceKey, marker string) {
	t.Helper()
	id, err := toolspkg.CanonicalToolID("ext", key.Name, "search")
	if err != nil {
		t.Fatalf("CanonicalToolID() error = %v", err)
	}
	result, err := manager.CallToolForInstance(testutil.Context(t), key, toolspkg.ExtensionToolCallRequest{
		ToolID:  id,
		Handler: "search",
		Input:   json.RawMessage(`{"query":"alpha"}`),
	})
	if err != nil {
		t.Fatalf("CallToolForInstance(%s) error = %v", marker, err)
	}
	want := `generation:` + marker + `:{"query":"alpha"}`
	if len(result.Content) != 1 || result.Content[0].Text != want {
		t.Fatalf("CallToolForInstance(%s) content = %#v, want %q", marker, result.Content, want)
	}
}

func devManifest(name, version, command string) string {
	manifest := fmt.Sprintf(`[extension]
name = %q
version = %q
min_compozy_version = "0.3.0-beta.1"

[capabilities]
provides = []

[permissions]
requires = []
`, name, version)
	if command != "" {
		manifest += fmt.Sprintf("\n[subprocess]\ncommand = %q\n", command)
	}
	return manifest
}

func assertDevRuntimeMatchesLink(
	t *testing.T,
	manager *Manager,
	registry *Registry,
	key InstanceKey,
) {
	t.Helper()
	link, err := registry.GetDevLink(key.Name, key.WorkspaceID)
	if err != nil {
		t.Fatalf("GetDevLink(%s) error = %v", key.runtimeID(), err)
	}
	current, err := manager.GetForInstance(key)
	if err != nil {
		t.Fatalf("GetForInstance(%s) error = %v", key.runtimeID(), err)
	}
	if current.Status.GenerationHash != link.BundleGeneration ||
		current.Status.LastGoodGeneration != link.BundleGeneration {
		t.Fatalf("runtime status = %#v, persisted generation = %q", current.Status, link.BundleGeneration)
	}
}
