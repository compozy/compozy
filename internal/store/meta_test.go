package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
	if err := WriteSessionMeta(path, &meta); err != nil {
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
		wg.Go(func() {
			meta := base
			meta.Name = filepath.Base(
				filepath.Join("name", time.Date(2026, 4, 3, 18, 0, i, 0, time.UTC).Format(time.RFC3339Nano)),
			)
			meta.UpdatedAt = base.UpdatedAt.Add(time.Duration(i) * time.Second)
			if err := WriteSessionMeta(path, &meta); err != nil {
				t.Errorf("WriteSessionMeta() error = %v", err)
			}
		})
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
		payload     []byte
		old         string
		replacement string
		valid       bool
	}{
		{name: "Should reopen the creation witness without rewriting it", valid: true},
		{name: "Should retain a version three creation witness", payload: legacyCreationWitnessForTest(t, 3), valid: true},
		{name: "Should retain a version four creation witness", payload: legacyCreationWitnessForTest(t, 4), valid: true},
		{name: "Should retain a version five creation witness", payload: legacyCreationWitnessForTest(t, 5), valid: true},
		{
			name: "Should reject a tampered retired sandbox witness", payload: legacyCreationWitnessForTest(t, 3),
			old: `"sandbox_ref":"local"`, replacement: `"sandbox_ref":"changed"`,
		},
		{
			name: "Should reject a tampered retired network witness", payload: legacyCreationWitnessForTest(t, 5),
			old: `"source":"built_in_local"`, replacement: `"source":"explicit_request"`,
		},
		{name: "Should reject a tampered policy", old: "Preserve this runtime policy.", replacement: "Tampered policy"},
		{name: "Should reject an unknown future version", old: `"version":6`, replacement: `"version":99`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := tc.payload
			if payload == nil {
				var err error
				payload, err = json.Marshal(sessionCreationWitnessForTest(t))
				if err != nil {
					t.Fatal(err)
				}
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
			if err := WriteSessionMeta(path, &meta); err != nil {
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

// The version three payload and identities were produced by the released beta.19 CLI.
// Later versions add only their versioned profile identity and ACP option fields here.
func legacyCreationWitnessForTest(t *testing.T, version int) []byte {
	t.Helper()
	profileFields := ""
	if version >= 5 {
		profileFields += `"acp_options":[{"id":"thinking","bool_value":true}],`
	}
	if version >= 4 {
		profileFields += `"profile_id":"00000000000000000000000000",`
	}
	profile := fmt.Sprintf(
		`{"version":%d,"agent_name":"general","provider":"codex","model":"gpt-5.6-sol","reasoning_effort":"medium","speed":"normal",%s"workspace_id":"ws_328f5cbfe158839e","cwd":"/Users/pedronauck","sandbox_mode":"ref","sandbox_ref":"local","permissions":"approve-all"}`,
		version,
		profileFields,
	)
	const options = `{"session_id":"sess-1910f4bd9212e8db","name":"Archived editorial planning","network_owner_key":"session:sess-1910f4bd9212e8db","network_participation":{"version":"network-participation/v1","mode":"local","source":"built_in_local"},"session_type":"user"}`
	ref := "profile:sha256:" + sha256Hex([]byte(profile))
	policy := "policy:sha256:" + sha256Hex([]byte("compozy-session-policy-v1\x00"+profile))
	creation := "creation:sha256:" + sha256Hex([]byte(policy+"\x00"+options))
	if version == 3 && (ref != "profile:sha256:22bcc2473f9689e8aa4f92f53ca53331765614f1bbc95afac2eeea7ba3a1d46a" ||
		policy != "policy:sha256:5168aa73c315468c831f959f3a98a9d36830733cbb020e3a32b8c830cbe61441" ||
		creation != "creation:sha256:d121383e7be30af98503201720760b95cdc132d1345c8a360e349b1a0a008690") {
		t.Fatal("fixture no longer matches the released creation witness")
	}
	return []byte(
		fmt.Sprintf(
			`{"id":"sess-1910f4bd9212e8db","agent_name":"general","workspace_id":"ws_328f5cbfe158839e","state":"stopped","runtime_status":"unbound","creation_profile":%s,"creation_options":%s,"creation_profile_ref":%q,"policy_spec_digest":%q,"creation_digest":%q,"created_at":"2026-10-04T09:35:00Z","updated_at":"2026-10-04T09:35:00Z"}`,
			profile,
			options,
			ref,
			policy,
			creation,
		),
	)
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

func TestUpgradeSessionLineageKind(t *testing.T) {
	t.Parallel()

	t.Run("Should apply the catalog backfill rule to kindless lineage", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			sessionType string
			lineage     *SessionLineage
			want        LineageKind
			changed     bool
		}{
			{
				name:        "Should mark a spawned session as spawn",
				sessionType: "spawned",
				lineage: &SessionLineage{
					ParentSessionID: "sess-parent",
					RootSessionID:   "sess-parent",
					SpawnDepth:      1,
				},
				want:    LineageKindSpawn,
				changed: true,
			},
			{
				name:        "Should mark a spawn role as spawn",
				sessionType: "coordinator",
				lineage:     &SessionLineage{RootSessionID: "sess-root", SpawnRole: "worker"},
				want:        LineageKindSpawn,
				changed:     true,
			},
			{
				name:        "Should mark a parented user session as provenance",
				sessionType: "user",
				lineage: &SessionLineage{
					ParentSessionID: "sess-parent",
					RootSessionID:   "sess-parent",
					SpawnDepth:      1,
				},
				want:    LineageKindProvenance,
				changed: true,
			},
			{
				name:        "Should keep a root session unkinded",
				sessionType: "user",
				lineage:     &SessionLineage{RootSessionID: "sess-root"},
				want:        LineageKindRoot,
				changed:     false,
			},
			{
				name:        "Should keep an existing kind",
				sessionType: "user",
				lineage: &SessionLineage{
					ParentSessionID: "sess-parent", RootSessionID: "sess-parent", SpawnDepth: 1,
					Kind: LineageKindRecovery,
				},
				want:    LineageKindRecovery,
				changed: false,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				before := *tt.lineage
				changed := UpgradeSessionLineageKind(tt.sessionType, tt.lineage)
				if changed != tt.changed || tt.lineage.Kind != tt.want {
					t.Fatalf(
						"UpgradeSessionLineageKind(%q) = %v kind %q, want %v kind %q",
						tt.sessionType, changed, tt.lineage.Kind, tt.changed, tt.want,
					)
				}
				before.Kind = tt.lineage.Kind
				if !reflect.DeepEqual(*tt.lineage, before) {
					t.Fatalf("UpgradeSessionLineageKind() changed other fields: %#v, want %#v", *tt.lineage, before)
				}
				if UpgradeSessionLineageKind(tt.sessionType, tt.lineage) {
					t.Fatal("UpgradeSessionLineageKind() second call changed lineage, want idempotent")
				}
			})
		}
		if UpgradeSessionLineageKind("user", nil) {
			t.Fatal("UpgradeSessionLineageKind(nil) = true, want false")
		}
	})

	t.Run("Should upgrade a pre-feature metadata document at read without rewriting it", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), SessionMetaName)
		payload := []byte(`{
  "id": "sess-child",
  "agent_name": "coder",
  "workspace_id": "ws-current",
  "session_type": "user",
  "lineage": {"parent_session_id": "sess-parent", "root_session_id": "sess-parent", "spawn_depth": 1},
  "state": "stopped",
  "runtime_status": "unbound",
  "created_at": "2026-04-03T17:00:00Z",
  "updated_at": "2026-04-03T17:01:00Z"
}
`)
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		meta, err := ReadSessionMeta(path)
		if err != nil {
			t.Fatalf("ReadSessionMeta(pre-feature) error = %v", err)
		}
		if meta.Lineage == nil || meta.Lineage.Kind != LineageKindProvenance ||
			meta.Lineage.ParentSessionID != "sess-parent" {
			t.Fatalf("ReadSessionMeta(pre-feature).Lineage = %#v, want upgraded provenance", meta.Lineage)
		}
		onDisk, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if !bytes.Equal(onDisk, payload) {
			t.Fatalf("ReadSessionMeta() rewrote the document:\n%s", onDisk)
		}
	})
}
