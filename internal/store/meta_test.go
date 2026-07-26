package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

func TestWriteSessionMetaAndReadBack(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), SessionMetaName)
	stopReason := StopHookStopped
	meta := SessionMeta{
		ID:                   "sess-meta",
		Name:                 "Session Meta",
		AgentName:            "coder",
		WorkspaceID:          "ws-meta",
		NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
		SessionType:          "system",
		State:                "stopped",
		StopReason:           &stopReason,
		StopDetail:           "hook denied continuation",
		Sandbox: &SessionSandboxMeta{
			SandboxID:             "env-123",
			Backend:               "daytona",
			Profile:               "daytona-dev",
			State:                 "ready",
			InstanceID:            "sandbox-123",
			RuntimeRootDir:        "/home/daytona/workspace",
			RuntimeAdditionalDirs: []string{"/home/daytona/shared"},
			ProviderState:         json.RawMessage(`{"sandbox_id":"sandbox-123"}`),
			SSHAccessExpiresAt:    new(time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)),
			LastSyncAt:            new(time.Date(2026, 4, 3, 17, 59, 0, 0, time.UTC)),
			LastSyncError:         "sync warning",
		},
		CreatedAt: time.Date(2026, 4, 3, 17, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 17, 1, 0, 0, time.UTC),
	}

	if err := WriteSessionMeta(path, meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	readBack, err := ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta() error = %v", err)
	}
	if readBack.ID != meta.ID ||
		readBack.AgentName != meta.AgentName ||
		readBack.WorkspaceID != meta.WorkspaceID ||
		readBack.NetworkSpecSnapshot() != meta.NetworkSpecSnapshot() ||
		readBack.State != meta.State ||
		readBack.SessionType != meta.SessionType ||
		readBack.StopDetail != meta.StopDetail {
		t.Fatalf("ReadSessionMeta() = %#v, want %#v", readBack, meta)
	}
	if readBack.StopReason == nil {
		t.Fatal("ReadSessionMeta().StopReason = nil, want non-nil")
	}
	if *readBack.StopReason != *meta.StopReason {
		t.Fatalf("ReadSessionMeta().StopReason = %q, want %q", *readBack.StopReason, *meta.StopReason)
	}
	if readBack.Sandbox == nil {
		t.Fatal("ReadSessionMeta().Sandbox = nil, want metadata")
	}
	if readBack.Sandbox.SandboxID != "env-123" ||
		readBack.Sandbox.State != "ready" ||
		readBack.Sandbox.InstanceID != "sandbox-123" ||
		readBack.Sandbox.LastSyncError != "sync warning" {
		t.Fatalf("ReadSessionMeta().Sandbox = %#v, want persisted sandbox metadata", readBack.Sandbox)
	}
	var providerState struct {
		SandboxID string `json:"sandbox_id"`
	}
	if err := json.Unmarshal(readBack.Sandbox.ProviderState, &providerState); err != nil {
		t.Fatalf("json.Unmarshal(ProviderState) error = %v", err)
	}
	if providerState.SandboxID != "sandbox-123" {
		t.Fatalf("ProviderState sandbox_id = %q, want sandbox-123", providerState.SandboxID)
	}
	if readBack.Sandbox.SSHAccessExpiresAt == nil ||
		!readBack.Sandbox.SSHAccessExpiresAt.Equal(*meta.Sandbox.SSHAccessExpiresAt) {
		t.Fatalf("SSHAccessExpiresAt = %#v, want %#v",
			readBack.Sandbox.SSHAccessExpiresAt,
			meta.Sandbox.SSHAccessExpiresAt,
		)
	}
	if readBack.Sandbox.LastSyncAt == nil ||
		!readBack.Sandbox.LastSyncAt.Equal(*meta.Sandbox.LastSyncAt) {
		t.Fatalf("LastSyncAt = %#v, want %#v",
			readBack.Sandbox.LastSyncAt,
			meta.Sandbox.LastSyncAt,
		)
	}
}

