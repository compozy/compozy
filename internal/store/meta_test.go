package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWriteSessionMetaAndReadBack(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), SessionMetaName)
	stopReason := StopHookStopped
	meta := SessionMeta{
		ID:            "sess-meta",
		Name:          "Session Meta",
		AgentName:     "coder",
		WorkspaceID:   "ws-meta",
		SessionType:   "system",
		State:         "stopped",
		RuntimeStatus: SessionRuntimeReady,
		StopReason:    &stopReason,
		StopDetail:    "hook denied continuation",

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
		readBack.State != meta.State ||
		readBack.SessionType != meta.SessionType ||
		readBack.StopDetail != meta.StopDetail {
		t.Fatalf("ReadSessionMeta() = %#v, want %#v", readBack, meta)
	}
	if readBack.RuntimeStatus != SessionRuntimeReady {
		t.Fatalf("ReadSessionMeta().RuntimeStatus = %q, want %q", readBack.RuntimeStatus, SessionRuntimeReady)
	}
	if readBack.StopReason == nil {
		t.Fatal("ReadSessionMeta().StopReason = nil, want non-nil")
	}
	if *readBack.StopReason != *meta.StopReason {
		t.Fatalf("ReadSessionMeta().StopReason = %q, want %q", *readBack.StopReason, *meta.StopReason)
	}
}

func TestWriteSessionMetaConcurrentWritesDoNotCorruptFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), SessionMetaName)
	base := SessionMeta{
		ID:            "sess-meta-concurrent",
		AgentName:     "coder",
		WorkspaceID:   "ws-meta-concurrent",
		State:         "active",
		RuntimeStatus: SessionRuntimeReady,
		CreatedAt:     time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC),
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
	if meta.RuntimeStatus != SessionRuntimeReady {
		t.Fatalf("ReadSessionMeta().RuntimeStatus = %q, want %q", meta.RuntimeStatus, SessionRuntimeReady)
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
  "session_type": "user",
  "state": "stopped",
  "runtime_status": "ready",
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

func TestSessionMetaCreationWitness(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		old         string
		replacement string
		valid       bool
	}{
		{name: "Should reopen the creation witness without rewriting it", valid: true},
		{name: "Should reject a tampered policy", old: "Preserve this runtime policy.", replacement: "Tampered policy"},
		{name: "Should reject an unknown future version", old: `"version":6`, replacement: `"version":99`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload, err := json.Marshal(sessionCreationWitnessForTest(t))
			if err != nil {
				t.Fatal(err)
			}
			if tc.old != "" {
				payload = bytes.ReplaceAll(payload, []byte(tc.old), []byte(tc.replacement))
			}
			path := filepath.Join(t.TempDir(), SessionMetaName)
			if err := os.WriteFile(path, payload, 0600); err != nil {
				t.Fatal(err)
			}
			meta, err := ReadSessionMeta(path)
			if !tc.valid {
				if err == nil {
					t.Fatal("accepted invalid creation witness")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(payload, after) {
				t.Fatal("read rewrote creation metadata")
			}
			if err := WriteSessionMeta(path, meta); err != nil {
				t.Fatal(err)
			}
			reopened, err := ReadSessionMeta(path)
			if err != nil {
				t.Fatal(err)
			}
			if reopened.CreationProfileRef != meta.CreationProfileRef ||
				reopened.CreationDigest != meta.CreationDigest ||
				reopened.PolicySpecDigest != meta.PolicySpecDigest ||
				reopened.CreationProfile.PromptOverlay != meta.CreationProfile.PromptOverlay {
				t.Fatal("metadata rewrite changed the creation witness")
			}
		})
	}
}

func sessionCreationWitnessForTest(t *testing.T) SessionMeta {
	t.Helper()
	profile := validSessionCreationProfile(t)
	profile.PromptOverlay = "Preserve this runtime policy."
	options := SessionCreationOptions{SessionID: "sess-creation-witness", SessionType: "user"}
	profileRef, err := profile.Ref()
	if err != nil {
		t.Fatal(err)
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		t.Fatal(err)
	}
	creationDigest, err := profile.CreationDigest(options)
	if err != nil {
		t.Fatal(err)
	}
	return SessionMeta{
		ID: options.SessionID, AgentName: profile.AgentName, ProfileID: profile.ProfileID,
		WorkspaceID: profile.WorkspaceID, SessionType: options.SessionType,
		State: "stopped", RuntimeStatus: SessionRuntimeReady,
		CreationProfile: &profile, CreationOptions: &options,
		CreationProfileRef: profileRef, PolicySpecDigest: policyDigest, CreationDigest: creationDigest,
		CreatedAt: time.Date(2026, 4, 3, 19, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 19, 1, 0, 0, time.UTC),
	}
}
