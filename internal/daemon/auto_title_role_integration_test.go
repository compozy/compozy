//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	sessionpkg "github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
)

func TestAutoTitleRoleIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should apply auto-title role toggles without a daemon restart", func(t *testing.T) {
		t.Parallel()
		runAutoTitleRoleLiveToggleIntegration(t)
	})

	t.Run("Should fall back after a pre-acceptance provider failure and expose the durable attempt", func(t *testing.T) {
		t.Parallel()
		runAutoTitleRoleFallbackIntegration(t)
	})

	t.Run("Should clean up every exhausted pre-acceptance attempt", func(t *testing.T) {
		t.Parallel()
		runAutoTitleRoleExhaustionIntegration(t)
	})

	t.Run("Should stop fallback forever after an accepted attempt fails", func(t *testing.T) {
		t.Parallel()
		runAutoTitleRolePostAcceptanceFailureIntegration(t)
	})
}

func startAutoTitleRoleHarness(
	t *testing.T,
	mutate func(*aghconfig.Config),
) *e2etest.RuntimeHarness {
	t.Helper()
	acpmock.RequireDriver(t)
	return e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *aghconfig.Config) {
			cfg.Roles.MemoryExtractor.Enabled = false
			if mutate != nil {
				mutate(cfg)
			}
		}},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "auto_title_fixture.json"),
			FixtureAgent: "auto-title-agent",
			AgentName:    "auto-title-agent",
		}},
	})
}

func runAutoTitleRoleExhaustionIntegration(t *testing.T) {
	t.Helper()

	harness := startAutoTitleRoleHarness(t, func(cfg *aghconfig.Config) {
		for _, provider := range []string{"unreachable-primary", "unreachable-fallback"} {
			cfg.Providers[provider] = aghconfig.ProviderConfig{
				Command:      "/missing/" + provider,
				Harness:      aghconfig.ProviderHarnessACP,
				AuthMode:     aghconfig.ProviderAuthModeNone,
				NoneSecurity: aghconfig.ProviderNoneSecurityLocalTransport,
			}
		}
		cfg.Roles.AutoTitle.Provider = "unreachable-primary"
		cfg.Roles.AutoTitle.Model = "primary-model"
		cfg.Roles.AutoTitle.FallbackChain = []aghconfig.RoleFallback{{
			Provider: "unreachable-fallback",
			Model:    "fallback-model",
		}}
	})
	registration, ok := harness.MockAgentRegistration("auto-title-agent")
	if !ok {
		t.Fatal("MockAgentRegistration(auto-title-agent) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	root := createFixtureBackedSession(t, ctx, harness, "auto-title-agent", "")
	if _, err := harness.PromptSession(ctx, root.ID, "Implement checkout retry fencing"); err != nil {
		t.Fatalf("PromptSession(exhausted role) error = %v", err)
	}

	logsPath := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) +
		"&type=role.fallback.used&limit=10"
	sessionReader := openWorkspaceRoleSessionReader(t, ctx, harness)
	waitForRuntimeCondition(t, "exhausted role cleanup", 10*time.Second, func() bool {
		var logs aghcontract.LogsListResponse
		if err := harness.UDSJSON(ctx, http.MethodGet, logsPath, nil, &logs); err != nil || len(logs.Events) != 1 {
			return false
		}
		sessions := sessionReader()
		return len(sessions) == 1 && sessions[0].ID == root.ID && sessions[0].State == string(sessionpkg.StateActive)
	})

	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(exhausted role) error = %v", err)
	}
	if got := len(acpmock.PromptDiagnostics(records)); got != 1 {
		t.Fatalf("exhausted role prompt count = %d, want only the root prompt", got)
	}
	current, err := harness.GetSession(ctx, root.ID)
	if err != nil {
		t.Fatalf("GetSession(exhausted role root) error = %v", err)
	}
	if current.Name != "" {
		t.Fatalf("exhausted role root title = %q, want unchanged", current.Name)
	}
}

