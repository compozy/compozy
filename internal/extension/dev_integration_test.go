//go:build integration && !windows

package extensionpkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/extension/agentplugin"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestManagerDevelopmentLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should drain an admitted development candidate before stop snapshots instances", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		workspace := newDevTestWorkspace(t, "workspace-stop-admission")
		origin := filepath.Join(workspace.RootDir, "stopping-extension")
		generationHash := writeDevTestGeneration(
			t,
			origin,
			devManifest("stopping-extension", "1.0.0", "fake-extension"),
		)
		initialized := make(chan struct{})
		releaseInitialize := make(chan struct{})
		process := newFakeProcess(992)
		process.initHook = func() {
			close(initialized)
			<-releaseInitialize
		}
		checker := &CapabilityChecker{}
		sourceSessions := &recordingSourceSessionManager{}
		manager := NewManager(
			env.registry,
			WithCapabilityChecker(checker),
			WithSourceSessionManager(sourceSessions),
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
			withProcessLauncher((&fakeLauncher{queue: []*fakeProcess{process}}).launch),
		)
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}

		key := InstanceKey{Name: "stopping-extension", WorkspaceID: workspace.WorkspaceID}
		linkResult := make(chan error, 1)
		go func() {
			_, err := manager.LinkDevelopment(testutil.Context(t), key, origin, generationHash)
			linkResult <- err
		}()
		<-initialized

		stopResult := make(chan error, 1)
		go func() {
			stopResult <- manager.Stop(testutil.Context(t))
		}()
		<-manager.lifecycleDone()
		close(releaseInitialize)

		if err := <-linkResult; !errors.Is(err, context.Canceled) {
			t.Fatalf("LinkDevelopment() error = %v, want canceled admitted candidate", err)
		}
		if err := <-stopResult; err != nil {
			t.Fatalf("Manager.Stop() error = %v", err)
		}
		manager.mu.RLock()
		started := manager.started
		devInstanceCount := len(manager.devExtensions)
		manager.mu.RUnlock()
		if started || devInstanceCount != 0 {
			t.Fatalf("manager after stop = started %v dev instances %d, want false/0", started, devInstanceCount)
		}
		if _, err := env.registry.GetDevLink(key.Name, key.WorkspaceID); !errors.Is(err, ErrExtensionNotDevLinked) {
			t.Fatalf("GetDevLink(after stopped candidate) error = %v, want ErrExtensionNotDevLinked", err)
		}
		if got := sourceSessions.activeNonce(extensionResourceSource(key)); got != "" {
			t.Fatalf("active source nonce after stopped candidate = %q, want empty", got)
		}
		checker.mu.RLock()
		grantCount := len(checker.grants)
		checker.mu.RUnlock()
		if grantCount != 0 {
			t.Fatalf("capability grant count after stopped candidate = %d, want zero", grantCount)
		}
		select {
		case <-process.Done():
		default:
			t.Fatal("candidate process remained live after Stop()")
		}
	})

	t.Run("Should drain an admitted reload candidate before stop snapshots instances", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		workspace := newDevTestWorkspace(t, "workspace-stop-reload-admission")
		origin := filepath.Join(workspace.RootDir, "stopping-reload-extension")
		firstGeneration := writeDevTestGeneration(
			t,
			origin,
			devManifest("stopping-reload-extension", "1.0.0", "fake-extension"),
		)
		secondGeneration := writeDevTestGeneration(
			t,
			origin,
			devManifest("stopping-reload-extension", "2.0.0", "fake-extension"),
		)
		initialized := make(chan struct{})
		releaseInitialize := make(chan struct{})
		firstProcess := newFakeProcess(993)
		candidateProcess := newFakeProcess(994)
		candidateProcess.initHook = func() {
			close(initialized)
			<-releaseInitialize
		}
		checker := &CapabilityChecker{}
		sourceSessions := &recordingSourceSessionManager{}
		manager := NewManager(
			env.registry,
			WithCapabilityChecker(checker),
			WithSourceSessionManager(sourceSessions),
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
			withProcessLauncher((&fakeLauncher{queue: []*fakeProcess{firstProcess, candidateProcess}}).launch),
		)
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}

		key := InstanceKey{Name: "stopping-reload-extension", WorkspaceID: workspace.WorkspaceID}
		if _, err := manager.LinkDevelopment(testutil.Context(t), key, origin, firstGeneration); err != nil {
			t.Fatalf("LinkDevelopment(initial) error = %v", err)
		}
		reloadResult := make(chan error, 1)
		go func() {
			_, err := manager.ReloadExtension(testutil.Context(t), key, secondGeneration)
			reloadResult <- err
		}()
		<-initialized

		stopResult := make(chan error, 1)
		go func() {
			stopResult <- manager.Stop(testutil.Context(t))
		}()
		<-manager.lifecycleDone()
		close(releaseInitialize)

		if err := <-reloadResult; !errors.Is(err, context.Canceled) {
			t.Fatalf("ReloadExtension() error = %v, want canceled admitted candidate", err)
		}
		if err := <-stopResult; err != nil {
			t.Fatalf("Manager.Stop() error = %v", err)
		}
		manager.mu.RLock()
		started := manager.started
		devInstanceCount := len(manager.devExtensions)
		retained := manager.instanceLocked(key)
		retainedStopped := retained != nil && retained.process == nil && !retained.active &&
			!retained.registered && retained.sessionNonce == "" && retained.phase == ExtensionPhaseStop
		manager.mu.RUnlock()
		if started || devInstanceCount != 1 || !retainedStopped {
			t.Fatalf(
				"manager after stop = started %v dev instances %d retained stopped %v, want false/1/true",
				started,
				devInstanceCount,
				retainedStopped,
			)
		}
		link, err := env.registry.GetDevLink(key.Name, key.WorkspaceID)
		if err != nil {
			t.Fatalf("GetDevLink(after stopped reload) error = %v", err)
		}
		if link.BundleGeneration != firstGeneration {
			t.Fatalf("retained dev link generation = %q, want prior %q", link.BundleGeneration, firstGeneration)
		}
		if got := sourceSessions.activeNonce(extensionResourceSource(key)); got != "" {
			t.Fatalf("active source nonce after stopped reload = %q, want empty", got)
		}
		checker.mu.RLock()
		grantCount := len(checker.grants)
		checker.mu.RUnlock()
		if grantCount != 0 {
			t.Fatalf("capability grant count after stopped reload = %d, want zero", grantCount)
		}
		for _, process := range []*fakeProcess{firstProcess, candidateProcess} {
			select {
			case <-process.Done():
			default:
				t.Fatal("development process remained live after Stop()")
			}
		}
	})

	t.Run("Should clean source resources when an admitted unlink crosses stop", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		resourceKernel, err := resources.NewKernel(env.registry.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}
		workspace := newDevTestWorkspace(t, "workspace-stop-unlink-admission")
		origin := filepath.Join(workspace.RootDir, "stopping-unlink-extension")
		generationHash := writeDevTestGeneration(
			t,
			origin,
			devManifest("stopping-unlink-extension", "1.0.0", "fake-extension"),
		)
		process := newFakeProcess(995)
		manager := NewManager(
			env.registry,
			WithSourceSessionManager(resourceKernel),
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
			withProcessLauncher((&fakeLauncher{queue: []*fakeProcess{process}}).launch),
		)
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}

		key := InstanceKey{Name: "stopping-unlink-extension", WorkspaceID: workspace.WorkspaceID}
		if _, err := manager.LinkDevelopment(testutil.Context(t), key, origin, generationHash); err != nil {
			t.Fatalf("LinkDevelopment() error = %v", err)
		}
		manager.mu.RLock()
		managed := manager.instanceLocked(key)
		if managed == nil {
			manager.mu.RUnlock()
			t.Fatal("linked development extension = nil")
		}
		sessionNonce := managed.sessionNonce
		manager.mu.RUnlock()
		if sessionNonce == "" {
			t.Fatal("linked development extension session nonce = empty")
		}
		seedManagedSourceRecord(t, resourceKernel, key, sessionNonce, "unlink-stop-record")

		shutdownStarted := make(chan struct{})
		releaseShutdown := make(chan struct{})
		process.shutdownFn = func(context.Context) error {
			close(shutdownStarted)
			<-releaseShutdown
			process.close(nil)
			return nil
		}
		unlinkCtx, cancelUnlink := context.WithCancel(testutil.Context(t))
		t.Cleanup(cancelUnlink)
		unlinkResult := make(chan error, 1)
		go func() {
			unlinkResult <- manager.UnlinkDevelopment(unlinkCtx, key)
		}()
		<-shutdownStarted

		stopResult := make(chan error, 1)
		go func() {
			stopResult <- manager.Stop(testutil.Context(t))
		}()
		<-manager.lifecycleDone()
		cancelUnlink()
		<-unlinkCtx.Done()
		close(releaseShutdown)

		if err := <-unlinkResult; err != nil {
			t.Fatalf("UnlinkDevelopment() error = %v", err)
		}
		if err := <-stopResult; err != nil {
			t.Fatalf("Manager.Stop() error = %v", err)
		}
		assertManagedSourceEmpty(t, resourceKernel, env.db, key)
		if _, err := env.registry.GetDevLink(key.Name, key.WorkspaceID); !errors.Is(err, ErrExtensionNotDevLinked) {
			t.Fatalf("GetDevLink(after unlink) error = %v, want ErrExtensionNotDevLinked", err)
		}
		select {
		case <-process.Done():
		default:
			t.Fatal("unlinked development process remained live after Stop()")
		}
	})

	t.Run("Should bind a development link to the resolved workspace registration", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		workspace := newDevTestWorkspace(t, "workspace-registry")
		workspace.WorkspaceID = "workspace-local-identity"
		origin := filepath.Join(workspace.RootDir, "registry-extension")
		generationHash := writeDevTestGeneration(
			t,
			origin,
			devManifest("registry-extension", "0.1.0", ""),
		)
		manager := NewManager(
			env.registry,
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
		)
		startDevTestManager(t, manager)

		linked, err := manager.LinkDevelopmentFromOrigin(
			testutil.Context(t),
			workspace.ID,
			origin,
			generationHash,
		)
		if err != nil {
			t.Fatalf("LinkDevelopmentFromOrigin() error = %v", err)
		}
		if linked.Status.WorkspaceID != workspace.ID {
			t.Fatalf(
				"linked workspace id = %q, want registration %q",
				linked.Status.WorkspaceID,
				workspace.ID,
			)
		}
	})

	t.Run(
		"Should invoke the active subprocess generation and restore the published extension after unlink",
		func(t *testing.T) {
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
		},
	)

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
		key := InstanceKey{Name: "dev-clock", WorkspaceID: workspace.WorkspaceID}
		manager.logRingFor(key).append("log from the first link", firstHash)
		firstSnapshot, err := manager.Logs(key, ExtensionLogCursor{})
		if err != nil {
			t.Fatalf("Logs(first link) error = %v", err)
		}
		if firstSnapshot.StreamEpoch == "" || len(firstSnapshot.Entries) == 0 {
			t.Fatalf("Logs(first link) = %#v, want identified non-empty ring", firstSnapshot)
		}
		firstCursor := ExtensionLogCursor{
			Sequence:    firstSnapshot.Entries[len(firstSnapshot.Entries)-1].Sequence,
			StreamEpoch: firstSnapshot.StreamEpoch,
		}

		reloaded, err := manager.ReloadExtension(
			testutil.Context(t),
			key,
			secondHash,
		)
		if err != nil {
			t.Fatalf("ReloadExtension() error = %v", err)
		}
		if reloaded.Status.GenerationHash != secondHash || reloaded.Status.LastGoodGeneration != secondHash {
			t.Fatalf("reloaded status = %#v, want generation %q", reloaded.Status, secondHash)
		}
		reloadedLogs, err := manager.Logs(key, ExtensionLogCursor{})
		if err != nil {
			t.Fatalf("Logs(reloaded) error = %v", err)
		}
		if reloadedLogs.StreamEpoch != firstSnapshot.StreamEpoch {
			t.Fatalf(
				"Logs(reloaded) stream epoch = %q, want retained %q",
				reloadedLogs.StreamEpoch,
				firstSnapshot.StreamEpoch,
			)
		}

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

		if _, err := manager.LinkDevelopmentFromOrigin(
			testutil.Context(t), workspace.WorkspaceID, origin, secondHash,
		); err != nil {
			t.Fatalf("LinkDevelopmentFromOrigin(relink) error = %v", err)
		}
		logs, err := manager.Logs(key, firstCursor)
		if err != nil {
			t.Fatalf("Logs(relink) error = %v", err)
		}
		if logs.StreamEpoch == firstSnapshot.StreamEpoch {
			t.Fatalf("Logs(relink) stream epoch = %q, want new ring identity", logs.StreamEpoch)
		}
		for _, entry := range logs.Entries {
			if entry.Message == "log from the first link" {
				t.Fatalf("Logs(relink) retained unlinked entry: %#v", logs)
			}
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
			t.Fatalf(
				"workspace generations = %q/%q, want %q/%q",
				first.Status.GenerationHash,
				second.Status.GenerationHash,
				firstHash,
				secondHash,
			)
		}
		if first.RootDir == second.RootDir {
			t.Fatalf("workspace roots both = %q, want isolated generations", first.RootDir)
		}
	})

	t.Run(
		"Should retain the active generation and palette projection after a failed reload [UT-063,IT-018]",
		func(t *testing.T) {
			t.Parallel()

			env := newRegistryTestEnv(t)
			workspace := newDevTestWorkspace(t, "workspace-last-good")
			origin := filepath.Join(workspace.RootDir, "resilient-extension")
			goodHash := writeDevTestGenerationFiles(t, origin, map[string]string{
				manifestJSONFileName: extensionToolPaletteManifestJSON(
					"resilient",
					helperCommand(t),
					helperArgs(),
					helperEnv("tool_call_generation_marker", "first"),
					true,
					"First search",
				),
			})
			badHash := writeDevTestGenerationFiles(t, origin, map[string]string{
				manifestJSONFileName: extensionToolPaletteManifestJSON(
					"resilient",
					"./missing-extension-binary",
					nil,
					nil,
					true,
					"Broken search",
				),
			})
			secondHash := writeDevTestGenerationFiles(t, origin, map[string]string{
				manifestJSONFileName: extensionToolPaletteManifestJSON(
					"resilient",
					helperCommand(t),
					helperArgs(),
					helperEnv("tool_call_generation_marker", "second"),
					true,
					"Second search",
				),
			})
			sourceSessions := &recordingSourceSessionManager{}
			checker := &CapabilityChecker{}
			manager := NewManager(
				env.registry,
				WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
				WithSourceSessionManager(sourceSessions),
				WithCapabilityChecker(checker),
			)
			startDevTestManager(t, manager)
			if _, err := manager.LinkDevelopmentFromOrigin(
				testutil.Context(t), workspace.WorkspaceID, origin, goodHash,
			); err != nil {
				t.Fatalf("LinkDevelopmentFromOrigin() error = %v", err)
			}
			assertCmdPaletteTitle(t, manager, workspace.WorkspaceID, "First search")

			key := InstanceKey{Name: "resilient", WorkspaceID: workspace.WorkspaceID}
			manager.mu.RLock()
			initial := manager.instanceLocked(key)
			if initial == nil {
				manager.mu.RUnlock()
				t.Fatal("initial development extension = nil")
			}
			initialProcess := initial.process
			initialGrantID := initial.capabilityGrantID
			initialNonce := initial.sessionNonce
			manager.mu.RUnlock()
			if initialProcess == nil || initialGrantID == "" || initialNonce == "" {
				t.Fatalf(
					"initial generation authority = process %v, grant %q, nonce %q; want all set",
					initialProcess,
					initialGrantID,
					initialNonce,
				)
			}
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
			assertCmdPaletteTitle(t, manager, workspace.WorkspaceID, "First search")
			link, err := env.registry.GetDevLink(key.Name, key.WorkspaceID)
			if err != nil {
				t.Fatalf("GetDevLink() error = %v", err)
			}
			if link.BundleGeneration != goodHash {
				t.Fatalf("persisted generation = %q, want %q", link.BundleGeneration, goodHash)
			}
			manager.mu.RLock()
			retained := manager.instanceLocked(key)
			retainedProcess := retained.process
			retainedGrantID := retained.capabilityGrantID
			manager.mu.RUnlock()
			if retainedProcess != initialProcess || retainedGrantID != initialGrantID {
				t.Fatalf(
					"failed reload authority = process %v grant %q, want original process %v grant %q",
					retainedProcess,
					retainedGrantID,
					initialProcess,
					initialGrantID,
				)
			}
			if got := sourceSessions.activeNonce(extensionResourceSource(key)); got != initialNonce {
				t.Fatalf("active source nonce after failed reload = %q, want %q", got, initialNonce)
			}
			checker.mu.RLock()
			_, originalGrantActive := checker.grants[initialGrantID]
			grantCount := len(checker.grants)
			checker.mu.RUnlock()
			if !originalGrantActive || grantCount != 1 {
				t.Fatalf(
					"capability grants after failed reload = count %d original active %v, want 1/true",
					grantCount,
					originalGrantActive,
				)
			}

			if _, err := manager.ReloadExtension(testutil.Context(t), key, secondHash); err != nil {
				t.Fatalf("ReloadExtension(clean generation) error = %v", err)
			}
			assertCmdPaletteTitle(t, manager, workspace.WorkspaceID, "Second search")
			manager.mu.RLock()
			replaced := manager.instanceLocked(key)
			replacedGrantID := replaced.capabilityGrantID
			replacedNonce := replaced.sessionNonce
			manager.mu.RUnlock()
			if replacedGrantID == "" || replacedGrantID == initialGrantID || replacedNonce == initialNonce {
				t.Fatalf(
					"clean reload authority = grant %q nonce %q, want new values distinct from %q/%q",
					replacedGrantID,
					replacedNonce,
					initialGrantID,
					initialNonce,
				)
			}
			if got := sourceSessions.activeNonce(extensionResourceSource(key)); got != replacedNonce {
				t.Fatalf("active source nonce after clean reload = %q, want %q", got, replacedNonce)
			}
			checker.mu.RLock()
			_, originalGrantActive = checker.grants[initialGrantID]
			_, replacementGrantActive := checker.grants[replacedGrantID]
			grantCount = len(checker.grants)
			checker.mu.RUnlock()
			if originalGrantActive || !replacementGrantActive || grantCount != 1 {
				t.Fatalf(
					"capability grants after clean reload = count %d old %v new %v, want 1/false/true",
					grantCount,
					originalGrantActive,
					replacementGrantActive,
				)
			}
		},
	)

	t.Run(
		"Should reload portable source generations atomically and retain the last good diagnostics",
		func(t *testing.T) {
			t.Parallel()

			env := newRegistryTestEnv(t)
			workspace := newDevTestWorkspace(t, "workspace-portable")
			origin := filepath.Join(workspace.RootDir, "portable-dev")
			if err := os.MkdirAll(origin, 0o755); err != nil {
				t.Fatalf("MkdirAll(portable origin) error = %v", err)
			}
			writeFile(t, filepath.Join(origin, agentPluginManifestFileName), fmt.Sprintf(
				`{"$schema":%q,"name":"portable-dev","version":"1.0.0"}`,
				agentplugin.PluginSchemaID,
			))
			skillPath := filepath.Join(origin, "skills", "review", "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
				t.Fatalf("MkdirAll(portable skill) error = %v", err)
			}
			writeFile(t, skillPath, "---\nname: review\ndescription: Review changes\n---\nReview the change.\n")
			firstHash, err := ComputeDirectoryChecksum(origin)
			if err != nil {
				t.Fatalf("ComputeDirectoryChecksum(first) error = %v", err)
			}
			homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			manager := NewManager(
				env.registry,
				WithHomePaths(homePaths),
				WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
			)
			startDevTestManager(t, manager)
			if _, err := manager.LinkDevelopmentFromOrigin(
				testutil.Context(t), workspace.WorkspaceID, origin, firstHash,
			); err != nil {
				t.Fatalf("LinkDevelopmentFromOrigin() error = %v", err)
			}

			key := InstanceKey{Name: "portable-dev", WorkspaceID: workspace.WorkspaceID}
			firstLink, err := env.registry.GetDevLink(key.Name, key.WorkspaceID)
			if err != nil {
				t.Fatalf("GetDevLink(first) error = %v", err)
			}
			if firstLink.Format != FormatAgentPlugin || len(firstLink.IngestDiagnostics) != 0 {
				t.Fatalf("first dev link = %#v, want portable without diagnostics", firstLink)
			}
			wantDataDir, err := homePaths.ExtensionDataPath(key.Name, key.WorkspaceID, "")
			if err != nil {
				t.Fatalf("ExtensionDataPath() error = %v", err)
			}
			current, err := manager.GetForInstance(key)
			if err != nil {
				t.Fatalf("GetForInstance(first) error = %v", err)
			}
			if current.Manifest == nil || current.Manifest.Format != FormatAgentPlugin {
				t.Fatalf("current manifest = %#v, want portable", current.Manifest)
			}
			if _, statErr := os.Stat(wantDataDir); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("portable dev data dir stat error = %v, want not created", statErr)
			}

			writeFile(t, skillPath, "---\nname: other\ndescription: Mismatch\n---\nMismatch.\n")
			secondHash, err := ComputeDirectoryChecksum(origin)
			if err != nil {
				t.Fatalf("ComputeDirectoryChecksum(second) error = %v", err)
			}
			if _, err := manager.ReloadExtension(testutil.Context(t), key, secondHash); err != nil {
				t.Fatalf("ReloadExtension(second) error = %v", err)
			}
			secondLink, err := env.registry.GetDevLink(key.Name, key.WorkspaceID)
			if err != nil {
				t.Fatalf("GetDevLink(second) error = %v", err)
			}
			if secondLink.BundleGeneration != secondHash || len(secondLink.IngestDiagnostics) != 1 {
				t.Fatalf("second dev link = %#v, want generation and one diagnostic replaced atomically", secondLink)
			}

			writeFile(t, filepath.Join(origin, agentPluginManifestFileName), fmt.Sprintf(
				`{"$schema":%q,"name":"Invalid Name","version":"1.0.0"}`,
				agentplugin.PluginSchemaID,
			))
			fatalHash, err := ComputeDirectoryChecksum(origin)
			if err != nil {
				t.Fatalf("ComputeDirectoryChecksum(fatal) error = %v", err)
			}
			if _, err := manager.ReloadExtension(testutil.Context(t), key, fatalHash); err == nil {
				t.Fatal("ReloadExtension(fatal manifest) error = nil, want validation failure")
			}
			retainedLink, err := env.registry.GetDevLink(key.Name, key.WorkspaceID)
			if err != nil {
				t.Fatalf("GetDevLink(retained) error = %v", err)
			}
			if retainedLink.BundleGeneration != secondHash ||
				!reflect.DeepEqual(retainedLink.IngestDiagnostics, secondLink.IngestDiagnostics) {
				t.Fatalf(
					"retained dev link = %#v, want last-good generation and diagnostics %#v",
					retainedLink,
					secondLink,
				)
			}
		},
	)
}

