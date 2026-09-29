//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	sessionpkg "github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

func TestAutoTitleRoleIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should apply auto-title role toggles without a daemon restart", func(t *testing.T) {
		t.Parallel()
		runAutoTitleRoleLiveToggleIntegration(t)
	})

	t.Run(
		"Should fall back after a pre-acceptance provider failure without a durable attempt",
		func(t *testing.T) {
			t.Parallel()
			runAutoTitleRoleFallbackIntegration(t)
		},
	)

	t.Run("Should launch a fallback route on its account command", func(t *testing.T) { // IT-001, IT-002, E2E-001
		t.Parallel()
		runAutoTitleRoleAccountRouteIntegration(t)
	})

	t.Run("Should stop the chain when ACP accepted the start and configuration failed", func(t *testing.T) { // IT-006
		t.Parallel()
		runAutoTitleRoleAcceptedStartFailureIntegration(t)
	})

	t.Run("Should clean up every exhausted pre-acceptance attempt", func(t *testing.T) {
		t.Parallel()
		runAutoTitleRoleExhaustionIntegration(t)
	})

	t.Run("Should stop fallback forever after an accepted attempt fails", func(t *testing.T) {
		t.Parallel()
		runAutoTitleRolePostAcceptanceFailureIntegration(t)
	})

	t.Run("Should run only the role chain when the inherited agent also declares one", func(t *testing.T) { // IT-007
		t.Parallel()
		runAutoTitleRoleSingleChainOwnerIntegration(t)
	})
}

// runAutoTitleRoleSingleChainOwnerIntegration proves one chain owner per launch: the
// auto-title role inherits an agent whose own fallback_chain route would accept, so the
// role's refused primary must advance through the role chain (ChainOwnerCaller) and never
// through the agent chain.
func runAutoTitleRoleSingleChainOwnerIntegration(t *testing.T) {
	t.Helper()

	const (
		ownerAgent    = "title-owner"
		agentProvider = "agent-chain-a1"
		agentModel    = "primary-title-model"
		roleModel     = "fallback-title-model"
	)
	harness := startAutoTitleRoleHarness(t, func(cfg *compozyconfig.Config) {
		for _, provider := range []string{"role-primary", "role-r1"} {
			cfg.Providers[provider] = compozyconfig.ProviderConfig{
				Command:      "/missing/" + provider,
				Harness:      compozyconfig.ProviderHarnessACP,
				AuthMode:     compozyconfig.ProviderAuthModeNone,
				NoneSecurity: compozyconfig.ProviderNoneSecurityLocalTransport,
			}
		}
		cfg.Providers[agentProvider] = acpmock.ProviderConfig("/missing/" + agentProvider)
		cfg.Roles.AutoTitle.Provider = "role-primary"
		cfg.Roles.AutoTitle.Model = "primary-model"
		cfg.Roles.AutoTitle.FallbackChain = []compozyconfig.RoleFallback{
			{Provider: "role-r1", Model: "r1-model"},
			{Provider: acpmock.ProviderName, Model: roleModel},
		}
	})
	registration, ok := harness.MockAgentRegistration("auto-title-agent")
	if !ok {
		t.Fatal("MockAgentRegistration(auto-title-agent) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	// The agent chain route A1 launches the working mock: had it run, the role's primary
	// attempt would have been accepted on A1 with no role fallback at all.
	var created compozycontract.AgentPayload
	if err := harness.CLI.RunJSONInDir(
		ctx, harness.WorkspaceRoot, &created,
		"agent", "create", ownerAgent,
		"--workspace", harness.WorkspaceRoot,
		"--provider", acpmock.ProviderName,
		"--model", roleModel,
		"--command", registration.Command,
		"--prompt", "Implement work.",
		"--fallback-route", "provider="+agentProvider+",model="+agentModel+",command="+registration.Command,
		"-o", "json",
	); err != nil {
		t.Fatalf("CLI agent create %s error = %v", ownerAgent, err)
	}
	if len(created.FallbackChain) != 1 {
		t.Fatalf("agent fallback_chain = %#v, want the A1 route", created.FallbackChain)
	}

	root := createFixtureBackedSession(t, ctx, harness, ownerAgent, "")
	if _, err := harness.PromptSession(ctx, root.ID, "Implement checkout retry fencing"); err != nil {
		t.Fatalf("PromptSession(single chain owner) error = %v", err)
	}
	waitForRuntimeCondition(t, "role-chain auto-title applied", 15*time.Second, func() bool {
		current, err := harness.GetSession(ctx, root.ID)
		return err == nil && current.Name == "Checkout Retry Fencing"
	})

	readEvents := func(eventType string) []compozycontract.LogEventPayload {
		var logs compozycontract.LogsListResponse
		path := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) + "&type=" + eventType + "&limit=10"
		if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &logs); err != nil {
			t.Fatalf("UDS %s logs error = %v", eventType, err)
		}
		return logs.Events
	}
	roleEvents := readEvents("role.fallback.used")
	if len(roleEvents) != 2 {
		t.Fatalf("role.fallback.used events = %#v, want R1 and R2", roleEvents)
	}
	attempts := map[int]string{}
	for _, event := range roleEvents {
		var content roleFallbackEventPayload
		if err := json.Unmarshal(event.Content, &content); err != nil {
			t.Fatalf("json.Unmarshal(role fallback event) error = %v", err)
		}
		attempts[content.Attempt] = content.Provider
	}
	if attempts[1] != "role-r1" || attempts[2] != acpmock.ProviderName {
		t.Fatalf("role fallback attempts = %#v, want 1=role-r1 2=%s", attempts, acpmock.ProviderName)
	}
	if sessionEvents := readEvents("session.fallback.used"); len(sessionEvents) != 0 {
		t.Fatalf("session.fallback.used events = %#v, want none for a role-owned launch", sessionEvents)
	}

	var children []store.SessionInfo
	for _, candidate := range readWorkspaceRoleSessions(t, ctx, harness) {
		if candidate.Lineage != nil && candidate.Lineage.SpawnRole == sessionpkg.SpawnRoleAutoTitle {
			children = append(children, candidate)
		}
	}
	if len(children) != 1 || children[0].Provider != acpmock.ProviderName || children[0].Model != roleModel {
		t.Fatalf("auto-title children = %#v, want only the accepted R2 child", children)
	}
	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(single chain owner) error = %v", err)
	}
	for _, record := range acpmock.ProtocolDiagnostics(records) {
		if record.ProtocolMethod == acpsdk.AgentMethodSessionSetConfigOption && record.ConfigOptionValue == agentModel {
			t.Fatalf("agent chain route A1 launched for a role-owned launch: %#v", record)
		}
	}
}

