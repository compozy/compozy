//go:build integration && !windows

package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/providerauth"
	"github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	shellquote "github.com/kballard/go-shellquote"
)

// Public-entry coverage for the agy built-in: the installed CLI is launched as
// `agy acp`, login and readiness reuse the real shared ACP paths, and one
// durable prompt selects Hydrogen and confirms thought_level=xhigh before the
// turn runs.

func TestAgyACPBuiltInEntry(t *testing.T) {
	// not parallel: phases share AGY_ENTRY_STATE/PATH via t.Setenv and must run in order.
	fixture := setupAgyEntryFixture(t)
	ctx := testutil.Context(t)

	t.Run("Should resolve the agy built-in without a custom block", func(t *testing.T) {
		if got := strings.TrimSpace(fixture.provider.AuthACPMethod); got != "oauth-personal" {
			t.Fatalf("agy auth method = %q, want oauth-personal", got)
		}
		if _, err := os.Stat(filepath.Join(fixture.binDir, "agy_acp_server.par")); !os.IsNotExist(err) {
			t.Fatalf("standalone server is reachable on the fixture PATH: %v", err)
		}
	})

	t.Run("Should refuse a constrained resolved built-in before launching the CLI", func(t *testing.T) {
		isolated := setupAgyEntryFixture(t)
		manager, workspace := newAgyEntryManager(t, isolated)
		_, err := manager.Create(testutil.Context(t), new(session.CreateOpts{
			AgentName: "coder", Workspace: workspace.ID, Provider: "agy",
			Permissions: compozyconfig.PermissionModeApproveReads,
		}))
		if !errors.Is(err, acp.ErrNativeAgyToolPolicyUnsupported) {
			t.Fatalf("Create() error = %v, want the native policy refusal", err)
		}
		for _, name := range []string{"launch.log", "argv.log", "peer.log"} {
			if _, err := os.Stat(filepath.Join(isolated.stateDir, name)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s exists (%v), want no CLI launch or probe", name, err)
			}
		}
	})

	t.Run("Should report logged-out readiness without consent or prompts", func(t *testing.T) {
		setAgyEntryAuthed(t, fixture.stateDir, false)
		err := acp.ProbeAuthentication(ctx, fixture.inspection())
		if !acp.IsAuthRequiredError(err) {
			t.Fatalf("ProbeAuthentication(logged out) error = %v, want auth-required refusal", err)
		}
		lines := readAgyEntryLog(t, fixture.stateDir, "peer.log")
		if got := countAgyEntryLog(lines, "authenticate"); got != 0 {
			t.Fatalf("peer log = %q, want no authenticate during readiness", lines)
		}
		if got := countAgyEntryLog(lines, "prompt"); got != 0 {
			t.Fatalf("peer log = %q, want no prompt during readiness", lines)
		}
	})

	t.Run("Should login through the advertised personal method", func(t *testing.T) {
		if err := acp.Authenticate(ctx, acp.AuthenticationRequest{
			SessionInspectionRequest: fixture.inspection(),
			MethodID:                 "oauth-personal",
		}); err != nil {
			t.Fatalf("Authenticate() error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(fixture.stateDir, "authed")); err != nil {
			t.Fatalf("login did not persist peer consent: %v", err)
		}
		lines := readAgyEntryLog(t, fixture.stateDir, "peer.log")
		if got := countAgyEntryLog(lines, "authenticate"); got != 1 {
			t.Fatalf("peer log = %q, want exactly one authenticate", lines)
		}
		if got := countAgyEntryLog(lines, "authenticate methodId=oauth-personal"); got != 1 {
			t.Fatalf("peer log = %q, want the oauth-personal authenticate", lines)
		}
		if got := countAgyEntryLog(lines, "prompt"); got != 0 {
			t.Fatalf("peer log = %q, want no prompt during login", lines)
		}
	})

	t.Run("Should verify signed-in readiness without authenticating", func(t *testing.T) {
		isolated := setupAgyEntryFixture(t)
		setAgyEntryAuthed(t, isolated.stateDir, true)
		if err := acp.ProbeAuthentication(testutil.Context(t), isolated.inspection()); err != nil {
			t.Fatalf("ProbeAuthentication() error = %v", err)
		}
		lines := readAgyEntryLog(t, isolated.stateDir, "peer.log")
		if got := countAgyEntryLog(lines, "authenticate"); got != 0 {
			t.Fatalf("peer log = %q, want no authenticate during probe", lines)
		}
		if got := countAgyEntryLog(lines, "prompt"); got != 0 {
			t.Fatalf("peer log = %q, want no prompt during probe", lines)
		}
	})

	t.Run("Should select Hydrogen xhigh before prompting", func(t *testing.T) {
		t.Setenv(agyEntryPhaseKey, agyEntryPhaseSession)
		setAgyEntryAuthed(t, fixture.stateDir, true)
		manager, resolvedWorkspace := newAgyEntryManager(t, fixture)
		created, err := manager.Create(testutil.Context(t), new(session.CreateOpts{
			AgentName:       "coder",
			Name:            "agy entry session",
			Workspace:       resolvedWorkspace.ID,
			Provider:        "agy",
			Model:           agyEntryHydrogenID,
			ReasoningEffort: agyEntryXHighEffort,
		}))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		if info := created.Info(); info == nil || strings.TrimSpace(info.Provider) != "agy" {
			t.Fatalf("session provider = %+v, want agy", info)
		}
		events, err := manager.Prompt(testutil.Context(t), created.ID, "entry hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		if got := collectAgyEntryPromptText(t, events); !strings.Contains(got, agyEntryPeerReply) {
			t.Fatalf("prompt text = %q, want peer reply", got)
		}
		lines := readAgyEntryLog(t, fixture.stateDir, "peer.log")
		modelIndex := agyEntryLogIndex(t, lines, "model="+agyEntryHydrogenID)
		effortIndex := agyEntryLogIndex(t, lines, "thought_level="+agyEntryXHighEffort)
		promptIndex := agyEntryLogIndex(t, lines, "prompt")
		if modelIndex >= effortIndex || effortIndex >= promptIndex {
			t.Fatalf("peer log = %q, want model selection, confirmed effort, then the prompt", lines)
		}
		if got := countAgyEntryLog(lines, "prompt"); got != 1 {
			t.Fatalf("peer log = %q, want exactly one served prompt", lines)
		}
		if got := countAgyEntryLog(lines, "prompt-unconfirmed"); got != 0 {
			t.Fatalf("peer log = %q, want Hydrogen xhigh confirmed before execution", lines)
		}
		if got := countAgyEntryLog(lines, "mode=yolo") + countAgyEntryLog(lines, "mode-rejected yolo"); got != 0 {
			t.Fatalf("peer log = %q, want no obsolete yolo request", lines)
		}
		modeIndex := agyEntryLogIndex(t, lines, "mode=")
		modeLine := strings.TrimPrefix(lines[modeIndex], "mode=")
		if !slices.Contains(agyEntryModes, modeLine) {
			t.Fatalf("selected mode = %q, want one of the advertised native modes %v", modeLine, agyEntryModes)
		}
		argv := readAgyEntryLog(t, fixture.stateDir, "argv.log")
		if len(argv) == 0 {
			t.Fatal("agy shim recorded no launch")
		}
		for _, entry := range argv {
			if entry != "acp" {
				t.Fatalf("agy launch argv = %q, want the sole %q argument", entry, "acp")
			}
		}
		retained, ok := manager.Get(created.ID)
		if !ok {
			t.Fatal("created session is not retained after prompt")
		}
		effective := retained.Info().EffectiveRuntime()
		if effective.Provider != "agy" || effective.Model != agyEntryHydrogenID ||
			effective.ReasoningEffort != agyEntryXHighEffort {
			t.Fatalf("effective runtime = %#v, want the acknowledged Hydrogen xhigh selection", effective)
		}
	})

	t.Run("Should retain a legacy session when its native replacement cannot enforce the policy", func(t *testing.T) {
		isolated := setupAgyEntryFixture(t)
		t.Setenv(agyEntryPhaseKey, agyEntryPhaseSession)
		setAgyEntryAuthed(t, isolated.stateDir, true)
		shim, err := os.ReadFile(filepath.Join(isolated.binDir, "agy"))
		if err != nil {
			t.Fatalf("read CLI shim error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(isolated.binDir, "agy_acp_server.par"), shim, 0o755); err != nil {
			t.Fatalf("write legacy command shim error = %v", err)
		}
		manager, workspace := newAgyEntryManager(t, isolated)
		created, err := manager.Create(testutil.Context(t), new(session.CreateOpts{
			AgentName: "coder", Workspace: workspace.ID, Provider: "agy",
			Model: agyEntryHydrogenID, ReasoningEffort: agyEntryXHighEffort,
			Permissions: compozyconfig.PermissionModeApproveReads,
			Command:     "agy_acp_server.par acp",
		}))
		if err != nil {
			t.Fatalf("Create(legacy command) error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		events, err := manager.Prompt(testutil.Context(t), created.ID, "retained legacy conversation")
		if err != nil {
			t.Fatalf("Prompt(legacy command) error = %v", err)
		}
		if got := collectAgyEntryPromptText(t, events); !strings.Contains(got, agyEntryPeerReply) {
			t.Fatalf("legacy prompt text = %q, want peer reply", got)
		}
		if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop(legacy command) error = %v", err)
		}
		metaPath := store.SessionMetaFile(filepath.Join(isolated.homePaths.SessionsDir, created.ID))
		before, err := store.ReadSessionMeta(metaPath)
		if err != nil {
			t.Fatalf("ReadSessionMeta(before) error = %v", err)
		}
		before.SetACPOptions([]store.SessionACPOptionSelection{{ID: "mode", ValueID: "yolo"}})
		if err := store.WriteSessionMeta(metaPath, &before); err != nil {
			t.Fatalf("WriteSessionMeta(legacy options) error = %v", err)
		}
		query := store.EventQuery{Limit: 1000}
		beforeEvents, err := manager.Events(testutil.Context(t), created.ID, query)
		if err != nil {
			t.Fatalf("Events(before) error = %v", err)
		}
		if !agyEntryEventsContain(beforeEvents, acp.EventTypeUserMessage, "retained legacy conversation") ||
			!agyEntryEventsContain(beforeEvents, acp.EventTypeAgentMessage, agyEntryPeerReply) {
			t.Fatalf("legacy transcript = %+v, want the user message and the peer reply", beforeEvents)
		}
		beforeLaunch := readAgyEntryLog(t, isolated.stateDir, "launch.log")
		beforeArgv := readAgyEntryLog(t, isolated.stateDir, "argv.log")
		beforePeer := readAgyEntryLog(t, isolated.stateDir, "peer.log")
		_, err = manager.Resume(testutil.Context(t), created.ID)
		if !errors.Is(err, acp.ErrNativeAgyToolPolicyUnsupported) {
			t.Fatalf("Resume() error = %v, want the native policy refusal", err)
		}
		after, err := store.ReadSessionMeta(metaPath)
		if err != nil {
			t.Fatalf("ReadSessionMeta(after) error = %v", err)
		}
		before.UpdatedAt = after.UpdatedAt
		if !reflect.DeepEqual(before, after) {
			t.Fatal("refused resume changed retained metadata, route, native ID, options, or creation identity")
		}
		afterEvents, err := manager.Events(testutil.Context(t), created.ID, query)
		if err != nil {
			t.Fatalf("Events(after) error = %v", err)
		}
		if len(beforeEvents) == 0 || len(afterEvents) < len(beforeEvents) ||
			!reflect.DeepEqual(beforeEvents, afterEvents[:len(beforeEvents)]) {
			t.Fatal("refused resume changed the retained transcript")
		}
		if !slices.Equal(beforeLaunch, readAgyEntryLog(t, isolated.stateDir, "launch.log")) ||
			!slices.Equal(beforeArgv, readAgyEntryLog(t, isolated.stateDir, "argv.log")) ||
			!slices.Equal(beforePeer, readAgyEntryLog(t, isolated.stateDir, "peer.log")) {
			t.Fatal("refused resume launched the replacement CLI")
		}
	})
}

func collectAgyEntryPromptText(t *testing.T, events <-chan acp.AgentEvent) string {
	t.Helper()

	var text strings.Builder
	timeout := time.After(60 * time.Second)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return text.String()
			}
			text.WriteString(event.Text)
		case <-timeout:
			t.Fatal("Prompt() events did not complete")
		}
	}
}

