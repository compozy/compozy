package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kballard/go-shellquote"

	acpsdk "github.com/coder/acp-go-sdk"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/toolruntime"
)

const (
	testHelperEnvKey      = "AGH_TEST_ACP_HELPER"
	testHelperScenarioKey = "AGH_TEST_ACP_SCENARIO"
	testHelperFileKey     = "AGH_TEST_ACP_FILE"
	testHelperCaptureKey  = "AGH_TEST_ACP_CAPTURE_FILE"
	testWrapperEnvKey     = "AGH_TEST_ACP_WRAPPER"
)

func TestACPHelperProcess(_ *testing.T) {
	if os.Getenv(testHelperEnvKey) != "1" {
		return
	}

	agent := &helperACPAgent{
		scenario: os.Getenv(testHelperScenarioKey),
		filePath: os.Getenv(testHelperFileKey),
	}
	input := io.Reader(os.Stdin)
	capturePath := strings.TrimSpace(os.Getenv(testHelperCaptureKey))
	if capturePath != "" {
		captureFile, err := os.Create(capturePath)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "create capture file: %v\n", err)
			os.Exit(1)
		}
		input = io.TeeReader(os.Stdin, captureFile)

		conn := acpsdk.NewAgentSideConnection(agent, os.Stdout, input)
		agent.conn = conn
		<-conn.Done()
		if err := captureFile.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close capture file: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	conn := acpsdk.NewAgentSideConnection(agent, os.Stdout, input)
	agent.conn = conn
	<-conn.Done()
	os.Exit(0)
}

func TestACPWrapperProcess(_ *testing.T) {
	if os.Getenv(testWrapperEnvKey) != "1" {
		return
	}

	bin, err := os.Executable()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "resolve test binary: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.CommandContext(context.Background(), bin, "-test.run=TestACPHelperProcess")
	cmd.Env = append([]string(nil), os.Environ()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "start wrapped helper: %v\n", err)
		os.Exit(1)
	}

	if pidFile := strings.TrimSpace(os.Getenv(testWrapperPIDFileEnvKey)); pidFile != "" {
		if writeErr := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); writeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "write pid file: %v\n", writeErr)
			_ = cmd.Process.Kill()
			os.Exit(1)
		}
	}

	if err := cmd.Wait(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "wrapped helper exited: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func TestParseCommandString(t *testing.T) {
	t.Parallel()

	command, args, err := parseCommandString(`npx -y "agent client" --flag='hello world'`)
	if err != nil {
		t.Fatalf("parseCommandString() error = %v", err)
	}
	if command != "npx" {
		t.Fatalf("parseCommandString() command = %q, want %q", command, "npx")
	}
	wantArgs := []string{"-y", "agent client", "--flag=hello world"}
	if !slices.Equal(args, wantArgs) {
		t.Fatalf("parseCommandString() args = %#v, want %#v", args, wantArgs)
	}
}

func TestPermissionPolicyModes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	policies := map[string]struct {
		mode       aghconfig.PermissionMode
		readOK     bool
		writeOK    bool
		terminalOK bool
	}{
		"deny-all": {
			mode:       aghconfig.PermissionModeDenyAll,
			readOK:     false,
			writeOK:    false,
			terminalOK: false,
		},
		"approve-reads": {
			mode:       aghconfig.PermissionModeApproveReads,
			readOK:     true,
			writeOK:    false,
			terminalOK: false,
		},
		"approve-all": {
			mode:       aghconfig.PermissionModeApproveAll,
			readOK:     true,
			writeOK:    true,
			terminalOK: true,
		},
	}

	for name, tc := range policies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			policy, err := newPermissionPolicy(tc.mode, root)
			if err != nil {
				t.Fatalf("newPermissionPolicy() error = %v", err)
			}

			assertPermissionResult(t, policy.authorize(permissionReadTextFile), tc.readOK)
			assertPermissionResult(t, policy.authorize(permissionWriteTextFile), tc.writeOK)
			assertPermissionResult(t, policy.authorize(permissionCreateTerminal), tc.terminalOK)
		})
	}
}

func TestPermissionPolicyResolvePathSandbox(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	policy, err := newPermissionPolicy(aghconfig.PermissionModeApproveAll, root)
	if err != nil {
		t.Fatalf("newPermissionPolicy() error = %v", err)
	}

	insideFile := filepath.Join(root, "nested", "file.txt")
	resolvedInside, err := policy.resolvePath(insideFile)
	if err != nil {
		t.Fatalf("resolvePath(%q) error = %v", insideFile, err)
	}
	if !strings.HasSuffix(resolvedInside, filepath.Join("nested", "file.txt")) {
		t.Fatalf(
			"resolvePath(%q) = %q, want suffix %q",
			insideFile,
			resolvedInside,
			filepath.Join("nested", "file.txt"),
		)
	}

	if _, err := policy.resolvePath(filepath.Join(root, "..", "escape.txt")); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("resolvePath(outside) error = %v, want ErrPathOutsideWorkspace", err)
	}
}

func TestTokenUsageParsing(t *testing.T) {
	t.Parallel()

	inputTokens := int64(10)
	outputTokens := int64(12)
	totalTokens := int64(22)
	thoughtTokens := int64(3)
	cacheReadTokens := int64(4)
	cacheWriteTokens := int64(5)
	used := int64(80)
	size := int64(100)
	amount := 1.25
	currency := "USD"

	promptUsage := tokenUsageFromPromptResponse("turn-1", &wireUsage{
		InputTokens:      &inputTokens,
		OutputTokens:     &outputTokens,
		TotalTokens:      &totalTokens,
		ThoughtTokens:    &thoughtTokens,
		CacheReadTokens:  &cacheReadTokens,
		CacheWriteTokens: &cacheWriteTokens,
	})
	if promptUsage.InputTokens == nil || *promptUsage.InputTokens != inputTokens {
		t.Fatalf("tokenUsageFromPromptResponse() input_tokens = %#v, want %d", promptUsage.InputTokens, inputTokens)
	}
	if promptUsage.CacheWriteTokens == nil || *promptUsage.CacheWriteTokens != cacheWriteTokens {
		t.Fatalf(
			"tokenUsageFromPromptResponse() cache_write_tokens = %#v, want %d",
			promptUsage.CacheWriteTokens,
			cacheWriteTokens,
		)
	}

	merged := promptUsage.Merge(tokenUsageFromUsageUpdate("turn-1", wireUsageUpdate{
		Used: &used,
		Size: &size,
		Cost: &wireCost{
			Amount:   &amount,
			Currency: &currency,
		},
	}))
	if merged.ContextUsed == nil || *merged.ContextUsed != used {
		t.Fatalf("merged.ContextUsed = %#v, want %d", merged.ContextUsed, used)
	}
	if merged.CostCurrency == nil || *merged.CostCurrency != currency {
		t.Fatalf("merged.CostCurrency = %#v, want %q", merged.CostCurrency, currency)
	}

	empty := tokenUsageFromPromptResponse("turn-2", nil)
	if !empty.IsZero() {
		t.Fatalf("tokenUsageFromPromptResponse(nil) should be zero, got %#v", empty)
	}
}

func TestDriverRejectsUninitializedProcessState(t *testing.T) {
	t.Parallel()

	driver := New()

	t.Run("Should prompt requires connection", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{SessionID: "session-1"}
		events, err := driver.Prompt(context.Background(), proc, PromptRequest{
			TurnID:  "turn-1",
			Message: "hello",
		})
		if err == nil {
			t.Fatalf("Prompt() error = nil, want %v", errProcessConnectionUninitialized)
		}
		if !errors.Is(err, errProcessConnectionUninitialized) {
			t.Fatalf("Prompt() error = %v, want %v", err, errProcessConnectionUninitialized)
		}
		if events != nil {
			t.Fatalf("Prompt() events = %v, want nil", events)
		}
	})

	t.Run("Should cancel requires connection and does not panic", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{SessionID: "session-1"}
		var (
			err    error
			panicV any
		)
		func() {
			defer func() {
				panicV = recover()
			}()
			err = driver.Cancel(context.Background(), proc)
		}()

		if panicV != nil {
			t.Fatalf("Cancel() panicked: %v", panicV)
		}
		if !errors.Is(err, errProcessConnectionUninitialized) {
			t.Fatalf("Cancel() error = %v, want %v", err, errProcessConnectionUninitialized)
		}
	})

	t.Run("Should stop requires lifecycle", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		err := driver.Stop(ctx, &AgentProcess{})
		if err == nil {
			t.Fatalf("Stop() error = nil, want %v", errProcessLifecycleUninitialized)
		}
		if !errors.Is(err, errProcessLifecycleUninitialized) {
			t.Fatalf("Stop() error = %v, want %v", err, errProcessLifecycleUninitialized)
		}
	})
}

func TestPromptPrependsSystemPromptOnce(t *testing.T) {
	t.Parallel()

	driver := New()
	proc := startHelperProcess(t, driver, "echo_prompt", "", StartOpts{
		SystemPrompt: "Memory context first.\nThen agent prompt.",
	})
	defer stopProcess(t, driver, proc)

	firstEventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
		TurnID:  "turn-1",
		Message: "first request",
	})
	if err != nil {
		t.Fatalf("Prompt(first) error = %v", err)
	}
	firstEvents := collectEvents(t, firstEventsCh)
	if len(firstEvents) == 0 {
		t.Fatal("Prompt(first) returned no events")
	}
	if !strings.Contains(firstEvents[0].Text, "Session instructions") {
		t.Fatalf("first prompt text = %q, want injected system prompt prefix", firstEvents[0].Text)
	}
	if !strings.Contains(firstEvents[0].Text, "Memory context first.\nThen agent prompt.") {
		t.Fatalf("first prompt text = %q, want system prompt content", firstEvents[0].Text)
	}
	if !strings.Contains(firstEvents[0].Text, "User request:\n\nfirst request") {
		t.Fatalf("first prompt text = %q, want user request content", firstEvents[0].Text)
	}

	secondEventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
		TurnID:  "turn-2",
		Message: "second request",
	})
	if err != nil {
		t.Fatalf("Prompt(second) error = %v", err)
	}
	secondEvents := collectEvents(t, secondEventsCh)
	if len(secondEvents) == 0 {
		t.Fatal("Prompt(second) returned no events")
	}
	if secondEvents[0].Text != "second request" {
		t.Fatalf("second prompt text = %q, want plain user request", secondEvents[0].Text)
	}
}

func TestPromptAttachesSystemPromptDeliveryMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should annotate first-turn system prompt fallback", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "echo_prompt_meta", "", StartOpts{
			SystemPrompt: "AGH runtime envelope.",
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-system-meta",
			Message: "first request",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() returned no events")
		}

		var payload PromptMeta
		if err := json.Unmarshal([]byte(events[0].Text), &payload); err != nil {
			t.Fatalf("json.Unmarshal(prompt meta echo) error = %v", err)
		}
		if payload.System == nil {
			t.Fatal("payload.System = nil, want system prompt delivery metadata")
		}
		if got, want := payload.System.PromptDelivery, string(SystemPromptDeliveryFirstTurnPrefix); got != want {
			t.Fatalf("payload.System.PromptDelivery = %q, want %q", got, want)
		}
	})

	t.Run("Should annotate native system prompt delivery without fallback prefix", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "echo_prompt_meta", "", StartOpts{
			SystemPrompt:         "AGH runtime envelope.",
			SystemPromptDelivery: SystemPromptDeliveryNative,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-system-native",
			Message: "first request",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() returned no events")
		}

		var payload PromptMeta
		if err := json.Unmarshal([]byte(events[0].Text), &payload); err != nil {
			t.Fatalf("json.Unmarshal(prompt meta echo) error = %v", err)
		}
		if payload.System == nil {
			t.Fatal("payload.System = nil, want system prompt delivery metadata")
		}
		if got, want := payload.System.PromptDelivery, string(SystemPromptDeliveryNative); got != want {
			t.Fatalf("payload.System.PromptDelivery = %q, want %q", got, want)
		}
	})
}

func TestPromptCacheControlForStartOpts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		opts    StartOpts
		want    bool
		wantTTL string
	}{
		{
			name: "Should skip unsupported provider",
			opts: StartOpts{ProviderName: "codex"},
		},
		{
			name: "Should enable short-lived cache control for Claude provider",
			opts: StartOpts{ProviderName: "claude"},
			want: true,
		},
		{
			name: "Should enable long-lived cache control for Anthropic endpoint",
			opts: StartOpts{
				ProviderName: "pi",
				ProviderConfig: &aghconfig.ProviderConfig{
					RuntimeProvider: "anthropic",
					BaseURL:         "https://api.anthropic.com/v1",
				},
			},
			want:    true,
			wantTTL: "1h",
		},
		{
			name: "Should enable long-lived cache control for Vertex Anthropic endpoint",
			opts: StartOpts{
				ProviderName: "pi",
				ProviderConfig: &aghconfig.ProviderConfig{
					RuntimeProvider: "anthropic",
					BaseURL:         "https://us-east5-aiplatform.googleapis.com/v1",
				},
			},
			want:    true,
			wantTTL: "1h",
		},
		{
			name: "Should skip OpenRouter even when a base URL is present",
			opts: StartOpts{
				ProviderName: "openrouter",
				ProviderConfig: &aghconfig.ProviderConfig{
					RuntimeProvider: "openrouter",
					BaseURL:         "https://openrouter.ai/api/v1",
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := promptCacheControlForStartOpts(tc.opts)
			if !tc.want {
				if got != nil {
					t.Fatalf("promptCacheControlForStartOpts() = %#v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("promptCacheControlForStartOpts() = nil, want cache control")
			}
			if got.Type != "ephemeral" {
				t.Fatalf("cache control type = %q, want ephemeral", got.Type)
			}
			if got.TTL != tc.wantTTL {
				t.Fatalf("cache control TTL = %q, want %q", got.TTL, tc.wantTTL)
			}
		})
	}
}

func TestBuildWirePromptRequestAttachesPromptCacheControlMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should annotate text content without changing prompt text", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{
			SessionID: "sess-cache",
			promptCacheControl: &promptCacheControl{
				Type: "ephemeral",
				TTL:  "1h",
			},
		}
		request, err := buildWirePromptRequest(proc, PromptRequest{
			TurnID:  "turn-cache",
			Message: "hello cache",
		})
		if err != nil {
			t.Fatalf("buildWirePromptRequest() error = %v", err)
		}
		if got, want := len(request.Prompt), 1; got != want {
			t.Fatalf("len(Prompt) = %d, want %d", got, want)
		}
		text := request.Prompt[0].Text
		if text == nil {
			t.Fatal("Prompt[0].Text = nil, want text block")
		}
		if got, want := text.Text, "hello cache"; got != want {
			t.Fatalf("Prompt[0].Text.Text = %q, want %q", got, want)
		}
		cacheControl, ok := text.Meta["cache_control"].(map[string]any)
		if !ok {
			t.Fatalf("Prompt[0].Text.Meta = %#v, want cache_control map", text.Meta)
		}
		if got, want := cacheControl["type"], "ephemeral"; got != want {
			t.Fatalf("cache_control.type = %#v, want %q", got, want)
		}
		if got, want := cacheControl["ttl"], "1h"; got != want {
			t.Fatalf("cache_control.ttl = %#v, want %q", got, want)
		}
	})

	t.Run("Should leave text content metadata empty when provider is unsupported", func(t *testing.T) {
		t.Parallel()

		request, err := buildWirePromptRequest(&AgentProcess{SessionID: "sess-cache"}, PromptRequest{
			TurnID:  "turn-cache",
			Message: "hello cache",
		})
		if err != nil {
			t.Fatalf("buildWirePromptRequest() error = %v", err)
		}
		if request.Prompt[0].Text == nil {
			t.Fatal("Prompt[0].Text = nil, want text block")
		}
		if len(request.Prompt[0].Text.Meta) != 0 {
			t.Fatalf("Prompt[0].Text.Meta = %#v, want empty metadata", request.Prompt[0].Text.Meta)
		}
	})
}

func TestPromptSkipsFirstTurnPrefixForNativeSystemPromptDelivery(t *testing.T) {
	t.Parallel()

	t.Run("Should send plain user request when system prompt is native", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "echo_prompt", "", StartOpts{
			SystemPrompt:         "AGH runtime envelope.",
			SystemPromptDelivery: SystemPromptDeliveryNative,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-native-text",
			Message: "first request",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() returned no events")
		}
		if got, want := events[0].Text, "first request"; got != want {
			t.Fatalf("first prompt text = %q, want %q", got, want)
		}
		if strings.Contains(events[0].Text, "AGH runtime envelope.") ||
			strings.Contains(events[0].Text, "Session instructions") {
			t.Fatalf("first prompt text = %q, want no fallback prefix", events[0].Text)
		}
	})
}

func TestPromptActivityReporterReportsWhilePromptIsInFlight(t *testing.T) {
	t.Run("ShouldReportWhilePromptIsInFlight", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()

		reports := make(chan PromptActivityReport, 4)
		stop := startPromptActivityReporter(ctx, PromptRequest{
			TurnID:                    "turn-reporter",
			Message:                   "hello",
			ActivityHeartbeatInterval: 5 * time.Millisecond,
			ActivityReporter: func(report PromptActivityReport) {
				select {
				case reports <- report:
				default:
				}
			},
		})
		defer stop()

		first := readPromptActivityReport(t, reports)
		if got, want := first.Kind, "agent_waiting"; got != want {
			t.Fatalf("first report kind = %q, want %q", got, want)
		}
		if first.Timestamp.IsZero() {
			t.Fatal("first report timestamp is zero")
		}

		second := readPromptActivityReport(t, reports)
		if got, want := second.Kind, "agent_waiting"; got != want {
			t.Fatalf("second report kind = %q, want %q", got, want)
		}
		if second.Timestamp.Before(first.Timestamp) {
			t.Fatalf("second report timestamp %s before first %s", second.Timestamp, first.Timestamp)
		}
	})
}

func readPromptActivityReport(t *testing.T, reports <-chan PromptActivityReport) PromptActivityReport {
	t.Helper()

	select {
	case report := <-reports:
		return report
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for prompt activity report")
	}
	return PromptActivityReport{}
}

func TestPromptTransmitsStructuredMetadata(t *testing.T) {
	t.Parallel()

	driver := New()
	proc := startHelperProcess(t, driver, "echo_prompt_meta", "", StartOpts{})
	defer stopProcess(t, driver, proc)

	eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
		TurnID:  "turn-meta",
		Message: "network delivery",
		Meta: PromptMeta{
			TurnSource: PromptTurnSourceNetwork,
			Network: &PromptNetworkMeta{
				MessageID:   "msg-meta-1",
				Kind:        "say",
				Channel:     "builders",
				Surface:     "direct",
				DirectID:    "direct_meta_1",
				From:        "ops.peer",
				To:          "worker.peer",
				WorkID:      "work-meta-1",
				ReplyTo:     "msg-root-1",
				TraceID:     "trace-meta-1",
				CausationID: "msg-root-1",
				Trust:       "untrusted",
			},
		},
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}

	events := collectEvents(t, eventsCh)
	if len(events) == 0 {
		t.Fatal("Prompt() returned no events")
	}

	var payload PromptMeta
	if err := json.Unmarshal([]byte(events[0].Text), &payload); err != nil {
		t.Fatalf("json.Unmarshal(prompt meta echo) error = %v", err)
	}
	if got, want := payload.TurnSource, PromptTurnSourceNetwork; got != want {
		t.Fatalf("payload.TurnSource = %q, want %q", got, want)
	}
	if payload.Network == nil {
		t.Fatal("payload.Network = nil, want populated network metadata")
	}
	if got, want := payload.Network.MessageID, "msg-meta-1"; got != want {
		t.Fatalf("payload.Network.MessageID = %q, want %q", got, want)
	}
	if got, want := payload.Network.Surface, "direct"; got != want {
		t.Fatalf("payload.Network.Surface = %q, want %q", got, want)
	}
	if got, want := payload.Network.DirectID, "direct_meta_1"; got != want {
		t.Fatalf("payload.Network.DirectID = %q, want %q", got, want)
	}
	if got, want := payload.Network.WorkID, "work-meta-1"; got != want {
		t.Fatalf("payload.Network.WorkID = %q, want %q", got, want)
	}
	if got, want := payload.Network.Trust, "untrusted"; got != want {
		t.Fatalf("payload.Network.Trust = %q, want %q", got, want)
	}
}

