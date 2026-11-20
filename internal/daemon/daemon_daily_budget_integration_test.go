//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil/acpmock"
)

func TestDaemonDailyBudgetRealDriverJourney(t *testing.T) {
	t.Run("Should enforce an instance cap across real daemon restart and day rollover", func(t *testing.T) {
		// Not parallel: integrationHomePaths uses t.Setenv for the real daemon.
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()
		driver := acpmock.RequireDriver(t)
		home := integrationHomePaths(t)
		cfg := testConfig(t, home)
		realUDS := dailyBudgetUnixSocketAvailable(t, cfg.Daemon.Socket)
		cfg.Defaults.Provider = acpmock.ProviderName
		cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(driver)
		fixture := acpmock.Fixture{Version: acpmock.FixtureVersion, Agents: []acpmock.AgentFixture{{
			Name: "budget", Provider: acpmock.ProviderName, Permissions: "approve-all",
			Turns: []acpmock.TurnFixture{
				{Match: acpmock.TurnMatch{UserTextContains: "first budget turn"},
					Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "first done"}},
					Usage: &acpmock.TurnUsage{InputTokens: 80, OutputTokens: 5}},
				{Match: acpmock.TurnMatch{UserTextContains: "second budget turn"},
					Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "second done"}},
					Usage: &acpmock.TurnUsage{InputTokens: 14, OutputTokens: 6}},
				{Match: acpmock.TurnMatch{UserTextContains: "isolated profile turn"},
					Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "isolated done without usage"}}},
				{Match: acpmock.TurnMatch{UserTextContains: "after rollover"},
					Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "resumed done"}},
					Usage: &acpmock.TurnUsage{InputTokens: 5, OutputTokens: 1}},
			},
		}}}
		fixtureData, err := json.Marshal(fixture)
		if err != nil {
			t.Fatal(err)
		}
		fixturePath := filepath.Join(t.TempDir(), "daily-budget-fixture.json")
		if err := os.WriteFile(fixturePath, fixtureData, 0o600); err != nil {
			t.Fatal(err)
		}
		registered, err := acpmock.Register(home, acpmock.RegisterOptions{
			DriverPath: driver, FixturePath: fixturePath, AgentName: "budget", FixtureAgent: "budget",
		})
		if err != nil {
			t.Fatal(err)
		}
		definition, err := os.ReadFile(registered.AgentDefPath)
		if err != nil {
			t.Fatal(err)
		}
		definition = []byte(strings.Replace(string(definition), "name: budget\n", "name: budget\nkind: always_on\n", 1))
		if err := os.WriteFile(registered.AgentDefPath, definition, 0o600); err != nil {
			t.Fatal(err)
		}
		writeDaemonIntegrationProviderConfig(t, home, acpmock.ProviderName, driver, compozyconfig.ProviderAuthModeNone)
		var clock atomic.Int64
		wallNow := time.Now().UTC().Truncate(time.Second)
		clock.Store(wallNow.Unix())
		// Provider Done events use wall time. Center the local day on the real
		// fixture start so those events and the daemon stay in one day.
		originalLocal := time.Local
		utcMidnight := time.Date(wallNow.Year(), wallNow.Month(), wallNow.Day(), 0, 0, 0, 0, time.UTC)
		time.Local = time.FixedZone("budget-test", int((12*time.Hour-wallNow.Sub(utcMidnight))/time.Second))
		t.Cleanup(func() { time.Local = originalLocal })
		start := func() *Daemon {
			t.Helper()
			d := newTestDaemon(t, home, &cfg)
			d.now = func() time.Time { return time.Unix(clock.Load(), 0).UTC() }
			if !realUDS {
				d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
					return &fakeServer{name: "uds"}, nil
				}
			}
			if err := d.boot(ctx); err != nil {
				t.Fatalf("daemon boot failed: %v", err)
			}
			return d
		}
		d := start()
		t.Cleanup(func() {
			if err := d.Shutdown(context.Background()); err != nil {
				t.Errorf("shutdown daily budget daemon: %v", err)
			}
		})
		workspace := resolveDaemonWorkspace(t, d.workspaceResolver, t.TempDir())
		db := d.registry.(*globaldb.GlobalDB)
		ceiling, err := json.Marshal(store.SessionPermissionPolicy{
			WorkspacePaths: []string{workspace.RootDir},
		})
		if err != nil {
			t.Fatal(err)
		}
		instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, WorkspaceID: workspace.ID,
			AgentName: "budget", CreatedByKind: "human", PermissionCeiling: string(ceiling),
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		main, err := d.sessions.Create(ctx, new(session.CreateOpts{
			InstanceID: instance.ID, ProfileID: instance.ProfileID,
			AgentName: instance.AgentName, Workspace: workspace.ID,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().BindMain(ctx, instance.ID, main.ID); err != nil {
			t.Fatal(err)
		}
		instancePath := "/api/agent-instances/" + instance.ID
		status, raw := instanceIntegrationHTTP(t, d, http.MethodPatch, instancePath,
			[]byte(`{"daily_budget":{"tokens":100}}`), nil)
		if status != http.StatusOK {
			t.Fatalf("set budget over HTTP = %d %s", status, raw)
		}
		prompt := func(sessionID, message string) {
			t.Helper()
			stream, err := d.sessions.Prompt(ctx, sessionID, message)
			if err != nil {
				t.Fatalf("provider turn %q admission: %v", message, err)
			}
			var terminal bool
			for event := range stream {
				if event.Type == acp.EventTypeError {
					t.Fatalf("provider turn %q failed: %+v", message, event)
				}
				terminal = terminal || event.Type == acp.EventTypeDone
			}
			if !terminal {
				t.Fatalf("provider turn %q lacked Done event", message)
			}
		}
		prompt(main.ID, "first budget turn")
		usage, err := db.Instances().ReadDailyBudgetUsage(ctx, instance.ID, time.Unix(clock.Load(), 0))
		if err != nil || usage.Tokens != 85 {
			t.Fatalf("first provider usage=%+v err=%v", usage, err)
		}
		prompt(main.ID, "second budget turn")
		usage, err = db.Instances().ReadDailyBudgetUsage(ctx, instance.ID, time.Unix(clock.Load(), 0))
		if err != nil || usage.Tokens != 105 {
			t.Fatalf("second provider usage=%+v err=%v", usage, err)
		}
		assertReceipts := func(want int) {
			t.Helper()
			var receipts int
			if err := db.DB().QueryRowContext(ctx,
				`SELECT count(*) FROM token_usage_receipts WHERE instance_id=?`, instance.ID).
				Scan(&receipts); err != nil || receipts != want {
				t.Fatalf("durable provider receipts=%d err=%v, want %d", receipts, err, want)
			}
		}
		assertReceipts(2)
		var notices int
		if err := db.DB().QueryRowContext(ctx,
			`SELECT count(*) FROM agent_instance_daily_budget_notices WHERE instance_id=?`, instance.ID).
			Scan(&notices); err != nil || notices != 1 {
			t.Fatalf("daily warning count=%d err=%v, want 1", notices, err)
		}
		if _, err := d.sessions.Prompt(
			ctx,
			main.ID,
			"must be denied",
		); !errors.Is(
			err,
			store.ErrDailyBudgetTurnBlocked,
		) {
			t.Fatalf("third provider turn error=%v, want budget block", err)
		}
		paused, err := db.Instances().Get(ctx, instance.ID)
		if err != nil || paused.State != "paused" || paused.PauseReason != "daily_budget" {
			t.Fatalf("persisted cap pause=%+v err=%v", paused, err)
		}
		assertDailyBudgetPublicPause(t, d, instancePath, realUDS)

		profiles, err := profilepkg.NewManager(profilepkg.WithStore(db), profilepkg.WithHomePaths(home))
		if err != nil {
			t.Fatal(err)
		}
		isolated, err := profiles.Create(ctx, profilepkg.CreateInput{Name: "isolated-budget-owner"})
		if err != nil {
			t.Fatal(err)
		}
		other, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: isolated.ID, AgentName: instance.AgentName, WorkspaceID: workspace.ID,
			CreatedByKind: "human", PermissionCeiling: string(ceiling),
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		otherMain, err := d.sessions.Create(ctx, new(session.CreateOpts{
			InstanceID: other.ID, ProfileID: isolated.ID, AgentName: other.AgentName, Workspace: workspace.ID,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().BindMain(ctx, other.ID, otherMain.ID); err != nil {
			t.Fatal(err)
		}
		prompt(otherMain.ID, "isolated profile turn")
		peerUsage, err := db.Instances().ReadDailyBudgetUsage(ctx, other.ID, time.Unix(clock.Load(), 0))
		if err != nil || peerUsage.Tokens != 0 || !peerUsage.HasUnpricedUsage {
			t.Fatalf("uncapped peer usage=%+v err=%v, want an unpriced provider completion", peerUsage, err)
		}
		var peerAggregates int
		if err := db.DB().QueryRowContext(ctx,
			`SELECT count(*) FROM token_stats WHERE session_id=?`, otherMain.ID).Scan(&peerAggregates); err != nil ||
			peerAggregates != 0 {
			t.Fatalf("unreported usage invented a session aggregate: count=%d err=%v", peerAggregates, err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, other.ID,
			agentinstance.DailyBudgetLimits{USD: new(float64(1))}); err != nil {
			t.Fatal(err)
		}
		if _, err := d.sessions.Prompt(ctx, otherMain.ID, "unpriced turn must be held"); !errors.Is(
			err, store.ErrDailyBudgetTurnBlocked,
		) {
			t.Fatalf("unpriced provider completion allowed another USD-capped turn: %v", err)
		}
		assertReceipts(2)

		status, raw = instanceIntegrationHTTP(t, d, http.MethodPost, instancePath+"/resume", nil, nil)
		if status != http.StatusConflict {
			t.Fatalf("same-day operator resume over HTTP = %d %s, want conflict", status, raw)
		}
		// Reproduce a loss of projection after the peer's durable Done, while
		// keeping the main instance's complete receipts for replay checks.
		for _, statement := range []struct{ sql, id string }{
			{`DELETE FROM daily_budget_recovery_cursors WHERE session_id=?`, otherMain.ID},
			{`DELETE FROM token_usage_receipts WHERE session_id=?`, otherMain.ID},
			{`DELETE FROM agent_instance_daily_budget_usage WHERE instance_id=?`, other.ID},
		} {
			if _, err := db.DB().ExecContext(ctx, statement.sql, statement.id); err != nil {
				t.Fatal(err)
			}
		}
		if err := d.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		d = start()
		db = d.registry.(*globaldb.GlobalDB)
		paused, err = db.Instances().Get(ctx, instance.ID)
		if err != nil || paused.State != "paused" || paused.PauseReason != "daily_budget" {
			t.Fatalf("restart pause=%+v err=%v", paused, err)
		}
		assertReceipts(2) // boot replay must never count completed turns twice
		var peerReceipts int
		var peerDay string
		if err := db.DB().QueryRowContext(ctx,
			`SELECT count(*), COALESCE(min(day), '') FROM token_usage_receipts WHERE instance_id=?`, other.ID).
			Scan(&peerReceipts, &peerDay); err != nil || peerReceipts != 1 || peerDay != store.LocalDay(wallNow) {
			t.Fatalf(
				"restart unpriced receipt count=%d day=%q err=%v, want one receipt on the original day",
				peerReceipts,
				peerDay,
				err,
			)
		}
		peerUsage, err = db.Instances().ReadDailyBudgetUsage(ctx, other.ID, time.Unix(clock.Load(), 0))
		if err != nil || !peerUsage.HasUnpricedUsage {
			t.Fatalf("restart lost the peer's unknown cost: %+v err=%v", peerUsage, err)
		}
		usage, err = db.Instances().ReadDailyBudgetUsage(ctx, instance.ID, time.Unix(clock.Load(), 0))
		if err != nil || usage.Tokens != 105 {
			t.Fatalf("restart usage=%+v err=%v, want unchanged 105", usage, err)
		}
		clock.Add(int64(25 * time.Hour / time.Second))
		paused, err = db.Instances().Get(ctx, instance.ID)
		if err != nil || paused.State != "paused" {
			t.Fatalf("day rollover resumed without operator: %+v err=%v", paused, err)
		}
		if _, err := db.Instances().Resume(ctx, instance.ID, 10, false); !errors.Is(err, agentinstance.ErrForbidden) {
			t.Fatalf("nonoperator next-day resume error=%v, want forbidden", err)
		}
		status, raw = instanceIntegrationHTTP(t, d, http.MethodPost, instancePath+"/resume", nil, nil)
		if status != http.StatusOK {
			t.Fatalf("next-day operator resume over HTTP = %d %s", status, raw)
		}
		newUsage, err := db.Instances().ReadDailyBudgetUsage(ctx, instance.ID, time.Unix(clock.Load(), 0))
		if err != nil || newUsage.Tokens != 0 {
			t.Fatalf("next-day usage=%+v err=%v, want fresh window", newUsage, err)
		}
		mainID, err := db.Instances().Get(ctx, instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		main.ID = mainID.MainSessionID
		prompt(main.ID, "after rollover")
		// The ACP subprocess timestamps the last Done using actual wall time;
		// only daemon admission uses the injected next-day clock. Therefore we
		// assert a new durable receipt without claiming a fabricated day bucket.
		assertReceipts(3)
	})
}

func dailyBudgetUnixSocketAvailable(t *testing.T, path string) bool {
	t.Helper()
	listener, err := net.Listen("unix", path)
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		t.Logf("Unix socket parity unavailable: %v", err)
		return false
	}
	if err != nil {
		t.Fatalf("probe daemon Unix socket: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close Unix socket probe: %v", err)
	}
	return true
}

func assertDailyBudgetPublicPause(t *testing.T, d *Daemon, path string, realUDS bool) {
	t.Helper()
	status, raw := instanceIntegrationHTTP(t, d, http.MethodGet, path, nil, nil)
	assertDailyBudgetPublicPayload(t, "HTTP", status, raw)
	if realUDS {
		client := gatewayDaemonUDSClient(d.homePaths.DaemonSocket)
		defer client.CloseIdleConnections()
		response := gatewayDaemonRequest(t, client, http.MethodGet, "http://unix"+path, "", nil)
		assertDailyBudgetPublicPayload(t, "UDS", response.StatusCode, gatewayDaemonReadBody(t, response))
	}
}

func assertDailyBudgetPublicPayload(t *testing.T, transport string, status int, raw []byte) {
	t.Helper()
	var payload contract.AgentInstanceResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("%s status payload=%s error=%v", transport, raw, err)
	}
	usage := payload.Instance.DailyBudgetUsage
	if status != http.StatusOK || payload.Instance.PauseReason != "daily_budget" ||
		usage == nil || usage.Tokens != 105 {
		t.Fatalf("%s status = %d %s, want daily budget pause with 105 confirmed tokens", transport, status, raw)
	}
}
