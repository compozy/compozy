package observe

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/soul"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
)

func TestReconciliationIndexesSessionDirNotInDB(t *testing.T) {
	t.Parallel()

	t.Run("Should index a session directory not already in the registry", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sessionDir := filepath.Join(h.home.SessionsDir, "sess-new")
		metaPath := store.SessionMetaFile(sessionDir)
		now := h.now.Add(30 * time.Minute)
		stopReason := store.StopUserCanceled

		if err := store.WriteSessionMeta(metaPath, &store.SessionMeta{
			ID:            "sess-new",
			InitiatorKind: "extension",
			InitiatorID:   "ext-recovered",
			ProfileID:     store.DefaultProfileID,
			Name:          "New",
			AgentName:     "coder",
			Provider:      "claude",
			WorkspaceID:   h.workspaceID,
			State:         "stopped",
			RuntimeStatus: store.SessionRuntimeUnbound,
			StopReason:    &stopReason,
			StopDetail:    "requested by API",
			StopEscalated: true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}); err != nil {
			t.Fatalf("WriteSessionMeta() error = %v", err)
		}

		result, err := h.observer.Reconcile(testutil.Context(t))
		if err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		slices.Sort(result.Indexed)
		if got, want := result.Indexed, []string{"sess-new"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("Indexed = %#v, want %#v", got, want)
		}

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if got, want := len(sessions), 1; got != want {
			t.Fatalf("len(sessions) = %d, want %d", got, want)
		}
		if sessions[0].State != "stopped" {
			t.Fatalf("sessions[0].State = %q, want stopped", sessions[0].State)
		}
		if sessions[0].InitiatorKind != "extension" || sessions[0].InitiatorID != "ext-recovered" {
			t.Fatalf(
				"reconciliation lost persisted initiator: %q/%q",
				sessions[0].InitiatorKind,
				sessions[0].InitiatorID,
			)
		}
		if sessions[0].StopReason != store.StopUserCanceled {
			t.Fatalf("sessions[0].StopReason = %q, want %q", sessions[0].StopReason, store.StopUserCanceled)
		}
		if !sessions[0].StopEscalated {
			t.Fatal("reconciliation lost persisted stop escalation")
		}
		if sessions[0].StopDetail != "requested by API" {
			t.Fatalf("sessions[0].StopDetail = %q, want %q", sessions[0].StopDetail, "requested by API")
		}
		if sessions[0].Provider != "claude" {
			t.Fatalf("sessions[0].Provider = %q, want claude", sessions[0].Provider)
		}

		meta, err := store.ReadSessionMeta(metaPath)
		if err != nil {
			t.Fatalf("ReadSessionMeta() error = %v", err)
		}
		if meta.State != "stopped" {
			t.Fatalf("meta.State = %q, want stopped", meta.State)
		}
	})
}