func TestDaemonMatchedEnvPinsCurrentBinary(t *testing.T) {
	t.Parallel()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(
		executable,
	); resolveErr == nil &&
		strings.TrimSpace(resolved) != "" {
		executable = resolved
	}
	binDir := filepath.Dir(executable)

	env := daemonMatchedEnv([]string{
		"PATH=/should-be-ignored",
		"FOO=bar",
		"AGH_BIN=/should-be-replaced",
		"PATH=/usr/local/bin" + string(os.PathListSeparator) + binDir + string(os.PathListSeparator) + "/usr/bin",
		"AGH_BIN=/should-also-be-replaced",
	})

	gotAGHBin, ok := envValue(env, "AGH_BIN")
	if !ok || gotAGHBin != executable {
		t.Fatalf("daemonMatchedEnv() AGH_BIN = %q, %v, want %q", gotAGHBin, ok, executable)
	}

	gotPath, ok := envValue(env, "PATH")
	if !ok {
		t.Fatal("daemonMatchedEnv() PATH missing")
	}
	wantPath := binDir + string(os.PathListSeparator) + "/usr/local/bin" + string(os.PathListSeparator) + "/usr/bin"
	if gotPath != wantPath {
		t.Fatalf("daemonMatchedEnv() PATH = %q, want %q", gotPath, wantPath)
	}

	pathCount := 0
	aghBinCount := 0
	for _, variable := range env {
		switch {
		case strings.HasPrefix(variable, "PATH="):
			pathCount++
		case strings.HasPrefix(variable, "AGH_BIN="):
			aghBinCount++
		}
	}
	if pathCount != 1 || aghBinCount != 1 {
		t.Fatalf("daemonMatchedEnv() duplicate entries remain: PATH=%d AGH_BIN=%d env=%#v", pathCount, aghBinCount, env)
	}
}

func TestPromptStreamsSessionUpdates(t *testing.T) {
	t.Parallel()

	t.Run("Should stream session updates and refresh session metadata from prompt events", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "stream_updates", "", StartOpts{})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-stream",
			Message: "hello",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() returned no events")
		}

		var eventTypes []string
		for _, event := range events {
			eventTypes = append(eventTypes, event.Type)
		}
		if !slices.Contains(eventTypes, EventTypeAgentMessage) {
			t.Fatalf("Prompt() event types = %#v, want agent message", eventTypes)
		}
		if !slices.Contains(eventTypes, EventTypeThought) {
			t.Fatalf("Prompt() event types = %#v, want thought", eventTypes)
		}
		if !slices.Contains(eventTypes, EventTypeToolCall) {
			t.Fatalf("Prompt() event types = %#v, want tool call", eventTypes)
		}
		if !slices.Contains(eventTypes, EventTypeDone) {
			t.Fatalf("Prompt() event types = %#v, want done", eventTypes)
		}
		for _, event := range events {
			if event.Type == EventTypeDone && event.PromptStopReason != PromptStopReasonEndTurn {
				t.Fatalf("Prompt() stop reason = %q, want %q", event.PromptStopReason, PromptStopReasonEndTurn)
			}
		}
		if proc.SessionID != "sess-new" {
			t.Fatalf("Start() session id = %q, want %q", proc.SessionID, "sess-new")
		}
		if !slices.Equal(proc.Caps.SupportedModes, []string{"new-mode"}) {
			t.Fatalf("Start() supported modes = %#v, want %#v", proc.Caps.SupportedModes, []string{"new-mode"})
		}
	})
}

func TestStartResumeUsesLoadSession(t *testing.T) {
	t.Parallel()

	t.Run("Should resume through loadSession when a prior session ID is provided", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "load_session", "", StartOpts{
			ResumeSessionID: "sess-existing",
		})
		defer stopProcess(t, driver, proc)

		if proc.SessionID != "sess-existing" {
			t.Fatalf("Start() session id = %q, want %q", proc.SessionID, "sess-existing")
		}
		if !proc.Caps.SupportsLoadSession {
			t.Fatal("Start() SupportsLoadSession = false, want true")
		}
		if !slices.Equal(proc.Caps.SupportedModes, []string{"loaded-mode"}) {
			t.Fatalf("Start() supported modes = %#v, want %#v", proc.Caps.SupportedModes, []string{"loaded-mode"})
		}
	})
}

func TestStartApproveAllSetsPermissiveSessionModeWhenSupported(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-mode-new.jsonl")
	proc := startHelperProcess(t, driver, "mode_mapping", "", StartOpts{
		Permissions: aghconfig.PermissionModeApproveAll,
		Env:         helperEnvWithCapture("mode_mapping", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
	request := decodeCapturedSetSessionModeRequest(t, params)
	if got, want := request.SessionID, "sess-new"; got != want {
		t.Fatalf("set-mode session id = %q, want %q", got, want)
	}
	if got, want := request.ModeID, "bypassPermissions"; got != want {
		t.Fatalf("set-mode mode id = %q, want %q", got, want)
	}
}

func TestStartWithToolGatewayPreservesPermissiveSessionMode(t *testing.T) {
	t.Parallel()

	t.Run("Should keep approve-all semantics when tool execution is intercepted", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-set-mode-gateway.jsonl")
		proc := startHelperProcess(t, driver, "mode_mapping", "", StartOpts{
			Permissions: aghconfig.PermissionModeApproveAll,
			Env:         helperEnvWithCapture("mode_mapping", "", captureFile),
			ToolGateway: toolExecutionGatewayFunc(
				func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
					return req, nil
				},
			),
		})
		defer stopProcess(t, driver, proc)

		params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
		request := decodeCapturedSetSessionModeRequest(t, params)
		if got, want := request.SessionID, "sess-new"; got != want {
			t.Fatalf("set-mode session id = %q, want %q", got, want)
		}
		if got, want := request.ModeID, "bypassPermissions"; got != want {
			t.Fatalf("set-mode mode id = %q, want %q", got, want)
		}
	})

	t.Run("Should select Cursor agent mode and expose it as current", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-set-mode-cursor.jsonl")
		proc := startHelperProcess(t, driver, "cursor_mode_mapping", "", StartOpts{
			Permissions: aghconfig.PermissionModeApproveAll,
			Env:         helperEnvWithCapture("cursor_mode_mapping", "", captureFile),
			ToolGateway: toolExecutionGatewayFunc(
				func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
					return req, nil
				},
			),
		})
		defer stopProcess(t, driver, proc)

		request := decodeCapturedSetSessionModeRequest(
			t,
			captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode),
		)
		if got, want := request.ModeID, "agent"; got != want {
			t.Fatalf("set-mode mode id = %q, want %q", got, want)
		}
		assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "mode", "agent", "agent", "plan", "ask")
	})

	t.Run("Should avoid changing Cursor when its native mode is already agent", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-current-agent-cursor.jsonl")
		proc := startHelperProcess(t, driver, "cursor_mode_current_agent", "", StartOpts{
			Permissions: aghconfig.PermissionModeApproveAll,
			Env:         helperEnvWithCapture("cursor_mode_current_agent", "", captureFile),
			ToolGateway: toolExecutionGatewayFunc(
				func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
					return req, nil
				},
			),
		})
		defer stopProcess(t, driver, proc)

		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetMode) {
			t.Fatal("set_mode was sent when Cursor already reported agent mode")
		}
		assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "mode", "agent", "agent", "plan", "ask")
	})
}

func TestStartResumeApproveReadsSetsReadOnlyLikeSessionModeWhenSupported(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-mode-load.jsonl")
	proc := startHelperProcess(t, driver, "load_mode_mapping", "", StartOpts{
		ResumeSessionID: "sess-existing",
		Permissions:     aghconfig.PermissionModeApproveReads,
		Env:             helperEnvWithCapture("load_mode_mapping", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
	request := decodeCapturedSetSessionModeRequest(t, params)
	if got, want := request.SessionID, "sess-existing"; got != want {
		t.Fatalf("set-mode session id = %q, want %q", got, want)
	}
	if got, want := request.ModeID, "plan"; got != want {
		t.Fatalf("set-mode mode id = %q, want %q", got, want)
	}
}

func TestStartResumeWithToolGatewayPrefersApprovalMediatedMode(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-mode-load-gateway.jsonl")
	proc := startHelperProcess(t, driver, "load_mode_mapping", "", StartOpts{
		ResumeSessionID: "sess-existing",
		Permissions:     aghconfig.PermissionModeApproveReads,
		Env:             helperEnvWithCapture("load_mode_mapping", "", captureFile),
		ToolGateway: toolExecutionGatewayFunc(
			func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
				return req, nil
			},
		),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
	request := decodeCapturedSetSessionModeRequest(t, params)
	if got, want := request.SessionID, "sess-existing"; got != want {
		t.Fatalf("set-mode session id = %q, want %q", got, want)
	}
	if got, want := request.ModeID, "default"; got != want {
		t.Fatalf("set-mode mode id = %q, want %q", got, want)
	}
}

func TestStartDenyAllWithToolGatewayPrefersApprovalMediatedSessionMode(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-mode-deny-gateway.jsonl")
	proc := startHelperProcess(t, driver, "mode_mapping", "", StartOpts{
		Permissions: aghconfig.PermissionModeDenyAll,
		Env:         helperEnvWithCapture("mode_mapping", "", captureFile),
		ToolGateway: toolExecutionGatewayFunc(
			func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
				return req, nil
			},
		),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
	request := decodeCapturedSetSessionModeRequest(t, params)
	if got, want := request.SessionID, "sess-new"; got != want {
		t.Fatalf("set-mode session id = %q, want %q", got, want)
	}
	if got, want := request.ModeID, "default"; got != want {
		t.Fatalf("set-mode mode id = %q, want %q", got, want)
	}
}

func TestStartCapturesSessionConfigOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		scenario      string
		resumeSession string
		wantModel     string
		wantReasoning string
	}{
		{
			name:          "Should capture config options from session new",
			scenario:      "config_options",
			wantModel:     "new-model",
			wantReasoning: "medium",
		},
		{
			name:          "Should capture config options from session load",
			scenario:      "load_config_options",
			resumeSession: "sess-existing",
			wantModel:     "loaded-model",
			wantReasoning: "high",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			driver := New()
			proc := startHelperProcess(t, driver, tc.scenario, "", StartOpts{
				ResumeSessionID: tc.resumeSession,
			})
			defer stopProcess(t, driver, proc)

			caps := proc.CapsSnapshot()
			assertConfigOption(t, caps.ConfigOptions, "model", tc.wantModel, "new-model", "loaded-model", "other-model")
			assertConfigOption(t, caps.ConfigOptions, "reasoning_effort", tc.wantReasoning, "minimal", "high", "xhigh")
		})
	}
}

func TestStartUsesSetConfigOptionForPreferredModelWhenAvailable(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-config-model.jsonl")
	proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
		PreferredModel: "other-model",
		Env:            helperEnvWithCapture("config_options", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	request := decodeCapturedSetSessionConfigOptionRequest(
		t,
		captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
	)
	if got := request.SessionID; got != "sess-new" {
		t.Fatalf("set-config session id = %q, want sess-new", got)
	}
	if got := request.ConfigID; got != "model" {
		t.Fatalf("set-config config id = %q, want model", got)
	}
	if got := request.Value; got != "other-model" {
		t.Fatalf("set-config value = %q, want other-model", got)
	}
	assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "model", "other-model", "other-model")
}

func TestStartHandlesCurrentSessionConfigValues(t *testing.T) {
	t.Parallel()

	t.Run("Should explicitly apply a preferred model that is already current", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-current-model.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			PreferredModel: "new-model",
			Env:            helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		request := decodeCapturedSetSessionConfigOptionRequest(
			t,
			captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
		)
		if request.ConfigID != "model" || request.Value != "new-model" {
			t.Fatalf("set-config request = %#v, want explicit current model", request)
		}
	})

	t.Run("Should skip an explicit reasoning effort that is already current", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-current-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			ReasoningEffort: "medium",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("set_config_option was sent when the requested reasoning effort was already current")
		}
	})
}

