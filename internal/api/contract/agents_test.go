package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestAgentContractNormalizationAndJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize agent context JSON without raw claim tokens", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.April, 26, 12, 0, 0, 0, time.UTC)
		ttl := now.Add(2 * time.Hour)

		lease := TaskRunLeaseSummaryPayload{
			TaskID:         "task-1",
			RunID:          "run-1",
			Status:         taskpkg.TaskRunStatusRunning,
			SessionID:      "sess-child",
			ClaimTokenHash: "sha256:abc",
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
			Workspace: AgentWorkspacePayload{ID: "ws-1", Name: "compozy", RootDir: "/workspace/compozy"},
			Session: AgentSessionPayload{
				ID:    "sess-child",
				Name:  "worker",
				Type:  session.SessionTypeUser,
				State: session.StateActive,

				Lineage:   lineage,
				CreatedAt: now,
				UpdatedAt: now,
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
		assertContractKeys(
			t,
			contextObject,
			"self",
			"workspace",
			"session",
			"task",
			"capabilities",
			"limits",
			"provenance",
		)

		sessionObject := nestedContractObject(t, contextObject, "session")
		assertContractKeys(t, sessionObject, "id", "name", "type", "state",
			"lineage", "created_at", "updated_at")

		lineageObject := nestedContractObject(t, sessionObject, "lineage")
		assertContractKeys(t, lineageObject, "parent_session_id", "root_session_id", "spawn_depth", "spawn_role",
			"ttl_expires_at", "auto_stop_on_parent", "spawn_budget", "permission_policy")
		permissionPolicy := nestedContractObject(t, lineageObject, "permission_policy")
		assertContractArray(t, permissionPolicy, "tools")
		assertContractArray(t, permissionPolicy, "skills")
		assertContractArray(t, permissionPolicy, "mcp_servers")
		assertContractArray(t, permissionPolicy, "workspace_paths")

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
