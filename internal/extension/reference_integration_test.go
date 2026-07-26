//go:build integration

package extensionpkg_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	devcycle "github.com/compozy/agh/extensions/dev-cycle"
	"github.com/compozy/agh/internal/cli"
	aghconfig "github.com/compozy/agh/internal/config"
	daemonpkg "github.com/compozy/agh/internal/daemon"
	extensionpkg "github.com/compozy/agh/internal/extension"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/testutil/acpmock"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
	"github.com/kballard/go-shellquote"
)

const (
	referenceACPHelperEnvKey     = "AGH_TEST_REFERENCE_ACP_HELPER"
	referencePromptLogPathEnvKey = "AGH_TEST_REFERENCE_PROMPT_LOG_PATH"
)

type referenceHandshakeMarker struct {
	Request  subprocess.InitializeRequest  `json:"request"`
	Response subprocess.InitializeResponse `json:"response"`
	PID      int                           `json:"pid"`
}

type referenceHostCallMarker struct {
	SessionCount int               `json:"session_count"`
	Sessions     []json.RawMessage `json:"sessions"`
	Error        string            `json:"error,omitempty"`
	PID          int               `json:"pid"`
}

type referenceCapabilityMarker struct {
	Denied  bool           `json:"denied"`
	Message string         `json:"message,omitempty"`
	Code    int            `json:"code,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type referencePromptLogEntry struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

type referenceHarness struct {
	client       cli.DaemonClient
	daemonCancel context.CancelFunc
	daemonErrCh  chan error
	homePaths    aghconfig.HomePaths
	httpBaseURL  string
	logBuffer    *lockedBuffer
	repoRoot     string
	workspace    workspacepkg.ResolvedWorkspace

	promptLogPath string

	secretHandshakePath string
	secretHostCallPath  string
	secretStartsPath    string
	secretCrashOncePath string
	secretShutdownPath  string

	promptHandshakePath  string
	promptHostCallPath   string
	promptCapabilityPath string
	promptShutdownPath   string
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(payload []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(payload)
}

func (b *lockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Len()
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

type referenceACPAgent struct{}

func TestReferenceExtensionACPHelperProcess(t *testing.T) {
	if os.Getenv(referenceACPHelperEnvKey) != "1" {
		return
	}

	conn := acpsdk.NewAgentSideConnection(referenceACPAgent{}, os.Stdout, os.Stdin)
	<-conn.Done()
	os.Exit(0)
}

func TestReferenceExtensionsEndToEnd(t *testing.T) {
	// not parallel: the reference extensions read process environment marker paths.
	if runtime.GOOS == "windows" {
		t.Skip("reference extension integration uses Unix domain sockets and shell command quoting")
	}

	repoRoot := referenceRepoRoot(t)
	buildReferenceArtifacts(t, repoRoot)

	harness := newReferenceHarness(t, repoRoot)
	promptEnhancerInstallSource := referencePromptEnhancerInstallSource(t, repoRoot)

	secret := harness.installExtension(t, "sdk/examples/secret-guard")
	if secret.Name != "secret-guard" {
		t.Fatalf("secret install name = %q, want secret-guard", secret.Name)
	}
	if secret.State != "active" || secret.Health != "healthy" {
		t.Fatalf("secret install status = %#v, want active/healthy", secret)
	}

	promptEnhancer := harness.installExtension(t, promptEnhancerInstallSource)
	if promptEnhancer.Name != "prompt-enhancer" {
		t.Fatalf("prompt install name = %q, want prompt-enhancer", promptEnhancer.Name)
	}
	if promptEnhancer.State != "active" || promptEnhancer.Health != "healthy" {
		t.Fatalf("prompt install status = %#v, want active/healthy", promptEnhancer)
	}

	clarifyTool := harness.installExtension(t, "sdk/examples/clarify-tool")
	if clarifyTool.Name != "clarify-tool" {
		t.Fatalf("clarify install name = %q, want clarify-tool", clarifyTool.Name)
	}
	if clarifyTool.State != "active" || clarifyTool.Health != "healthy" {
		t.Fatalf("clarify install status = %#v, want active/healthy", clarifyTool)
	}

	secretPID := waitForStartedPID(t, harness.secretStartsPath, 2, 10*time.Second)
	secretHandshake := waitForHandshakeMarker(t, harness.secretHandshakePath, secretPID, 10*time.Second)
	if secretHandshake.Request.ProtocolVersion != "1" {
		t.Fatalf("secret handshake protocol = %q, want 1", secretHandshake.Request.ProtocolVersion)
	}
	if secretHandshake.PID <= 0 {
		t.Fatalf("secret handshake pid = %d, want > 0", secretHandshake.PID)
	}
	if got := secretHandshake.Request.Capabilities.GrantedActions; len(got) != 1 || got[0] != "sessions/list" {
		t.Fatalf("secret granted actions = %#v, want sessions/list", got)
	}
	if got := secretHandshake.Request.Capabilities.GrantedSecurity; len(got) != 1 || got[0] != "session.read" {
		t.Fatalf("secret granted security = %#v, want session.read", got)
	}
	if got := secretHandshake.Response.SupportedHookEvents; len(got) != 1 ||
		got[0] != string(hookspkg.HookInputPreSubmit) {
		t.Fatalf("secret supported hook events = %#v, want input.pre_submit", got)
	}

	promptHandshake := waitForJSONFile[referenceHandshakeMarker](t, harness.promptHandshakePath, 10*time.Second)
	if promptHandshake.Request.ProtocolVersion != "1" {
		t.Fatalf("prompt handshake protocol = %q, want 1", promptHandshake.Request.ProtocolVersion)
	}
	if promptHandshake.PID <= 0 {
		t.Fatalf("prompt handshake pid = %d, want > 0", promptHandshake.PID)
	}
	if got := promptHandshake.Request.Capabilities.GrantedActions; len(got) != 1 || got[0] != "sessions/list" {
		t.Fatalf("prompt granted actions = %#v, want sessions/list", got)
	}
	if got := promptHandshake.Request.Capabilities.GrantedSecurity; len(got) != 1 || got[0] != "session.read" {
		t.Fatalf("prompt granted security = %#v, want session.read", got)
	}
	if got := promptHandshake.Response.SupportedHookEvents; len(got) != 1 ||
		got[0] != string(hookspkg.HookPromptPostAssemble) {
		t.Fatalf("prompt supported hook events = %#v, want prompt.post_assemble", got)
	}

	secretHostCall := waitForHostCallMarker(t, harness.secretHostCallPath, secretPID, 10*time.Second)
	if secretHostCall.Error != "" {
		t.Fatalf("secret host call error = %q, want empty", secretHostCall.Error)
	}

	promptHostCall := waitForHostCallMarker(t, harness.promptHostCallPath, promptHandshake.PID, 10*time.Second)
	if promptHostCall.Error != "" {
		t.Fatalf("prompt host call error = %q, want empty", promptHostCall.Error)
	}

	capabilityMarker := waitForJSONFile[referenceCapabilityMarker](t, harness.promptCapabilityPath, 10*time.Second)
	if !capabilityMarker.Denied {
		t.Fatalf("capability marker = %#v, want denied=true", capabilityMarker)
	}
	if capabilityMarker.Code != -32001 {
		t.Fatalf("capability code = %d, want -32001", capabilityMarker.Code)
	}
	if got := strings.TrimSpace(fmt.Sprint(capabilityMarker.Data["method"])); got != "sessions/create" {
		t.Fatalf("capability denied method = %q, want sessions/create", got)
	}

	hooks := harness.hookCatalog(t)
	if !hookCatalogContains(hooks, "secret-guard-hook", string(hookspkg.HookInputPreSubmit)) {
		t.Fatalf("hook catalog = %#v, want secret-guard input hook", hooks)
	}
	if !hookCatalogContains(hooks, "workspace-context", string(hookspkg.HookPromptPostAssemble)) {
		t.Fatalf("hook catalog = %#v, want prompt-enhancer prompt hook", hooks)
	}

	session := harness.createSession(t)
	if session.WorkspaceID == "" {
		t.Fatal("create session workspace id = empty, want resolved workspace")
	}

	harness.assertClarificationRoundTrip(t, session)

	if _, err := harness.promptSession(t, session.ID, "Summarize the current workspace."); err != nil {
		t.Fatalf("safe PromptSession() error = %v", err)
	}

	promptEntries := harness.waitForPromptEntries(t, 1)
	firstPrompt := promptEntries[0].Text
	if !containsFragmentsInOrder(
		firstPrompt,
		"[Workspace: "+harness.workspace.RootDir+"]",
		"You are a coding assistant.",
		"User request:",
		"Summarize the current workspace.",
	) {
		t.Fatalf("first prompt = %q, want workspace prefix plus user request", firstPrompt)
	}

	if _, err := harness.promptSession(t, session.ID, "please keep sk-abc123 safe"); err == nil {
		t.Fatal("secret PromptSession() error = nil, want denied hook error")
	} else if !strings.Contains(err.Error(), "input.pre_submit") && !strings.Contains(strings.ToLower(err.Error()), "denied") {
		t.Fatalf("secret PromptSession() error = %v, want hook denial", err)
	}

	harness.ensurePromptEntryCount(t, 1, 500*time.Millisecond)

	runs := harness.hookRuns(t, session.ID, 32)
	if !hookRunContains(runs, string(hookspkg.HookInputPreSubmit), "denied", "sk-") {
		t.Fatalf("hook runs = %#v, want denied input.pre_submit patch", runs)
	}

	secretBefore, err := harness.extensionStatus("secret-guard")
	if err != nil {
		t.Fatalf("ExtensionStatus(secret-guard) error = %v", err)
	}
	if secretBefore.PID <= 0 {
		t.Fatalf("secret-guard pid = %d, want active process pid", secretBefore.PID)
	}
	startsBefore := len(waitForNonEmptyFileLines(t, harness.secretStartsPath, 10*time.Second))
	if err := syscall.Kill(secretBefore.PID, syscall.SIGKILL); err != nil {
		t.Fatalf("syscall.Kill(%d, SIGKILL) error = %v", secretBefore.PID, err)
	}
	waitForCondition(t, 15*time.Second, "secret-guard restart after crash", func() bool {
		lines, err := readFileLines(harness.secretStartsPath)
		if err != nil {
			return false
		}
		status, statusErr := harness.extensionStatus("secret-guard")
		return statusErr == nil &&
			status.State == "active" &&
			status.Health == "healthy" &&
			status.PID > 0 &&
			status.PID != secretBefore.PID &&
			len(lines) >= startsBefore+1
	})

	harness.shutdown(t)

	secretShutdownLines := waitForNonEmptyFileLines(t, harness.secretShutdownPath, 10*time.Second)
	if len(secretShutdownLines) == 0 {
		t.Fatal("secret shutdown marker = empty, want at least one shutdown line")
	}
	promptShutdownLines := waitForNonEmptyFileLines(t, harness.promptShutdownPath, 10*time.Second)
	if len(promptShutdownLines) == 0 {
		t.Fatal("prompt shutdown marker = empty, want at least one shutdown line")
	}
}

func newReferenceHarness(t *testing.T, repoRoot string) *referenceHarness {
	t.Helper()

	homePaths := referenceHomePaths(t)
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	markersDir := referenceMarkerDir(t)
	harness := &referenceHarness{
		homePaths: homePaths,
		logBuffer: &lockedBuffer{},
		repoRoot:  repoRoot,

		promptLogPath: filepath.Join(markersDir, "acp-prompts.jsonl"),

		secretHandshakePath: filepath.Join(markersDir, "secret-handshake.json"),
		secretHostCallPath:  filepath.Join(markersDir, "secret-host-call.json"),
		secretStartsPath:    filepath.Join(markersDir, "secret-starts.log"),
		secretCrashOncePath: filepath.Join(markersDir, "secret-crash-once.json"),
		secretShutdownPath:  filepath.Join(markersDir, "secret-shutdown.log"),

		promptHandshakePath:  filepath.Join(markersDir, "prompt-handshake.json"),
		promptHostCallPath:   filepath.Join(markersDir, "prompt-host-call.json"),
		promptCapabilityPath: filepath.Join(markersDir, "prompt-capability.json"),
		promptShutdownPath:   filepath.Join(markersDir, "prompt-shutdown.log"),
	}

	t.Setenv(referencePromptLogPathEnvKey, harness.promptLogPath)
	t.Setenv("GIN_MODE", "release")
	gin.SetMode(gin.ReleaseMode)
	t.Setenv("AGH_SECRET_GUARD_HANDSHAKE_PATH", harness.secretHandshakePath)
	t.Setenv("AGH_SECRET_GUARD_HOST_CALL_PATH", harness.secretHostCallPath)
	t.Setenv("AGH_SECRET_GUARD_STARTS_PATH", harness.secretStartsPath)
	t.Setenv("AGH_SECRET_GUARD_CRASH_ONCE_PATH", "")
	t.Setenv("AGH_SECRET_GUARD_SHUTDOWN_PATH", harness.secretShutdownPath)
	t.Setenv("AGH_PROMPT_ENHANCER_HANDSHAKE_PATH", harness.promptHandshakePath)
	t.Setenv("AGH_PROMPT_ENHANCER_HOST_CALL_PATH", harness.promptHostCallPath)
	t.Setenv("AGH_PROMPT_ENHANCER_CAPABILITY_PATH", harness.promptCapabilityPath)
	t.Setenv("AGH_PROMPT_ENHANCER_SHUTDOWN_PATH", harness.promptShutdownPath)

	command := referenceACPHelperCommand(t)
	cfg := referenceConfig(t, homePaths, command)
	harness.httpBaseURL = fmt.Sprintf("http://%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	referenceWriteProviderConfig(t, homePaths, acpmock.ProviderName, command)
	referenceWriteAgentDef(t, homePaths, "coder", command)
	harness.workspace = referenceSeedWorkspace(t, homePaths, cfg, filepath.Join(t.TempDir(), "workspace"))
	referenceDisableBundledDevCycle(t, homePaths)

	logger := slog.New(slog.NewTextHandler(harness.logBuffer, nil))
	daemon, err := daemonpkg.New(
		daemonpkg.WithHomePaths(homePaths),
		daemonpkg.WithConfig(&cfg),
		daemonpkg.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("daemon.New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	harness.daemonCancel = cancel
	harness.daemonErrCh = make(chan error, 1)
	go func() {
		harness.daemonErrCh <- daemon.Run(ctx)
	}()

	client, err := cli.NewClient(homePaths.DaemonSocket)
	if err != nil {
		t.Fatalf("cli.NewClient() error = %v", err)
	}
	harness.client = client
	harness.waitForDaemonReady(t)

	t.Cleanup(func() {
		harness.shutdown(t)
		if t.Failed() && harness.logBuffer.Len() > 0 {
			t.Logf("reference daemon logs:\n%s", harness.logBuffer.String())
		}
	})

	return harness
}

func referenceDisableBundledDevCycle(t *testing.T, homePaths aghconfig.HomePaths) {
	t.Helper()

	db, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(disable bundled extension) error = %v", err)
	}
	defer func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(disable bundled extension) error = %v", err)
		}
	}()

	registry := extensionpkg.NewRegistry(db.DB())
	if err := devcycle.EnsureManagedInstall(homePaths, registry); err != nil {
		t.Fatalf("EnsureManagedInstall(dev-cycle) error = %v", err)
	}
	if err := registry.Disable(devcycle.Name); err != nil {
		t.Fatalf("Disable(dev-cycle) error = %v", err)
	}
}

func referenceMarkerDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "agh-reference-extension-markers-")
	if err != nil {
		t.Fatalf("os.MkdirTemp(reference markers) error = %v", err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("reference marker directory retained at %s", dir)
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("os.RemoveAll(%q) error = %v", dir, err)
		}
	})
	return dir
}

func (h *referenceHarness) installExtension(t *testing.T, relativePath string) cli.ExtensionRecord {
	t.Helper()

	root := relativePath
	if !filepath.IsAbs(root) {
		root = filepath.Join(h.repoRoot, relativePath)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(root)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", root, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	record, err := h.client.InstallExtension(ctx, cli.InstallExtensionRequest{
		Path:            root,
		Checksum:        checksum,
		AllowUnverified: true,
	})
	if err != nil {
		t.Fatalf("InstallExtension(%q) error = %v", relativePath, err)
	}

	waitForCondition(t, 10*time.Second, "extension active "+record.Name, func() bool {
		status, statusErr := h.extensionStatus(record.Name)
		return statusErr == nil && status.State == "active" && status.Health == "healthy"
	})

	status, err := h.extensionStatus(record.Name)
	if err != nil {
		t.Fatalf("ExtensionStatus(%q) error = %v", record.Name, err)
	}
	return status
}

func (h *referenceHarness) extensionStatus(name string) (cli.ExtensionRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return h.client.ExtensionStatus(ctx, name)
}

func (h *referenceHarness) waitForDaemonReady(t *testing.T) cli.DaemonStatus {
	t.Helper()

	var status cli.DaemonStatus
	waitForCondition(t, 10*time.Second, "daemon ready", func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		current, err := h.client.DaemonStatus(ctx)
		if err != nil {
			return false
		}
		status = current
		return strings.TrimSpace(current.Status) == "running"
	})
	return status
}

func (h *referenceHarness) hookCatalog(t *testing.T) []cli.HookCatalogRecord {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hooks, err := h.client.HookCatalog(ctx, cli.HookCatalogQuery{})
	if err != nil {
		t.Fatalf("HookCatalog() error = %v", err)
	}
	return hooks
}

func (h *referenceHarness) createSession(t *testing.T) cli.SessionRecord {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := h.client.CreateSession(ctx, cli.CreateSessionRequest{
		AgentName:     "coder",
		WorkspacePath: h.workspace.RootDir,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	return session
}

func (h *referenceHarness) assertClarificationRoundTrip(t *testing.T, session cli.SessionRecord) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	view, err := h.client.GetTool(ctx, "ext__clarify_tool__ask", cli.ToolQuery{
		WorkspaceID: session.WorkspaceID,
		SessionID:   session.ID,
		AgentName:   "coder",
	})
	cancel()
	if err != nil {
		t.Fatalf("GetTool(clarify) error = %v", err)
	}
	if !view.Tool.Decision.Callable {
		t.Fatalf("clarify interactive=%t reasons=%v", view.Tool.Descriptor.RequiresInteraction, view.Tool.Decision.ReasonCodes)
	}

	choice := 1
	h.assertOneClarificationRoundTrip(t, session, referenceClarificationCase{
		input: json.RawMessage(
			`{"question":" Which release lane? ","choices":[" Stable ","Canary"]}`,
		),
		question: "Which release lane?",
		choices:  []string{"Stable", "Canary"},
		request:  cli.ClarificationAnswerRequest{ChoiceIndex: &choice},
		want:     cli.ClarificationAnswerRecord{Choice: &choice},
	})
	h.assertOneClarificationRoundTrip(t, session, referenceClarificationCase{
		input:    json.RawMessage(`{"question":" What deployment label? "}`),
		question: "What deployment label?",
		request:  cli.ClarificationAnswerRequest{Text: " canary-42 "},
		want:     cli.ClarificationAnswerRecord{Text: "canary-42"},
		viaHTTP:  true,
	})
}

type referenceClarificationCase struct {
	input    json.RawMessage
	question string
	choices  []string
	request  cli.ClarificationAnswerRequest
	want     cli.ClarificationAnswerRecord
	viaHTTP  bool
}

func (h *referenceHarness) assertOneClarificationRoundTrip(
	t *testing.T,
	session cli.SessionRecord,
	testCase referenceClarificationCase,
) {
	t.Helper()

	type invokeResult struct {
		response cli.ToolInvokeResponseRecord
		err      error
	}
	resultCh := make(chan invokeResult, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		response, err := h.client.InvokeTool(ctx, "ext__clarify_tool__ask", cli.ToolInvokeRequest{
			SessionID: session.ID,
			Input:     testCase.input,
		})
		resultCh <- invokeResult{response: response, err: err}
	}()

	var pending cli.ClarificationPendingRecord
	waitForCondition(t, 10*time.Second, "extension clarification pending", func() bool {
		select {
		case result := <-resultCh:
			if result.err != nil {
				t.Fatalf("InvokeTool(clarify before pending) error = %v", result.err)
			}
			t.Fatalf("InvokeTool(clarify before pending) returned %#v", result.response)
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		response, err := h.client.ListSessionClarifications(ctx, session.ID)
		if err != nil || len(response.Clarifications) != 1 {
			return false
		}
		pending = response.Clarifications[0]
		return true
	})
	if pending.SessionID != session.ID || pending.AgentName != "coder" {
		t.Fatalf("pending clarification scope = %#v, want session %q agent coder", pending, session.ID)
	}
	if pending.Question != testCase.question || !slices.Equal(pending.Choices, testCase.choices) {
		t.Fatalf("pending clarification = %#v, want normalized question and choices", pending)
	}
	httpPending := h.listSessionClarificationsHTTP(t, session)
	if len(httpPending.Clarifications) != 1 || httpPending.Clarifications[0].RequestID != pending.RequestID {
		t.Fatalf("HTTP pending clarifications = %#v, want request %q", httpPending, pending.RequestID)
	}

	var answer cli.ClarificationAnswerRecord
	if testCase.viaHTTP {
		answer = h.answerSessionClarificationHTTP(t, session, pending.RequestID, testCase.request)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var err error
		answer, err = h.client.AnswerSessionClarification(ctx, session.ID, pending.RequestID, testCase.request)
		cancel()
		if err != nil {
			t.Fatalf("AnswerSessionClarification() error = %v", err)
		}
	}
	assertReferenceClarificationAnswer(t, answer, testCase.want)

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("InvokeTool(clarify) error = %v", result.err)
		}
		var returned cli.ClarificationAnswerRecord
		if err := json.Unmarshal(result.response.Result.Structured, &returned); err != nil {
			t.Fatalf("decode clarification tool result: %v", err)
		}
		assertReferenceClarificationAnswer(t, returned, testCase.want)
	case <-time.After(10 * time.Second):
		t.Fatal("InvokeTool(clarify) did not resume after answer")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	remaining, err := h.client.ListSessionClarifications(ctx, session.ID)
	if err != nil {
		t.Fatalf("ListSessionClarifications(after answer) error = %v", err)
	}
	if len(remaining.Clarifications) != 0 {
		t.Fatalf("pending clarifications after answer = %#v, want empty", remaining.Clarifications)
	}
}

func assertReferenceClarificationAnswer(
	t *testing.T,
	got cli.ClarificationAnswerRecord,
	want cli.ClarificationAnswerRecord,
) {
	t.Helper()
	choiceMatches := got.Choice == nil && want.Choice == nil
	if got.Choice != nil && want.Choice != nil {
		choiceMatches = *got.Choice == *want.Choice
	}
	if !choiceMatches || got.Text != want.Text || got.Fallback != want.Fallback {
		t.Fatalf("clarification answer = %#v, want %#v", got, want)
	}
}

func (h *referenceHarness) listSessionClarificationsHTTP(
	t *testing.T,
	session cli.SessionRecord,
) cli.ClarificationsRecord {
	t.Helper()
	path := "/api/workspaces/" + url.PathEscape(session.WorkspaceID) +
		"/sessions/" + url.PathEscape(session.ID) + "/clarifications"
	var response cli.ClarificationsRecord
	h.referenceHTTPJSON(t, http.MethodGet, path, nil, &response)
	return response
}

func (h *referenceHarness) answerSessionClarificationHTTP(
	t *testing.T,
	session cli.SessionRecord,
	requestID string,
	request cli.ClarificationAnswerRequest,
) cli.ClarificationAnswerRecord {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(clarification answer) error = %v", err)
	}
	path := "/api/workspaces/" + url.PathEscape(session.WorkspaceID) +
		"/sessions/" + url.PathEscape(session.ID) + "/clarifications/" +
		url.PathEscape(requestID) + "/answer"
	var response cli.ClarificationAnswerRecord
	h.referenceHTTPJSON(t, http.MethodPost, path, payload, &response)
	return response
}

func (h *referenceHarness) referenceHTTPJSON(
	t *testing.T,
	method string,
	path string,
	payload []byte,
	target any,
) {
	t.Helper()
	body := io.Reader(http.NoBody)
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, h.httpBaseURL+path, body)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(%s %s) error = %v", method, path, err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("HTTP %s %s error = %v", method, path, err)
	}
	data, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("HTTP %s %s read response error = %v", method, path, err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %s %s status = %d, want %d; body=%s", method, path, response.StatusCode, http.StatusOK, data)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("HTTP %s %s decode response error = %v", method, path, err)
	}
}

func (h *referenceHarness) promptSession(
	t *testing.T,
	sessionID string,
	message string,
) ([]cli.AgentEventRecord, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return h.client.PromptSession(ctx, sessionID, message)
}

func (h *referenceHarness) waitForPromptEntries(t *testing.T, want int) []referencePromptLogEntry {
	t.Helper()

	var entries []referencePromptLogEntry
	waitForCondition(t, 10*time.Second, "prompt log entries", func() bool {
		payload, err := os.ReadFile(h.promptLogPath)
		if err != nil {
			return false
		}
		decoded, decodeErr := decodeJSONLines[referencePromptLogEntry](payload)
		if decodeErr != nil {
			return false
		}
		entries = decoded
		return len(entries) >= want
	})
	return entries
}

func (h *referenceHarness) ensurePromptEntryCount(t *testing.T, want int, duration time.Duration) {
	t.Helper()

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		payload, err := os.ReadFile(h.promptLogPath)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		entries, decodeErr := decodeJSONLines[referencePromptLogEntry](payload)
		if decodeErr == nil && len(entries) != want {
			t.Fatalf("prompt entry count = %d, want %d", len(entries), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (h *referenceHarness) hookRuns(t *testing.T, sessionID string, last int) []cli.HookRunRecord {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	runs, err := h.client.HookRuns(ctx, h.workspace.WorkspaceID, cli.HookRunsQuery{
		Session: sessionID,
		Last:    last,
	})
	if err != nil {
		t.Fatalf("HookRuns() error = %v", err)
	}
	return runs
}

func (h *referenceHarness) shutdown(t *testing.T) {
	t.Helper()

	if h.daemonCancel == nil {
		return
	}

	h.daemonCancel()
	h.daemonCancel = nil

	if h.daemonErrCh == nil {
		return
	}

	select {
	case err := <-h.daemonErrCh:
		h.daemonErrCh = nil
		if err != nil {
			t.Fatalf("daemon.Run() error = %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for daemon shutdown")
	}
}

func (referenceACPAgent) Authenticate(
	context.Context,
	acpsdk.AuthenticateRequest,
) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (referenceACPAgent) Logout(context.Context, acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

func (referenceACPAgent) Initialize(context.Context, acpsdk.InitializeRequest) (acpsdk.InitializeResponse, error) {
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AgentCapabilities: acpsdk.AgentCapabilities{
			LoadSession: true,
		},
		AuthMethods: []acpsdk.AuthMethod{},
	}, nil
}

func (referenceACPAgent) Cancel(context.Context, acpsdk.CancelNotification) error {
	return nil
}

func (referenceACPAgent) CloseSession(
	context.Context,
	acpsdk.CloseSessionRequest,
) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, nil
}

func (referenceACPAgent) ListSessions(
	context.Context,
	acpsdk.ListSessionsRequest,
) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{}, nil
}

func (referenceACPAgent) NewSession(context.Context, acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	return acpsdk.NewSessionResponse{
		SessionId: "reference-extension-helper",
	}, nil
}

func (referenceACPAgent) LoadSession(context.Context, acpsdk.LoadSessionRequest) (acpsdk.LoadSessionResponse, error) {
	return acpsdk.LoadSessionResponse{}, nil
}

func (referenceACPAgent) ResumeSession(
	context.Context,
	acpsdk.ResumeSessionRequest,
) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, nil
}

func (referenceACPAgent) Prompt(_ context.Context, params acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	entry := referencePromptLogEntry{
		SessionID: string(params.SessionId),
		Text:      promptText(params.Prompt),
	}
	if err := appendJSONLine(os.Getenv(referencePromptLogPathEnvKey), entry); err != nil {
		return acpsdk.PromptResponse{}, err
	}
	return acpsdk.PromptResponse{
		StopReason: acpsdk.StopReasonEndTurn,
	}, nil
}

func (referenceACPAgent) SetSessionMode(
	context.Context,
	acpsdk.SetSessionModeRequest,
) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

func (referenceACPAgent) SetSessionConfigOption(
	context.Context,
	acpsdk.SetSessionConfigOptionRequest,
) (acpsdk.SetSessionConfigOptionResponse, error) {
	return acpsdk.SetSessionConfigOptionResponse{ConfigOptions: []acpsdk.SessionConfigOption{}}, nil
}

func promptText(blocks []acpsdk.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch {
		case block.Text != nil:
			parts = append(parts, block.Text.Text)
		case block.ResourceLink != nil:
			parts = append(parts, block.ResourceLink.Uri)
		}
	}
	return strings.Join(parts, "\n\n")
}

func appendJSONLine(path string, value any) (err error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "%s\n", payload)
	return err
}

func buildReferenceArtifacts(t *testing.T, repoRoot string) {
	t.Helper()

	runCommand(
		t,
		repoRoot,
		"go",
		"build",
		"-o",
		"./sdk/examples/secret-guard/bin/secret-guard",
		"./sdk/examples/secret-guard",
	)
	runCommand(
		t,
		repoRoot,
		"go",
		"build",
		"-o",
		"./sdk/examples/clarify-tool/bin/clarify-tool",
		"./sdk/examples/clarify-tool",
	)
	runCommand(t, repoRoot, "npm", "run", "build", "--workspace", "@agh/extension-sdk")
	runCommand(t, repoRoot, "npm", "run", "build", "--workspace", "@agh/example-prompt-enhancer")
}

func referencePromptEnhancerInstallSource(t *testing.T, repoRoot string) string {
	t.Helper()

	sourceDir := filepath.Join(repoRoot, "sdk", "examples", "prompt-enhancer")
	targetDir := filepath.Join(t.TempDir(), "prompt-enhancer")

	copyReferenceFile(t, filepath.Join(sourceDir, "extension.toml"), filepath.Join(targetDir, "extension.toml"))
	copyReferenceFile(t, filepath.Join(sourceDir, "package.json"), filepath.Join(targetDir, "package.json"))
	copyReferenceDir(t, filepath.Join(sourceDir, "dist"), filepath.Join(targetDir, "dist"))

	sdkSource := filepath.Join(repoRoot, "sdk", "typescript")
	sdkTarget := filepath.Join(targetDir, "node_modules", "@agh", "extension-sdk")
	copyReferenceFile(t, filepath.Join(sdkSource, "package.json"), filepath.Join(sdkTarget, "package.json"))
	copyReferenceDir(t, filepath.Join(sdkSource, "dist"), filepath.Join(sdkTarget, "dist"))

	return targetDir
}

func copyReferenceDir(t *testing.T, sourceDir string, targetDir string) {
	t.Helper()

	if err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetDir, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected symlink in reference fixture source %q", path)
		}
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFileWithMode(path, targetPath, info.Mode().Perm())
	}); err != nil {
		t.Fatalf("copy reference dir %q to %q: %v", sourceDir, targetDir, err)
	}
}

func copyReferenceFile(t *testing.T, sourcePath string, targetPath string) {
	t.Helper()

	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat reference file %q: %v", sourcePath, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("reference file %q mode = %v, want regular file", sourcePath, info.Mode())
	}
	if err := copyFileWithMode(sourcePath, targetPath, info.Mode().Perm()); err != nil {
		t.Fatalf("copy reference file %q to %q: %v", sourcePath, targetPath, err)
	}
}

func copyFileWithMode(sourcePath string, targetPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	payload, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, payload, mode)
}

func runCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s error = %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
}

func referenceRepoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func referenceHomePaths(t *testing.T) aghconfig.HomePaths {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("AGH_HOME", homeDir)
	t.Setenv("HOME", homeDir)

	homePaths, err := aghconfig.ResolveHomePathsFrom(homeDir)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	homePaths.DaemonSocket = referenceShortSocketPath(t)
	return homePaths
}

func referenceConfig(t *testing.T, homePaths aghconfig.HomePaths, command string) aghconfig.Config {
	t.Helper()

	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = referenceFreeTCPPort(t)
	cfg.Daemon.Socket = homePaths.DaemonSocket
	cfg.Defaults.Agent = "coder"
	cfg.Defaults.Provider = acpmock.ProviderName
	cfg.Memory.Enabled = false
	cfg.Skills.Enabled = false
	cfg.Extensions.Marketplace.AllowUnverified = true
	cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)
	return cfg
}

func referenceSeedWorkspace(
	t *testing.T,
	homePaths aghconfig.HomePaths,
	cfg aghconfig.Config,
	root string,
) workspacepkg.ResolvedWorkspace {
	t.Helper()

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", root, err)
	}

	db, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	defer func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	resolver, err := workspacepkg.NewResolver(
		db,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		workspacepkg.WithConfigLoader(func(string) (aghconfig.Config, error) {
			return cfg, nil
		}),
	)
	if err != nil {
		t.Fatalf("workspace.NewResolver() error = %v", err)
	}

	resolved, err := resolver.ResolveOrRegister(testutil.Context(t), root)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", root, err)
	}

	if resolved.Config.Defaults.Agent != cfg.Defaults.Agent {
		t.Fatalf("resolved default agent = %q, want %q", resolved.Config.Defaults.Agent, cfg.Defaults.Agent)
	}
	return resolved
}

func referenceACPHelperCommand(t *testing.T) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	return shellquote.Join(
		"env",
		referenceACPHelperEnvKey+"=1",
		referencePromptLogPathEnvKey+"="+os.Getenv(referencePromptLogPathEnvKey),
		bin,
		"-test.run=TestReferenceExtensionACPHelperProcess",
	)
}

func referenceWriteAgentDef(t *testing.T, homePaths aghconfig.HomePaths, name string, command string) {
	t.Helper()

	path := filepath.Join(homePaths.AgentsDir, name, "AGENT.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}

	content := strings.Join([]string{
		"---",
		"name: " + name,
		"provider: " + acpmock.ProviderName,
		"command: " + command,
		"tools: [ext__clarify_tool__ask]",
		"---",
		"You are a coding assistant.",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func referenceWriteProviderConfig(
	t *testing.T,
	homePaths aghconfig.HomePaths,
	providerName string,
	command string,
) {
	t.Helper()

	content := strings.Join([]string{
		"[defaults]",
		"agent = \"coder\"",
		"provider = " + fmt.Sprintf("%q", providerName),
		"",
		"[providers." + providerName + "]",
		"command = " + fmt.Sprintf("%q", command),
		"harness = \"acp\"",
		"auth_mode = \"none\"",
		"none_security = \"local_transport\"",
		"",
	}, "\n")
	if err := os.WriteFile(homePaths.ConfigFile, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", homePaths.ConfigFile, err)
	}
}

func referenceShortSocketPath(t *testing.T) string {
	t.Helper()

	path := filepath.Join(os.TempDir(), fmt.Sprintf("agh-reference-%d.sock", time.Now().UTC().UnixNano()))
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Remove(%q) cleanup error = %v", path, err)
		}
	})
	return path
}

func referenceFreeTCPPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Fatalf("listener Close() cleanup error = %v", err)
		}
	}()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener addr type = %T, want *net.TCPAddr", ln.Addr())
	}
	return addr.Port
}

func waitForJSONFile[T any](t *testing.T, path string, timeout time.Duration) T {
	t.Helper()

	var decoded T
	waitForCondition(t, timeout, "json file "+path, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return false
		}
		return true
	})
	return decoded
}

func waitForHandshakeMarker(t *testing.T, path string, pid int, timeout time.Duration) referenceHandshakeMarker {
	t.Helper()

	var marker referenceHandshakeMarker
	waitForCondition(t, timeout, fmt.Sprintf("handshake marker %s for pid=%d", path, pid), func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		if err := json.Unmarshal(payload, &marker); err != nil {
			return false
		}
		return marker.PID == pid
	})
	return marker
}

func waitForHostCallMarker(t *testing.T, path string, pid int, timeout time.Duration) referenceHostCallMarker {
	t.Helper()

	var marker referenceHostCallMarker
	waitForCondition(t, timeout, fmt.Sprintf("host call marker %s for pid=%d", path, pid), func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		if err := json.Unmarshal(payload, &marker); err != nil {
			return false
		}
		return marker.PID == pid
	})
	return marker
}

func waitForStartedPID(t *testing.T, path string, wantCount int, timeout time.Duration) int {
	t.Helper()

	var latest int
	waitForCondition(t, timeout, fmt.Sprintf("started pid %s count>=%d", path, wantCount), func() bool {
		lines, err := readFileLines(path)
		if err != nil || len(lines) < wantCount {
			return false
		}
		pid, ok := parsePIDLine(lines[len(lines)-1])
		if !ok {
			return false
		}
		latest = pid
		return latest > 0
	})
	return latest
}

func parsePIDLine(line string) (int, bool) {
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(line), "pid=%d", &pid); err != nil {
		return 0, false
	}
	return pid, pid > 0
}

func waitForNonEmptyFileLines(t *testing.T, path string, timeout time.Duration) []string {
	t.Helper()

	var lines []string
	waitForCondition(t, timeout, "non-empty file lines "+path, func() bool {
		current, err := readFileLines(path)
		if err != nil || len(current) == 0 {
			return false
		}
		lines = current
		return true
	})
	return lines
}

func readFileLines(path string) ([]string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(string(payload)), nil
}

func waitForCondition(t *testing.T, timeout time.Duration, label string, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s after %v", label, timeout)
}

func hookCatalogContains(hooks []cli.HookCatalogRecord, name string, event string) bool {
	for _, item := range hooks {
		if item.Name == name && item.Event == event && item.ExecutorKind == "subprocess" {
			return true
		}
	}
	return false
}

func hookRunContains(runs []cli.HookRunRecord, event string, outcome string, fragment string) bool {
	for _, item := range runs {
		if item.Event != event || item.Outcome != outcome {
			continue
		}
		if fragment == "" || strings.Contains(string(item.PatchApplied), fragment) ||
			strings.Contains(item.Error, fragment) {
			return true
		}
	}
	return false
}