func TestReconciliationPreservesDurableSessionProjectionMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve failure lineage and soul provenance metadata", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		rootID := "sess-a-root"
		parentID := "sess-b-parent"
		childID := "sess-c-child"
		now := h.now.Add(35 * time.Minute)
		stopReason := store.StopAgentCrashed
		ttl := now.Add(time.Hour)
		acpSessionID := "acp-child"
		creationProfile := store.SessionCreationProfile{
			Version: store.SessionCreationProfileVersion, AgentName: "coder", Provider: "claude",
			ProfileID:   store.DefaultProfileID,
			WorkspaceID: h.workspaceID, CWD: h.workspace,
			Permissions: "approve-reads",
		}
		creationOptions := store.SessionCreationOptions{
			SessionID:   childID,
			Name:        "Child",
			SessionType: string(session.SessionTypeSpawned),
		}
		creationProfileRef, err := creationProfile.Ref()
		if err != nil {
			t.Fatalf("SessionCreationProfile.Ref() error = %v", err)
		}
		policySpecDigest, err := creationProfile.PolicySpecDigest()
		if err != nil {
			t.Fatalf("SessionCreationProfile.PolicySpecDigest() error = %v", err)
		}
		creationDigest, err := creationProfile.CreationDigest(creationOptions)
		if err != nil {
			t.Fatalf("SessionCreationProfile.CreationDigest() error = %v", err)
		}
		creationIdentity := store.SessionCreationIdentity{
			CreationProfileRef: creationProfileRef,
			PolicySpecDigest:   policySpecDigest,
			CreationDigest:     creationDigest,
		}
		snapshot, err := h.registry.UpsertSoulSnapshot(testutil.Context(t), soul.Snapshot{
			ID:          "soul-snapshot-1",
			WorkspaceID: h.workspaceID,
			AgentName:   "coder",
			SourcePath:  "AGENT.md",
			Digest:      "soul-digest-1",
			ProfileJSON: json.RawMessage("{\"schema_version\":1}"),
			Body:        "active soul profile",
			CreatedAt:   now,
		})
		if err != nil {
			t.Fatalf("UpsertSoulSnapshot() error = %v", err)
		}

		if err := store.WriteSessionMeta(
			store.SessionMetaFile(filepath.Join(h.home.SessionsDir, rootID)), &store.SessionMeta{
				ID:            rootID,
				ProfileID:     store.DefaultProfileID,
				Name:          "Root",
				AgentName:     "coder",
				Provider:      "claude",
				WorkspaceID:   h.workspaceID,
				State:         "stopped",
				RuntimeStatus: store.SessionRuntimeUnbound,
				CreatedAt:     now,
				UpdatedAt:     now,
			}); err != nil {
			t.Fatalf("WriteSessionMeta(root) error = %v", err)
		}
		if err := store.WriteSessionMeta(
			store.SessionMetaFile(filepath.Join(h.home.SessionsDir, parentID)), &store.SessionMeta{
				ID:            parentID,
				ProfileID:     store.DefaultProfileID,
				Name:          "Parent",
				AgentName:     "coder",
				Provider:      "claude",
				WorkspaceID:   h.workspaceID,
				State:         "stopped",
				RuntimeStatus: store.SessionRuntimeUnbound,
				Lineage: &store.SessionLineage{
					ParentSessionID: rootID,
					RootSessionID:   rootID,
					SpawnDepth:      1,
					Kind:            store.LineageKindProvenance,
				},
				CreatedAt: now,
				UpdatedAt: now,
			}); err != nil {
			t.Fatalf("WriteSessionMeta(parent) error = %v", err)
		}
		if err := store.WriteSessionMeta(
			store.SessionMetaFile(filepath.Join(h.home.SessionsDir, childID)), &store.SessionMeta{
				ID:              childID,
				ProfileID:       store.DefaultProfileID,
				Name:            "Child",
				AgentName:       "coder",
				Provider:        "claude",
				Model:           " gpt-5.6 ",
				ReasoningEffort: " high ",
				Speed:           speedpkg.SpeedFast,
				SessionRuntimeDetails: &store.SessionRuntimeDetails{
					ACPOptions: []store.SessionACPOptionSelection{{ID: "mode", ValueID: "plan"}},
				},
				RuntimeStatus:     store.SessionRuntimeReady,
				RuntimeTransition: store.SessionRuntimeTransitionProcessReplacement,
				RuntimeGeneration: 2,
				SessionRuntimeBindingState: &store.SessionRuntimeBindingState{
					RuntimeFailure: store.SessionRuntimeFailurePointer(" runtime warning "),
					RuntimeSelection: store.NewSessionRuntimeSelectionState(&store.SessionRuntimeSelection{
						Provider:        "cursor",
						Model:           "grok-4.6",
						ReasoningEffort: "xhigh",
						Speed:           speedpkg.SpeedFast,
						ACPOptions: []store.SessionACPOptionSelection{{
							ID: "thinking", BoolValue: new(true),
						}},
					}, 1),
				},
				WorkspaceID:  h.workspaceID,
				SessionType:  string(session.SessionTypeSpawned),
				State:        "stopped",
				ACPSessionID: &acpSessionID,
				StopReason:   &stopReason,
				StopDetail:   "agent process exited",
				Failure: &store.SessionFailure{
					Kind:            store.FailureProcess,
					Summary:         "agent exited with status 1",
					CrashBundlePath: "/tmp/compozy-crash-bundle",
				},
				Lineage: &store.SessionLineage{
					ParentSessionID:  parentID,
					RootSessionID:    rootID,
					SpawnDepth:       2,
					SpawnRole:        "delegate_task",
					Kind:             store.LineageKindSpawn,
					TTLExpiresAt:     &ttl,
					AutoStopOnParent: true,
					SpawnBudget: store.SessionSpawnBudget{
						MaxChildren:           2,
						MaxDepth:              3,
						TTLSeconds:            3600,
						MaxActivePerWorkspace: 1,
					},
					PermissionPolicy: store.SessionPermissionPolicy{
						Skills:         []string{"skill-alpha"},
						WorkspacePaths: []string{h.workspace},
					},
				},
				SoulSnapshotID:     " " + snapshot.ID + " ",
				SoulDigest:         " " + snapshot.Digest + " ",
				ParentSoulDigest:   " parent-soul-digest ",
				CreationProfile:    &creationProfile,
				CreationOptions:    &creationOptions,
				CreationProfileRef: creationIdentity.CreationProfileRef,
				PolicySpecDigest:   creationIdentity.PolicySpecDigest,
				CreationDigest:     creationIdentity.CreationDigest,
				CreatedAt:          now,
				UpdatedAt:          now,
			}); err != nil {
			t.Fatalf("WriteSessionMeta(child) error = %v", err)
		}

		result, err := h.observer.Reconcile(testutil.Context(t))
		if err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		slices.Sort(result.Indexed)
		if got, want := result.Indexed, []string{rootID, parentID, childID}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("Indexed = %#v, want %#v", got, want)
		}

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if got, want := len(sessions), 3; got != want {
			t.Fatalf("len(sessions) = %d, want %d", got, want)
		}
		indexed := store.SessionInfo{}
		for _, session := range sessions {
			if session.ID == childID {
				indexed = session
				break
			}
		}
		if indexed.ID == "" {
			t.Fatalf("ListSessions() = %#v, want child session", sessions)
		}
		if indexed.Model != "gpt-5.6" || indexed.ReasoningEffort != "high" ||
			indexed.RuntimeFailure != "runtime warning" {
			t.Fatalf("indexed runtime metadata = %#v, want trimmed metadata", indexed)
		}
		if indexed.Speed != speedpkg.SpeedFast || indexed.RuntimeStatus != store.SessionRuntimeReady ||
			indexed.RuntimeTransition != store.SessionRuntimeTransitionProcessReplacement ||
			indexed.RuntimeGeneration != 2 {
			t.Fatalf("indexed effective runtime = %#v, want complete durable runtime", indexed)
		}
		indexedOptions := indexed.ACPOptionsValue()
		if len(indexedOptions) != 1 || indexedOptions[0].ID != "mode" ||
			indexedOptions[0].ValueID != "plan" {
			t.Fatalf("indexed ACP options = %#v, want mode=plan", indexedOptions)
		}
		if indexed.SelectedRuntime == nil || indexed.SelectedRuntime.Provider != "cursor" ||
			indexed.SelectedRuntime.Model != "grok-4.6" ||
			indexed.SelectedRuntime.ReasoningEffort != "xhigh" ||
			indexed.SelectedRuntime.Speed != speedpkg.SpeedFast || indexed.RuntimeSelectionRevision != 1 {
			t.Fatalf(
				"indexed selected runtime = %#v revision %d, want Grok 4.6 xhigh fast at revision 1",
				indexed.SelectedRuntime,
				indexed.RuntimeSelectionRevision,
			)
		}
		if len(indexed.SelectedRuntime.ACPOptions) != 1 ||
			indexed.SelectedRuntime.ACPOptions[0].ID != "thinking" ||
			indexed.SelectedRuntime.ACPOptions[0].BoolValue == nil ||
			!*indexed.SelectedRuntime.ACPOptions[0].BoolValue {
			t.Fatalf("indexed selected ACP options = %#v, want thinking=true", indexed.SelectedRuntime.ACPOptions)
		}
		if indexed.Lineage == nil ||
			indexed.Lineage.ParentSessionID != parentID ||
			indexed.Lineage.RootSessionID != rootID ||
			indexed.Lineage.SpawnDepth != 2 ||
			indexed.Lineage.SpawnRole != "delegate_task" ||
			indexed.Lineage.TTLExpiresAt == nil ||
			!indexed.Lineage.TTLExpiresAt.Equal(ttl) ||
			!indexed.Lineage.AutoStopOnParent {
			t.Fatalf("indexed.Lineage = %#v, want durable lineage metadata", indexed.Lineage)
		}
		if indexed.Failure == nil || indexed.Failure.Kind != store.FailureProcess {
			t.Fatalf("indexed.Failure = %#v, want process failure", indexed.Failure)
		}
		if got, want := indexed.Failure.Summary, "agent exited with status 1"; got != want {
			t.Fatalf("indexed.Failure.Summary = %q, want %q", got, want)
		}
		if got, want := indexed.Failure.CrashBundlePath, "/tmp/compozy-crash-bundle"; got != want {
			t.Fatalf("indexed.Failure.CrashBundlePath = %q, want %q", got, want)
		}
		if got, want := indexed.SoulSnapshotID, "soul-snapshot-1"; got != want {
			t.Fatalf("indexed.SoulSnapshotID = %q, want %q", got, want)
		}
		if got, want := indexed.SoulDigest, "soul-digest-1"; got != want {
			t.Fatalf("indexed.SoulDigest = %q, want %q", got, want)
		}
		if got, want := indexed.ParentSoulDigest, "parent-soul-digest"; got != want {
			t.Fatalf("indexed.ParentSoulDigest = %q, want %q", got, want)
		}
		indexedIdentity, err := h.registry.GetSessionCreationIdentity(testutil.Context(t), childID)
		if err != nil {
			t.Fatalf("GetSessionCreationIdentity() error = %v", err)
		}
		if indexedIdentity != creationIdentity {
			t.Fatalf("GetSessionCreationIdentity() = %#v, want %#v", indexedIdentity, creationIdentity)
		}
		indexedProfile, err := h.registry.GetSessionCreationProfile(testutil.Context(t), creationProfileRef)
		if err != nil {
			t.Fatalf("GetSessionCreationProfile() error = %v", err)
		}
		indexedProfileJSON, err := indexedProfile.CanonicalJSON()
		if err != nil {
			t.Fatalf("indexed SessionCreationProfile.CanonicalJSON() error = %v", err)
		}
		wantProfileJSON, err := creationProfile.CanonicalJSON()
		if err != nil {
			t.Fatalf("expected SessionCreationProfile.CanonicalJSON() error = %v", err)
		}
		if !bytes.Equal(indexedProfileJSON, wantProfileJSON) {
			t.Fatalf("GetSessionCreationProfile() = %s, want %s", indexedProfileJSON, wantProfileJSON)
		}
		registration, err := h.registry.RegisterSessionWithCreationIdentity(
			testutil.Context(t),
			&(indexed),
			creationIdentity,
		)
		if err != nil {
			t.Fatalf("RegisterSessionWithCreationIdentity(reuse) error = %v", err)
		}
		if registration.Created {
			t.Fatal("RegisterSessionWithCreationIdentity(reuse) created duplicate session")
		}

		health, err := h.observer.Health(testutil.Context(t))
		if err != nil {
			t.Fatalf("Health() error = %v", err)
		}
		if got, want := health.Failures.Total, 1; got != want {
			t.Fatalf("Health().Failures.Total = %d, want %d", got, want)
		}
		if got, want := health.Failures.ByKind[store.FailureProcess], 1; got != want {
			t.Fatalf("Health().Failures.ByKind[process] = %d, want %d", got, want)
		}
		if got, want := len(health.Failures.Recent), 1; got != want {
			t.Fatalf("len(Health().Failures.Recent) = %d, want %d", got, want)
		}
		if recent := health.Failures.Recent[0]; recent.SessionID != childID ||
			recent.FailureKind != store.FailureProcess ||
			recent.Summary != "agent exited with status 1" {
			t.Fatalf("Health().Failures.Recent[0] = %#v, want reconstructed failure", recent)
		}
	})
}

