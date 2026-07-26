package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
)

func TestAgentSoulCommands(t *testing.T) {
	t.Run("Should inspect soul as json and human with workspace scope", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getAgentSoulFn: func(_ context.Context, name string, query AgentQuery) (AgentSoulRecord, error) {
				if name != "coder" {
					t.Fatalf("GetAgentSoul() name = %q, want coder", name)
				}
				if query.Workspace != "checkout-api" {
					t.Fatalf("GetAgentSoul() workspace = %q, want checkout-api", query.Workspace)
				}
				return AgentSoulRecord{
					AgentName:        "coder",
					Enabled:          true,
					Present:          true,
					Active:           true,
					Valid:            true,
					ValidationStatus: contract.AuthoredValidationValid,
					Digest:           "sha256:soul",
					SourcePath:       ".agh/agents/coder/SOUL.md",
				}, nil
			},
		}
		deps := newTestDeps(t, client)

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"agent",
			"soul",
			"inspect",
			"coder",
			"--workspace",
			"checkout-api",
			"--json",
		)
		if err != nil {
			t.Fatalf("agent soul inspect --json error = %v", err)
		}
		if strings.Contains(stdout, "/Users/") {
			t.Fatalf("agent soul inspect leaked absolute source path: %s", stdout)
		}
		var payload AgentSoulRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(agent soul inspect) error = %v", err)
		}
		if payload.AgentName != "coder" || payload.Digest != "sha256:soul" || !payload.Valid {
			t.Fatalf("payload = %#v, want valid coder soul", payload)
		}

		human, _, err := executeRootCommand(
			t,
			deps,
			"agent",
			"soul",
			"inspect",
			"coder",
			"--workspace",
			"checkout-api",
		)
		if err != nil {
			t.Fatalf("agent soul inspect human error = %v", err)
		}
		if !strings.Contains(human, "Agent Soul") || !strings.Contains(human, "sha256:soul") {
			t.Fatalf("human output = %q, want soul summary", human)
		}
	})

	t.Run("Should write soul with expected digest and file body", func(t *testing.T) {
		t.Parallel()

		bodyPath := filepath.Join(t.TempDir(), "SOUL.md")
		if err := os.WriteFile(bodyPath, []byte("# Soul\n\nBe precise.\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(SOUL.md) error = %v", err)
		}
		client := &stubClient{
			putAgentSoulFn: func(_ context.Context, name string, request AgentSoulPutRequest) (AgentSoulMutationRecord, error) {
				if name != "coder" || request.AgentName != "coder" {
					t.Fatalf("PutAgentSoul() agent = %q/%q, want coder", name, request.AgentName)
				}
				if request.WorkspaceID != "checkout-api" {
					t.Fatalf("PutAgentSoul() workspace = %q, want checkout-api", request.WorkspaceID)
				}
				if request.ExpectedDigest != "sha256:old" {
					t.Fatalf("PutAgentSoul() expected_digest = %q, want sha256:old", request.ExpectedDigest)
				}
				if request.Body != "# Soul\n\nBe precise.\n" {
					t.Fatalf("PutAgentSoul() body = %q", request.Body)
				}
				return AgentSoulMutationRecord{
					Soul: AgentSoulRecord{
						AgentName:        "coder",
						Valid:            true,
						ValidationStatus: contract.AuthoredValidationValid,
						Digest:           "sha256:new",
					},
					Revision: AgentSoulRevisionRecord{
						ID:        "rev-1",
						AgentName: "coder",
						Action:    contract.AgentSoulRevisionPut,
						NewDigest: "sha256:new",
						CreatedAt: fixedTestNow,
					},
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"agent",
			"soul",
			"write",
			"coder",
			"--file",
			bodyPath,
			"--expected-digest",
			"sha256:old",
			"--workspace",
			"checkout-api",
			"--json",
		)
		if err != nil {
			t.Fatalf("agent soul write error = %v", err)
		}
		var payload AgentSoulMutationRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(agent soul write) error = %v", err)
		}
		if payload.Revision.ID != "rev-1" || payload.Soul.Digest != "sha256:new" {
			t.Fatalf("payload = %#v, want mutation response", payload)
		}
	})

	t.Run("Should create soul without expected digest when file is absent", func(t *testing.T) {
		t.Parallel()

		bodyPath := filepath.Join(t.TempDir(), "SOUL.md")
		if err := os.WriteFile(bodyPath, []byte("# Soul\n\nCreate me.\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(SOUL.md) error = %v", err)
		}
		client := &stubClient{
			putAgentSoulFn: func(_ context.Context, name string, request AgentSoulPutRequest) (AgentSoulMutationRecord, error) {
				if name != "coder" || request.AgentName != "coder" {
					t.Fatalf("PutAgentSoul() agent = %q/%q, want coder", name, request.AgentName)
				}
				if request.ExpectedDigest != "" {
					t.Fatalf("PutAgentSoul() expected_digest = %q, want empty create digest", request.ExpectedDigest)
				}
				if request.WorkspaceID != "checkout-api" || request.Body != "# Soul\n\nCreate me.\n" {
					t.Fatalf("PutAgentSoul() request = %#v", request)
				}
				return AgentSoulMutationRecord{
					Soul: AgentSoulRecord{
						AgentName:        "coder",
						Valid:            true,
						ValidationStatus: contract.AuthoredValidationValid,
						Digest:           "sha256:created",
					},
					Revision: AgentSoulRevisionRecord{
						ID:        "rev-create",
						AgentName: "coder",
						Action:    contract.AgentSoulRevisionPut,
						NewDigest: "sha256:created",
						CreatedAt: fixedTestNow,
					},
				}, nil
			},
		}

		stdout, stderr, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"agent",
			"soul",
			"write",
			"coder",
			"--file",
			bodyPath,
			"--workspace",
			"checkout-api",
			"--json",
		)
		if err != nil {
			t.Fatalf("agent soul create error = %v; stderr=%s; stdout=%s", err, stderr, stdout)
		}
		var payload AgentSoulMutationRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(agent soul create) error = %v", err)
		}
		if payload.Revision.ID != "rev-create" || payload.Soul.Digest != "sha256:created" {
			t.Fatalf("payload = %#v, want created soul mutation", payload)
		}
	})

	t.Run("Should report stale soul conflicts deterministically", func(t *testing.T) {
		t.Parallel()

		bodyPath := filepath.Join(t.TempDir(), "SOUL.md")
		if err := os.WriteFile(bodyPath, []byte("# Soul\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(SOUL.md) error = %v", err)
		}
		client := &stubClient{
			putAgentSoulFn: func(context.Context, string, AgentSoulPutRequest) (AgentSoulMutationRecord, error) {
				return AgentSoulMutationRecord{}, errors.New("soul_conflict: expected_digest is stale")
			},
		}
		_, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"agent",
			"soul",
			"write",
			"coder",
			"--file",
			bodyPath,
			"--expected-digest",
			"sha256:old",
			"--json",
		)
		if err == nil || !strings.Contains(err.Error(), "soul_conflict") {
			t.Fatalf("agent soul write error = %v, want soul_conflict", err)
		}
	})
}