func TestManagerDevelopmentReloadConcurrency(t *testing.T) {
	t.Run(
		"Should serialize reloads per instance without blocking other workspaces",
		testManagerDevelopmentReloadConcurrency,
	)
}

func testManagerDevelopmentReloadConcurrency(t *testing.T) {
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
	t.Run(
		"Should activate only generations published beyond the coordinator barrier",
		testManagerDevelopmentCoordinatorBarrier,
	)
}

func testManagerDevelopmentCoordinatorBarrier(t *testing.T) {
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
		originPath := filepath.Join(workspace.RootDir, "gone")
		if _, err := env.registry.LinkDev(DevLinkRequest{
			Name:           "missing-origin",
			WorkspaceID:    workspace.WorkspaceID,
			OriginPath:     originPath,
			GenerationHash: strings.Repeat("a", 64),
		}); err != nil {
			t.Fatalf("LinkDev() error = %v", err)
		}
		var logs bytes.Buffer
		manager := NewManager(
			env.registry,
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
			WithLogger(slog.New(slog.NewTextHandler(&logs, nil))),
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
		assertSafeDevBootFailureProjection(
			t,
			extension,
			extensionFailureMissingOrigin,
			workspace.RootDir,
			originPath,
		)
		if !strings.Contains(logs.String(), originPath) {
			t.Fatalf("local boot log = %q, want detailed origin cause", logs.String())
		}
	})

	t.Run("Should sanitize candidate startup failure without blocking daemon startup", func(t *testing.T) {
		t.Parallel()

		const (
			startupCauseCanary  = "DEV-BOOT-STARTUP-CAUSE-CANARY"
			startupSecretCanary = "sk-dev-boot-startup-secret-123456"
		)
		env := newRegistryTestEnv(t)
		workspace := newDevTestWorkspace(t, "workspace-start-failure")
		originPath := filepath.Join(workspace.RootDir, "start-failure")
		generationHash := writeDevTestGeneration(
			t,
			originPath,
			devManifest("start-failure", "1.0.0", helperCommand(t)),
		)
		if _, err := env.registry.LinkDev(DevLinkRequest{
			Name:           "start-failure",
			WorkspaceID:    workspace.WorkspaceID,
			OriginPath:     originPath,
			GenerationHash: generationHash,
		}); err != nil {
			t.Fatalf("LinkDev() error = %v", err)
		}
		launcher := &devBootFailingLauncher{err: fmt.Errorf(
			"%s at %s api_key=%s",
			startupCauseCanary,
			originPath,
			startupSecretCanary,
		)}
		var logs bytes.Buffer
		manager := NewManager(
			env.registry,
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
			WithLogger(slog.New(slog.NewTextHandler(&logs, nil))),
			withProcessLauncher(launcher.launch),
		)
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v, want non-blocking candidate failure", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t)); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		extension, err := manager.GetForInstance(InstanceKey{
			Name: "start-failure", WorkspaceID: workspace.WorkspaceID,
		})
		if err != nil {
			t.Fatalf("GetForInstance() error = %v", err)
		}
		assertSafeDevBootFailureProjection(
			t,
			extension,
			extensionFailureActivationFailed,
			workspace.RootDir,
			originPath,
			startupCauseCanary,
			startupSecretCanary,
		)
		if got := launcher.launchCount(); got != 1 {
			t.Fatalf("launch count = %d, want one failed candidate launch", got)
		}
		if !strings.Contains(logs.String(), startupCauseCanary) ||
			strings.Contains(logs.String(), startupSecretCanary) ||
			!strings.Contains(logs.String(), "[REDACTED]") {
			t.Fatalf("local boot log = %q, want detailed startup cause with secret redacted", logs.String())
		}
	})

	t.Run("Should sanitize publish activation failure without blocking daemon startup", func(t *testing.T) {
		t.Parallel()

		const (
			activationCauseCanary  = "DEV-BOOT-ACTIVATION-CAUSE-CANARY"
			activationSecretCanary = "sk-dev-boot-activation-secret-123456"
		)
		env := newRegistryTestEnv(t)
		workspace := newDevTestWorkspace(t, "workspace-activation-failure")
		originPath := filepath.Join(workspace.RootDir, "activation-failure")
		generationHash := writeDevTestGeneration(
			t,
			originPath,
			devManifest("activation-failure", "1.0.0", helperCommand(t)),
		)
		if _, err := env.registry.LinkDev(DevLinkRequest{
			Name:           "activation-failure",
			WorkspaceID:    workspace.WorkspaceID,
			OriginPath:     originPath,
			GenerationHash: generationHash,
		}); err != nil {
			t.Fatalf("LinkDev() error = %v", err)
		}
		process := newFakeProcess(979)
		launcher := &fakeLauncher{queue: []*fakeProcess{process}}
		activationErr := fmt.Errorf(
			"%s at %s api_key=%s",
			activationCauseCanary,
			originPath,
			activationSecretCanary,
		)
		var logs bytes.Buffer
		manager := NewManager(
			env.registry,
			WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(workspace)),
			WithSourceSessionManager(devBootFailingSourceSessions{activationErr: activationErr}),
			WithLogger(slog.New(slog.NewTextHandler(&logs, nil))),
			withProcessLauncher(launcher.launch),
		)
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v, want non-blocking activation failure", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t)); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		extension, err := manager.GetForInstance(InstanceKey{
			Name: "activation-failure", WorkspaceID: workspace.WorkspaceID,
		})
		if err != nil {
			t.Fatalf("GetForInstance() error = %v", err)
		}
		assertSafeDevBootFailureProjection(
			t,
			extension,
			extensionFailureActivationFailed,
			workspace.RootDir,
			originPath,
			activationCauseCanary,
			activationSecretCanary,
		)
		if !strings.Contains(logs.String(), activationCauseCanary) ||
			strings.Contains(logs.String(), activationSecretCanary) ||
			!strings.Contains(logs.String(), "[REDACTED]") {
			t.Fatalf("local boot log = %q, want detailed cause with secret redacted", logs.String())
		}
	})

	const (
		rawCauseCanary = "DEV-BOOT-RAW-CAUSE-CANARY"
		secretCanary   = "sk-dev-boot-secret-canary-123456"
	)
	tests := []struct {
		name     string
		wantCode string
		prepare  func(t *testing.T, workspace *workspacepkg.ResolvedWorkspace) (DevLinkRequest, workspacepkg.RuntimeResolver)
	}{
		{
			name:     "Should classify an origin outside its workspace as an activation failure",
			wantCode: extensionFailureActivationFailed,
			prepare: func(t *testing.T, workspace *workspacepkg.ResolvedWorkspace) (DevLinkRequest, workspacepkg.RuntimeResolver) {
				t.Helper()
				return DevLinkRequest{
					OriginPath:     t.TempDir(),
					GenerationHash: strings.Repeat("a", 64),
				}, newHostAPIFakeWorkspaceResolver(workspace)
			},
		},
		{
			name:     "Should classify an escaping generation symlink as an activation failure",
			wantCode: extensionFailureActivationFailed,
			prepare: func(t *testing.T, workspace *workspacepkg.ResolvedWorkspace) (DevLinkRequest, workspacepkg.RuntimeResolver) {
				t.Helper()
				origin := filepath.Join(workspace.RootDir, "symlink-origin")
				dist := filepath.Join(origin, "dist")
				outside := t.TempDir()
				if err := os.MkdirAll(dist, 0o755); err != nil {
					t.Fatalf("os.MkdirAll(%q) error = %v", dist, err)
				}
				writeFile(t, filepath.Join(outside, manifestTOMLFileName), devManifest("boot-symlink", "1.0.0", ""))
				hash, err := ComputeDirectoryChecksum(outside)
				if err != nil {
					t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", outside, err)
				}
				if err := os.Symlink(outside, filepath.Join(dist, generationPrefix+hash)); err != nil {
					t.Fatalf("os.Symlink() error = %v", err)
				}
				return DevLinkRequest{
					OriginPath:     origin,
					GenerationHash: hash,
				}, newHostAPIFakeWorkspaceResolver(workspace)
			},
		},
		{
			name:     "Should classify a message-only development-origin failure as an activation failure",
			wantCode: extensionFailureActivationFailed,
			prepare: func(t *testing.T, workspace *workspacepkg.ResolvedWorkspace) (DevLinkRequest, workspacepkg.RuntimeResolver) {
				t.Helper()
				return DevLinkRequest{
						OriginPath:     filepath.Join(workspace.RootDir, "unused"),
						GenerationHash: strings.Repeat("a", 64),
					}, devBootFailureResolver{err: fmt.Errorf(
						"development origin became unavailable: %s api_key=%s",
						rawCauseCanary,
						secretCanary,
					)}
			},
		},
		{
			name:     "Should classify a raw not-exist error as an activation failure",
			wantCode: extensionFailureActivationFailed,
			prepare: func(t *testing.T, workspace *workspacepkg.ResolvedWorkspace) (DevLinkRequest, workspacepkg.RuntimeResolver) {
				t.Helper()
				return DevLinkRequest{
					OriginPath:     filepath.Join(workspace.RootDir, "unused"),
					GenerationHash: strings.Repeat("a", 64),
				}, devBootFailureResolver{err: os.ErrNotExist}
			},
		},
		{
			name:     "Should classify a wrapped workspace-root failure as a missing origin",
			wantCode: extensionFailureMissingOrigin,
			prepare: func(t *testing.T, workspace *workspacepkg.ResolvedWorkspace) (DevLinkRequest, workspacepkg.RuntimeResolver) {
				t.Helper()
				return DevLinkRequest{
					OriginPath:     filepath.Join(workspace.RootDir, "unused"),
					GenerationHash: strings.Repeat("a", 64),
				}, devBootFailureResolver{err: fmt.Errorf("workspace registration: %w", workspacepkg.ErrWorkspaceRootMissing)}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := newRegistryTestEnv(t)
			workspace := newDevTestWorkspace(
				t,
				"workspace-boot-"+strings.ReplaceAll(strings.ToLower(tt.name), " ", "-"),
			)
			link, resolver := tt.prepare(t, workspace)
			link.Name = "boot-failure"
			link.WorkspaceID = workspace.WorkspaceID
			if _, err := env.registry.LinkDev(DevLinkRequest{
				Name:           link.Name,
				WorkspaceID:    link.WorkspaceID,
				OriginPath:     link.OriginPath,
				GenerationHash: link.GenerationHash,
			}); err != nil {
				t.Fatalf("LinkDev() error = %v", err)
			}
			launcher := &fakeLauncher{}
			var logs bytes.Buffer
			manager := NewManager(
				env.registry,
				WithWorkspaceResolver(resolver),
				WithLogger(slog.New(slog.NewTextHandler(&logs, nil))),
				withProcessLauncher(launcher.launch),
			)
			if err := manager.Start(testutil.Context(t)); err != nil {
				t.Fatalf("Start() error = %v, want non-blocking boot failure", err)
			}
			t.Cleanup(func() {
				if err := manager.Stop(testutil.Context(t)); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			})

			key := InstanceKey{Name: link.Name, WorkspaceID: link.WorkspaceID}
			extension, err := manager.GetForInstance(key)
			if err != nil {
				t.Fatalf("GetForInstance() error = %v", err)
			}
			if extension.Status.FailureCode != tt.wantCode || extension.Status.Phase != ExtensionPhase("errored") ||
				extension.Status.WorkspaceID != workspace.WorkspaceID {
				t.Fatalf(
					"boot failure status = %#v, want %q errored in workspace %q",
					extension.Status,
					tt.wantCode,
					workspace.WorkspaceID,
				)
			}
			assertSafeDevBootFailureProjection(
				t,
				extension,
				tt.wantCode,
				workspace.RootDir,
				link.OriginPath,
				rawCauseCanary,
				secretCanary,
			)
			if strings.Contains(tt.name, "message-only") {
				if !strings.Contains(logs.String(), rawCauseCanary) {
					t.Fatalf("local boot log = %q, want detailed raw-cause canary", logs.String())
				}
				if strings.Contains(logs.String(), secretCanary) || !strings.Contains(logs.String(), "[REDACTED]") {
					t.Fatalf("local boot log = %q, want secret redacted", logs.String())
				}
			}
			if got := launcher.launchCount(); got != 0 {
				t.Fatalf("launch count = %d, want no candidate process", got)
			}
			manager.mu.RLock()
			managed := manager.instanceLocked(key)
			published := managed != nil &&
				(managed.process != nil || managed.startup != nil || managed.active || managed.registered)
			manager.mu.RUnlock()
			if managed == nil || published {
				t.Fatalf("boot failure candidate = %#v, want an unpublished error record", managed)
			}
			for _, info := range manager.ListForWorkspace("other-workspace") {
				if info.Name == link.Name {
					t.Fatalf("ListForWorkspace(other) leaked %q", link.Name)
				}
			}
		})
	}
}