func TestReconciliationPreservesWorktreeBinding(t *testing.T) {
	t.Parallel()

	t.Run("Should refresh an identity-bound session without losing its worktree", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		ctx := testutil.Context(t)
		now := h.now.Add(40 * time.Minute)
		sessionID := "sess-worktree-bound"
		worktreeID := "wt-session-reconcile"
		worktreePath := filepath.Join(h.workspace, ".worktrees", "session-reconcile")
		if err := h.registry.Worktrees.Insert(ctx, worktreepkg.Worktree{
			ProfileID:   store.DefaultProfileID,
			ID:          worktreeID,
			WorkspaceID: h.workspaceID,
			Name:        "session-reconcile",
			Branch:      "fix/session-reconcile",
			Path:        worktreePath,
			State:       worktreepkg.StateReady,
			Origin:      worktreepkg.OriginManual,
			SetupState:  worktreepkg.SetupNone,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			t.Fatalf("Insert(worktree) error = %v", err)
		}

		creationProfile := store.SessionCreationProfile{
			Version: store.SessionCreationProfileVersion, AgentName: "coder", Provider: "claude",
			ProfileID:   store.DefaultProfileID,
			WorkspaceID: h.workspaceID, CWD: worktreePath, WorktreeRef: worktreeID,
			Permissions: "approve-reads",
		}
		creationOptions := store.SessionCreationOptions{
			SessionID:   sessionID,
			Name:        "Worktree-bound session",
			SessionType: "user",
		}
		creationProfileRef, err := h.registry.PutSessionCreationProfile(ctx, creationProfile)
		if err != nil {
			t.Fatalf("PutSessionCreationProfile() error = %v", err)
		}
		policySpecDigest, err := creationProfile.PolicySpecDigest()
		if err != nil {
			t.Fatalf("SessionCreationProfile.PolicySpecDigest() error = %v", err)
		}
		creationDigest, err := creationProfile.CreationDigest(creationOptions)
		if err != nil {
			t.Fatalf("SessionCreationProfile.CreationDigest() error = %v", err)
		}
		creationIdentity := store.SessionCreationIdentity{
			CreationProfileRef: creationProfileRef,
			PolicySpecDigest:   policySpecDigest,
			CreationDigest:     creationDigest,
		}
		info := store.SessionInfo{
			ProfileID:     store.DefaultProfileID,
			ID:            sessionID,
			Name:          creationOptions.Name,
			AgentName:     creationProfile.AgentName,
			Provider:      creationProfile.Provider,
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   h.workspaceID,
			WorktreeID:    worktreeID,
			SessionType:   creationOptions.SessionType,
			State:         "stopped",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if _, err := h.registry.RegisterSessionWithCreationIdentity(ctx, &(info), creationIdentity); err != nil {
			t.Fatalf("RegisterSessionWithCreationIdentity() error = %v", err)
		}

		meta := store.SessionMeta{
			ProfileID:          store.DefaultProfileID,
			ID:                 sessionID,
			Name:               creationOptions.Name,
			AgentName:          creationProfile.AgentName,
			Provider:           creationProfile.Provider,
			RuntimeStatus:      store.SessionRuntimeUnbound,
			WorkspaceID:        h.workspaceID,
			SessionType:        creationOptions.SessionType,
			State:              "stopped",
			CreationProfile:    &creationProfile,
			CreationOptions:    &creationOptions,
			CreationProfileRef: creationIdentity.CreationProfileRef,
			PolicySpecDigest:   creationIdentity.PolicySpecDigest,
			CreationDigest:     creationIdentity.CreationDigest,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		meta.SetCWD(worktreePath)
		meta.SetWorktreeID(worktreeID)
		if err := store.WriteSessionMeta(
			store.SessionMetaFile(filepath.Join(h.home.SessionsDir, sessionID)), &meta); err != nil {
			t.Fatalf("WriteSessionMeta() error = %v", err)
		}

		firstResult, err := h.observer.Reconcile(ctx)
		if err != nil {
			t.Fatalf("first Reconcile() error = %v", err)
		}
		if len(firstResult.Indexed) != 0 {
			t.Fatalf("first Indexed = %#v, want empty for an existing session", firstResult.Indexed)
		}

		secondResult, err := h.observer.Reconcile(ctx)
		if err != nil {
			t.Fatalf("second Reconcile() error = %v", err)
		}
		if len(secondResult.Indexed) != 0 {
			t.Fatalf("second Indexed = %#v, want empty for an existing session", secondResult.Indexed)
		}
		sessions, err := h.registry.ListSessions(ctx, store.SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if got, want := len(sessions), 1; got != want {
			t.Fatalf("len(sessions) = %d, want %d", got, want)
		}
		if got, want := sessions[0].WorktreeID, worktreeID; got != want {
			t.Fatalf("sessions[0].WorktreeID = %q, want %q", got, want)
		}
	})
}

func TestReconciliationMarksMissingDirectoryAsOrphaned(t *testing.T) {
	t.Parallel()

	t.Run("Should mark indexed sessions missing from disk as orphaned", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		now := h.now
		if err := h.observer.registry.RegisterSession(testutil.Context(t), new(store.SessionInfo{
			ID:            "sess-orphan",
			ProfileID:     store.DefaultProfileID,
			Name:          "Orphan",
			AgentName:     "coder",
			Provider:      "claude",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   h.workspaceID,
			State:         "active",
			CreatedAt:     now,
			UpdatedAt:     now,
		})); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		result, err := h.observer.Reconcile(testutil.Context(t))
		if err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		slices.Sort(result.Orphaned)
		if got, want := result.Orphaned, []string{"sess-orphan"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("Orphaned = %#v, want %#v", got, want)
		}

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if got, want := len(sessions), 1; got != want {
			t.Fatalf("len(sessions) = %d, want %d", got, want)
		}
		if sessions[0].State != "orphaned" {
			t.Fatalf("sessions[0].State = %q, want orphaned", sessions[0].State)
		}
	})
}

func TestReconciliationSkipsSessionMetadataWithoutProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should skip invalid metadata without mutating it and continue indexing valid sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)

		validDir := filepath.Join(h.home.SessionsDir, "sess-valid")
		validMetaPath := store.SessionMetaFile(validDir)
		now := h.now.Add(45 * time.Minute)
		if err := store.WriteSessionMeta(validMetaPath, &store.SessionMeta{
			ID:            "sess-valid",
			ProfileID:     store.DefaultProfileID,
			Name:          "Valid",
			AgentName:     "coder",
			Provider:      "claude",
			WorkspaceID:   h.workspaceID,
			State:         "active",
			RuntimeStatus: store.SessionRuntimeUnbound,
			CreatedAt:     now,
			UpdatedAt:     now,
		}); err != nil {
			t.Fatalf("WriteSessionMeta(valid) error = %v", err)
		}

		invalidMetaPath := store.SessionMetaFile(filepath.Join(h.home.SessionsDir, "sess-without-provider"))
		if err := store.WriteSessionMeta(invalidMetaPath, &store.SessionMeta{
			ID:            "sess-without-provider",
			ProfileID:     store.DefaultProfileID,
			Name:          "Missing Provider",
			AgentName:     "coder",
			WorkspaceID:   h.workspaceID,
			State:         "stopped",
			RuntimeStatus: store.SessionRuntimeUnbound,
			CreatedAt:     now,
			UpdatedAt:     now,
		}); err != nil {
			t.Fatalf("WriteSessionMeta(invalid) error = %v", err)
		}
		before, err := os.ReadFile(invalidMetaPath)
		if err != nil {
			t.Fatalf("ReadFile(invalid metadata before reconcile) error = %v", err)
		}

		result, err := h.observer.Reconcile(testutil.Context(t))
		if err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		slices.Sort(result.Indexed)
		if got, want := result.Indexed, []string{"sess-valid"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("Indexed = %#v, want %#v", got, want)
		}

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if got, want := len(sessions), 1; got != want {
			t.Fatalf("len(sessions) = %d, want %d", got, want)
		}
		if sessions[0].ID != "sess-valid" {
			t.Fatalf("sessions[0].ID = %q, want %q", sessions[0].ID, "sess-valid")
		}
		after, err := os.ReadFile(invalidMetaPath)
		if err != nil {
			t.Fatalf("ReadFile(invalid metadata after reconcile) error = %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatalf("invalid metadata changed during reconcile:\n before: %s\n after: %s", before, after)
		}
	})
}