func runAutoTitleRolePostAcceptanceFailureIntegration(t *testing.T) {
	t.Helper()

	harness := startAutoTitleRoleHarness(t, func(cfg *aghconfig.Config) {
		cfg.Roles.AutoTitle.Provider = acpmock.ProviderName
		cfg.Roles.AutoTitle.Model = "primary-title-model"
		cfg.Roles.AutoTitle.FallbackChain = []aghconfig.RoleFallback{{
			Provider: acpmock.ProviderName,
			Model:    "fallback-title-model",
		}}
	})
	registration, ok := harness.MockAgentRegistration("auto-title-agent")
	if !ok {
		t.Fatal("MockAgentRegistration(auto-title-agent) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	root := createFixtureBackedSession(t, ctx, harness, "auto-title-agent", "")
	if _, err := harness.PromptSession(ctx, root.ID, "Implement post-acceptance fencing"); err != nil {
		t.Fatalf("PromptSession(post-acceptance failure) error = %v", err)
	}

	var child store.SessionInfo
	sessionReader := openWorkspaceRoleSessionReader(t, ctx, harness)
	waitForRuntimeCondition(t, "accepted role child stopped", 10*time.Second, func() bool {
		sessions := sessionReader()
		active := 0
		foundChild := false
		for _, candidate := range sessions {
			if candidate.State == string(sessionpkg.StateActive) {
				active++
			}
			if candidate.Lineage != nil && candidate.Lineage.SpawnRole == sessionpkg.SpawnRoleAutoTitle {
				child = candidate
				foundChild = candidate.State == string(sessionpkg.StateStopped)
			}
		}
		return foundChild && active == 1
	})

	var logs aghcontract.LogsListResponse
	logsPath := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) +
		"&type=role.fallback.used&limit=10"
	if err := harness.UDSJSON(ctx, http.MethodGet, logsPath, nil, &logs); err != nil {
		t.Fatalf("UDS post-acceptance fallback logs error = %v", err)
	}
	if len(logs.Events) != 0 {
		t.Fatalf("post-acceptance fallback events = %#v, want none", logs.Events)
	}

	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(post-acceptance failure) error = %v", err)
	}
	childPrompts := 0
	for _, record := range acpmock.ProtocolDiagnostics(acpmock.DiagnosticsForAGHSession(records, child.ID)) {
		if record.ProtocolMethod == acpsdk.AgentMethodSessionPrompt {
			childPrompts++
		}
	}
	if childPrompts != 1 {
		t.Fatalf("post-acceptance synthetic prompt attempts = %d, want exactly 1", childPrompts)
	}
	current, err := harness.GetSession(ctx, root.ID)
	if err != nil {
		t.Fatalf("GetSession(post-acceptance root) error = %v", err)
	}
	if current.Name != "" {
		t.Fatalf("post-acceptance root title = %q, want unchanged", current.Name)
	}
}

func readWorkspaceRoleSessions(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) []store.SessionInfo {
	t.Helper()
	return openWorkspaceRoleSessionReader(t, ctx, harness)()
}

func openWorkspaceRoleSessionReader(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) func() []store.SessionInfo {
	t.Helper()

	db, err := globaldb.OpenGlobalDB(ctx, harness.HomePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(workspace roles) error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(context.Background()); closeErr != nil {
			t.Errorf("Close(workspace roles global db) error = %v", closeErr)
		}
	})
	return func() []store.SessionInfo {
		sessions, listErr := db.ListSessions(ctx, store.SessionListQuery{
			WorkspaceID: harness.WorkspaceID,
			Limit:       100,
		})
		if listErr != nil {
			t.Fatalf("ListSessions(workspace roles) error = %v", listErr)
		}
		return sessions
	}
}