func TestWriteSessionMetaConcurrentWritesDoNotCorruptFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), SessionMetaName)
	base := SessionMeta{
		ID:                   "sess-meta-concurrent",
		AgentName:            "coder",
		WorkspaceID:          "ws-meta-concurrent",
		NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
		State:                "active",
		CreatedAt:            time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC),
	}

	var wg sync.WaitGroup
	for i := range 25 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			meta := base
			meta.Name = filepath.Base(
				filepath.Join("name", time.Date(2026, 4, 3, 18, 0, i, 0, time.UTC).Format(time.RFC3339Nano)),
			)
			meta.UpdatedAt = base.UpdatedAt.Add(time.Duration(i) * time.Second)
			if err := WriteSessionMeta(path, meta); err != nil {
				t.Errorf("WriteSessionMeta() error = %v", err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("meta file is empty after concurrent writes")
	}

	meta, err := ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta() error = %v", err)
	}
	if meta.ID != base.ID || meta.AgentName != base.AgentName || meta.WorkspaceID != base.WorkspaceID {
		t.Fatalf(
			"ReadSessionMeta() = %#v, want id=%q agent=%q workspace_id=%q",
			meta,
			base.ID,
			base.AgentName,
			base.WorkspaceID,
		)
	}
}

func TestSessionMetaValidateRejectsParticipationOutsideCreationIdentity(t *testing.T) {
	t.Parallel()

	profile := SessionCreationProfile{
		Version:     SessionCreationProfileVersion,
		AgentName:   "coder",
		Provider:    "codex",
		WorkspaceID: "ws-meta-identity",
		CWD:         "/work/meta-identity",
		SandboxMode: SessionCreationSandboxNone,
		Permissions: "approve-all",
	}
	options := SessionCreationOptions{
		SessionID:            "sess-meta-identity",
		NetworkOwnerKey:      "session:sess-meta-identity",
		NetworkParticipation: participation.LocalSpec(),
		SessionType:          "user",
	}
	profileRef, err := profile.Ref()
	if err != nil {
		t.Fatalf("SessionCreationProfile.Ref() error = %v", err)
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		t.Fatalf("SessionCreationProfile.PolicySpecDigest() error = %v", err)
	}
	creationDigest, err := profile.CreationDigest(options)
	if err != nil {
		t.Fatalf("SessionCreationProfile.CreationDigest() error = %v", err)
	}
	live := participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     profile.WorkspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       "other-channel",
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         4,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  4096,
			MaxWakeDepth:     4,
			CoalesceWindow:   "250ms",
		},
	}
	meta := SessionMeta{
		ID:                   options.SessionID,
		AgentName:            profile.AgentName,
		WorkspaceID:          profile.WorkspaceID,
		NetworkParticipation: participation.CloneSpec(live),
		SessionType:          options.SessionType,
		State:                "stopped",
		CreationProfile:      &profile,
		CreationOptions:      &options,
		CreationProfileRef:   profileRef,
		PolicySpecDigest:     policyDigest,
		CreationDigest:       creationDigest,
		CreatedAt:            time.Date(2026, 4, 3, 19, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 3, 19, 1, 0, 0, time.UTC),
	}

	if err := meta.Validate(); err == nil {
		t.Fatal("SessionMeta.Validate() error = nil, want creation participation mismatch")
	}
}

func TestReadSessionMetaStopFieldsOmitted(t *testing.T) {
	t.Run("Should handle optional stop fields omitted", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), SessionMetaName)
		payload := []byte(`{
  "id": "sess-current",
  "name": "Current Session",
  "agent_name": "coder",
  "workspace_id": "ws-current",
  "network_participation": {
    "version": "network-participation/v1",
    "mode": "local",
    "source": "built_in_local"
  },
  "session_type": "user",
  "state": "stopped",
  "created_at": "2026-04-03T17:00:00Z",
  "updated_at": "2026-04-03T17:01:00Z"
}
`)
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		meta, err := ReadSessionMeta(path)
		if err != nil {
			t.Fatalf("ReadSessionMeta() error = %v", err)
		}
		if meta.StopReason != nil {
			t.Fatalf("ReadSessionMeta().StopReason = %v, want nil", *meta.StopReason)
		}
		if meta.StopDetail != "" {
			t.Fatalf("ReadSessionMeta().StopDetail = %q, want empty", meta.StopDetail)
		}
	})
}