func TestReconciliationSkipsSessionMetadataMissingWorkspaceID(t *testing.T) {
	t.Parallel()

	t.Run("Should skip metadata missing workspace id", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)

		sessionDir := filepath.Join(h.home.SessionsDir, "sess-missing-workspace")
		meta := `{
  "id": "sess-missing-workspace",
  "name": "Missing Workspace",
  "agent_name": "coder",
  "state": "active",
  "created_at": "2026-04-03T18:30:00Z",
  "updated_at": "2026-04-03T18:30:00Z"
}
`
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(sessionDir) error = %v", err)
		}
		if err := os.WriteFile(store.SessionMetaFile(sessionDir), []byte(meta), 0o644); err != nil {
			t.Fatalf("WriteFile(meta) error = %v", err)
		}

		result, err := h.observer.Reconcile(testutil.Context(t))
		if err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		if len(result.Indexed) != 0 {
			t.Fatalf("Indexed = %#v, want empty", result.Indexed)
		}
		if len(result.Orphaned) != 0 {
			t.Fatalf("Orphaned = %#v, want empty", result.Orphaned)
		}

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if len(sessions) != 0 {
			t.Fatalf("len(sessions) = %d, want 0", len(sessions))
		}
	})
}

func TestReconciliationRecoveryMetadataBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		sessionType string
		spawnRole   string
		reason      string
	}{
		{name: "Should leave a retired session type inert", sessionType: "dream", reason: "unknown_session_type"},
		{name: "Should leave an extractor spawn role inert", sessionType: "spawned", spawnRole: "memory-extractor", reason: "retired_spawn_role"},
		{name: "Should leave a checkpoint summary spawn role inert", sessionType: "spawned", spawnRole: "checkpoint-summary", reason: "retired_spawn_role"},
		{name: "Should refuse any unsupported session type", sessionType: "future-type", reason: "unknown_session_type"},
		{name: "Should recover an orphan with an arbitrary advisory spawn role", sessionType: "spawned", spawnRole: "future-role"},
		{name: "Should recover an orphan with a reviewer spawn role", sessionType: "spawned", spawnRole: "reviewer"},
		{name: "Should recover a normal user orphan", sessionType: "user"},
		{name: "Should recover a system orphan", sessionType: "system"},
		{name: "Should recover a coordinator orphan", sessionType: "coordinator", spawnRole: string(session.SessionTypeCoordinator)},
		{name: "Should recover metadata with omitted lifecycle fields"},
		{name: "Should recover a worker orphan", sessionType: "spawned", spawnRole: session.DefaultSpawnRole},
		{name: "Should recover an auto title orphan", sessionType: "spawned", spawnRole: session.SpawnRoleAutoTitle},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			ctx := t.Context()
			const sessionID = "orphan-session"
			sessionDir := filepath.Join(h.home.SessionsDir, sessionID)
			dbPath := store.SessionDBFile(sessionDir)
			db, err := sessiondb.OpenSessionDB(ctx, store.SessionDBOwner{
				SessionID: sessionID, WorkspaceID: h.workspaceID,
			}, dbPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Close(ctx); err != nil {
				t.Fatal(err)
			}
			metaPath := store.SessionMetaFile(sessionDir)
			lineage := &store.SessionLineage{SpawnRole: tc.spawnRole}
			if tc.sessionType == string(session.SessionTypeSpawned) {
				lineage.Kind = store.LineageKindSpawn
			}
			if err := store.WriteSessionMeta(metaPath, &store.SessionMeta{
				ID: sessionID, ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
				Provider: "claude", AgentName: "coder", SessionType: tc.sessionType,
				Lineage: lineage, State: "stopped",
				RuntimeStatus: store.SessionRuntimeUnbound, CreatedAt: h.now, UpdatedAt: h.now,
			}); err != nil {
				t.Fatal(err)
			}
			before := snapshotRecoveryFiles(t, sessionDir)
			var logs bytes.Buffer
			h.observer.logger = slog.New(slog.NewJSONHandler(&logs, nil))
			for attempt := range 2 {
				result, err := h.observer.Reconcile(ctx)
				if err != nil {
					t.Fatal(err)
				}
				wantIndexed := 0
				if tc.reason == "" && attempt == 0 {
					wantIndexed = 1
				}
				if len(result.Indexed) != wantIndexed {
					t.Fatalf("indexed = %v, want count %d", result.Indexed, wantIndexed)
				}
			}
			rows, err := h.registry.ListSessions(ctx, store.SessionListQuery{
				ReadScope: store.ReadScope{AllProfiles: true},
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.reason == "" {
				if len(rows) != 1 || rows[0].ID != sessionID {
					t.Fatalf("recovered rows = %+v, want normal orphan", rows)
				}
				if tc.spawnRole != "" && (rows[0].Lineage == nil || rows[0].Lineage.SpawnRole != tc.spawnRole) {
					t.Fatalf("recovered lineage = %+v, want spawn role %q", rows[0].Lineage, tc.spawnRole)
				}
				return
			}
			if len(rows) != 0 {
				t.Fatalf("unsupported metadata was recataloged: %+v", rows)
			}
			after := snapshotRecoveryFiles(t, sessionDir)
			if len(after) != len(before) {
				t.Fatalf("recovery changed file set: before %v, after %v", before, after)
			}
			for path, original := range before {
				current, ok := after[path]
				if !ok || !bytes.Equal(original.content, current.content) ||
					!original.modified.Equal(current.modified) {
					t.Fatalf("recovery changed unsupported session file %s", path)
				}
			}
			var warning map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning); err != nil {
				t.Fatalf("want exactly one structured warning: %v; logs: %s", err, logs.String())
			}
			for key, want := range map[string]string{
				"level": "WARN", "msg": "observe.session_recovery_skipped", "session_id": sessionID,
				"session_type": tc.sessionType, "spawn_role": tc.spawnRole, "reason": tc.reason,
			} {
				if warning[key] != want {
					t.Fatalf("warning[%s] = %v, want %s", key, warning[key], want)
				}
			}
		})
	}
}