func runAutoTitleRoleFallbackIntegration(t *testing.T) {
	t.Helper()

	harness := startAutoTitleRoleHarness(t, func(cfg *aghconfig.Config) {
		cfg.Providers["unreachable-role-provider"] = aghconfig.ProviderConfig{
			Command:      "/missing/agh-role-provider",
			Harness:      aghconfig.ProviderHarnessACP,
			AuthMode:     aghconfig.ProviderAuthModeNone,
			NoneSecurity: aghconfig.ProviderNoneSecurityLocalTransport,
		}
		cfg.Roles.AutoTitle.Provider = "unreachable-role-provider"
		cfg.Roles.AutoTitle.Model = "unreachable-model"
		cfg.Roles.AutoTitle.FallbackChain = []aghconfig.RoleFallback{{
			Provider: acpmock.ProviderName,
			Model:    "fallback-title-model",
		}}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := createFixtureBackedSession(t, ctx, harness, "auto-title-agent", "")
	if _, err := harness.PromptSession(ctx, session.ID, "Implement checkout retry fencing"); err != nil {
		t.Fatalf("PromptSession(role fallback) error = %v", err)
	}
	waitForRuntimeCondition(t, "fallback auto-title applied", 10*time.Second, func() bool {
		current, err := harness.GetSession(ctx, session.ID)
		return err == nil && current.Name == "Checkout Retry Fencing"
	})

	var roleResponse aghcontract.RoleStatusResponse
	rolePath := "/api/roles/auto_title?workspace=" + url.QueryEscape(harness.WorkspaceRoot)
	if err := harness.UDSJSON(ctx, http.MethodGet, rolePath, nil, &roleResponse); err != nil {
		t.Fatalf("UDS auto_title role status error = %v", err)
	}
	if len(roleResponse.Role.Diagnostics) != 0 {
		t.Fatalf("auto_title diagnostics = %#v, want success path unaffected", roleResponse.Role.Diagnostics)
	}

	var logs aghcontract.LogsListResponse
	logsPath := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) +
		"&type=role.fallback.used&limit=10"
	waitForRuntimeCondition(t, "durable role fallback event", 10*time.Second, func() bool {
		logs = aghcontract.LogsListResponse{}
		return harness.UDSJSON(ctx, http.MethodGet, logsPath, nil, &logs) == nil && len(logs.Events) == 1
	})
	event := logs.Events[0]
	if event.Type != "role.fallback.used" || event.WorkspaceID != harness.WorkspaceID || event.SessionID != session.ID {
		t.Fatalf("fallback event correlation = %#v, want workspace and parent session", event)
	}
	var content roleFallbackEventPayload
	if err := json.Unmarshal(event.Content, &content); err != nil {
		t.Fatalf("json.Unmarshal(fallback event content) error = %v", err)
	}
	if content.Role != string(aghconfig.RoleAutoTitle) || content.Attempt != 1 ||
		content.Provider != acpmock.ProviderName || content.Model != "fallback-title-model" {
		t.Fatalf("fallback event content = %#v, want ordered acpmock attempt", content)
	}
}

func runAutoTitleRoleLiveToggleIntegration(t *testing.T) {
	t.Helper()

	harness := startAutoTitleRoleHarness(t, func(cfg *aghconfig.Config) {
		cfg.Roles.AutoTitle.Enabled = false
	})
	registration, ok := harness.MockAgentRegistration("auto-title-agent")
	if !ok {
		t.Fatal("MockAgentRegistration(auto-title-agent) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	disabled := createFixtureBackedSession(t, ctx, harness, "auto-title-agent", "")
	waitForRuntimeCondition(t, "disabled auto-title session active", 10*time.Second, func() bool {
		current, getErr := harness.GetSession(ctx, disabled.ID)
		return getErr == nil && current.State == sessionpkg.StateActive
	})
	if _, err := harness.PromptSession(ctx, disabled.ID, "Implement checkout retry fencing"); err != nil {
		t.Fatalf("PromptSession(disabled auto-title) error = %v", err)
	}
	assertAutoTitlePromptCountRemains(t, registration.DiagnosticsPath, 1, 750*time.Millisecond)
	currentDisabled, err := harness.GetSession(ctx, disabled.ID)
	if err != nil {
		t.Fatalf("GetSession(disabled auto-title) error = %v", err)
	}
	if got, want := currentDisabled.Name, ""; got != want {
		t.Fatalf("disabled session name = %q, want %q", got, want)
	}

	var applyResult map[string]any
	if err := harness.CLI.RunJSON(
		ctx,
		&applyResult,
		"config",
		"set",
		"roles.auto_title.enabled",
		"true",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("CLI config set roles.auto_title.enabled=true error = %v", err)
	}
	if applied, ok := applyResult["applied"].(bool); !ok || !applied {
		t.Fatalf("CLI config set apply result = %#v, want applied=true", applyResult)
	}

	enabled := createFixtureBackedSession(t, ctx, harness, "auto-title-agent", "")
	waitForRuntimeCondition(t, "enabled auto-title session active", 10*time.Second, func() bool {
		current, getErr := harness.GetSession(ctx, enabled.ID)
		return getErr == nil && current.State == sessionpkg.StateActive
	})
	if _, err := harness.PromptSession(ctx, enabled.ID, "Implement checkout retry fencing"); err != nil {
		t.Fatalf("PromptSession(enabled auto-title) error = %v", err)
	}
	waitForRuntimeCondition(t, "enabled auto-title applied", 10*time.Second, func() bool {
		current, getErr := harness.GetSession(ctx, enabled.ID)
		return getErr == nil && current.Name == "Checkout Retry Fencing"
	})

	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(auto-title) error = %v", err)
	}
	if got, want := len(acpmock.PromptDiagnostics(records)), 3; got != want {
		t.Fatalf("prompt diagnostics = %#v, want two user prompts and one live-enabled title prompt", records)
	}
}

func assertAutoTitlePromptCountRemains(
	t testing.TB,
	diagnosticsPath string,
	want int,
	duration time.Duration,
) {
	t.Helper()
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		records, err := acpmock.ReadDiagnostics(diagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(auto-title disabled) error = %v", err)
		}
		if got := len(acpmock.PromptDiagnostics(records)); got != want {
			t.Fatalf("disabled auto-title prompt count = %d, want stable %d", got, want)
		}
		select {
		case <-deadline.C:
			return
		case <-ticker.C:
		}
	}
}