func TestStartUsesSetConfigOptionForReasoningEffortWhenAvailable(t *testing.T) {
	t.Parallel()

	t.Run("Should apply reasoning effort through session/set_config_option", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-set-config-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			ReasoningEffort: "high",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		request := decodeCapturedSetSessionConfigOptionRequest(
			t,
			captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
		)
		if got := request.ConfigID; got != "reasoning_effort" {
			t.Fatalf("set-config config id = %q, want reasoning_effort", got)
		}
		if got := request.Value; got != "high" {
			t.Fatalf("set-config value = %q, want high", got)
		}
		assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "reasoning_effort", "high", "high")
	})
}

func TestStartRejectsReasoningWithoutAnApplyStrategyBeforeLaunch(t *testing.T) {
	t.Parallel()

	t.Run("Should return a typed error without launching the provider", func(t *testing.T) {
		t.Parallel()

		launcher := &recordingLauncher{err: errors.New("provider must not launch")}
		driver := New(WithLauncher(launcher))
		proc, err := driver.Start(testutil.Context(t), StartOpts{
			AgentName:       "helper",
			Command:         "helper",
			Cwd:             t.TempDir(),
			Permissions:     aghconfig.PermissionModeApproveAll,
			ReasoningEffort: "high",
			ProviderConfig: &aghconfig.ProviderConfig{Models: aghconfig.ProviderModelsConfig{
				Reasoning: aghconfig.ProviderReasoningConfig{Apply: aghconfig.ReasoningApplyNone},
			}},
		})
		if proc != nil {
			t.Fatal("Start() process != nil, want no launched process")
		}
		var negotiationErr *NegotiationError
		if !errors.As(err, &negotiationErr) || negotiationErr.Code != NegotiationCodeReasoningOptionMissing {
			t.Fatalf("Start() error = %v, want reasoning_option_missing NegotiationError", err)
		}
		if _, called := launcher.lastSpec(); called {
			t.Fatal("provider launcher was called before reasoning strategy validation")
		}
	})
}

func TestStartPassesThroughEveryAdvertisedCanonicalReasoningEffort(t *testing.T) {
	t.Parallel()

	for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
		t.Run("Should pass through "+effort, func(t *testing.T) {
			t.Parallel()

			driver := New()
			captureFile := filepath.Join(t.TempDir(), "session-reasoning-"+effort+".jsonl")
			proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
				ReasoningEffort: effort,
				ProviderConfig:  reasoningACPProviderConfig(),
				Env:             helperEnvWithCapture("config_options", "", captureFile),
			})
			defer stopProcess(t, driver, proc)

			if effort == "medium" {
				if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
					t.Fatal("set_config_option was sent for the already-current reasoning effort")
				}
				assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "reasoning_effort", effort, effort)
				return
			}

			request := decodeCapturedSetSessionConfigOptionRequest(
				t,
				captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
			)
			if got := request.Value; got != effort {
				t.Fatalf("set-config value = %q, want %q", got, effort)
			}
		})
	}
}

func TestStartDistinguishesProviderDefaultFromExplicitNone(t *testing.T) {
	t.Parallel()

	t.Run("Should skip effort RPC for empty provider default", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-default-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			ProviderConfig: reasoningACPProviderConfig(),
			Env:            helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("set_config_option was sent for empty provider-default effort")
		}
	})

	t.Run("Should send explicit none when advertised", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-none-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			ReasoningEffort: "none",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		request := decodeCapturedSetSessionConfigOptionRequest(
			t,
			captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
		)
		if got, want := request.Value, "none"; got != want {
			t.Fatalf("set-config value = %q, want %q", got, want)
		}
	})
}

func TestStartAppliesModelBeforeModelSpecificReasoning(t *testing.T) {
	t.Parallel()

	t.Run("Should apply mode then model then refreshed model-specific effort", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-model-specific-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "model_specific_config_options", "", StartOpts{
			Permissions:     aghconfig.PermissionModeApproveAll,
			PreferredModel:  "other-model",
			ReasoningEffort: "max",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("model_specific_config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)
		got := captureNegotiationSequence(t, captureFile)
		want := []string{"mode:bypassPermissions", "model:other-model", "effort:max"}
		if !slices.Equal(got, want) {
			t.Fatalf("negotiation sequence = %#v, want %#v", got, want)
		}

		requests := captureRequestParamsForMethod(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption)
		if got, want := len(requests), 2; got != want {
			t.Fatalf("set_config_option request count = %d, want %d", got, want)
		}
		modelRequest := decodeCapturedSetSessionConfigOptionRequest(t, requests[0])
		reasoningRequest := decodeCapturedSetSessionConfigOptionRequest(t, requests[1])
		if modelRequest.ConfigID != "model" || modelRequest.Value != "other-model" {
			t.Fatalf("first set-config request = %#v, want model=other-model", modelRequest)
		}
		if reasoningRequest.ConfigID != "effort" || reasoningRequest.Value != "max" {
			t.Fatalf("second set-config request = %#v, want effort=max", reasoningRequest)
		}
	})
}

func TestStartRejectsReasoningEffortWhenConfigOptionIsAbsent(t *testing.T) {
	t.Parallel()

	t.Run("Should fail before prompting when the effort option is absent", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-no-reasoning-config.jsonl")
		proc, err := driver.Start(testutil.Context(t), StartOpts{
			AgentName:       "helper",
			Command:         helperCommand(t),
			Cwd:             t.TempDir(),
			Permissions:     aghconfig.PermissionModeApproveAll,
			ReasoningEffort: "xhigh",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("config_options_no_reasoning", "", captureFile),
		})
		if proc != nil {
			defer stopProcess(t, driver, proc)
		}
		if err == nil {
			t.Fatal("Start() error = nil, want reasoning_option_missing")
		}
		var negotiationErr *NegotiationError
		if !errors.As(err, &negotiationErr) || negotiationErr.Code != NegotiationCodeReasoningOptionMissing {
			t.Fatalf("Start() error = %v, want reasoning_option_missing NegotiationError", err)
		}

		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("set_config_option was sent without a reasoning config option")
		}
	})
}

func TestStartRejectsPreferredModelWhenModelConfigOptionIsAbsent(t *testing.T) {
	t.Parallel()

	t.Run("Should reject preferred model without ACP model config option", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-no-model-config.jsonl")
		proc, err := driver.Start(testutil.Context(t), StartOpts{
			AgentName:      "helper",
			Command:        helperCommand(t),
			Cwd:            t.TempDir(),
			Env:            helperEnvWithCapture("config_options_no_model", "", captureFile),
			Permissions:    aghconfig.PermissionModeApproveAll,
			PreferredModel: "new-model",
		})
		if proc != nil {
			defer stopProcess(t, driver, proc)
		}
		if err == nil {
			t.Fatal("Start() error = nil, want missing model config option error")
		}
		if !errors.Is(err, errModelConfigOptionRequired) {
			t.Fatalf("Start() error = %v, want model config option required", err)
		}
		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("set_config_option was sent when no model config option was available")
		}
	})
}

func TestStartRejectsUnavailableSessionConfigOptionValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		opts            StartOpts
		wantCode        string
		forbiddenMethod string
	}{
		{
			name: "Should reject preferred model absent from model config option values",
			opts: StartOpts{
				PreferredModel: "missing-model",
			},
			wantCode:        NegotiationCodeModelUnavailable,
			forbiddenMethod: acpsdk.AgentMethodSessionSetConfigOption,
		},
		{
			name: "Should reject reasoning effort absent from reasoning config option values",
			opts: StartOpts{
				ReasoningEffort: "turbo",
			},
			wantCode:        NegotiationCodeReasoningEffortUnsupported,
			forbiddenMethod: acpsdk.AgentMethodSessionSetConfigOption,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			driver := New()
			captureFile := filepath.Join(t.TempDir(), "session-unavailable-config-option.jsonl")
			opts := StartOpts{
				AgentName:   "helper",
				Command:     helperCommand(t),
				Cwd:         t.TempDir(),
				Env:         helperEnvWithCapture("config_options", "", captureFile),
				Permissions: aghconfig.PermissionModeApproveAll,
			}
			opts.PreferredModel = tc.opts.PreferredModel
			opts.ReasoningEffort = tc.opts.ReasoningEffort
			if opts.ReasoningEffort != "" {
				opts.ProviderConfig = reasoningACPProviderConfig()
			}
			proc, err := driver.Start(testutil.Context(t), opts)
			if proc != nil {
				defer stopProcess(t, driver, proc)
			}
			if err == nil {
				t.Fatal("Start() error = nil, want unavailable config option error")
			}
			var negotiationErr *NegotiationError
			if !errors.As(err, &negotiationErr) || negotiationErr.Code != tc.wantCode {
				t.Fatalf("Start() error = %v, want NegotiationError code %q", err, tc.wantCode)
			}
			if captureMethodExists(t, captureFile, tc.forbiddenMethod) {
				t.Fatalf("forbidden method %q was sent after unavailable config value", tc.forbiddenMethod)
			}
		})
	}
}