type recoveryFileSnapshot struct {
	content  []byte
	modified time.Time
}

func snapshotRecoveryFiles(t *testing.T, dir string) map[string]recoveryFileSnapshot {
	t.Helper()
	files := make(map[string]recoveryFileSnapshot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("unexpected nested recovery fixture directory %s", entry.Name())
		}
		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = recoveryFileSnapshot{content: content, modified: info.ModTime()}
	}
	return files
}

func TestReconciliationReadsOnlyChangedSessions(t *testing.T) {
	t.Parallel()

	// Invariant: reconcile cost follows change. A session whose files and catalog row are both
	// identical to the last reconcile stays unread and indexed; a changed metadata document, a
	// catalog row changed behind its back, and a removed directory are all still reconciled.
	t.Run("Should leave unchanged sessions unread and still reconcile changes and orphans", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		ctx := testutil.Context(t)
		var logs bytes.Buffer
		h.observer.logger = slog.New(slog.NewJSONHandler(&logs, nil))
		stopReason := store.StopUserCanceled
		metaPath := func(id string) string {
			return store.SessionMetaFile(filepath.Join(h.home.SessionsDir, id))
		}
		writeMeta := func(id, name string, age time.Duration) {
			t.Helper()
			if err := store.WriteSessionMeta(metaPath(id), &store.SessionMeta{
				ID:            id,
				ProfileID:     store.DefaultProfileID,
				Name:          name,
				AgentName:     "coder",
				Provider:      "claude",
				WorkspaceID:   h.workspaceID,
				State:         "stopped",
				RuntimeStatus: store.SessionRuntimeUnbound,
				StopReason:    &stopReason,
				CreatedAt:     h.now,
				UpdatedAt:     h.now,
			}); err != nil {
				t.Fatalf("WriteSessionMeta(%s) error = %v", id, err)
			}
			settledAt := time.Now().Add(-age)
			if err := os.Chtimes(metaPath(id), settledAt, settledAt); err != nil {
				t.Fatalf("Chtimes(%s) error = %v", id, err)
			}
		}
		reconcile := func() (result store.ReconcileResult, read int, unchanged int) {
			t.Helper()
			logs.Reset()
			result, err := h.observer.Reconcile(ctx)
			if err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}
			decoder := json.NewDecoder(&logs)
			for decoder.More() {
				var entry map[string]any
				if err := decoder.Decode(&entry); err != nil {
					t.Fatalf("decode reconcile log: %v", err)
				}
				if entry["msg"] == "observe: session reconcile complete" {
					readCount, _ := entry["read_sessions"].(float64)
					unchangedCount, _ := entry["unchanged_sessions"].(float64)
					return result, int(readCount), int(unchangedCount)
				}
			}
			t.Fatal("Reconcile() did not report its read and unchanged session counts")
			return store.ReconcileResult{}, 0, 0
		}
		catalogName := func(id string) string {
			t.Helper()
			rows, err := h.registry.ListSessions(ctx, store.SessionListQuery{
				ReadScope: store.ReadScope{AllProfiles: true}, ID: id,
			})
			if err != nil || len(rows) != 1 {
				t.Fatalf("ListSessions(%s) = %d rows, %v; want one row", id, len(rows), err)
			}
			return rows[0].Name
		}

		writeMeta("sess-idle", "Idle", time.Hour)
		writeMeta("sess-busy", "Busy", time.Hour)
		if result, read, _ := reconcile(); read != 2 || !slices.Equal(result.Indexed, []string{"sess-busy", "sess-idle"}) {
			t.Fatalf("first reconcile read %d sessions and indexed %v, want both", read, result.Indexed)
		}
		// The first read may persist a recovery classification; the proof is taken from a settled read.
		for _, id := range []string{"sess-idle", "sess-busy"} {
			settledAt := time.Now().Add(-2 * time.Hour)
			if err := os.Chtimes(metaPath(id), settledAt, settledAt); err != nil {
				t.Fatalf("Chtimes(%s) error = %v", id, err)
			}
		}
		reconcile()
		if result, read, unchanged := reconcile(); read != 0 || unchanged != 2 ||
			len(result.Indexed) != 0 || len(result.Orphaned) != 0 {
			t.Fatalf("unchanged reconcile read=%d unchanged=%d result=%+v, want nothing read or orphaned",
				read, unchanged, result)
		}

		writeMeta("sess-busy", "Renamed", 30*time.Minute)
		if _, read, unchanged := reconcile(); read != 1 || unchanged != 1 || catalogName("sess-busy") != "Renamed" {
			t.Fatalf("changed metadata read=%d unchanged=%d name=%q, want only the renamed session read",
				read, unchanged, catalogName("sess-busy"))
		}

		if err := h.registry.RegisterSession(ctx, new(store.SessionInfo{
			ID:            "sess-busy",
			ProfileID:     store.DefaultProfileID,
			Name:          "Drifted",
			AgentName:     "coder",
			Provider:      "claude",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   h.workspaceID,
			State:         "stopped",
			StopReason:    stopReason,
			CreatedAt:     h.now,
			UpdatedAt:     h.now,
		})); err != nil {
			t.Fatalf("RegisterSession(drifted) error = %v", err)
		}
		if _, read, unchanged := reconcile(); read != 1 || unchanged != 1 || catalogName("sess-busy") != "Renamed" {
			t.Fatalf("drifted catalog row read=%d unchanged=%d name=%q, want the row reconciled from metadata",
				read, unchanged, catalogName("sess-busy"))
		}

		// A timestamp inside the settle window cannot distinguish a later write, so it is never trusted.
		writeMeta("sess-fresh", "Fresh", -time.Minute)
		for range 2 {
			if _, read, unchanged := reconcile(); read != 1 || unchanged != 2 {
				t.Fatalf("freshly written metadata read=%d unchanged=%d, want it read on every reconcile",
					read, unchanged)
			}
		}

		if err := os.RemoveAll(filepath.Join(h.home.SessionsDir, "sess-busy")); err != nil {
			t.Fatalf("RemoveAll(sess-busy) error = %v", err)
		}
		result, _, unchanged := reconcile()
		if !slices.Equal(result.Orphaned, []string{"sess-busy"}) || unchanged != 1 {
			t.Fatalf("removed directory orphaned=%v unchanged=%d, want only the removed session orphaned",
				result.Orphaned, unchanged)
		}
	})
}
