//go:build integration && !windows

package session

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	shellquote "github.com/kballard/go-shellquote"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	opencodeInvocationTestModel  = "openrouter/acme/coding-default"
	opencodeInvocationTestEffort = "high"
	opencodeInvocationAgent      = "opencode-native"
	opencodeRejectionAgent       = "opencode-reject"
)

// openCodeInvocationFixture is one temporary native installation serving discovery and ACP.
type openCodeInvocationFixture struct {
	dir string
	bin string
	exe string
	log string
}

// not parallel: the fixture exports PATH and OpenCode env through t.Setenv.
func writeOpenCodeInvocationFixture(
	t *testing.T,
	driver string,
	fixtureAgent string,
	diagnosticsPath string,
	npmOnly bool,
) openCodeInvocationFixture {
	t.Helper()

	dir := t.TempDir()
	script := `#!/bin/sh
log="${OPENCODE_FIXTURE_LOG:?missing log}"
printf 'exe=%s\n' "$0" >> "$log"
printf 'args=%s\n' "$*" >> "$log"
printf 'cwd=%s\n' "$PWD" >> "$log"
if [ -n "$OPENCODE_FIXTURE_ACCOUNT" ]; then printf 'account=%s\n' "$OPENCODE_FIXTURE_ACCOUNT" >> "$log"; fi
data="${OPENCODE_FIXTURE_DIR:?missing fixture dir}"
if [ -f "$data/acp-fail" ]; then echo "opencode acp startup failed" >&2; exit 1; fi
major="$(/bin/cat "$data/major")"
dispatch() {
  case "$1" in
    --version) /bin/cat "$data/version";;
    models) if [ "$major" = "1" ]; then /bin/cat "$data/v1.txt"; else echo "unexpected opencode discovery: $* (fixture major $major)" >&2; exit 1; fi;;
    api) if [ "$major" = "2" ]; then /bin/cat "$data/v2.json"; else echo "unexpected opencode discovery: $* (fixture major $major)" >&2; exit 1; fi;;
    acp) if [ "$#" -ne 1 ]; then echo "unexpected opencode acp args: $*" >&2; exit 1; fi
      exec "${OPENCODE_FIXTURE_DRIVER:?missing driver}" --fixture "${OPENCODE_FIXTURE_JSON:?missing fixture}" --agent "${OPENCODE_FIXTURE_AGENT:?missing agent}" --diagnostics "${OPENCODE_FIXTURE_DIAG:?missing diagnostics}" --provider opencode;;
    *) echo "unexpected opencode command: $*" >&2; exit 1;;
  esac
}
if [ "${OPENCODE_FIXTURE_NPM:-0}" = "1" ]; then
  if [ "$1" = "-y" ]; then shift; fi
  case "$1" in opencode-ai*) shift;; *) echo "unexpected npm command: $*" >&2; exit 1;; esac
fi
dispatch "$@"
`
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	name := "opencode"
	if npmOnly {
		name = "npx"
	}
	exe := filepath.Join(binDir, name)
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(fixture executable) error = %v", err)
	}
	writeOpenCodeInvocationFile(t, dir, "v1.txt",
		"openrouter/acme/coding-default\n"+
			`{"name":"Coding default","capabilities":{"reasoning":true,"toolcall":true},`+
			`"limit":{"context":262144},"variants":{"high":{},"low":{}}}`+"\n")
	writeOpenCodeInvocationFile(t, dir, "v2.json", `{"location":{},"data":[
{"providerID":"openrouter","id":"acme/coding-default","name":"Coding default","enabled":true,
"capabilities":{"tools":true},"limit":{"context":262144,"input":200000,"output":32768},
"variants":[{"id":"high"},{"id":"low"}]},
{"providerID":"openai","id":"qa-next","name":"QA Next","enabled":true,"limit":{"context":128000},"variants":[]}
]}`)
	fx := openCodeInvocationFixture{dir: dir, bin: binDir, exe: exe, log: filepath.Join(dir, "calls.log")}
	setOpenCodeInvocationVersion(t, fx, "1", "opencode v1.18.33\n")
	t.Setenv("OPENCODE_FIXTURE_DRIVER", driver)
	t.Setenv("OPENCODE_FIXTURE_JSON", opencodeInvocationFixturePath(t))
	t.Setenv("OPENCODE_FIXTURE_AGENT", fixtureAgent)
	t.Setenv("OPENCODE_FIXTURE_DIAG", diagnosticsPath)
	t.Setenv("OPENCODE_FIXTURE_DIR", dir)
	t.Setenv("OPENCODE_FIXTURE_LOG", fx.log)
	if npmOnly {
		t.Setenv("OPENCODE_FIXTURE_NPM", "1")
		t.Setenv("PATH", binDir)
	} else {
		t.Setenv("OPENCODE_FIXTURE_NPM", "0")
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return fx
}

func writeOpenCodeInvocationFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}

func setOpenCodeInvocationVersion(t *testing.T, fx openCodeInvocationFixture, major string, version string) {
	t.Helper()
	writeOpenCodeInvocationFile(t, fx.dir, "major", major+"\n")
	writeOpenCodeInvocationFile(t, fx.dir, "version", version)
}

func opencodeInvocationFixturePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "testutil", "acpmock", "testdata", "opencode_session_fixture.json"))
	if err != nil {
		t.Fatalf("Abs(fixture) error = %v", err)
	}
	return path
}

func newOpenCodeInvocationManager(t *testing.T, agents []compozyconfig.AgentDef) *harness {
	t.Helper()

	h := newHarness(t)
	// Drop the harness driver rewrite so OpenCode resolves through the built-in registry.
	delete(h.cfg.Providers, "opencode")
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		ID:      h.workspaceID,
		RootDir: h.workspace,
		Name:    h.workspaceName,
		Config:  h.cfg,
		Agents:  agents,
	})
	driver := newIntegrationACPDriver(
		acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		acp.WithStopTimeout(100*time.Millisecond),
	)
	h.manager = newManagerWithHarness(t, h, WithDriver(NewACPDriverAdapter(driver)))
	return h
}

func openCodeInvocationAgent(prompt string) compozyconfig.AgentDef {
	return compozyconfig.AgentDef{Name: "coder", Provider: "opencode", Prompt: prompt}
}

func stopOpenCodeInvocationSession(t *testing.T, manager *Manager, sess *Session) {
	t.Helper()
	if sess == nil {
		return
	}
	if err := manager.Stop(testutil.Context(t), sess.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	waitForStoppedSession(t, manager, sess)
}

func readOpenCodeInvocationLog(t *testing.T, fx openCodeInvocationFixture) []string {
	t.Helper()
	raw, err := os.ReadFile(fx.log)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("ReadFile(calls.log) error = %v", err)
	}
	var lines []string
	for line := range strings.Lines(strings.TrimSpace(string(raw))) {
		lines = append(lines, strings.TrimSpace(line))
	}
	return lines
}

func logExeMarkers(lines []string) []string {
	return openCodeLogValues(lines, "exe=")
}

func openCodeLogValues(lines []string, prefix string) []string {
	var values []string
	for _, line := range lines {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			values = append(values, value)
		}
	}
	return values
}