func agyEntryEventsContain(events []store.SessionEvent, eventType string, text string) bool {
	for _, event := range events {
		if event.Type == eventType && strings.Contains(event.Content, text) {
			return true
		}
	}
	return false
}

func newAgyEntryManager(t *testing.T, fixture *agyEntryFixture) (*session.Manager, workspacepkg.ResolvedWorkspace) {
	t.Helper()
	daemonInstance, capturedDeps := bootHarnessPolicyDaemon(t, fixture.homePaths, &fixture.cfg)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	workspaceRoot := filepath.Join(fixture.homePaths.HomeDir, "workspace")
	if err := compozyconfig.EnsureHomeLayout(fixture.homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	workspace := workspacepkg.ResolvedWorkspace{
		ID: "ws-agy-entry", RootDir: workspaceRoot, Name: "workspace", Config: fixture.cfg,
		Agents: []compozyconfig.AgentDef{{Name: "coder", Provider: "agy", Prompt: "You are a coding assistant."}},
	}
	driver := session.NewACPDriverAdapter(acp.New(acp.WithProviderPreStarter(daemonInstance.providerPreStarter)))
	return newHarnessIntegrationManager(t, fixture.homePaths, capturedDeps, workspace, driver), workspace
}

// TestAgyACPBuiltInEntryMissingCLI proves a standalone-only installation is not
// a usable Antigravity runtime: startup classification reports the missing CLI
// and the standalone executable is never launched.
func TestAgyACPBuiltInEntryMissingCLI(t *testing.T) {
	t.Run("Should report missing_cli when only the standalone server is installed", func(t *testing.T) {
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		provider, err := cfg.ResolveProvider("agy")
		if err != nil {
			t.Fatalf("ResolveProvider(agy) error = %v", err)
		}
		standaloneDir := t.TempDir()
		marker := filepath.Join(standaloneDir, "par-ran")
		par := "#!/bin/sh\n: > " + shellquote.Join(marker) + "\n"
		if err := os.WriteFile(filepath.Join(standaloneDir, "agy_acp_server.par"), []byte(par), 0o755); err != nil {
			t.Fatalf("write standalone fixture error = %v", err)
		}
		t.Setenv("PATH", standaloneDir)
		env, err := providerauth.CommandEnv(homePaths, "agy", provider, os.Environ())
		if err != nil {
			t.Fatalf("CommandEnv(agy) error = %v", err)
		}

		report := providers.NewPreStarter().PreStart(
			testutil.Context(t),
			provider,
			&providers.ProbeEnv{
				ProviderName: "agy",
				HomePaths:    homePaths,
				CommandEnv:   env,
				CommandDir:   t.TempDir(),
				BypassCache:  true,
			},
		)
		if report.Item == nil {
			t.Fatal("PreStart(standalone only) = no diagnostic, want missing_cli")
		}
		if got := report.Item.Code; got != diagcontract.CodeProviderCLIMissing {
			t.Fatalf("PreStart(standalone only) code = %q, want %q", got, diagcontract.CodeProviderCLIMissing)
		}
		if got := report.Item.Evidence["action"]; got != string(providers.ProviderFailureActionInstallCLI) {
			t.Fatalf("PreStart(standalone only) action = %v, want install_cli", got)
		}
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Fatalf("standalone server was executed as a fallback: %v", statErr)
		}
	})
}