func TestAgentHeartbeatCommands(t *testing.T) {
	t.Run("Should return heartbeat status json with session health", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getAgentHeartbeatStatusFn: func(
				_ context.Context,
				name string,
				request AgentHeartbeatStatusRequest,
			) (AgentHeartbeatStatusRecord, error) {
				if name != "coder" || request.AgentName != "coder" {
					t.Fatalf("GetAgentHeartbeatStatus() agent = %q/%q, want coder", name, request.AgentName)
				}
				if request.WorkspaceID != "checkout-api" || request.SessionID != "sess-1" {
					t.Fatalf("GetAgentHeartbeatStatus() request = %#v", request)
				}
				if !request.IncludeSessionHealth {
					t.Fatalf("GetAgentHeartbeatStatus() IncludeSessionHealth = false, want true")
				}
				return AgentHeartbeatStatusRecord{
					AgentName:        "coder",
					Enabled:          true,
					Present:          true,
					Active:           true,
					Valid:            true,
					ValidationStatus: contract.AuthoredValidationValid,
					Digest:           "sha256:heartbeat",
					ConfigDigest:     "sha256:config",
					SessionHealth: &SessionHealthRecord{
						SessionID:       "sess-1",
						WorkspaceID:     "ws-1",
						AgentName:       "coder",
						State:           contract.SessionHealthStateIdle,
						Health:          contract.SessionHealthHealthy,
						Attachable:      true,
						EligibleForWake: true,
						UpdatedAt:       fixedTestNow,
					},
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"agent",
			"heartbeat",
			"status",
			"coder",
			"--workspace",
			"checkout-api",
			"--session",
			"sess-1",
			"--json",
		)
		if err != nil {
			t.Fatalf("agent heartbeat status error = %v", err)
		}
		var payload AgentHeartbeatStatusRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(agent heartbeat status) error = %v", err)
		}
		if payload.ConfigDigest != "sha256:config" || payload.SessionHealth == nil ||
			!payload.SessionHealth.EligibleForWake {
			t.Fatalf("payload = %#v, want config digest and eligible session health", payload)
		}
	})

	t.Run("Should write heartbeat with expected digest mapped to expected digest", func(t *testing.T) {
		t.Parallel()

		bodyPath := filepath.Join(t.TempDir(), "HEARTBEAT.md")
		if err := os.WriteFile(bodyPath, []byte("# Heartbeat\n\nCheck in.\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(HEARTBEAT.md) error = %v", err)
		}
		client := &stubClient{
			putAgentHeartbeatFn: func(
				_ context.Context,
				name string,
				request AgentHeartbeatPutRequest,
			) (AgentHeartbeatMutationRecord, error) {
				if name != "coder" || request.ExpectedDigest != "sha256:old" {
					t.Fatalf("PutAgentHeartbeat() = %q/%#v, want expected digest", name, request)
				}
				if request.WorkspaceID != "checkout-api" || request.Body != "# Heartbeat\n\nCheck in.\n" {
					t.Fatalf("PutAgentHeartbeat() request = %#v", request)
				}
				return AgentHeartbeatMutationRecord{
					Heartbeat: AgentHeartbeatRecord{
						AgentName:        "coder",
						Valid:            true,
						ValidationStatus: contract.AuthoredValidationValid,
						Digest:           "sha256:new",
					},
					Revision: AgentHeartbeatRevisionRecord{
						ID:        "rev-hb-1",
						AgentName: "coder",
						Operation: contract.HeartbeatRevisionWrite,
						NewDigest: "sha256:new",
						CreatedAt: fixedTestNow,
					},
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"agent",
			"heartbeat",
			"write",
			"coder",
			"--file",
			bodyPath,
			"--expected-digest",
			"sha256:old",
			"--workspace",
			"checkout-api",
			"--json",
		)
		if err != nil {
			t.Fatalf("agent heartbeat write error = %v", err)
		}
		var payload AgentHeartbeatMutationRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(agent heartbeat write) error = %v", err)
		}
		if payload.Revision.ID != "rev-hb-1" || payload.Heartbeat.Digest != "sha256:new" {
			t.Fatalf("payload = %#v, want heartbeat mutation", payload)
		}
	})

	t.Run("Should create heartbeat without expected digest when file is absent", func(t *testing.T) {
		t.Parallel()

		bodyPath := filepath.Join(t.TempDir(), "HEARTBEAT.md")
		if err := os.WriteFile(bodyPath, []byte("# Heartbeat\n\nCreate policy.\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(HEARTBEAT.md) error = %v", err)
		}
		client := &stubClient{
			putAgentHeartbeatFn: func(
				_ context.Context,
				name string,
				request AgentHeartbeatPutRequest,
			) (AgentHeartbeatMutationRecord, error) {
				if name != "coder" || request.AgentName != "coder" {
					t.Fatalf("PutAgentHeartbeat() agent = %q/%q, want coder", name, request.AgentName)
				}
				if request.ExpectedDigest != "" {
					t.Fatalf(
						"PutAgentHeartbeat() expected_digest = %q, want empty create digest",
						request.ExpectedDigest,
					)
				}
				if request.WorkspaceID != "checkout-api" || request.Body != "# Heartbeat\n\nCreate policy.\n" {
					t.Fatalf("PutAgentHeartbeat() request = %#v", request)
				}
				return AgentHeartbeatMutationRecord{
					Heartbeat: AgentHeartbeatRecord{
						AgentName:        "coder",
						Valid:            true,
						ValidationStatus: contract.AuthoredValidationValid,
						Digest:           "sha256:heartbeat-created",
					},
					Revision: AgentHeartbeatRevisionRecord{
						ID:        "rev-hb-create",
						AgentName: "coder",
						Operation: contract.HeartbeatRevisionWrite,
						NewDigest: "sha256:heartbeat-created",
						CreatedAt: fixedTestNow,
					},
				}, nil
			},
		}

		stdout, stderr, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"agent",
			"heartbeat",
			"write",
			"coder",
			"--file",
			bodyPath,
			"--workspace",
			"checkout-api",
			"--json",
		)
		if err != nil {
			t.Fatalf("agent heartbeat create error = %v; stderr=%s; stdout=%s", err, stderr, stdout)
		}
		var payload AgentHeartbeatMutationRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(agent heartbeat create) error = %v", err)
		}
		if payload.Revision.ID != "rev-hb-create" || payload.Heartbeat.Digest != "sha256:heartbeat-created" {
			t.Fatalf("payload = %#v, want heartbeat create mutation", payload)
		}
	})
}