func TestSessionConfigOptionUpdateMutatesCaps(t *testing.T) {
	t.Parallel()

	driver := New()
	proc := startHelperProcess(t, driver, "config_option_update", "", StartOpts{})
	defer stopProcess(t, driver, proc)

	events, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
		TurnID:  "turn-config-options",
		Message: "update config",
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	collectEvents(t, events)

	caps := proc.CapsSnapshot()
	assertConfigOption(t, caps.ConfigOptions, "model", "other-model", "other-model")
	assertConfigOption(t, caps.ConfigOptions, "reasoning_effort", "xhigh", "xhigh")
}

func TestStartWithEmptyAdditionalDirsKeepsBaselinePayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts StartOpts
	}{
		{
			name: "nil additional dirs",
			opts: StartOpts{},
		},
		{
			name: "explicit empty additional dirs",
			opts: StartOpts{AdditionalDirs: []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			driver := New()
			captureFile := filepath.Join(t.TempDir(), strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			opts := tt.opts
			opts.Env = helperEnvWithCapture("stream_updates", "", captureFile)

			proc := startHelperProcess(t, driver, "stream_updates", "", opts)
			defer stopProcess(t, driver, proc)

			params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionNew)
			if _, exists := params["additional_dirs"]; exists {
				t.Fatalf("session/new params include additional_dirs for %s: %#v", tt.name, params)
			}
		})
	}
}

func TestStartIncludesAdditionalDirsInNewSessionPayload(t *testing.T) {
	t.Parallel()

	driver := New()
	root := t.TempDir()
	additionalOne := t.TempDir()
	additionalTwo := t.TempDir()
	captureFile := filepath.Join(t.TempDir(), "session-new.jsonl")

	proc := startHelperProcess(t, driver, "stream_updates", "", StartOpts{
		Cwd:            root,
		AdditionalDirs: []string{additionalOne, additionalTwo},
		Env:            helperEnvWithCapture("stream_updates", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionNew)
	request := decodeCapturedNewSessionRequest(t, params)
	if got, want := request.Cwd, mustCanonicalDir(t, root); got != want {
		t.Fatalf("session/new cwd = %q, want %q", got, want)
	}
	if got, want := request.AdditionalDirs, []string{
		mustCanonicalDir(t, additionalOne),
		mustCanonicalDir(t, additionalTwo),
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("session/new additional_dirs = %#v, want %#v", got, want)
	}
}

func TestStartIncludesAdditionalDirsInLoadSessionPayload(t *testing.T) {
	t.Parallel()

	driver := New()
	root := t.TempDir()
	additionalOne := t.TempDir()
	additionalTwo := t.TempDir()
	captureFile := filepath.Join(t.TempDir(), "session-load.jsonl")

	proc := startHelperProcess(t, driver, "load_session", "", StartOpts{
		Cwd:             root,
		AdditionalDirs:  []string{additionalOne, additionalTwo},
		ResumeSessionID: "sess-existing",
		Env:             helperEnvWithCapture("load_session", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionLoad)
	request := decodeCapturedLoadSessionRequest(t, params)
	if got, want := request.Cwd, mustCanonicalDir(t, root); got != want {
		t.Fatalf("session/load cwd = %q, want %q", got, want)
	}
	if request.SessionID != "sess-existing" {
		t.Fatalf("session/load sessionId = %q, want %q", request.SessionID, "sess-existing")
	}
	if got, want := request.AdditionalDirs, []string{
		mustCanonicalDir(t, additionalOne),
		mustCanonicalDir(t, additionalTwo),
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("session/load additional_dirs = %#v, want %#v", got, want)
	}
}

func TestStartMCPServersSkipsRemoteTransports(t *testing.T) {
	t.Parallel()

	t.Run("Should skip remote transports when starting MCP servers", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-new-mcp.jsonl")
		proc := startHelperProcess(t, driver, "stream_updates", "", StartOpts{
			Cwd: t.TempDir(),
			Env: helperEnvWithCapture("stream_updates", "", captureFile),
			MCPServers: []aghconfig.MCPServer{
				{
					Name:      "agh-hosted-tools",
					Transport: aghconfig.MCPServerTransportStdio,
					Command:   "/bin/agh",
					Args:      []string{"tool", "mcp", "--session", "sess-1", "--bind-nonce", "nonce"},
					Env:       map[string]string{"AGH_HOME": "/tmp/agh-home"},
				},
				{
					Name:      "remote-http",
					Transport: aghconfig.MCPServerTransportHTTP,
					URL:       "https://example.test/mcp",
				},
				{
					Name:      "remote-sse",
					Transport: aghconfig.MCPServerTransportSSE,
					URL:       "https://example.test/sse",
				},
			},
		})
		defer stopProcess(t, driver, proc)

		params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionNew)
		request := decodeCapturedNewSessionRequest(t, params)
		if got, want := len(request.MCPServers), 1; got != want {
			t.Fatalf("session/new mcpServers = %#v, want only hosted stdio entry", request.MCPServers)
		}
		stdio := request.MCPServers[0].Stdio
		if stdio == nil {
			t.Fatalf("session/new mcpServers[0] = %#v, want stdio variant", request.MCPServers[0])
		}
		if stdio.Name != "agh-hosted-tools" || stdio.Command != "/bin/agh" {
			t.Fatalf("hosted stdio entry = %#v, want hosted command", stdio)
		}
		if !slices.Equal(stdio.Args, []string{"tool", "mcp", "--session", "sess-1", "--bind-nonce", "nonce"}) {
			t.Fatalf("hosted stdio args = %#v, want tool mcp bind args", stdio.Args)
		}
		if got, want := len(stdio.Env), 1; got != want || stdio.Env[0].Name != "AGH_HOME" {
			t.Fatalf("hosted stdio env = %#v, want AGH_HOME only", stdio.Env)
		}
	})
}

func TestStartResumeReturnsSentinelErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		envScenario string
		wantErr     error
	}{
		"load session failure": {
			envScenario: "load_session_error",
			wantErr:     ErrLoadSessionFailed,
		},
		"agent missing load session support": {
			envScenario: "stream_updates",
			wantErr:     ErrAgentDoesNotSupportSession,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			driver := New()
			_, err := driver.Start(testutil.Context(t), StartOpts{
				AgentName:       "helper",
				Command:         helperCommand(t),
				Cwd:             t.TempDir(),
				Env:             helperEnv(tc.envScenario, ""),
				Permissions:     aghconfig.PermissionModeApproveAll,
				ResumeSessionID: "sess-existing",
			})
			if err == nil {
				t.Fatalf("Start(%s) error = nil, want non-nil", tc.envScenario)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Start(%s) error = %v, want errors.Is(..., %v)", tc.envScenario, err, tc.wantErr)
			}
		})
	}
}