func assertSafeDevBootFailureProjection(
	t *testing.T,
	extension *Extension,
	wantCode string,
	forbidden ...string,
) {
	t.Helper()

	wantDiagnostic := "extension development activation failed"
	if wantCode == extensionFailureMissingOrigin {
		wantDiagnostic = "extension development origin is unavailable"
	}
	if extension.Status.LastError != wantDiagnostic {
		t.Fatalf("stored boot diagnostic = %q, want %q", extension.Status.LastError, wantDiagnostic)
	}
	payload := DescribeExtension(extension, true, time.Now().UTC())
	if payload.FailureCode != wantCode || payload.LastError != wantDiagnostic {
		t.Fatalf(
			"DescribeExtension() failure = code %q diagnostic %q, want %q/%q",
			payload.FailureCode,
			payload.LastError,
			wantCode,
			wantDiagnostic,
		)
	}
	if payload.OriginPath != "" {
		t.Fatalf("DescribeExtension().OriginPath = %q, want omitted for errored state", payload.OriginPath)
	}
	serialized, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(DescribeExtension()) error = %v", err)
	}
	if !bytes.Contains(serialized, []byte(`"failure_code":"`+wantCode+`"`)) {
		t.Fatalf("DescribeExtension() JSON = %s, want failure code %q", serialized, wantCode)
	}
	if bytes.Contains(serialized, []byte(`"origin_path"`)) {
		t.Fatalf("DescribeExtension() JSON = %s, want origin_path omitted", serialized)
	}
	for _, token := range forbidden {
		if token = strings.TrimSpace(token); token != "" && bytes.Contains(serialized, []byte(token)) {
			t.Fatalf("DescribeExtension() JSON = %s, leaked forbidden token %q", serialized, token)
		}
	}
}