func TestSessionAuthoredContextCommands(t *testing.T) {
	t.Run("Should return session health and inspect json with closed enum values", func(t *testing.T) {
		t.Parallel()

		health := SessionHealthRecord{
			SessionID:           "sess-1",
			WorkspaceID:         "ws-1",
			AgentName:           "coder",
			State:               contract.SessionHealthStateIdle,
			Health:              contract.SessionHealthHealthy,
			Attachable:          true,
			EligibleForWake:     true,
			IneligibilityReason: "",
			UpdatedAt:           fixedTestNow,
		}
		client := &stubClient{
			getSessionHealthFn: func(_ context.Context, id string) (SessionHealthRecord, error) {
				if id != "sess-1" {
					t.Fatalf("GetSessionHealth() id = %q, want sess-1", id)
				}
				return health, nil
			},
			inspectSessionFn: func(_ context.Context, id string, query SessionInspectQuery) (SessionInspectRecord, error) {
				if id != "sess-1" || !query.IncludeRecentWakeEvents {
					t.Fatalf("InspectSession() = %q/%#v, want wake event expansion", id, query)
				}
				return SessionInspectRecord{SessionID: "sess-1", Health: health, ConfigDigest: "sha256:config"}, nil
			},
		}
		deps := newTestDeps(t, client)

		healthOut, _, err := executeRootCommand(t, deps, "session", "health", "sess-1", "--json")
		if err != nil {
			t.Fatalf("session health error = %v", err)
		}
		var healthPayload SessionHealthRecord
		if err := json.Unmarshal([]byte(healthOut), &healthPayload); err != nil {
			t.Fatalf("json.Unmarshal(session health) error = %v", err)
		}
		if healthPayload.State != contract.SessionHealthStateIdle ||
			healthPayload.Health != contract.SessionHealthHealthy ||
			!healthPayload.EligibleForWake {
			t.Fatalf("health payload = %#v, want closed healthy idle state", healthPayload)
		}

		inspectOut, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"inspect",
			"sess-1",
			"--include-wake-events",
			"--json",
		)
		if err != nil {
			t.Fatalf("session inspect error = %v", err)
		}
		var inspectPayload SessionInspectRecord
		if err := json.Unmarshal([]byte(inspectOut), &inspectPayload); err != nil {
			t.Fatalf("json.Unmarshal(session inspect) error = %v", err)
		}
		if inspectPayload.Health.State != contract.SessionHealthStateIdle ||
			inspectPayload.ConfigDigest != "sha256:config" {
			t.Fatalf("inspect payload = %#v, want health and config digest", inspectPayload)
		}
	})

	t.Run("Should refresh session soul through managed client", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			refreshSessionSoulFn: func(
				_ context.Context,
				id string,
				request SessionSoulRefreshRequest,
			) (AgentSoulRecord, error) {
				if id != "sess-1" || request.ExpectedDigest != "sha256:old" {
					t.Fatalf("RefreshSessionSoul() = %q/%#v, want expected digest", id, request)
				}
				return AgentSoulRecord{
					AgentName:        "coder",
					Valid:            true,
					ValidationStatus: contract.AuthoredValidationValid,
					Digest:           "sha256:new",
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"session",
			"soul",
			"refresh",
			"sess-1",
			"--expected-digest",
			"sha256:old",
			"--json",
		)
		if err != nil {
			t.Fatalf("session soul refresh error = %v", err)
		}
		var payload AgentSoulRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(session soul refresh) error = %v", err)
		}
		if payload.Digest != "sha256:new" {
			t.Fatalf("payload = %#v, want refreshed digest", payload)
		}
	})

	t.Run("Should not expose session heartbeat command", func(t *testing.T) {
		t.Parallel()

		root := newRootCommand(newTestDeps(t, &stubClient{}))
		found, _, err := root.Find([]string{"session", "heartbeat"})
		if err == nil && found != nil && found.Name() == "heartbeat" {
			t.Fatal("session heartbeat command exists")
		}
		sessionCmd, _, err := root.Find([]string{"session"})
		if err != nil {
			t.Fatalf("find session command: %v", err)
		}
		usage := sessionCmd.UsageString()
		if strings.Contains(usage, "heartbeat") {
			t.Fatalf("session help contains forbidden heartbeat command:\n%s", usage)
		}
	})
}
