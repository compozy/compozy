package contract

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestAgentContractNormalizationAndJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize agent context JSON without raw claim tokens", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.April, 26, 12, 0, 0, 0, time.UTC)
		ttl := now.Add(2 * time.Hour)
		channel := CoordinationChannelPayload{
			ID:          "coord-run-1",
			DisplayName: "TASK-1 coordination",
			WorkspaceID: "ws-1",
			TaskID:      "task-1",
			RunID:       "run-1",
		}
		lease := TaskRunLeaseSummaryPayload{
			TaskID:         "task-1",
			RunID:          "run-1",
			Status:         taskpkg.TaskRunStatusRunning,
			SessionID:      "sess-child",
			ClaimTokenHash: "sha256:abc",
			ResolvedNetworkParticipation: participation.CloneSpec(
				contractTestLiveParticipation("ws-1", channel.ID),
			),
			CoordinationChannel: &channel,
		}
		lineage := &SessionLineagePayload{
			ParentSessionID:  "sess-parent",
			RootSessionID:    "sess-parent",
			SpawnDepth:       1,
			SpawnRole:        "worker",
			TTLExpiresAt:     &ttl,
			AutoStopOnParent: true,
			SpawnBudget: SpawnBudgetPayload{
				MaxChildren: 5,
				MaxDepth:    1,
				TTLSeconds:  int64((2 * time.Hour).Seconds()),
			},
		}

		payload := NormalizeAgentContextPayload(&AgentContextPayload{
			Self: AgentIdentityPayload{
				SessionID: "sess-child",
				AgentName: "codex",
				Provider:  "openai",
				Model:     "gpt-5.4",
			},
			Workspace: AgentWorkspacePayload{ID: "ws-1", Name: "agh", RootDir: "/workspace/agh"},
			Session: AgentSessionPayload{
				ID:                           "sess-child",
				Name:                         "worker",
				Type:                         session.SessionTypeUser,
				State:                        session.StateActive,
				ResolvedNetworkParticipation: participation.CloneSpec(*lease.ResolvedNetworkParticipation),
				Lineage:                      lineage,
				CreatedAt:                    now,
				UpdatedAt:                    now,
			},
			Task: AgentTaskContextPayload{
				Available: true,
				Task: &TaskReferencePayload{
					ID:          "task-1",
					Identifier:  "TASK-1",
					Title:       "Implement contracts",
					Status:      taskpkg.TaskStatusInProgress,
					Priority:    taskpkg.PriorityHigh,
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-1",
				},
				Lease: &lease,
			},
			CoordinationChannel: AgentCoordinationChannelContextPayload{Available: true, Channel: &channel},
			InboxSummary: AgentInboxSummaryPayload{
				Section: AgentContextSectionMetaPayload{Limit: 20},
			},
			PeerRoster: AgentPeerRosterPayload{
				Section: AgentContextSectionMetaPayload{Limit: 10},
			},
			Capabilities: AgentCapabilitySectionPayload{
				Section: AgentContextSectionMetaPayload{Limit: 10},
			},
			Limits: AgentLimitsPayload{
				MaxChildren:         5,
				MaxSpawnDepth:       1,
				MaxActiveTaskLeases: 1,
				ContextSectionLimit: 20,
			},
			Provenance: AgentContextProvenancePayload{GeneratedAt: now, Source: "test"},
		})

		object := marshalContractObject(t, AgentContextResponse{Context: payload})
		contextObject := nestedContractObject(t, object, "context")
		assertContractKeys(t, contextObject, "self", "workspace", "session", "task", "coordination_channel",
			"inbox_summary", "peer_roster", "capabilities", "limits", "provenance")

		sessionObject := nestedContractObject(t, contextObject, "session")
		assertContractKeys(t, sessionObject, "id", "name", "type", "state", "resolved_network_participation",
			"lineage", "created_at", "updated_at")
		sessionParticipation := nestedContractObject(t, sessionObject, "resolved_network_participation")
		if sessionParticipation["channel_id"] != "coord-run-1" {
			t.Fatalf("session resolved participation JSON = %#v", sessionParticipation)
		}
		lineageObject := nestedContractObject(t, sessionObject, "lineage")
		assertContractKeys(t, lineageObject, "parent_session_id", "root_session_id", "spawn_depth", "spawn_role",
			"ttl_expires_at", "auto_stop_on_parent", "spawn_budget", "permission_policy")
		permissionPolicy := nestedContractObject(t, lineageObject, "permission_policy")
		assertContractArray(t, permissionPolicy, "tools")
		assertContractArray(t, permissionPolicy, "skills")
		assertContractArray(t, permissionPolicy, "mcp_servers")
		assertContractArray(t, permissionPolicy, "workspace_paths")
		assertContractArray(t, permissionPolicy, "network_channels")
		assertContractArray(t, permissionPolicy, "sandbox_profiles")

		coordination := nestedContractObject(t, contextObject, "coordination_channel")
		channelObject := nestedContractObject(t, coordination, "channel")
		if channelObject["id"] != "coord-run-1" || channelObject["display_name"] != "TASK-1 coordination" {
			t.Fatalf("coordination channel JSON = %#v", channelObject)
		}
		messageKinds := assertContractArray(t, channelObject, "allowed_message_kinds")
		if len(messageKinds) != len(CoordinationMessageKinds()) {
			t.Fatalf("allowed_message_kinds length = %d, want %d", len(messageKinds), len(CoordinationMessageKinds()))
		}

		inbox := nestedContractObject(t, contextObject, "inbox_summary")
		if items := assertContractArray(t, inbox, "items"); len(items) != 0 {
			t.Fatalf("inbox items length = %d, want 0", len(items))
		}
		peers := nestedContractObject(t, contextObject, "peer_roster")
		if peerList := assertContractArray(t, peers, "peers"); len(peerList) != 0 {
			t.Fatalf("peers length = %d, want 0", len(peerList))
		}
		capabilities := nestedContractObject(t, contextObject, "capabilities")
		if capabilityList := assertContractArray(t, capabilities, "capabilities"); len(capabilityList) != 0 {
			t.Fatalf("capabilities length = %d, want 0", len(capabilityList))
		}

		if err := ValidateNoRawClaimTokenField(AgentContextResponse{Context: payload}); err != nil {
			t.Fatalf("ValidateNoRawClaimTokenField(context) error = %v", err)
		}
	})
}

func TestClaimTokenExposureBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("Should expose only claim token hashes on public task payloads", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.April, 26, 12, 0, 0, 0, time.UTC)
		readRun := TaskRunPayload{
			ID:             "run-1",
			TaskID:         "task-1",
			Status:         taskpkg.TaskRunStatusClaimed,
			Attempt:        1,
			Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindAgentSession, Ref: "sess-1"},
			ClaimTokenHash: "sha256:abc",
			QueuedAt:       now,
		}

		readJSON := marshalContractString(t, TaskRunResponse{Run: readRun})
		if strings.Contains(readJSON, `"claim_token"`) {
			t.Fatalf("read model leaked raw claim token: %s", readJSON)
		}
		if !strings.Contains(readJSON, `"claim_token_hash"`) {
			t.Fatalf("read model missing claim token hash: %s", readJSON)
		}
		if err := ValidateNoRawClaimTokenField(TaskStreamEventPayload{
			Sequence: 1,
			Type:     "task.run.updated",
			Timeline: TaskTimelineItemPayload{
				Sequence: 1,
				EventID:  "evt-1",
				Task: TaskReferencePayload{
					ID:     "task-1",
					Title:  "Task",
					Status: taskpkg.TaskStatusInProgress,
					Scope:  taskpkg.ScopeWorkspace,
				},
				Run: &TaskRunSummaryPayload{
					ID:             "run-1",
					TaskID:         "task-1",
					Status:         taskpkg.TaskRunStatusClaimed,
					Attempt:        1,
					MaxAttempts:    1,
					ClaimTokenHash: "sha256:abc",
					QueuedAt:       now,
				},
				EventType: "task.run.updated",
				Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-1"},
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindAgentSession, Ref: "sess-1"},
				Timestamp: now,
			},
		}); err != nil {
			t.Fatalf("SSE task stream payload leaked raw claim token: %v", err)
		}

		claimResponse := AgentTaskClaimResponse{
			Claim: AgentTaskClaimPayload{
				Task: TaskReferencePayload{
					ID:     "task-1",
					Title:  "Task",
					Status: taskpkg.TaskStatusInProgress,
					Scope:  taskpkg.ScopeWorkspace,
				},
				Run: readRun,
				Lease: TaskRunLeaseSummaryPayload{
					TaskID:         "task-1",
					RunID:          "run-1",
					Status:         taskpkg.TaskRunStatusClaimed,
					ClaimTokenHash: "sha256:abc",
				},
			},
		}
		claimJSON := marshalContractString(t, claimResponse)
		if strings.Contains(claimJSON, `"claim_token"`) || strings.Contains(claimJSON, "raw-secret-token") {
			t.Fatalf("claim response leaked raw token: %s", claimJSON)
		}
		found, err := ContainsRawClaimTokenField(claimResponse)
		if err != nil {
			t.Fatalf("ContainsRawClaimTokenField(claimResponse) error = %v", err)
		}
		if found {
			t.Fatal("ContainsRawClaimTokenField(claimResponse) = true, want false")
		}
	})
}