func startAutoTitleRoleHarness(
	t *testing.T,
	mutate func(*compozyconfig.Config),
) *e2etest.RuntimeHarness {
	t.Helper()
	acpmock.RequireDriver(t)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
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
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		manifest, err := harness.RuntimeManifest()
		if err != nil {
			t.Logf("RuntimeManifest(auto-title failure) error = %v", err)
			return
		}
		processLog, err := os.ReadFile(manifest.Logs.ProcessLogFile)
		if err != nil {
			t.Logf("os.ReadFile(auto-title daemon process log) error = %v", err)
			return
		}
		t.Logf("auto-title daemon process log:\n%s", processLog)
	})
	return harness
}

func runAutoTitleRoleExhaustionIntegration(t *testing.T) {
	t.Helper()

	harness := startAutoTitleRoleHarness(t, func(cfg *compozyconfig.Config) {
		for _, provider := range []string{"unreachable-primary", "unreachable-fallback"} {
			cfg.Providers[provider] = compozyconfig.ProviderConfig{
				Command:      "/missing/" + provider,
				Harness:      compozyconfig.ProviderHarnessACP,
				AuthMode:     compozyconfig.ProviderAuthModeNone,
				NoneSecurity: compozyconfig.ProviderNoneSecurityLocalTransport,
			}
		}
		cfg.Roles.AutoTitle.Provider = "unreachable-primary"
		cfg.Roles.AutoTitle.Model = "primary-model"
		cfg.Roles.AutoTitle.FallbackChain = []compozyconfig.RoleFallback{{
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
		var logs compozycontract.LogsListResponse
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

	harness := startAutoTitleRoleHarness(t, func(cfg *compozyconfig.Config) {
		cfg.Roles.AutoTitle.Provider = acpmock.ProviderName
		cfg.Roles.AutoTitle.Model = "primary-title-model"
		cfg.Roles.AutoTitle.FallbackChain = []compozyconfig.RoleFallback{{
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

	var logs compozycontract.LogsListResponse
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
	for _, record := range acpmock.ProtocolDiagnostics(acpmock.DiagnosticsForCompozySession(records, child.ID)) {
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

	db, err := openDaemonTestGlobalDBAtPath(ctx, harness.HomePaths.DatabaseFile)
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
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
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

	harness := startAutoTitleRoleHarness(t, func(cfg *compozyconfig.Config) {
		cfg.Providers["unreachable-role-provider"] = compozyconfig.ProviderConfig{
			Command:      "/missing/compozy-role-provider",
			Harness:      compozyconfig.ProviderHarnessACP,
			AuthMode:     compozyconfig.ProviderAuthModeNone,
			NoneSecurity: compozyconfig.ProviderNoneSecurityLocalTransport,
		}
		cfg.Roles.AutoTitle.Provider = "unreachable-role-provider"
		cfg.Roles.AutoTitle.Model = "unreachable-model"
		cfg.Roles.AutoTitle.FallbackChain = []compozyconfig.RoleFallback{{
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

	var roleResponse compozycontract.RoleStatusResponse
	rolePath := "/api/roles/auto_title?workspace=" + url.QueryEscape(harness.WorkspaceRoot)
	if err := harness.UDSJSON(ctx, http.MethodGet, rolePath, nil, &roleResponse); err != nil {
		t.Fatalf("UDS auto_title role status error = %v", err)
	}
	if len(roleResponse.Role.Diagnostics) != 0 {
		t.Fatalf("auto_title diagnostics = %#v, want success path unaffected", roleResponse.Role.Diagnostics)
	}

	var logs compozycontract.LogsListResponse
	logsPath := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) +
		"&type=role.fallback.used&limit=10"
	waitForRuntimeCondition(t, "durable role fallback event", 10*time.Second, func() bool {
		logs = compozycontract.LogsListResponse{}
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
	if content.Role != string(compozyconfig.RoleAutoTitle) || content.Attempt != 1 ||
		content.Provider != acpmock.ProviderName || content.Model != "fallback-title-model" {
		t.Fatalf("fallback event content = %#v, want ordered acpmock attempt", content)
	}
}

func runAutoTitleRoleLiveToggleIntegration(t *testing.T) {
	t.Helper()

	harness := startAutoTitleRoleHarness(t, func(cfg *compozyconfig.Config) {
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

// runAutoTitleRoleAccountRouteIntegration proves the route account reaches the launch:
// the fallback provider's own command is broken, so the title only arrives when the
// route command is launched, and the ledger carries its fingerprint, never the command.
func runAutoTitleRoleAccountRouteIntegration(t *testing.T) {
	t.Helper()

	const seatProvider = "acpmock-seat-two"
	harness := startAutoTitleRoleHarness(t, func(cfg *compozyconfig.Config) {
		cfg.Providers["unreachable-role-provider"] = compozyconfig.ProviderConfig{
			Command:      "/missing/compozy-role-provider",
			Harness:      compozyconfig.ProviderHarnessACP,
			AuthMode:     compozyconfig.ProviderAuthModeNone,
			NoneSecurity: compozyconfig.ProviderNoneSecurityLocalTransport,
		}
		cfg.Providers[seatProvider] = acpmock.ProviderConfig("/missing/compozy-seat-one")
		cfg.Roles.AutoTitle.Provider = "unreachable-role-provider"
		cfg.Roles.AutoTitle.Model = "unreachable-model"
	})
	registration, ok := harness.MockAgentRegistration("auto-title-agent")
	if !ok {
		t.Fatal("MockAgentRegistration(auto-title-agent) = missing, want present")
	}
	routeCommand := registration.Command
	fingerprint := compozyconfig.CommandFingerprint(routeCommand)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var settings compozycontract.SettingsRolesResponse
	if err := harness.UDSJSON(ctx, http.MethodGet, "/api/settings/roles", nil, &settings); err != nil {
		t.Fatalf("UDS GET settings roles error = %v", err)
	}
	settings.Config.AutoTitle.FallbackChain = []compozycontract.SettingsRoleFallbackPayload{{
		Provider:   seatProvider,
		Model:      "fallback-title-model",
		ACPOptions: []compozycontract.AgentACPOptionSelection{},
		Command:    routeCommand,
	}}
	var applied map[string]any
	if err := harness.UDSJSON(
		ctx,
		http.MethodPatch,
		"/api/settings/roles",
		map[string]any{"config": settings.Config},
		&applied,
	); err != nil {
		t.Fatalf("UDS PATCH settings roles error = %v", err)
	}
	var persisted compozycontract.SettingsRolesResponse
	if err := harness.UDSJSON(ctx, http.MethodGet, "/api/settings/roles", nil, &persisted); err != nil {
		t.Fatalf("UDS GET settings roles after apply error = %v", err)
	}
	if chain := persisted.Config.AutoTitle.FallbackChain; len(chain) != 1 || chain[0].Command != routeCommand {
		t.Fatalf("settings auto_title fallback chain = %#v, want the route command round-tripped", chain)
	}
	var roleResponse compozycontract.RoleStatusResponse
	rolePath := "/api/roles/auto_title?workspace=" + url.QueryEscape(harness.WorkspaceRoot)
	if err := harness.UDSJSON(ctx, http.MethodGet, rolePath, nil, &roleResponse); err != nil {
		t.Fatalf("UDS auto_title role status error = %v", err)
	}
	if chain := roleResponse.Role.FallbackChain; len(chain) != 1 || chain[0].Command != routeCommand ||
		chain[0].CommandFingerprint != fingerprint {
		t.Fatalf("auto_title role fallback chain = %#v, want command and fingerprint", chain)
	}

	session := createFixtureBackedSession(t, ctx, harness, "auto-title-agent", "")
	if _, err := harness.PromptSession(ctx, session.ID, "Implement checkout retry fencing"); err != nil {
		t.Fatalf("PromptSession(account route) error = %v", err)
	}
	waitForRuntimeCondition(t, "account route auto-title applied", 10*time.Second, func() bool {
		current, err := harness.GetSession(ctx, session.ID)
		return err == nil && current.Name == "Checkout Retry Fencing"
	})

	var logs compozycontract.LogsListResponse
	logsPath := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) +
		"&type=role.fallback.used&limit=10"
	waitForRuntimeCondition(t, "account route fallback event", 10*time.Second, func() bool {
		logs = compozycontract.LogsListResponse{}
		return harness.UDSJSON(ctx, http.MethodGet, logsPath, nil, &logs) == nil && len(logs.Events) == 1
	})
	var content map[string]any
	if err := json.Unmarshal(logs.Events[0].Content, &content); err != nil {
		t.Fatalf("json.Unmarshal(account route event) error = %v", err)
	}
	if content["attempt"] != float64(1) || content["provider"] != seatProvider ||
		content["provider_command_fingerprint"] != fingerprint {
		t.Fatalf("account route event content = %#v, want attempt 1 with the route fingerprint", content)
	}
	if _, leaked := content["command"]; leaked ||
		strings.Contains(string(logs.Events[0].Content), registration.FixturePath) {
		t.Fatalf("account route event leaked the raw command: %s", logs.Events[0].Content)
	}
}

// runAutoTitleRoleAcceptedStartFailureIntegration scripts acpmock to accept session/new
// and then reject session/set_config_option: the driver reports acp.AcceptedStartError,
// so the role must not launch its fallback route.
func runAutoTitleRoleAcceptedStartFailureIntegration(t *testing.T) {
	t.Helper()

	harness := startAutoTitleRoleHarness(t, func(cfg *compozyconfig.Config) {
		cfg.Roles.AutoTitle.Provider = acpmock.ProviderName
		cfg.Roles.AutoTitle.Model = "rejected-title-model"
		cfg.Roles.AutoTitle.FallbackChain = []compozyconfig.RoleFallback{{
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
	if _, err := harness.PromptSession(ctx, root.ID, "Implement checkout retry fencing"); err != nil {
		t.Fatalf("PromptSession(accepted start failure) error = %v", err)
	}

	waitForRuntimeCondition(t, "rejected configuration attempted", 10*time.Second, func() bool {
		records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			return false
		}
		for _, record := range acpmock.ProtocolDiagnostics(records) {
			if record.ProtocolMethod == acpsdk.AgentMethodSessionSetConfigOption &&
				record.ConfigOptionValue == "rejected-title-model" {
				return true
			}
		}
		return false
	})
	assertAutoTitlePromptCountRemains(t, registration.DiagnosticsPath, 1, 750*time.Millisecond)

	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(accepted start failure) error = %v", err)
	}
	rejectedAttempts := 0
	for _, record := range acpmock.ProtocolDiagnostics(records) {
		if record.ProtocolMethod != acpsdk.AgentMethodSessionSetConfigOption {
			continue
		}
		switch record.ConfigOptionValue {
		case "rejected-title-model":
			rejectedAttempts++
		case "fallback-title-model":
			t.Fatalf("fallback route configured after acceptance: %#v", record)
		}
	}
	if rejectedAttempts != 1 {
		t.Fatalf("accepted role attempts = %d, want exactly one", rejectedAttempts)
	}
	var logs compozycontract.LogsListResponse
	logsPath := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) +
		"&type=role.fallback.used&limit=10"
	if err := harness.UDSJSON(ctx, http.MethodGet, logsPath, nil, &logs); err != nil {
		t.Fatalf("UDS accepted start failure logs error = %v", err)
	}
	if len(logs.Events) != 0 {
		t.Fatalf("role fallback events = %#v, want none after acceptance", logs.Events)
	}
	current, err := harness.GetSession(ctx, root.ID)
	if err != nil {
		t.Fatalf("GetSession(accepted start failure root) error = %v", err)
	}
	if current.Name != "" {
		t.Fatalf("root title = %q, want unchanged after the accepted failure", current.Name)
	}
}