func runOpenCodeInvocationDiscovery(
	t *testing.T,
	workDir string,
	baseEnv []string,
) []modelcatalog.Model {
	t.Helper()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), "global.db")
	store, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if closeErr := store.Close(closeCtx); closeErr != nil {
			t.Errorf("GlobalDB.Close() error = %v", closeErr)
		}
	})
	source, err := modelcatalog.NewLiveProviderSource(
		"opencode",
		compozyconfig.BuiltinProviders()["opencode"],
		&modelcatalog.LiveProviderSourcesConfig{BaseEnv: baseEnv, WorkingDir: workDir},
	)
	if err != nil {
		t.Fatalf("NewLiveProviderSource() error = %v", err)
	}
	service, err := modelcatalog.NewService(store, []modelcatalog.Source{source}, modelcatalog.MergeOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	execution := modelcatalog.CatalogExecutionContext{
		Scope:     modelcatalog.ExecutionScopeProfile,
		ProfileID: "it-opencode-invocation",
	}
	if err := service.SetDefaultExecutionContext(execution); err != nil {
		t.Fatalf("SetDefaultExecutionContext() error = %v", err)
	}
	if _, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
		ProviderID:       "opencode",
		ExecutionContext: execution,
		Force:            true,
		Now:              time.Now(),
	}); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	models, err := service.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID:       "opencode",
		ExecutionContext: execution,
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	return models
}

func assertOpenCodeInvocationDiscovery(
	t *testing.T,
	models []modelcatalog.Model,
	fx openCodeInvocationFixture,
	versionOp string,
) {
	t.Helper()

	found := false
	for _, model := range models {
		if model.ModelID == opencodeInvocationTestModel &&
			model.AvailabilityState == modelcatalog.AvailabilityStateAvailableLive {
			found = true
		}
	}
	if !found {
		t.Fatalf("discovery models = %#v, want live %q", models, opencodeInvocationTestModel)
	}
	lines := readOpenCodeInvocationLog(t, fx)
	exes := logExeMarkers(lines)
	if len(exes) == 0 {
		t.Fatalf("discovery log = %#v, want exe markers", lines)
	}
	for _, exe := range exes {
		if exe != fx.exe {
			t.Fatalf("discovery exe = %q, want %q", exe, fx.exe)
		}
	}
	joined := strings.Join(openCodeLogValues(lines, "args="), "\n")
	if !strings.Contains(joined, "--version") || !strings.Contains(joined, versionOp) {
		t.Fatalf("discovery args = %q, want --version then %q", joined, versionOp)
	}
}

func collectAssistantTexts(events []acp.AgentEvent) []string {
	var texts []string
	for _, event := range events {
		if event.Type == acp.EventTypeAgentMessage {
			texts = append(texts, event.Text)
		}
	}
	return texts
}

func assertAssistantText(t *testing.T, events []acp.AgentEvent, want string) {
	t.Helper()
	for _, text := range collectAssistantTexts(events) {
		if strings.Contains(text, want) {
			return
		}
	}
	t.Fatalf("assistant texts = %#v, want %q", collectAssistantTexts(events), want)
}

func openCodePromptDiagnostics(t *testing.T, diagnosticsPath string, sessionID string) []acpmock.DiagnosticsRecord {
	t.Helper()
	records, err := acpmock.ReadDiagnostics(diagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics() error = %v", err)
	}
	return acpmock.PromptDiagnostics(acpmock.DiagnosticsForCompozySession(records, sessionID))
}

func assertOpenCodeOptionsApplied(
	t *testing.T,
	diagnosticsPath string,
	sessionID string,
) {
	t.Helper()

	records, err := acpmock.ReadDiagnostics(diagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics() error = %v", err)
	}
	owned := acpmock.DiagnosticsForCompozySession(records, sessionID)
	setModel, setEffort, firstPrompt := -1, -1, -1
	for index, record := range owned {
		if strings.TrimSpace(record.ProtocolMethod) != "" {
			if record.ConfigOptionID == "model" && record.ConfigOptionValue == opencodeInvocationTestModel {
				setModel = index
			}
			if record.ConfigOptionValue == opencodeInvocationTestEffort {
				setEffort = index
			}
			continue
		}
		if record.PromptIndex > 0 && firstPrompt < 0 {
			firstPrompt = index
		}
	}
	if setModel < 0 || setEffort < 0 {
		t.Fatalf("protocol records = %#v, want model and effort application", owned)
	}
	if firstPrompt < 0 {
		t.Fatal("no prompt record, want completed inference after option application")
	}
	if setModel > firstPrompt || setEffort > firstPrompt {
		t.Fatalf("options applied at %d/%d after first prompt at %d", setModel, setEffort, firstPrompt)
	}
}