type devBootFailureResolver struct {
	err error
}

type devBootFailingLauncher struct {
	mu    sync.Mutex
	err   error
	calls int
}

func (l *devBootFailingLauncher) launch(
	ctx context.Context,
	_ subprocess.LaunchConfig,
) (processHandle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	return nil, l.err
}

func (l *devBootFailingLauncher) launchCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

var _ workspacepkg.RuntimeResolver = devBootFailureResolver{}

func (r devBootFailureResolver) Resolve(
	ctx context.Context,
	_ string,
) (workspacepkg.ResolvedWorkspace, error) {
	if err := ctx.Err(); err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	return workspacepkg.ResolvedWorkspace{}, r.err
}

func (r devBootFailureResolver) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	return r.Resolve(ctx, path)
}

type devBootFailingSourceSessions struct {
	activationErr error
}

var _ resources.SourceSessionManager = devBootFailingSourceSessions{}

func (s devBootFailingSourceSessions) ActivateSourceSession(
	ctx context.Context,
	_ resources.MutationActor,
	_ resources.ResourceSource,
	_ string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.activationErr
}

func (devBootFailingSourceSessions) ResetSourceIfActiveSession(
	ctx context.Context,
	_ resources.MutationActor,
	_ resources.ResourceSource,
	_ string,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func (devBootFailingSourceSessions) ResetSource(
	ctx context.Context,
	_ resources.MutationActor,
	_ resources.ResourceSource,
) error {
	return ctx.Err()
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
			if _, err := verifyDevGeneration(origin, hostile, nil); !errors.Is(err, ErrExtensionGenerationInvalid) {
				t.Fatalf("verifyDevGeneration(%q) error = %v, want ErrExtensionGenerationInvalid", hostile, err)
			}
		}
		writeFile(t, filepath.Join(origin, "dist", generationPrefix+validHash, "tampered.txt"), "tampered")
		if _, err := verifyDevGeneration(origin, validHash, nil); !errors.Is(err, ErrExtensionGenerationInvalid) {
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
		if _, err := verifyDevGeneration(origin, hash, nil); !errors.Is(err, ErrExtensionGenerationInvalid) {
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

func extensionToolPaletteManifestJSON(
	name string,
	command string,
	args []string,
	env map[string]string,
	readOnly bool,
	title string,
) string {
	payload := map[string]any{}
	if err := json.Unmarshal(
		[]byte(extensionToolManifestJSON(name, command, args, env, readOnly)),
		&payload,
	); err != nil {
		panic(fmt.Sprintf("decode extension tool manifest fixture: %v", err))
	}
	resources, ok := payload["resources"].(map[string]any)
	if !ok {
		panic("extension tool manifest fixture resources are missing")
	}
	resources["cmd_palette"] = map[string]any{
		"commands": []map[string]any{{
			"id": "search", "title": title, "section": "Development", "icon": "search",
			"action": map[string]any{"kind": "tool", "tool": "search"},
		}},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("encode extension palette manifest fixture: %v", err))
	}
	return string(data)
}

func assertCmdPaletteTitle(t *testing.T, manager *Manager, workspaceID, want string) {
	t.Helper()
	projection, err := manager.CmdPalette(workspaceID, ProfileLens{
		ID: "00000000000000000000000000", Name: "default",
	})
	if err != nil {
		t.Fatalf("Manager.CmdPalette() error = %v", err)
	}
	if len(projection.Commands) != 1 || projection.Commands[0].Title != want {
		t.Fatalf("Manager.CmdPalette() = %#v, want title %q", projection, want)
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
