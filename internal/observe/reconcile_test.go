package observe

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/testutil"

	"github.com/compozy/agh/internal/store"
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

		if err := store.WriteSessionMeta(metaPath, store.SessionMeta{
			ID:                   "sess-new",
			Name:                 "New",
			AgentName:            "coder",
			Provider:             "claude",
			WorkspaceID:          h.workspaceID,
			NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
			State:                "stopped",
			StopReason:           &stopReason,
			StopDetail:           "requested by API",
			CreatedAt:            now,
			UpdatedAt:            now,
		}); err != nil {
			t.Fatalf("WriteSessionMeta() error = %v", err)
		}

		result, err := h.observer.Reconcile(testutil.Context(t))
		if err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		sort.Strings(result.Indexed)
		if got, want := result.Indexed, []string{"sess-new"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("Indexed = %#v, want %#v", got, want)
		}

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if got, want := len(sessions), 1; got != want {
			t.Fatalf("len(sessions) = %d, want %d", got, want)
		}
		if sessions[0].State != "stopped" {
			t.Fatalf("sessions[0].State = %q, want stopped", sessions[0].State)
		}
		if sessions[0].StopReason != store.StopUserCanceled {
			t.Fatalf("sessions[0].StopReason = %q, want %q", sessions[0].StopReason, store.StopUserCanceled)
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
			WorkspaceID: h.workspaceID, CWD: h.workspace, SandboxMode: store.SessionCreationSandboxNone,
			Permissions: "approve-reads",
		}
		creationOptions := store.SessionCreationOptions{
			SessionID:            childID,
			Name:                 "Child",
			NetworkOwnerKey:      "session:" + childID,
			NetworkParticipation: participation.LocalSpec(),
			SessionType:          "worker",
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
			store.SessionMetaFile(filepath.Join(h.home.SessionsDir, rootID)),
			store.SessionMeta{
				ID:                   rootID,
				Name:                 "Root",
				AgentName:            "coder",
				Provider:             "claude",
				WorkspaceID:          h.workspaceID,
				NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
				State:                "stopped",
				CreatedAt:            now,
				UpdatedAt:            now,
			},
		); err != nil {
			t.Fatalf("WriteSessionMeta(root) error = %v", err)
		}
		if err := store.WriteSessionMeta(
			store.SessionMetaFile(filepath.Join(h.home.SessionsDir, parentID)),
			store.SessionMeta{
				ID:                   parentID,
				Name:                 "Parent",
				AgentName:            "coder",
				Provider:             "claude",
				WorkspaceID:          h.workspaceID,
				NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
				State:                "stopped",
				Lineage: &store.SessionLineage{
					ParentSessionID: rootID,
					RootSessionID:   rootID,
					SpawnDepth:      1,
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		); err != nil {
			t.Fatalf("WriteSessionMeta(parent) error = %v", err)
		}
		if err := store.WriteSessionMeta(
			store.SessionMetaFile(filepath.Join(h.home.SessionsDir, childID)),
			store.SessionMeta{
				ID:                   childID,
				Name:                 "Child",
				AgentName:            "coder",
				Provider:             "claude",
				WorkspaceID:          h.workspaceID,
				NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
				SessionType:          "worker",
				State:                "stopped",
				ACPSessionID:         &acpSessionID,
				StopReason:           &stopReason,
				StopDetail:           "agent process exited",
				Failure: &store.SessionFailure{
					Kind:            store.FailureProcess,
					Summary:         "agent exited with status 1",
					CrashBundlePath: "/tmp/agh-crash-bundle",
				},
				Lineage: &store.SessionLineage{
					ParentSessionID:  parentID,
					RootSessionID:    rootID,
					SpawnDepth:       2,
					SpawnRole:        "delegate_task",
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
			},
		); err != nil {
			t.Fatalf("WriteSessionMeta(child) error = %v", err)
		}

		result, err := h.observer.Reconcile(testutil.Context(t))
		if err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		sort.Strings(result.Indexed)
		if got, want := result.Indexed, []string{rootID, parentID, childID}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("Indexed = %#v, want %#v", got, want)
		}

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{})
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
		if got, want := indexed.Failure.CrashBundlePath, "/tmp/agh-crash-bundle"; got != want {
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
			indexed,
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

func TestReconciliationMarksMissingDirectoryAsOrphaned(t *testing.T) {
	t.Parallel()

	t.Run("Should mark indexed sessions missing from disk as orphaned", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		now := h.now
		if err := h.observer.registry.RegisterSession(testutil.Context(t), store.SessionInfo{
			ID:          "sess-orphan",
			Name:        "Orphan",
			AgentName:   "coder",
			Provider:    "claude",
			WorkspaceID: h.workspaceID,
			State:       "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		result, err := h.observer.Reconcile(testutil.Context(t))
		if err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		sort.Strings(result.Orphaned)
		if got, want := result.Orphaned, []string{"sess-orphan"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("Orphaned = %#v, want %#v", got, want)
		}

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{})
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

func TestReconciliationLegacyProviderRepair(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		setup  func(t *testing.T, h *harness) map[string]string
		assert func(t *testing.T, h *harness, result store.ReconcileResult, paths map[string]string)
	}{
		{
			name: "Should repair the legacy provider before indexing the session",
			setup: func(t *testing.T, h *harness) map[string]string {
				t.Helper()

				sessionDir := filepath.Join(h.home.SessionsDir, "sess-repair")
				metaPath := store.SessionMetaFile(sessionDir)
				now := h.now.Add(40 * time.Minute)

				if err := store.WriteSessionMeta(metaPath, store.SessionMeta{
					ID:                   "sess-repair",
					Name:                 "Repair",
					AgentName:            "coder",
					WorkspaceID:          h.workspaceID,
					NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
					State:                "stopped",
					CreatedAt:            now,
					UpdatedAt:            now,
				}); err != nil {
					t.Fatalf("WriteSessionMeta() error = %v", err)
				}

				return map[string]string{"repair": metaPath}
			},
			assert: func(t *testing.T, h *harness, result store.ReconcileResult, paths map[string]string) {
				t.Helper()

				if got, want := result.Indexed, []string{"sess-repair"}; !testutil.EqualStringSlices(got, want) {
					t.Fatalf("Indexed = %#v, want %#v", got, want)
				}

				sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{})
				if err != nil {
					t.Fatalf("ListSessions() error = %v", err)
				}
				if got, want := len(sessions), 1; got != want {
					t.Fatalf("len(sessions) = %d, want %d", got, want)
				}
				if got, want := sessions[0].Provider, "claude"; got != want {
					t.Fatalf("sessions[0].Provider = %q, want %q", got, want)
				}

				meta, err := store.ReadSessionMeta(paths["repair"])
				if err != nil {
					t.Fatalf("ReadSessionMeta() error = %v", err)
				}
				if got, want := meta.Provider, "claude"; got != want {
					t.Fatalf("meta.Provider = %q, want %q", got, want)
				}
			},
		},
		{
			name: "Should skip an unrecoverable legacy provider and continue indexing valid sessions",
			setup: func(t *testing.T, h *harness) map[string]string {
				t.Helper()

				validDir := filepath.Join(h.home.SessionsDir, "sess-valid-after-bad-repair")
				validMetaPath := store.SessionMetaFile(validDir)
				badDir := filepath.Join(h.home.SessionsDir, "sess-bad-repair")
				badMetaPath := store.SessionMetaFile(badDir)
				now := h.now.Add(50 * time.Minute)

				if err := store.WriteSessionMeta(validMetaPath, store.SessionMeta{
					ID:                   "sess-valid-after-bad-repair",
					Name:                 "Valid",
					AgentName:            "coder",
					Provider:             "claude",
					WorkspaceID:          h.workspaceID,
					NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
					State:                "active",
					CreatedAt:            now,
					UpdatedAt:            now,
				}); err != nil {
					t.Fatalf("WriteSessionMeta(valid) error = %v", err)
				}
				if err := store.WriteSessionMeta(badMetaPath, store.SessionMeta{
					ID:                   "sess-bad-repair",
					Name:                 "Bad Repair",
					AgentName:            "missing-agent",
					WorkspaceID:          h.workspaceID,
					NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
					State:                "stopped",
					CreatedAt:            now,
					UpdatedAt:            now,
				}); err != nil {
					t.Fatalf("WriteSessionMeta(bad) error = %v", err)
				}

				return map[string]string{"bad": badMetaPath}
			},
			assert: func(t *testing.T, h *harness, result store.ReconcileResult, paths map[string]string) {
				t.Helper()

				if got, want := result.Indexed, []string{
					"sess-valid-after-bad-repair",
				}; !testutil.EqualStringSlices(
					got,
					want,
				) {
					t.Fatalf("Indexed = %#v, want %#v", got, want)
				}

				sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{})
				if err != nil {
					t.Fatalf("ListSessions() error = %v", err)
				}
				if got, want := len(sessions), 1; got != want {
					t.Fatalf("len(sessions) = %d, want %d", got, want)
				}
				if got, want := sessions[0].ID, "sess-valid-after-bad-repair"; got != want {
					t.Fatalf("sessions[0].ID = %q, want %q", got, want)
				}

				meta, err := store.ReadSessionMeta(paths["bad"])
				if err != nil {
					t.Fatalf("ReadSessionMeta() error = %v", err)
				}
				if got := meta.Provider; got != "" {
					t.Fatalf("bad meta.Provider = %q, want empty after skipped repair", got)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			paths := tc.setup(t, h)

			result, err := h.observer.Reconcile(testutil.Context(t))
			if err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}
			sort.Strings(result.Indexed)
			tc.assert(t, h, result, paths)
		})
	}
}

func TestReconciliationSkipsLegacyStoppedSessionMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should skip legacy stopped metadata that cannot be normalized", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)

		validDir := filepath.Join(h.home.SessionsDir, "sess-valid")
		validMetaPath := store.SessionMetaFile(validDir)
		now := h.now.Add(45 * time.Minute)
		if err := store.WriteSessionMeta(validMetaPath, store.SessionMeta{
			ID:                   "sess-valid",
			Name:                 "Valid",
			AgentName:            "coder",
			Provider:             "claude",
			WorkspaceID:          h.workspaceID,
			NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
			State:                "active",
			CreatedAt:            now,
			UpdatedAt:            now,
		}); err != nil {
			t.Fatalf("WriteSessionMeta(valid) error = %v", err)
		}

		legacyDir := filepath.Join(h.home.SessionsDir, "sess-legacy")
		if err := os.MkdirAll(legacyDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(legacyDir) error = %v", err)
		}
		legacyMeta := `{
  "id": "sess-legacy",
  "name": "legacy-session",
  "goal": "Legacy prompt",
  "workspace": "` + h.workspace + `",
  "state": "stopped",
  "created_at": "2026-04-01T03:57:38.428414Z",
  "stopped_at": "2026-04-01T03:58:00.212132Z"
}
`
		if err := os.WriteFile(store.SessionMetaFile(legacyDir), []byte(legacyMeta), 0o644); err != nil {
			t.Fatalf("WriteFile(legacy meta) error = %v", err)
		}

		result, err := h.observer.Reconcile(testutil.Context(t))
		if err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		sort.Strings(result.Indexed)
		if got, want := result.Indexed, []string{"sess-valid"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("Indexed = %#v, want %#v", got, want)
		}

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if got, want := len(sessions), 1; got != want {
			t.Fatalf("len(sessions) = %d, want %d", got, want)
		}
		if sessions[0].ID != "sess-valid" {
			t.Fatalf("sessions[0].ID = %q, want %q", sessions[0].ID, "sess-valid")
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

		sessions, err := h.observer.registry.ListSessions(testutil.Context(t), store.SessionListQuery{})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if len(sessions) != 0 {
			t.Fatalf("len(sessions) = %d, want 0", len(sessions))
		}
	})
}