func TestManagerIntegrationOpenCodeInvocation(t *testing.T) {
	driver := acpmock.RequireDriver(t)

	for _, major := range []string{"1", "2"} {
		versionOp := "models --verbose"
		if major == "2" {
			versionOp = "api --standalone GET /api/model?"
		}
		t.Run("Should complete first and restarted prompts on the native v"+major+" installation", func(t *testing.T) {
			diagnosticsPath := filepath.Join(t.TempDir(), "diagnostics.jsonl")
			fx := writeOpenCodeInvocationFixture(t, driver, opencodeInvocationAgent, diagnosticsPath, false)
			if major == "1" {
				setOpenCodeInvocationVersion(t, fx, major, "opencode v1.18.33\n")
			} else {
				setOpenCodeInvocationVersion(t, fx, major, "opencode v2.0.32\n")
			}

			models := runOpenCodeInvocationDiscovery(t, t.TempDir(), []string{
				"PATH=" + fx.bin + string(os.PathListSeparator) + os.Getenv("PATH"),
				"HOME=" + t.TempDir(),
				"OPENCODE_FIXTURE_DIR=" + fx.dir,
				"OPENCODE_FIXTURE_LOG=" + fx.log,
			})
			assertOpenCodeInvocationDiscovery(t, models, fx, versionOp)
			beforeLaunch := len(readOpenCodeInvocationLog(t, fx))

			h := newOpenCodeInvocationManager(t, []compozyconfig.AgentDef{
				openCodeInvocationAgent("You are a coding assistant."),
			})
			sess, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
				AgentName:       "coder",
				Provider:        "opencode",
				Model:           opencodeInvocationTestModel,
				ReasoningEffort: opencodeInvocationTestEffort,
				Workspace:       h.workspaceID,
			}))
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			t.Cleanup(func() { stopActiveIntegrationSession(t, h, sess.ID) })

			if got := sess.providerRoutingSnapshot().Command; got != "opencode acp" {
				t.Fatalf("prepared command = %q, want inherited builtin %q", got, "opencode acp")
			}
			if !sess.providerRoutingSnapshot().CommandIsBuiltin {
				t.Fatal("prepared command origin is not builtin, want inherited origin preserved")
			}
			if got := sess.providerRoutingSnapshot().AuthMode; got == compozyconfig.ProviderAuthModeNone {
				t.Fatalf("prepared auth mode = %q, want native CLI handling, not none", got)
			}
			lines := readOpenCodeInvocationLog(t, fx)
			launch := lines[beforeLaunch:]
			if got := logExeMarkers(launch); len(got) == 0 || got[0] != fx.exe {
				t.Fatalf("launch exe markers = %#v, want %q", got, fx.exe)
			}
			cwds := openCodeLogValues(launch, "cwd=")
			if len(cwds) == 0 || cwds[0] != sess.Info().Workspace {
				t.Fatalf("launch cwd markers = %#v, want %q", cwds, sess.Info().Workspace)
			}

			events, err := h.manager.Prompt(testutil.Context(t), sess.ID, "opencode first prompt")
			if err != nil {
				t.Fatalf("Prompt(first) error = %v", err)
			}
			assertAssistantText(t, collectEvents(t, events), "opencode first response")
			assertOpenCodeOptionsApplied(t, diagnosticsPath, sess.ID)

			active := sess.processHandle()
			if active == nil {
				t.Fatal("active process = nil, want running ACP process")
			}
			activePID := active.PID

			// A catalog refresh while the process is active must not replace it.
			runOpenCodeInvocationDiscovery(t, t.TempDir(), []string{
				"PATH=" + fx.bin + string(os.PathListSeparator) + os.Getenv("PATH"),
				"HOME=" + t.TempDir(),
				"OPENCODE_FIXTURE_DIR=" + fx.dir,
				"OPENCODE_FIXTURE_LOG=" + fx.log,
			})
			if got := sess.processHandle(); got == nil || got.PID != activePID {
				t.Fatalf("active process changed across refresh: %+v, want pid %d", got, activePID)
			}
			if !sessionStopProcessAlive(activePID) {
				t.Fatalf("active process %d exited across refresh", activePID)
			}

			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
			waitForStoppedSession(t, h.manager, sess)
			waitForSessionStopProcessExit(t, activePID, 5*time.Second)

			resumed, err := h.manager.Resume(testutil.Context(t), sess.ID)
			if err != nil {
				t.Fatalf("Resume() error = %v", err)
			}
			t.Cleanup(func() { stopActiveIntegrationSession(t, h, resumed.ID) })
			if got := resumed.providerRoutingSnapshot().Command; got != "opencode acp" {
				t.Fatalf("rebound command = %q, want inherited builtin %q", got, "opencode acp")
			}
			rebound := resumed.processHandle()
			if rebound == nil || rebound.PID == activePID {
				t.Fatalf("rebound process = %+v, want fresh process unlike pid %d", rebound, activePID)
			}
			second, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "opencode second prompt")
			if err != nil {
				t.Fatalf("Prompt(second) error = %v", err)
			}
			assertAssistantText(t, collectEvents(t, second), "opencode second response")
			if got := len(openCodePromptDiagnostics(t, diagnosticsPath, resumed.ID)); got != 2 {
				t.Fatalf("prompt diagnostics = %d, want first and restarted prompts", got)
			}
			stopOpenCodeInvocationSession(t, h.manager, resumed)
			waitForSessionStopProcessExit(t, rebound.PID, 5*time.Second)
		})
	}

	t.Run("Should use the explicit selector and reject incompatible models truthfully", func(t *testing.T) {
		diagnosticsPath := filepath.Join(t.TempDir(), "diagnostics.jsonl")
		fx := writeOpenCodeInvocationFixture(t, driver, opencodeInvocationAgent, diagnosticsPath, false)
		explicit := writeOpenCodeInvocationFixture(t, driver, opencodeInvocationAgent, diagnosticsPath, false)
		beforeExplicit := len(readOpenCodeInvocationLog(t, fx))

		h := newOpenCodeInvocationManager(t, []compozyconfig.AgentDef{{
			Name:     "coder",
			Provider: "opencode",
			Command: shellquote.Join(
				"OPENCODE_FIXTURE_ACCOUNT=secondary",
				explicit.exe,
				"acp",
			),
			Prompt: "You are a coding assistant.",
		}})

		sess, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName:       "coder",
			Provider:        "opencode",
			Model:           opencodeInvocationTestModel,
			ReasoningEffort: opencodeInvocationTestEffort,
			Workspace:       h.workspaceID,
		}))
		if err != nil {
			t.Fatalf("Create(explicit) error = %v", err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, sess.ID) })
		routing := sess.providerRoutingSnapshot()
		if routing.CommandIsBuiltin {
			t.Fatal("explicit command origin is builtin, want authored origin")
		}
		if got, want := routing.Command, shellquote.Join(
			"OPENCODE_FIXTURE_ACCOUNT=secondary",
			explicit.exe,
			"acp",
		); got != want {
			t.Fatalf("prepared command = %q, want explicit %q", got, want)
		}
		lines := readOpenCodeInvocationLog(t, explicit)
		if got := logExeMarkers(lines); len(got) == 0 || got[0] != explicit.exe {
			t.Fatalf("explicit exe markers = %#v, want %q", got, explicit.exe)
		}
		if got := openCodeLogValues(lines, "account="); len(got) == 0 || got[0] != "secondary" {
			t.Fatalf("explicit account markers = %#v, want secondary assignment", got)
		}
		if got := len(readOpenCodeInvocationLog(t, fx)); got != beforeExplicit {
			t.Fatalf("earlier installation log lines = %d, want %d (no reuse)", got, beforeExplicit)
		}
		events, err := h.manager.Prompt(testutil.Context(t), sess.ID, "opencode first prompt")
		if err != nil {
			t.Fatalf("Prompt(explicit) error = %v", err)
		}
		assertAssistantText(t, collectEvents(t, events), "opencode first response")
		stopOpenCodeInvocationSession(t, h.manager, sess)

		rejectDiagnostics := filepath.Join(t.TempDir(), "reject-diagnostics.jsonl")
		writeOpenCodeInvocationFixture(t, driver, opencodeRejectionAgent, rejectDiagnostics, false)
		rejectHarness := newOpenCodeInvocationManager(t, []compozyconfig.AgentDef{{
			Name: "rejector", Provider: "opencode", Prompt: "You are a coding assistant.",
		}})
		rejected, err := rejectHarness.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "rejector",
			Workspace: rejectHarness.workspaceID,
		}))
		if err != nil {
			t.Fatalf("Create(reject harness) error = %v", err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, rejectHarness, rejected.ID) })
		updated, err := rejectHarness.manager.SetRuntimeSelection(
			testutil.Context(t),
			rejected.ID,
			RuntimeSelection{Provider: "opencode", Model: "openai/qa-next", ReasoningEffort: "high"},
			0,
		)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(incompatible) error = %v, want persisted intent", err)
		}
		if updated.SelectedRuntime == nil || updated.SelectedRuntime.Model != "openai/qa-next" {
			t.Fatalf("persisted selection = %#v, want incompatible intent recorded", updated.SelectedRuntime)
		}
		if got := rejected.Info().Model; got != opencodeInvocationTestModel {
			t.Fatalf("live model changed before negotiation = %q, want %q", got, opencodeInvocationTestModel)
		}
		_, err = rejectHarness.manager.Prompt(testutil.Context(t), rejected.ID, "opencode first prompt")
		if err == nil || !strings.Contains(err.Error(), "openai/qa-next") {
			t.Fatalf("Prompt(incompatible) error = %v, want truthful rejection", err)
		}
		if got := len(openCodePromptDiagnostics(t, rejectDiagnostics, rejected.ID)); got != 0 {
			t.Fatalf("rejected prompt diagnostics = %d, want no inference", got)
		}
	})

	t.Run("Should report native startup failure without falling back to another installation", func(t *testing.T) {
		diagnosticsPath := filepath.Join(t.TempDir(), "diagnostics.jsonl")
		fx := writeOpenCodeInvocationFixture(t, driver, opencodeInvocationAgent, diagnosticsPath, false)

		canaryDir := t.TempDir()
		canaryLog := filepath.Join(canaryDir, "npx.log")
		canary := "#!/bin/sh\nprintf 'canary args=%s\\n' \"$*\" >> \"" + canaryLog + "\"\nexit 1\n"
		if err := os.MkdirAll(filepath.Join(canaryDir, "bin"), 0o755); err != nil {
			t.Fatalf("MkdirAll(canary) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(canaryDir, "bin", "npx"), []byte(canary), 0o755); err != nil {
			t.Fatalf("WriteFile(canary npx) error = %v", err)
		}
		t.Setenv("PATH", fx.bin+string(os.PathListSeparator)+filepath.Join(canaryDir, "bin")+
			string(os.PathListSeparator)+os.Getenv("PATH"))

		writeOpenCodeInvocationFile(t, fx.dir, "acp-fail", "1\n")
		h := newOpenCodeInvocationManager(t, []compozyconfig.AgentDef{
			openCodeInvocationAgent("You are a coding assistant."),
		})
		sess, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName:       "coder",
			Provider:        "opencode",
			Model:           opencodeInvocationTestModel,
			ReasoningEffort: opencodeInvocationTestEffort,
			Workspace:       h.workspaceID,
		}))
		if err == nil {
			stopOpenCodeInvocationSession(t, h.manager, sess)
			t.Fatal("Create(failing native) error = nil, want startup failure")
		}
		t.Logf("Create(failing native) error = %v", err)
		if raw, readErr := os.ReadFile(canaryLog); !os.IsNotExist(readErr) {
			t.Fatalf("canary npx log = %q, want no fallback invocation", string(raw))
		}
		lines := readOpenCodeInvocationLog(t, fx)
		if got := strings.Join(openCodeLogValues(lines, "args="), "\n"); !strings.Contains(got, "acp") {
			raw, readErr := os.ReadFile(fx.log)
			t.Fatalf(
				"startup args = %q (raw=%q, read error=%v), want the selected native acp attempt",
				got,
				string(raw),
				readErr,
			)
		}
		if sess != nil {
			stopOpenCodeInvocationSession(t, h.manager, sess)
		}

		if err := os.Remove(filepath.Join(fx.dir, "acp-fail")); err != nil {
			t.Fatalf("Remove(acp-fail) error = %v", err)
		}
		recovered, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName:       "coder",
			Provider:        "opencode",
			Model:           opencodeInvocationTestModel,
			ReasoningEffort: opencodeInvocationTestEffort,
			Workspace:       h.workspaceID,
		}))
		if err != nil {
			t.Fatalf("Create(recovered) error = %v", err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, recovered.ID) })
		events, err := h.manager.Prompt(testutil.Context(t), recovered.ID, "opencode first prompt")
		if err != nil {
			t.Fatalf("Prompt(recovered) error = %v", err)
		}
		assertAssistantText(t, collectEvents(t, events), "opencode first response")
	})

	t.Run("Should complete a prompt through the npm-only installation", func(t *testing.T) {
		diagnosticsPath := filepath.Join(t.TempDir(), "diagnostics.jsonl")
		fx := writeOpenCodeInvocationFixture(t, driver, opencodeInvocationAgent, diagnosticsPath, true)
		setOpenCodeInvocationVersion(t, fx, "2", "opencode v2.0.32\n")

		baseEnv := []string{
			"PATH=" + fx.bin,
			"HOME=" + t.TempDir(),
			"OPENCODE_FIXTURE_DIR=" + fx.dir,
			"OPENCODE_FIXTURE_LOG=" + fx.log,
			"OPENCODE_FIXTURE_NPM=1",
		}
		models := runOpenCodeInvocationDiscovery(t, t.TempDir(), baseEnv)
		assertOpenCodeInvocationDiscovery(t, models, fx, "api --standalone GET /api/model?")
		beforeLaunch := len(readOpenCodeInvocationLog(t, fx))

		h := newOpenCodeInvocationManager(t, []compozyconfig.AgentDef{
			openCodeInvocationAgent("You are a coding assistant."),
		})
		sess, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName:       "coder",
			Provider:        "opencode",
			Model:           opencodeInvocationTestModel,
			ReasoningEffort: opencodeInvocationTestEffort,
			Workspace:       h.workspaceID,
		}))
		if err != nil {
			t.Fatalf("Create(npm) error = %v", err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, sess.ID) })
		if got := sess.providerRoutingSnapshot().Command; got != "opencode acp" {
			t.Fatalf("prepared command = %q, want inherited builtin %q", got, "opencode acp")
		}
		if !sess.providerRoutingSnapshot().CommandIsBuiltin {
			t.Fatal("npm prepared command origin is not builtin, want inherited origin preserved")
		}
		launch := readOpenCodeInvocationLog(t, fx)[beforeLaunch:]
		if got := logExeMarkers(launch); len(got) == 0 || got[0] != fx.exe {
			t.Fatalf("npm launch exe markers = %#v, want %q", got, fx.exe)
		}
		events, err := h.manager.Prompt(testutil.Context(t), sess.ID, "opencode first prompt")
		if err != nil {
			t.Fatalf("Prompt(npm) error = %v", err)
		}
		assertAssistantText(t, collectEvents(t, events), "opencode first response")
		assertOpenCodeOptionsApplied(t, diagnosticsPath, sess.ID)
	})
}