func TestStartIncludesAgentContextInLaunchErrors(t *testing.T) {
	t.Parallel()

	driver := New()
	_, err := driver.Start(testutil.Context(t), StartOpts{
		AgentName:   "missing-helper",
		Command:     "/definitely/missing-binary",
		Cwd:         t.TempDir(),
		Permissions: aghconfig.PermissionModeApproveAll,
	})
	if err == nil {
		t.Fatal("Start() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `start agent "missing-helper" subprocess "/definitely/missing-binary"`) {
		t.Fatalf("Start() error = %q, want agent and command context", err)
	}
}

func TestIsLoadSessionResourceMissing(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  error
		want bool
	}{
		"ShouldDetectResourceMissingRequestError": {
			err: fmt.Errorf(
				"%w: load session %q for %q: %w",
				ErrLoadSessionFailed,
				"sess-existing",
				"helper",
				&acpsdk.RequestError{
					Code:    requestErrorResourceNotFoundCode,
					Message: "Resource not found: sess-existing",
				},
			),
			want: true,
		},
		"ShouldRejectDifferentRequestError": {
			err: fmt.Errorf(
				"%w: load session %q for %q: %w",
				ErrLoadSessionFailed,
				"sess-existing",
				"helper",
				&acpsdk.RequestError{Code: -32603, Message: "Internal error"},
			),
			want: false,
		},
		"ShouldRejectNonLoadSessionError": {
			err:  errors.New("boom"),
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := IsLoadSessionResourceMissing(tc.err); got != tc.want {
				t.Fatalf("IsLoadSessionResourceMissing() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCleanupFailedStartReturnsJoinedErrorWhenStopFails(t *testing.T) {
	t.Parallel()

	driver := New()
	proc := &AgentProcess{
		done:   make(chan struct{}),
		stderr: &lockedBuffer{},
	}
	stopErr := errors.New("stop failed")
	proc.setWaitError(stopErr)
	close(proc.done)

	startErr := fmt.Errorf(
		"%w: load session %q for %q: %w",
		ErrLoadSessionFailed,
		"sess-existing",
		"helper",
		errors.New("load failed"),
	)
	err := driver.cleanupFailedStart(proc, startErr)
	if err == nil {
		t.Fatal("cleanupFailedStart() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrLoadSessionFailed) {
		t.Fatalf("cleanupFailedStart() error = %v, want ErrLoadSessionFailed", err)
	}
	if !errors.Is(err, stopErr) {
		t.Fatalf("cleanupFailedStart() error = %v, want stopErr", err)
	}
	if !strings.Contains(err.Error(), "stop failed while cleaning up failed start") {
		t.Fatalf("cleanupFailedStart() error = %v, want cleanup stop context", err)
	}
}

func TestProcessCrashDetected(t *testing.T) {
	t.Parallel()

	driver := New()
	proc := startHelperProcess(t, driver, "crash_on_prompt", "", StartOpts{})

	eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
		TurnID:  "turn-crash",
		Message: "trigger crash",
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}

	events := collectEvents(t, eventsCh)
	if len(events) == 0 || events[len(events)-1].Type != EventTypeError {
		t.Fatalf("Prompt() last event = %#v, want error", events)
	}

	waitErr := waitForProcess(t, proc)
	if waitErr == nil {
		t.Fatal("Wait() error = nil, want process crash")
	}
}

func TestPromptErrorPreservesRequestErrorData(t *testing.T) {
	t.Parallel()

	t.Run("Should emit structured request error data for downstream marker classification", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "prompt_request_error_with_reason", "", StartOpts{})
		t.Cleanup(func() {
			stopProcess(t, driver, proc)
		})

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-mcp-auth",
			Message: "trigger structured auth error",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() events = empty, want error event")
		}
		event := events[len(events)-1]
		if event.Type != EventTypeError {
			t.Fatalf("Prompt() last event type = %q, want %q", event.Type, EventTypeError)
		}
		var payload struct {
			Data struct {
				ReasonCodes []string `json:"reason_codes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(event.Raw, &payload); err != nil {
			t.Fatalf("json.Unmarshal(event.Raw) error = %v raw=%s", err, string(event.Raw))
		}
		if !slices.Contains(payload.Data.ReasonCodes, "mcp_auth_required") {
			t.Fatalf("request error reason codes = %#v, want mcp_auth_required", payload.Data.ReasonCodes)
		}
	})
}

func TestPromptStopDoesNotEmitRuntimeError(t *testing.T) {
	t.Parallel()

	t.Run("Should not emit runtime error after explicit stop", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "block_prompt_until_cancel", "", StartOpts{})
		stopped := false
		t.Cleanup(func() {
			if stopped {
				return
			}
			stopProcess(t, driver, proc)
		})

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-stop",
			Message: "block until stopped",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		select {
		case event := <-eventsCh:
			if got, want := event.Type, EventTypeAgentMessage; got != want {
				t.Fatalf("first prompt event = %q, want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for blocking prompt to start")
		}

		if err := driver.Stop(testutil.Context(t), proc); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stopped = true
		for _, event := range collectEvents(t, eventsCh) {
			if event.Type == EventTypeError {
				t.Fatalf("prompt events contain %q after explicit stop: %#v", EventTypeError, event)
			}
		}
	})
}

func TestShouldSuppressPromptErrorOnStop(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Should suppress context canceled errors",
			err:  context.Canceled,
			want: true,
		},
		{
			name: "Should suppress deadline exceeded errors",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "Should suppress wrapped canceled failures",
			err:  WrapFailure(store.FailureCanceled, "stopped", context.Canceled),
			want: true,
		},
		{
			name: "Should suppress request errors carrying canceled details",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "context canceled"},
			},
			want: true,
		},
		{
			name: "Should suppress peer disconnect request errors after stop",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "peer disconnected before response"},
			},
			want: true,
		},
		{
			name: "Should not suppress generic request failures",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"details": "Tool invocation failed"},
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldSuppressPromptErrorOnStop(tc.err); got != tc.want {
				t.Fatalf("shouldSuppressPromptErrorOnStop() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDriverApprovePermissionValidationAndForwarding(t *testing.T) {
	t.Parallel()

	driver := New(WithPermissionTimeout(123 * time.Millisecond))
	if driver.permissionWait != 123*time.Millisecond {
		t.Fatalf("permissionWait = %v, want %v", driver.permissionWait, 123*time.Millisecond)
	}

	proc := newDirectProcess(t, aghconfig.PermissionModeDenyAll)
	requestID, pending := proc.registerPendingPermission("turn-1", acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{ToolCallId: "tool-1"},
	})

	if err := driver.ApprovePermission(context.Background(), proc, ApproveRequest{
		RequestID: requestID,
		Decision:  string(decisionAllowOnce),
	}); err != nil {
		t.Fatalf("ApprovePermission() error = %v", err)
	}
	select {
	case decision := <-pending.response:
		if decision != decisionAllowOnce {
			t.Fatalf("pending response = %q, want %q", decision, decisionAllowOnce)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending permission response")
	}

	if err := driver.ApprovePermission(context.Background(), nil, ApproveRequest{
		RequestID: "req-1",
		Decision:  string(decisionAllowOnce),
	}); err == nil {
		t.Fatal("ApprovePermission(nil proc) error = nil, want non-nil")
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := driver.ApprovePermission(canceledCtx, proc, ApproveRequest{
		RequestID: "req-1",
		Decision:  string(decisionAllowOnce),
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ApprovePermission(canceled ctx) error = %v, want context.Canceled", err)
	}
}

func startHelperProcess(
	t *testing.T,
	driver *Driver,
	scenario string,
	filePath string,
	overrides StartOpts,
) *AgentProcess {
	t.Helper()

	command := helperCommand(t)
	opts := StartOpts{
		AgentName:   "helper",
		Command:     command,
		Cwd:         t.TempDir(),
		Env:         helperEnv(scenario, filePath),
		Permissions: aghconfig.PermissionModeApproveAll,
	}
	if overrides.AgentName != "" {
		opts.AgentName = overrides.AgentName
	}
	if overrides.Command != "" {
		opts.Command = overrides.Command
	}
	if overrides.Cwd != "" {
		opts.Cwd = overrides.Cwd
	}
	if overrides.AdditionalDirs != nil {
		opts.AdditionalDirs = append([]string(nil), overrides.AdditionalDirs...)
	}
	if overrides.Env != nil {
		opts.Env = overrides.Env
	}
	if overrides.Permissions != "" {
		opts.Permissions = overrides.Permissions
	}
	if overrides.MCPServers != nil {
		opts.MCPServers = overrides.MCPServers
	}
	if overrides.SystemPrompt != "" {
		opts.SystemPrompt = overrides.SystemPrompt
	}
	if overrides.SystemPromptDelivery != "" {
		opts.SystemPromptDelivery = overrides.SystemPromptDelivery
	}
	if overrides.PreferredModel != "" {
		opts.PreferredModel = overrides.PreferredModel
	}
	if overrides.ReasoningEffort != "" {
		opts.ReasoningEffort = overrides.ReasoningEffort
	}
	opts.ResumeSessionID = overrides.ResumeSessionID
	opts.Launcher = overrides.Launcher
	opts.ToolHost = overrides.ToolHost
	opts.ToolGateway = overrides.ToolGateway
	opts.ProviderName = overrides.ProviderName
	opts.ProviderConfig = overrides.ProviderConfig
	opts.ProviderAuthEnv = overrides.ProviderAuthEnv

	proc, err := driver.Start(testutil.Context(t), opts)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	return proc
}

func stopProcess(t *testing.T, driver *Driver, proc *AgentProcess) {
	t.Helper()
	if proc == nil {
		return
	}
	if err := driver.Stop(testutil.Context(t), proc); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestStopManagedProcessRespectsContext(t *testing.T) {
	t.Run("ShouldReturnDeadlineExceededWhenManagedProcessShutdownExceedsStopContext", func(t *testing.T) {
		t.Parallel()

		driver := New(WithStopTimeout(5 * time.Second))
		managed, err := subprocess.Launch(context.Background(), subprocess.LaunchConfig{
			Command:          "sh",
			Args:             []string{"-c", "sleep 30"},
			DisableTransport: true,
			ShutdownTimeout:  time.Second,
		})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}

		proc := &AgentProcess{
			managed: managed,
			done:    make(chan struct{}),
		}
		go proc.waitForExit(context.Background(), defaultProcessRecordTimeout)
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if shutdownErr := managed.Shutdown(cleanupCtx); shutdownErr != nil {
				t.Fatalf("managed.Shutdown() error = %v", shutdownErr)
			}
			select {
			case <-proc.Done():
			case <-cleanupCtx.Done():
				t.Fatalf("process did not exit during cleanup: %v", cleanupCtx.Err())
			}
		})

		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()

		startedAt := time.Now()
		err = driver.Stop(stopCtx, proc)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Stop() error = %v, want context deadline exceeded", err)
		}
		if elapsed := time.Since(startedAt); elapsed > time.Second {
			t.Fatalf("Stop() elapsed = %v, want <= 1s", elapsed)
		}
	})
}

func TestRegisterAgentProcessRetainsRegistryForPIDLessSandboxAgents(t *testing.T) {
	t.Run("Should keep registry available for external sandbox terminal tracking", func(t *testing.T) {
		t.Parallel()

		registry := toolruntime.NewRegistry(nil)
		driver := &Driver{processRegistry: registry}
		process := &AgentProcess{PID: 0}

		if err := driver.registerAgentProcess(context.Background(), process); err != nil {
			t.Fatalf("registerAgentProcess(PID=0) error = %v", err)
		}
		if process.processRegistry != registry {
			t.Fatalf("process.processRegistry = %p, want %p", process.processRegistry, registry)
		}
		if process.processRecord != nil {
			t.Fatalf("process.processRecord = %#v, want nil for PID-less agent", process.processRecord)
		}
	})
}

func TestProcessRecordContext(t *testing.T) {
	t.Run("Should detach cancellation while preserving a bounded deadline", func(t *testing.T) {
		t.Parallel()

		parent, cancelParent := context.WithCancel(context.Background())
		cancelParent()

		ctx, cancel := processRecordContext(parent, 25*time.Millisecond)
		defer cancel()
		if err := ctx.Err(); err != nil {
			t.Fatalf("processRecordContext() err = %v, want detached from parent cancellation", err)
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("processRecordContext() deadline missing")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > time.Second {
			t.Fatalf("processRecordContext() remaining deadline = %s, want bounded positive deadline", remaining)
		}
	})
}

func TestCheckpointProcessOwnerWrapsCheckpointErrors(t *testing.T) {
	t.Run("Should add ACP context while preserving checkpoint root error", func(t *testing.T) {
		t.Parallel()

		root := errors.New("checkpoint failed")
		registry := toolruntime.NewRegistry(&failingToolRuntimeStore{updateErr: root})
		handle, err := registry.Register(context.Background(), toolruntime.RegisterConfig{
			Source:  toolruntime.ProcessSourceACPAgent,
			Owner:   toolruntime.ProcessOwner{SessionID: "old-session"},
			Command: "agent",
		})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		process := &AgentProcess{
			SessionID:     "new-session",
			processRecord: handle,
		}

		err = process.checkpointProcessOwner(context.Background())
		if !errors.Is(err, root) || !strings.Contains(err.Error(), "checkpoint process owner") {
			t.Fatalf("checkpointProcessOwner() error = %v, want ACP context wrapping root", err)
		}
	})
}

type failingToolRuntimeStore struct {
	updateErr error
	upserts   int
}

func (s *failingToolRuntimeStore) UpsertProcessRecord(context.Context, toolruntime.ProcessRecord) error {
	s.upserts++
	if s.upserts > 1 {
		return s.updateErr
	}
	return nil
}

func (s *failingToolRuntimeStore) UpdateProcessRecordState(
	context.Context,
	toolruntime.ProcessStateUpdate,
) error {
	return s.updateErr
}

func (s *failingToolRuntimeStore) ListProcessRecords(
	context.Context,
	toolruntime.ProcessQuery,
) ([]toolruntime.ProcessRecord, error) {
	return nil, nil
}

func waitForProcess(t *testing.T, proc *AgentProcess) error {
	t.Helper()
	select {
	case <-proc.Done():
		return proc.Wait()
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for process exit")
		return nil
	}
}

func collectEvents(t *testing.T, eventsCh <-chan AgentEvent) []AgentEvent {
	t.Helper()

	events := make([]AgentEvent, 0, 8)
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case event, ok := <-eventsCh:
			if !ok {
				return events
			}
			events = append(events, event)
		case <-timeout.C:
			t.Fatalf("timeout waiting for prompt events; collected %#v", events)
		}
	}
}

func helperCommand(t *testing.T) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return shellquote.Join(bin, "-test.run=TestACPHelperProcess")
}

func helperEnv(scenario string, filePath string) []string {
	env := append([]string(nil), os.Environ()...)
	env = append(env,
		testHelperEnvKey+"=1",
		testHelperScenarioKey+"="+scenario,
	)
	if filePath != "" {
		env = append(env, testHelperFileKey+"="+filePath)
	}
	return env
}

func helperEnvWithCapture(scenario string, filePath string, capturePath string) []string {
	env := helperEnv(scenario, filePath)
	if strings.TrimSpace(capturePath) != "" {
		env = append(env, testHelperCaptureKey+"="+capturePath)
	}
	return env
}

type capturedRequestEnvelope struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type capturedNewSessionRequest struct {
	Cwd            string             `json:"cwd"`
	AdditionalDirs []string           `json:"additional_dirs,omitempty"`
	MCPServers     []acpsdk.McpServer `json:"mcpServers"`
}

type capturedLoadSessionRequest struct {
	Cwd            string             `json:"cwd"`
	AdditionalDirs []string           `json:"additional_dirs,omitempty"`
	MCPServers     []acpsdk.McpServer `json:"mcpServers"`
	SessionID      string             `json:"sessionId"`
}

type capturedSetSessionModeRequest struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

type capturedSetSessionConfigOptionRequest struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Value     string `json:"value"`
}

func captureRequestParams(t *testing.T, path string, method string) map[string]json.RawMessage {
	t.Helper()

	matches := captureRequestParamsForMethod(t, path, method)
	if len(matches) > 0 {
		return matches[0]
	}
	t.Fatalf("capture file %q does not contain method %q", path, method)
	return nil
}

func captureMethodExists(t *testing.T, path string, method string) bool {
	t.Helper()

	return len(captureRequestParamsForMethod(t, path, method)) > 0
}

func captureRequestParamsForMethod(t *testing.T, path string, method string) []map[string]json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}

	matches := make([]map[string]json.RawMessage, 0)
	lines := strings.SplitSeq(strings.TrimSpace(string(data)), "\n")
	for line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var envelope capturedRequestEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(captured envelope) error = %v", err)
		}
		if envelope.Method != method {
			continue
		}

		var params map[string]json.RawMessage
		if err := json.Unmarshal(envelope.Params, &params); err != nil {
			t.Fatalf("json.Unmarshal(captured params) error = %v", err)
		}
		matches = append(matches, params)
	}
	return matches
}

func captureNegotiationSequence(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	sequence := make([]string, 0, 3)
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var envelope capturedRequestEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(captured envelope) error = %v", err)
		}
		var params map[string]json.RawMessage
		switch envelope.Method {
		case acpsdk.AgentMethodSessionSetMode:
			if err := json.Unmarshal(envelope.Params, &params); err != nil {
				t.Fatalf("json.Unmarshal(set-mode params) error = %v", err)
			}
			request := decodeCapturedSetSessionModeRequest(t, params)
			sequence = append(sequence, "mode:"+request.ModeID)
		case acpsdk.AgentMethodSessionSetConfigOption:
			if err := json.Unmarshal(envelope.Params, &params); err != nil {
				t.Fatalf("json.Unmarshal(set-config params) error = %v", err)
			}
			request := decodeCapturedSetSessionConfigOptionRequest(t, params)
			sequence = append(sequence, request.ConfigID+":"+request.Value)
		}
	}
	return sequence
}

func decodeCapturedNewSessionRequest(t *testing.T, params map[string]json.RawMessage) capturedNewSessionRequest {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal(new-session params) error = %v", err)
	}
	var request capturedNewSessionRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("json.Unmarshal(new-session request) error = %v", err)
	}
	return request
}

func decodeCapturedLoadSessionRequest(t *testing.T, params map[string]json.RawMessage) capturedLoadSessionRequest {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal(load-session params) error = %v", err)
	}
	var request capturedLoadSessionRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("json.Unmarshal(load-session request) error = %v", err)
	}
	return request
}

func decodeCapturedSetSessionModeRequest(
	t *testing.T,
	params map[string]json.RawMessage,
) capturedSetSessionModeRequest {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal(set-session-mode params) error = %v", err)
	}
	var request capturedSetSessionModeRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("json.Unmarshal(set-session-mode request) error = %v", err)
	}
	return request
}

func decodeCapturedSetSessionConfigOptionRequest(
	t *testing.T,
	params map[string]json.RawMessage,
) capturedSetSessionConfigOptionRequest {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal(set-session-config-option params) error = %v", err)
	}
	var request capturedSetSessionConfigOptionRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("json.Unmarshal(set-session-config-option request) error = %v", err)
	}
	return request
}

func assertConfigOption(
	t *testing.T,
	options []SessionConfigOption,
	id string,
	current string,
	wantValues ...string,
) {
	t.Helper()

	var found *SessionConfigOption
	for index := range options {
		if options[index].ID == id {
			found = &options[index]
			break
		}
	}
	if found == nil {
		t.Fatalf("config option %q not found in %#v", id, options)
	}
	if got := found.Current; got != current {
		t.Fatalf("config option %q current = %q, want %q", id, got, current)
	}
	values := make([]string, 0, len(found.Values))
	for _, value := range found.Values {
		values = append(values, value.Value)
	}
	for _, want := range wantValues {
		if !slices.Contains(values, want) {
			t.Fatalf("config option %q values = %#v, want value %q", id, values, want)
		}
	}
}

func mustCanonicalDir(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q) error = %v", path, err)
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		t.Fatalf("filepath.Abs(%q) error = %v", resolved, err)
	}
	return filepath.Clean(absolute)
}

func assertPermissionResult(t *testing.T, err error, wantOK bool) {
	t.Helper()
	if wantOK && err != nil {
		t.Fatalf("authorize() error = %v, want nil", err)
	}
	if !wantOK && !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("authorize() error = %v, want ErrPermissionDenied", err)
	}
}

type helperACPAgent struct {
	conn            *acpsdk.AgentSideConnection
	scenario        string
	filePath        string
	configOptionsMu sync.Mutex
	configOptions   []acpsdk.SessionConfigOption
}

func (a *helperACPAgent) Authenticate(
	context.Context,
	acpsdk.AuthenticateRequest,
) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (a *helperACPAgent) Initialize(context.Context, acpsdk.InitializeRequest) (acpsdk.InitializeResponse, error) {
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AgentCapabilities: acpsdk.AgentCapabilities{
			LoadSession: a.scenario == "load_session" || a.scenario == "load_session_error" ||
				a.scenario == "load_mode_mapping" || a.scenario == "load_config_options",
		},
		AuthMethods: []acpsdk.AuthMethod{},
	}, nil
}

func (a *helperACPAgent) Cancel(context.Context, acpsdk.CancelNotification) error {
	return nil
}

func (a *helperACPAgent) CloseSession(
	context.Context,
	acpsdk.CloseSessionRequest,
) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, nil
}

func (a *helperACPAgent) Logout(context.Context, acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

func (a *helperACPAgent) ListSessions(
	context.Context,
	acpsdk.ListSessionsRequest,
) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{}, nil
}

func (a *helperACPAgent) ResumeSession(
	context.Context,
	acpsdk.ResumeSessionRequest,
) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, nil
}

func (a *helperACPAgent) NewSession(context.Context, acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	if a.scenario == "mode_mapping" {
		return acpsdk.NewSessionResponse{
			SessionId: "sess-new",
			Modes:     helperModeStateWithCurrent("default", "default", "plan", "bypassPermissions"),
		}, nil
	}
	if a.scenario == "cursor_mode_mapping" || a.scenario == "cursor_mode_current_agent" {
		current := "plan"
		if a.scenario == "cursor_mode_current_agent" {
			current = "agent"
		}
		configOptions := []acpsdk.SessionConfigOption{
			helperSelectConfigOption("mode", "Mode", current, "agent", "plan", "ask"),
		}
		a.setHelperConfigOptions(configOptions)
		return acpsdk.NewSessionResponse{
			SessionId:     "sess-new",
			Modes:         helperModeStateWithCurrent(current, "agent", "plan", "ask"),
			ConfigOptions: configOptions,
		}, nil
	}
	if a.scenario == "config_options" ||
		a.scenario == "config_options_no_model" ||
		a.scenario == "config_options_no_reasoning" ||
		a.scenario == "model_specific_config_options" ||
		a.scenario == "config_option_update" {
		configOptions := helperConfigOptions("new-model", "medium")
		if a.scenario == "model_specific_config_options" {
			configOptions = append(
				helperModelConfigOptions("new-model"),
				helperSelectConfigOption("effort", "Reasoning effort", "low", "low"),
			)
		}
		if a.scenario == "config_options_no_model" {
			configOptions = []acpsdk.SessionConfigOption{
				helperSelectConfigOption(
					"reasoning_effort",
					"Reasoning effort",
					"medium",
					"minimal",
					"medium",
					"xhigh",
				),
			}
		}
		if a.scenario == "config_options_no_reasoning" {
			configOptions = helperModelConfigOptions("new-model")
		}
		a.setHelperConfigOptions(configOptions)
		modes := helperModeState("new-mode")
		if a.scenario == "model_specific_config_options" {
			modes = helperModeStateWithCurrent("default", "default", "plan", "bypassPermissions")
		}
		return acpsdk.NewSessionResponse{
			SessionId:     "sess-new",
			Modes:         modes,
			ConfigOptions: configOptions,
		}, nil
	}
	return acpsdk.NewSessionResponse{
		SessionId: "sess-new",
		Modes:     helperModeState("new-mode"),
	}, nil
}

func (a *helperACPAgent) LoadSession(context.Context, acpsdk.LoadSessionRequest) (acpsdk.LoadSessionResponse, error) {
	if a.scenario == "load_session_error" {
		return acpsdk.LoadSessionResponse{}, errors.New("load failed")
	}
	if a.scenario == "load_mode_mapping" {
		return acpsdk.LoadSessionResponse{
			Modes: helperModeStateWithCurrent("default", "default", "plan", "bypassPermissions"),
		}, nil
	}
	if a.scenario == "load_config_options" {
		configOptions := helperConfigOptions("loaded-model", "high")
		a.setHelperConfigOptions(configOptions)
		return acpsdk.LoadSessionResponse{
			Modes:         helperModeState("loaded-mode"),
			ConfigOptions: configOptions,
		}, nil
	}
	return acpsdk.LoadSessionResponse{
		Modes: helperModeState("loaded-mode"),
	}, nil
}

func (a *helperACPAgent) Prompt(ctx context.Context, params acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	switch a.scenario {
	case "crash_on_prompt":
		os.Exit(23)
	case "prompt_request_error_with_reason":
		return acpsdk.PromptResponse{}, &acpsdk.RequestError{
			Code:    -32000,
			Message: "Authentication required",
			Data: map[string]any{
				"reason_codes": []string{"mcp_auth_required"},
			},
		}
	case "block_prompt_until_cancel":
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText("blocking"),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
		<-ctx.Done()
		return acpsdk.PromptResponse{}, ctx.Err()
	case "echo_prompt":
		text := ""
		if len(params.Prompt) > 0 && params.Prompt[0].Text != nil {
			text = params.Prompt[0].Text.Text
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(text),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "echo_prompt_meta":
		data, err := json.Marshal(params.Meta)
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(string(data)),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "config_option_update":
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update: acpsdk.SessionUpdate{
				ConfigOptionUpdate: &acpsdk.SessionConfigOptionUpdate{
					ConfigOptions: helperConfigOptions("other-model", "xhigh"),
				},
			},
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "fs_read":
		response, err := a.conn.ReadTextFile(ctx, acpsdk.ReadTextFileRequest{
			SessionId: params.SessionId,
			Path:      a.filePath,
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(response.Content),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "fs_write_terminal":
		if _, err := a.conn.WriteTextFile(ctx, acpsdk.WriteTextFileRequest{
			SessionId: params.SessionId,
			Path:      a.filePath,
			Content:   "from-write",
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
		readResponse, err := a.conn.ReadTextFile(ctx, acpsdk.ReadTextFileRequest{
			SessionId: params.SessionId,
			Path:      a.filePath,
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(readResponse.Content),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}

		cwd, err := os.Getwd()
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		createResp, err := a.conn.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: params.SessionId,
			Command:   "sh",
			Args:      []string{"-c", "printf terminal-ok"},
			Cwd:       new(cwd),
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if _, err := a.conn.WaitForTerminalExit(ctx, acpsdk.WaitForTerminalExitRequest{
			SessionId:  params.SessionId,
			TerminalId: createResp.TerminalId,
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
		outputResp, err := a.conn.TerminalOutput(ctx, acpsdk.TerminalOutputRequest{
			SessionId:  params.SessionId,
			TerminalId: createResp.TerminalId,
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(outputResp.Output),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "permission":
		title := "permission request"
		locationPath := a.filePath
		if locationPath == "" {
			locationPath = filepath.Join(string(filepath.Separator), "workspace", "demo.txt")
		}
		outcome, err := a.conn.RequestPermission(ctx, acpsdk.RequestPermissionRequest{
			SessionId: params.SessionId,
			Options: []acpsdk.PermissionOption{
				{OptionId: "allow-once", Name: "allow once", Kind: acpsdk.PermissionOptionKindAllowOnce},
				{OptionId: "allow-always", Name: "allow always", Kind: acpsdk.PermissionOptionKindAllowAlways},
				{OptionId: "reject-once", Name: "reject once", Kind: acpsdk.PermissionOptionKindRejectOnce},
				{OptionId: "reject-always", Name: "reject always", Kind: acpsdk.PermissionOptionKindRejectAlways},
			},
			ToolCall: acpsdk.ToolCallUpdate{
				ToolCallId: "tool-1",
				Title:      &title,
				Locations: []acpsdk.ToolCallLocation{
					{Path: locationPath},
				},
			},
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		selected := "canceled"
		if outcome.Outcome.Selected != nil {
			selected = string(outcome.Outcome.Selected.OptionId)
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(selected),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	case "network_guardrails":
		targetPath := a.filePath
		if targetPath == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return acpsdk.PromptResponse{}, err
			}
			targetPath = filepath.Join(cwd, "network-blocked.txt")
		}

		writeResult := "write_unexpected"
		if _, err := a.conn.WriteTextFile(ctx, acpsdk.WriteTextFileRequest{
			SessionId: params.SessionId,
			Path:      targetPath,
			Content:   "blocked",
		}); err != nil {
			writeResult = "write_blocked"
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(writeResult),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}

		shellResult := "shell_unexpected"
		if _, err := a.conn.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: params.SessionId,
			Command:   "sh",
			Args:      []string{"-c", "printf nope"},
		}); err != nil {
			shellResult = "shell_blocked"
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(shellResult),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}

		cwd, err := os.Getwd()
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		createResp, err := a.conn.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: params.SessionId,
			Command:   "agh",
			Args:      []string{"network", "status"},
			Cwd:       new(cwd),
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if _, err := a.conn.WaitForTerminalExit(ctx, acpsdk.WaitForTerminalExitRequest{
			SessionId:  params.SessionId,
			TerminalId: createResp.TerminalId,
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
		outputResp, err := a.conn.TerminalOutput(ctx, acpsdk.TerminalOutputRequest{
			SessionId:  params.SessionId,
			TerminalId: createResp.TerminalId,
		})
		if err != nil {
			return acpsdk.PromptResponse{}, err
		}
		if sendErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(outputResp.Output),
		}); sendErr != nil {
			return acpsdk.PromptResponse{}, sendErr
		}
	default:
		updates := []acpsdk.SessionUpdate{
			acpsdk.UpdateAgentMessageText("hello"),
			acpsdk.UpdateAgentThoughtText("thinking"),
			acpsdk.StartToolCall(
				"tool-1",
				"Read file",
				acpsdk.WithStartKind(acpsdk.ToolKindRead),
				acpsdk.WithStartStatus(acpsdk.ToolCallStatusInProgress),
			),
			acpsdk.UpdateToolCall(
				"tool-1",
				acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusCompleted),
				acpsdk.WithUpdateTitle("Read file"),
			),
		}
		for _, update := range updates {
			if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
				SessionId: params.SessionId,
				Update:    update,
			}); err != nil {
				return acpsdk.PromptResponse{}, err
			}
		}
	}

	return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil
}

func (a *helperACPAgent) SetSessionMode(
	context.Context,
	acpsdk.SetSessionModeRequest,
) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

func (a *helperACPAgent) SetSessionConfigOption(
	_ context.Context,
	request acpsdk.SetSessionConfigOptionRequest,
) (acpsdk.SetSessionConfigOptionResponse, error) {
	a.configOptionsMu.Lock()
	defer a.configOptionsMu.Unlock()
	if request.ValueId != nil {
		configID := string(request.ValueId.ConfigId)
		value := acpsdk.SessionConfigValueId(strings.TrimSpace(string(request.ValueId.Value)))
		if a.scenario == "model_specific_config_options" && configID == "model" {
			a.configOptions = append(
				helperModelConfigOptions(string(value)),
				helperSelectConfigOption("effort", "Reasoning effort", "none", "none", "max"),
			)
			return acpsdk.SetSessionConfigOptionResponse{
				ConfigOptions: append([]acpsdk.SessionConfigOption(nil), a.configOptions...),
			}, nil
		}
		for index := range a.configOptions {
			if a.configOptions[index].Select == nil || string(a.configOptions[index].Select.Id) != configID {
				continue
			}
			a.configOptions[index].Select.CurrentValue = value
		}
	}
	return acpsdk.SetSessionConfigOptionResponse{
		ConfigOptions: append([]acpsdk.SessionConfigOption(nil), a.configOptions...),
	}, nil
}

func (a *helperACPAgent) setHelperConfigOptions(options []acpsdk.SessionConfigOption) {
	a.configOptionsMu.Lock()
	defer a.configOptionsMu.Unlock()
	a.configOptions = append([]acpsdk.SessionConfigOption(nil), options...)
}

func helperConfigOptions(modelCurrent string, reasoningCurrent string) []acpsdk.SessionConfigOption {
	options := helperModelConfigOptions(modelCurrent)
	options = append(options, helperSelectConfigOption(
		"reasoning_effort",
		"Reasoning effort",
		reasoningCurrent,
		"minimal",
		"none",
		"low",
		"medium",
		"high",
		"xhigh",
		"max",
	))
	return options
}

func reasoningACPProviderConfig() *aghconfig.ProviderConfig {
	return &aghconfig.ProviderConfig{
		Models: aghconfig.ProviderModelsConfig{
			Reasoning: aghconfig.ProviderReasoningConfig{Apply: aghconfig.ReasoningApplyACPOption},
		},
	}
}

func helperModelConfigOptions(current string) []acpsdk.SessionConfigOption {
	return []acpsdk.SessionConfigOption{
		helperSelectConfigOption("model", "Model", current, "new-model", "loaded-model", "other-model"),
	}
}

func helperSelectConfigOption(
	id string,
	name string,
	current string,
	values ...string,
) acpsdk.SessionConfigOption {
	selectOptions := make(acpsdk.SessionConfigSelectOptionsUngrouped, 0, len(values))
	for _, value := range values {
		selectOptions = append(selectOptions, acpsdk.SessionConfigSelectOption{
			Value: acpsdk.SessionConfigValueId(value),
			Name:  value,
		})
	}
	return acpsdk.SessionConfigOption{
		Select: &acpsdk.SessionConfigOptionSelect{
			Id:           acpsdk.SessionConfigId(id),
			Name:         name,
			CurrentValue: acpsdk.SessionConfigValueId(current),
			Options: acpsdk.SessionConfigSelectOptions{
				Ungrouped: &selectOptions,
			},
			Type: "select",
		},
	}
}

func helperModeState(id string) *acpsdk.SessionModeState {
	return helperModeStateWithCurrent(id, id)
}

func helperModeStateWithCurrent(current string, available ...string) *acpsdk.SessionModeState {
	modes := make([]acpsdk.SessionMode, 0, len(available))
	for _, id := range available {
		modes = append(modes, acpsdk.SessionMode{
			Id:   acpsdk.SessionModeId(id),
			Name: id,
		})
	}
	return &acpsdk.SessionModeState{
		CurrentModeId:  acpsdk.SessionModeId(current),
		AvailableModes: modes,
	}
}