func TestCoordinationMessageMetadataValidationRejectsRawClaimTokens(t *testing.T) {
	t.Parallel()

	validJSON := []byte(`{
		"task_id":"task-1",
		"run_id":"run-1",
		"workflow_id":"workflow-1",
		"channel_id":"coord-run-1",
		"message_kind":"status",
		"correlation_id":"corr-1",
		"ext":{"safe":"true"}
	}`)
	var metadata CoordinationMessageMetadataPayload
	if err := json.Unmarshal(validJSON, &metadata); err != nil {
		t.Fatalf("json.Unmarshal(valid metadata) error = %v", err)
	}
	if err := metadata.Validate(); err != nil {
		t.Fatalf("metadata.Validate() error = %v", err)
	}

	testCases := []struct {
		name string
		body string
	}{
		{
			name: "Should reject top level claim token",
			body: `{"task_id":"task-1","run_id":"run-1","channel_id":"coord-run-1","message_kind":"status","correlation_id":"corr-1","claim_token":"raw"}`,
		},
		{
			name: "Should reject nested claim token",
			body: `{"task_id":"task-1","run_id":"run-1","channel_id":"coord-run-1","message_kind":"status","correlation_id":"corr-1","ext":{"nested":{"claim_token":"raw"}}}`,
		},
		{
			name: "Should reject token-shaped ext value",
			body: `{"task_id":"task-1","run_id":"run-1","channel_id":"coord-run-1","message_kind":"status","correlation_id":"corr-1","ext":{"debug":"contains agh_claim_raw"}}`,
		},
		{
			name: "Should reject uppercase token-shaped ext value",
			body: `{"task_id":"task-1","run_id":"run-1","channel_id":"coord-run-1","message_kind":"status","correlation_id":"corr-1","ext":{"debug":"contains AGH_CLAIM_RAW"}}`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var decoded CoordinationMessageMetadataPayload
			err := json.Unmarshal([]byte(tc.body), &decoded)
			if !errors.Is(err, ErrRawClaimTokenMetadata) {
				t.Fatalf("json.Unmarshal() error = %v, want ErrRawClaimTokenMetadata", err)
			}
		})
	}

	for _, field := range []string{"from", "proof"} {
		t.Run("Should reject caller supplied "+field+" on agent channel send", func(t *testing.T) {
			t.Parallel()

			raw := `{"body":{"text":"safe"},"metadata":` + string(validJSON) +
				`,"` + field + `":"alice@39f713d0a644253f04529421b9f51b9b"}`
			var request AgentChannelSendRequest
			err := json.Unmarshal([]byte(raw), &request)
			if err == nil || !strings.Contains(err.Error(), "sender identity and proof are daemon-derived") {
				t.Fatalf("json.Unmarshal(agent send %s) error = %v, want daemon-derived identity rejection", field, err)
			}
		})
	}

	var invalidKind CoordinationMessageMetadataPayload
	err := json.Unmarshal(
		[]byte(
			`{"task_id":"task-1","run_id":"run-1","channel_id":"coord-run-1","message_kind":"claim_token","correlation_id":"corr-1"}`,
		),
		&invalidKind,
	)
	if !errors.Is(err, ErrInvalidCoordinationMessageMetadata) {
		t.Fatalf("json.Unmarshal(invalid kind) error = %v, want ErrInvalidCoordinationMessageMetadata", err)
	}
}

func TestContainsUnsafePublicContractJSONRejectsDelimiterNormalizedKeys(t *testing.T) {
	t.Parallel()

	t.Run("Should reject delimiter-normalized secret keys", func(t *testing.T) {
		t.Parallel()

		if !containsUnsafePublicContractJSON([]byte(`{"claim.token":"raw"}`)) {
			t.Fatal("containsUnsafePublicContractJSON(claim.token) = false, want true")
		}
		if !containsUnsafePublicContractJSON([]byte(`{"api key":"raw"}`)) {
			t.Fatal("containsUnsafePublicContractJSON(api key) = false, want true")
		}
	})
}

func marshalContractString(t *testing.T, value any) string {
	t.Helper()

	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(content)
}

func marshalContractObject(t *testing.T, value any) map[string]any {
	t.Helper()

	var object map[string]any
	if err := json.Unmarshal([]byte(marshalContractString(t, value)), &object); err != nil {
		t.Fatalf("json.Unmarshal(object) error = %v", err)
	}
	return object
}

func nestedContractObject(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()

	nested, ok := object[key].(map[string]any)
	if !ok {
		t.Fatalf("%s type = %T, want object in %#v", key, object[key], object)
	}
	return nested
}

func assertContractArray(t *testing.T, object map[string]any, key string) []any {
	t.Helper()

	array, ok := object[key].([]any)
	if !ok {
		t.Fatalf("%s type = %T, want array in %#v", key, object[key], object)
	}
	return array
}

func assertContractKeys(t *testing.T, object map[string]any, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if _, ok := object[key]; !ok {
			t.Fatalf("object missing key %q: %#v", key, object)
		}
	}
}
