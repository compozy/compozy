package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/vault"
)

func TestLiveProviderSources(t *testing.T) {
	t.Parallel()

	t.Run("Should map Codex OpenAI list output into provider live rows", func(t *testing.T) {
		t.Parallel()

		var sawAuth atomic.Bool
		server := liveJSONServer(t, func(r *http.Request) string {
			if got, want := r.Header.Get("Authorization"), "Bearer sk-codex-test"; got == want {
				sawAuth.Store(true)
			}
			return `{"data":[{"id":"gpt-5.4","name":"GPT-5.4","supportsReasoning":true}]}`
		})
		provider := boundSecretProvider("OPENAI_API_KEY", "env:OPENAI_API_KEY")
		provider.Models.Discovery.Endpoint = server.URL
		source := newLiveSourceForTest(t, "codex", provider, &LiveProviderSourcesConfig{
			BaseEnv:        []string{"PATH=/bin", "OPENAI_API_KEY=ambient-secret"},
			SecretResolver: mapSecretResolver{"env:OPENAI_API_KEY": "sk-codex-test"},
		})

		rows, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "codex", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		row := requireSingleRow(t, rows)
		if !sawAuth.Load() {
			t.Fatal("Authorization header was not populated from bound_secret credential")
		}
		if row.ProviderID != "codex" || row.ModelID != "gpt-5.4" || row.SourceID != "provider_live:codex" {
			t.Fatalf("row identity = %#v, want codex/gpt-5.4 provider_live:codex", row)
		}
		if row.Available == nil || !*row.Available {
			t.Fatalf("Available = %v, want true", row.Available)
		}
		if row.SupportsReasoning == nil || !*row.SupportsReasoning {
			t.Fatalf("SupportsReasoning = %v, want true", row.SupportsReasoning)
		}
		if len(row.ReasoningEfforts) != 0 {
			t.Fatalf("ReasoningEfforts = %#v, want no invented levels", row.ReasoningEfforts)
		}
	})

	t.Run("Should map Claude Anthropic supported models and drop unknown reasoning levels", func(t *testing.T) {
		t.Parallel()

		server := liveJSONServer(t, func(r *http.Request) string {
			if got, want := r.Header.Get("x-api-key"), "sk-ant-test"; got != want {
				t.Fatalf("x-api-key = %q, want %q", got, want)
			}
			if got := r.Header.Get("anthropic-version"); got == "" {
				t.Fatal("anthropic-version header is empty")
			}
			return `{"data":[{` +
				`"id":"claude-sonnet-4-6",` +
				`"displayName":"Claude Sonnet 4.6",` +
				`"supportsEffort":true,` +
				`"supportedEffortLevels":["low","max","xhigh"]` +
				`}]}`
		})
		provider := boundSecretProvider("ANTHROPIC_API_KEY", "env:ANTHROPIC_API_KEY")
		provider.Models.Discovery.Endpoint = server.URL
		source := newLiveSourceForTest(t, "claude", provider, &LiveProviderSourcesConfig{
			BaseEnv:        []string{"PATH=/bin"},
			SecretResolver: mapSecretResolver{"env:ANTHROPIC_API_KEY": "sk-ant-test"},
		})

		rows, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "claude", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		row := requireSingleRow(t, rows)
		if row.DisplayName != "Claude Sonnet 4.6" {
			t.Fatalf("DisplayName = %q, want Claude Sonnet 4.6", row.DisplayName)
		}
		if !slices.Equal(
			row.ReasoningEfforts,
			[]ReasoningEffort{ReasoningEffortLow, ReasoningEffortMax, ReasoningEffortXHigh},
		) {
			t.Fatalf("ReasoningEfforts = %#v, want low/max/xhigh", row.ReasoningEfforts)
		}
	})

	t.Run("Should preserve Claude logical model ids and provider transport bindings", func(t *testing.T) {
		t.Parallel()

		provider := compozyconfig.BuiltinProviders()["claude"]
		provider.Command = "claude-acp"
		provider.AuthMode = compozyconfig.ProviderAuthModeNone
		probe := &fakeACPModelProbe{options: []acp.SessionConfigOption{{
			ID:       "model",
			Category: "model",
			Kind:     acp.SessionConfigOptionKindSelect,
			Values: []acp.SessionConfigOptionValue{
				{Value: "default", Label: "Default"},
				{Value: "sonnet", Label: "Sonnet"},
				{Value: "opus[1m]", Label: "Opus 5 1M"},
				{Value: "haiku", Label: "Haiku"},
				{Value: "claude-future-6", Label: "Claude Future 6"},
			},
		}}}
		source := newLiveSourceForTest(t, "claude", provider, &LiveProviderSourcesConfig{
			BaseEnv:  []string{"PATH=/bin"},
			ACPProbe: probe,
		})

		rows, err := source.ListModels(testutil.Context(t), ListOptions{
			ProviderID: "claude",
			Now:        testTime(0),
		})
		if err != nil {
			t.Fatalf("ListModels(Claude ACP) error = %v", err)
		}
		if got, want := rowModelIDs(rows), []string{
			"claude-future-6",
			"claude-haiku-4-5-20251001",
			"claude-opus-5",
			"claude-sonnet-5",
		}; !slices.Equal(got, want) {
			t.Fatalf("row ids = %#v, want %#v", got, want)
		}
		sonnet := requireModelRow(t, rows, "claude-sonnet-5")
		if sonnet.DisplayName != "Claude Sonnet 5" {
			t.Fatalf("Claude Sonnet display name = %q, want canonical seed label", sonnet.DisplayName)
		}
		assertClaudeTransportBinding(t, sonnet, "default")
		assertClaudeTransportBinding(t, sonnet, "sonnet")

		opus := requireModelRow(t, rows, "claude-opus-5")
		if opus.DisplayName != "Opus 5 1M" {
			t.Fatalf("Claude Opus display name = %q, want live label", opus.DisplayName)
		}
		assertClaudeTransportBinding(t, opus, "opus[1m]")
		future := requireModelRow(t, rows, "claude-future-6")
		assertClaudeTransportBinding(t, future, "claude-future-6")
	})

	t.Run(
		"Should record unavailable Claude runtime when native CLI auth cannot satisfy HTTP discovery",
		func(t *testing.T) {
			t.Parallel()

			store := newMemoryStore()
			source := newLiveSourceForTest(t, "claude", compozyconfig.ProviderConfig{}, &LiveProviderSourcesConfig{
				BaseEnv: []string{"PATH=/bin", "ANTHROPIC_API_KEY=ambient-secret"},
			})
			service := newTestService(t, store, []Source{source})

			statuses, err := service.Refresh(
				testutil.Context(t),
				RefreshOptions{ProviderID: "claude", Force: true, Now: testTime(0)},
			)
			if !errors.Is(err, ErrAllSourcesFailed) {
				t.Fatalf("Refresh() error = %v, want ErrAllSourcesFailed", err)
			}
			status := requireStatus(t, statuses, "provider_live:claude")
			if status.RefreshState != RefreshStateFailed {
				t.Fatalf("RefreshState = %q, want failed", status.RefreshState)
			}
			if strings.Contains(status.LastError, "ambient-secret") {
				t.Fatalf("LastError = %q, want no ambient secret", status.LastError)
			}
		},
	)

	t.Run("Should preserve OpenRouter and Vercel provider model ids", func(t *testing.T) {
		t.Parallel()

		openRouterServer := liveJSONServer(t, func(r *http.Request) string {
			if got, want := r.Header.Get("Authorization"), "Bearer sk-router"; got != want {
				t.Fatalf("OpenRouter Authorization = %q, want %q", got, want)
			}
			return `{"data":[{"id":"anthropic/claude-sonnet-4-6","name":"Claude via OpenRouter"}]}`
		})
		openRouter := boundSecretProvider("OPENROUTER_API_KEY", "env:OPENROUTER_API_KEY")
		openRouter.Models.Discovery.Endpoint = openRouterServer.URL
		routerRows, err := newLiveSourceForTest(t, "openrouter", openRouter, &LiveProviderSourcesConfig{
			BaseEnv:        []string{"PATH=/bin"},
			SecretResolver: mapSecretResolver{"env:OPENROUTER_API_KEY": "sk-router"},
		}).ListModels(testutil.Context(t), ListOptions{ProviderID: "openrouter", Now: testTime(0)})
		if err != nil {
			t.Fatalf("OpenRouter ListModels() error = %v", err)
		}
		if got, want := requireSingleRow(t, routerRows).ModelID, "anthropic/claude-sonnet-4-6"; got != want {
			t.Fatalf("OpenRouter ModelID = %q, want %q", got, want)
		}

		vercelServer := liveJSONServer(t, func(_ *http.Request) string {
			return `{"data":[{` +
				`"id":"openai/gpt-5.4",` +
				`"name":"GPT-5.4",` +
				`"context_window":1000000,` +
				`"max_tokens":32000,` +
				`"pricing":{"input":"0.000001","output":0.000002}` +
				`}]}`
		})
		vercel := compozyconfig.ProviderConfig{}
		vercel.Models.Discovery.Endpoint = vercelServer.URL
		vercelRows, err := newLiveSourceForTest(t, "vercel-ai-gateway", vercel, &LiveProviderSourcesConfig{
			BaseEnv: []string{"PATH=/bin"},
		}).ListModels(testutil.Context(t), ListOptions{ProviderID: "vercel-ai-gateway", Now: testTime(0)})
		if err != nil {
			t.Fatalf("Vercel ListModels() error = %v", err)
		}
		vercelRow := requireSingleRow(t, vercelRows)
		if vercelRow.ModelID != "openai/gpt-5.4" {
			t.Fatalf("Vercel ModelID = %q, want openai/gpt-5.4", vercelRow.ModelID)
		}
		if vercelRow.ContextWindow == nil || *vercelRow.ContextWindow != 1000000 {
			t.Fatalf("ContextWindow = %v, want 1000000", vercelRow.ContextWindow)
		}
		if vercelRow.CostInputPerMillion == nil || *vercelRow.CostInputPerMillion != 1 {
			t.Fatalf("CostInputPerMillion = %v, want 1", vercelRow.CostInputPerMillion)
		}
	})

	t.Run("Should parse Gemini model envelope fields", func(t *testing.T) {
		t.Parallel()

		server := liveJSONServer(t, func(r *http.Request) string {
			if got, want := r.Header.Get("x-goog-api-key"), "gemini-key"; got != want {
				t.Fatalf("x-goog-api-key = %q, want %q", got, want)
			}
			return `{"models":[{` +
				`"name":"models/gemini-3.1-pro",` +
				`"displayName":"Gemini 3.1 Pro",` +
				`"inputTokenLimit":1000000,` +
				`"outputTokenLimit":65536,` +
				`"supportedGenerationMethods":["generateContent"]` +
				`}]}`
		})
		provider := boundSecretProvider("GEMINI_API_KEY", "env:GEMINI_API_KEY")
		provider.Models.Discovery.Endpoint = server.URL
		rows, err := newLiveSourceForTest(t, "gemini", provider, &LiveProviderSourcesConfig{
			BaseEnv:        []string{"PATH=/bin"},
			SecretResolver: mapSecretResolver{"env:GEMINI_API_KEY": "gemini-key"},
		}).ListModels(testutil.Context(t), ListOptions{ProviderID: "gemini", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		row := requireSingleRow(t, rows)
		if row.ModelID != "gemini-3.1-pro" {
			t.Fatalf("ModelID = %q, want gemini-3.1-pro", row.ModelID)
		}
		if row.MaxInputTokens == nil || *row.MaxInputTokens != 1000000 {
			t.Fatalf("MaxInputTokens = %v, want 1000000", row.MaxInputTokens)
		}
		if row.SupportsTools == nil || !*row.SupportsTools {
			t.Fatalf("SupportsTools = %v, want true from generateContent capability", row.SupportsTools)
		}
	})

	t.Run("Should parse Ollama HTTP tags", func(t *testing.T) {
		t.Parallel()

		server := liveJSONServer(t, func(_ *http.Request) string {
			return `{"models":[{"name":"llama3:latest","model":"llama3:latest"}]}`
		})
		provider := compozyconfig.ProviderConfig{}
		provider.Models.Discovery.Endpoint = server.URL
		rows, err := newLiveSourceForTest(t, "ollama", provider, &LiveProviderSourcesConfig{
			BaseEnv: []string{"PATH=/bin"},
		}).ListModels(testutil.Context(t), ListOptions{ProviderID: "ollama", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := requireSingleRow(t, rows).ModelID, "llama3:latest"; got != want {
			t.Fatalf("ModelID = %q, want %q", got, want)
		}
	})

	t.Run("Should aggregate Cursor CLI aliases into logical models", func(t *testing.T) {
		t.Parallel()

		fixture, err := os.ReadFile("testdata/cursor-agent-models.txt")
		if err != nil {
			t.Fatalf("ReadFile(Cursor models fixture) error = %v", err)
		}
		executor := &fakeDiscoveryExecutor{result: DiscoveryCommandResult{Stdout: string(fixture)}}
		provider := compozyconfig.ProviderConfig{HomePolicy: compozyconfig.ProviderHomePolicyOperator}
		source := newLiveSourceForTest(t, "cursor", provider, &LiveProviderSourcesConfig{
			BaseEnv:         []string{"PATH=/bin", "HOME=/Users/operator"},
			CommandExecutor: executor,
		})

		rows, err := source.ListModels(testutil.Context(t), ListOptions{
			ProviderID: "cursor",
			Now:        testTime(0),
		})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := rowModelIDs(rows), []string{
			"claude-opus-5",
			"composer-2.5",
			"grok-4.5",
			"grok-4.6",
		}; !slices.Equal(got, want) {
			t.Fatalf("row ids = %#v, want %#v", got, want)
		}
		if rows[0].DisplayName != "Claude Opus 5 1M" || rows[1].DisplayName != "Composer 2.5" ||
			rows[2].DisplayName != "Cursor Grok 4.5" || rows[3].DisplayName != "Cursor Grok 4.6" {
			t.Fatalf("display names = %#v, want logical model labels", []string{
				rows[0].DisplayName,
				rows[1].DisplayName,
				rows[2].DisplayName,
				rows[3].DisplayName,
			})
		}
		opus := rows[0]
		if opus.SupportsReasoning == nil || !*opus.SupportsReasoning {
			t.Fatalf("Opus 5 SupportsReasoning = %v, want true", opus.SupportsReasoning)
		}
		if !slices.Equal(opus.ReasoningEfforts, []ReasoningEffort{
			ReasoningEffortLow,
			ReasoningEffortMedium,
			ReasoningEffortHigh,
			ReasoningEffortXHigh,
			ReasoningEffortMax,
		}) {
			t.Fatalf("Opus 5 ReasoningEfforts = %#v, want all fixture efforts", opus.ReasoningEfforts)
		}
		if opus.DefaultReasoningEffort == nil || *opus.DefaultReasoningEffort != ReasoningEffortHigh {
			t.Fatalf("Opus 5 DefaultReasoningEffort = %v, want high", opus.DefaultReasoningEffort)
		}
		if len(opus.ConfigOptions) != 1 || opus.ConfigOptions[0].ID != "thinking" ||
			opus.ConfigOptions[0].Kind != ModelOptionKindBoolean ||
			opus.ConfigOptions[0].CurrentBool == nil || *opus.ConfigOptions[0].CurrentBool {
			t.Fatalf("Opus 5 ConfigOptions = %#v, want thinking=false", opus.ConfigOptions)
		}
		assertCursorBinding(t, opus, "claude-opus-5-thinking-high-fast", ReasoningEffortHigh, true, new(true))
		assertCursorBinding(t, opus, "claude-opus-5-low", ReasoningEffortLow, false, new(false))
		if got, want := len(opus.TransportBindings), 16; got != want {
			t.Fatalf("Opus 5 transport bindings = %d, want %d", got, want)
		}

		grok45 := rows[2]
		if grok45.SupportsReasoning == nil || !*grok45.SupportsReasoning ||
			!slices.Equal(grok45.ReasoningEfforts, []ReasoningEffort{
				ReasoningEffortLow,
				ReasoningEffortMedium,
				ReasoningEffortHigh,
			}) {
			t.Fatalf("Grok 4.5 reasoning profile = %#v, want low/medium/high", grok45)
		}
		assertCursorBinding(t, grok45, "cursor-grok-4.5-high-fast", ReasoningEffortHigh, true, nil)
		if got, want := len(grok45.TransportBindings), 6; got != want {
			t.Fatalf("Grok 4.5 transport bindings = %d, want %d", got, want)
		}

		grok46 := rows[3]
		if grok46.SupportsReasoning == nil || !*grok46.SupportsReasoning ||
			!slices.Equal(grok46.ReasoningEfforts, []ReasoningEffort{
				ReasoningEffortLow,
				ReasoningEffortMedium,
				ReasoningEffortHigh,
				ReasoningEffortXHigh,
			}) {
			t.Fatalf("Grok 4.6 reasoning profile = %#v, want low/medium/high/xhigh", grok46.ReasoningEfforts)
		}
		if grok46.DefaultReasoningEffort == nil || *grok46.DefaultReasoningEffort != ReasoningEffortHigh {
			t.Fatalf("Grok 4.6 DefaultReasoningEffort = %v, want high", grok46.DefaultReasoningEffort)
		}
		assertCursorBinding(t, grok46, "cursor-grok-4.6-xhigh-fast", ReasoningEffortXHigh, true, nil)
		if got, want := len(grok46.TransportBindings), 8; got != want {
			t.Fatalf("Grok 4.6 transport bindings = %d, want %d", got, want)
		}

		if !source.BootstrapOnList() {
			t.Fatal("BootstrapOnList() = false, want true")
		}
		req := executor.singleRequest(t)
		if req.Command != "cursor-agent" || !slices.Equal(req.Args, []string{"models"}) {
			t.Fatalf("command = %q %v, want cursor-agent models", req.Command, req.Args)
		}
		if got := envValue(req.Env, "HOME"); got != "/Users/operator" {
			t.Fatalf("HOME = %q, want operator home", got)
		}
	})

	t.Run("Should use the resolved native Claude executable for ACP discovery", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("executable symlink fixture requires Unix semantics")
		}

		binDir := t.TempDir()
		testExecutable, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable() error = %v", err)
		}
		claudeExecutable := filepath.Join(binDir, "claude")
		if err := os.Symlink(testExecutable, claudeExecutable); err != nil {
			t.Fatalf("Symlink(claude executable) error = %v", err)
		}
		probe := &fakeACPModelProbe{options: []acp.SessionConfigOption{{
			ID: "model", Kind: acp.SessionConfigOptionKindSelect,
			Values: []acp.SessionConfigOptionValue{{Value: "claude-fable-5", Label: "Claude Fable 5"}},
		}}}
		provider := compozyconfig.ProviderConfig{
			Command: "npx -y @agentclientprotocol/claude-agent-acp@latest",
		}
		source := newLiveSourceForTest(t, "claude", provider, &LiveProviderSourcesConfig{
			BaseEnv:  []string{"PATH=" + binDir, "HOME=" + t.TempDir()},
			ACPProbe: probe,
		})

		rows, err := source.ListModels(testutil.Context(t), ListOptions{
			ProviderID: "claude",
			Now:        testTime(0),
		})
		if err != nil {
			t.Fatalf("ListModels(Claude ACP) error = %v", err)
		}
		if got := requireSingleRow(t, rows).ModelID; got != "claude-fable-5" {
			t.Fatalf("ModelID = %q, want claude-fable-5", got)
		}
		req := probe.singleRequest(t)
		if req.Command != provider.Command {
			t.Fatalf("ACP discovery command = %q, want %q", req.Command, provider.Command)
		}
		if got := envValue(req.Env, "CLAUDE_CODE_EXECUTABLE"); got != claudeExecutable {
			t.Fatalf("CLAUDE_CODE_EXECUTABLE = %q, want %q", got, claudeExecutable)
		}
	})

	t.Run("Should reject Cursor model command output without advertised model values", func(t *testing.T) {
		t.Parallel()

		executor := &fakeDiscoveryExecutor{
			result: DiscoveryCommandResult{Stdout: "Available models\n\nTip: use --model <id>"},
		}
		source := newLiveSourceForTest(t, "cursor", compozyconfig.ProviderConfig{}, &LiveProviderSourcesConfig{
			BaseEnv:         []string{"PATH=/bin"},
			CommandExecutor: executor,
		})

		_, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "cursor", Now: testTime(0)})
		if err == nil || !strings.Contains(err.Error(), "cursor model command returned no model rows") {
			t.Fatalf("ListModels() error = %v, want missing Cursor model rows", err)
		}
	})

	t.Run("Should reject a configured Cursor discovery endpoint", func(t *testing.T) {
		t.Parallel()

		provider := compozyconfig.ProviderConfig{}
		provider.Models.Discovery.Endpoint = "https://models.example.test/cursor"
		source := newLiveSourceForTest(t, "cursor", provider, &LiveProviderSourcesConfig{
			BaseEnv: []string{"PATH=/bin"},
		})

		_, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "cursor", Now: testTime(0)})
		if err == nil || !strings.Contains(err.Error(), "requires a model discovery command") {
			t.Fatalf("ListModels() error = %v, want model command configuration rejection", err)
		}
	})

	t.Run("Should parse OpenCode model command output and apply effective env home policy", func(t *testing.T) {
		t.Parallel()

		executor := &fakeDiscoveryExecutor{
			result: DiscoveryCommandResult{Stdout: "anthropic/claude-sonnet-4-6\nopenai/gpt-5.4\n"},
		}
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		provider := compozyconfig.ProviderConfig{
			EnvPolicy:  compozyconfig.ProviderEnvPolicyIsolated,
			HomePolicy: compozyconfig.ProviderHomePolicyIsolated,
		}
		source := newLiveSourceForTest(t, "opencode", provider, &LiveProviderSourcesConfig{
			HomePaths:       homePaths,
			BaseEnv:         []string{"PATH=/bin", "HOME=/Users/operator", "OPENAI_API_KEY=ambient-secret"},
			CommandExecutor: executor,
		})

		rows, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "opencode", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := rowModelIDs(
			rows,
		), []string{
			"anthropic/claude-sonnet-4-6",
			"openai/gpt-5.4",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("row ids = %#v, want %#v", got, want)
		}
		req := executor.singleRequest(t)
		if envValue(req.Env, "OPENAI_API_KEY") != "" {
			t.Fatalf("OPENAI_API_KEY = %q, want filtered", envValue(req.Env, "OPENAI_API_KEY"))
		}
		if got, want := envValue(req.Env, "HOME"), homePaths.HomeDir+"/providers/opencode"; got != want {
			t.Fatalf("HOME = %q, want %q", got, want)
		}
		if got := envValue(req.Env, "OPENCODE_CONFIG_DIR"); got == "" {
			t.Fatal("OPENCODE_CONFIG_DIR is empty, want isolated OpenCode config dir")
		}
	})

	t.Run("Should record unavailable OpenCode command status", func(t *testing.T) {
		t.Parallel()

		executor := &fakeDiscoveryExecutor{
			result: DiscoveryCommandResult{Stderr: "api_key=opencode-secret missing"},
			err:    errors.New("exec: opencode not found"),
		}
		source := newLiveSourceForTest(t, "opencode", compozyconfig.ProviderConfig{}, &LiveProviderSourcesConfig{
			BaseEnv:         []string{"PATH=/bin"},
			CommandExecutor: executor,
		})
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})

		statuses, err := service.Refresh(
			testutil.Context(t),
			RefreshOptions{ProviderID: "opencode", Force: true, Now: testTime(0)},
		)
		if !errors.Is(err, ErrAllSourcesFailed) {
			t.Fatalf("Refresh() error = %v, want ErrAllSourcesFailed", err)
		}
		status := requireStatus(t, statuses, "provider_live:opencode")
		if status.RefreshState != RefreshStateFailed {
			t.Fatalf("RefreshState = %q, want failed", status.RefreshState)
		}
		if strings.Contains(status.LastError, "opencode-secret") || !strings.Contains(status.LastError, "[REDACTED]") {
			t.Fatalf("LastError = %q, want redacted command detail", status.LastError)
		}
	})

	t.Run("Should fail closed for OpenClaw without configured discovery path", func(t *testing.T) {
		t.Parallel()

		source := newLiveSourceForTest(t, "openclaw", compozyconfig.ProviderConfig{}, &LiveProviderSourcesConfig{
			BaseEnv: []string{"PATH=/bin"},
		})
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})

		statuses, err := service.Refresh(
			testutil.Context(t),
			RefreshOptions{ProviderID: "openclaw", Force: true, Now: testTime(0)},
		)
		if !errors.Is(err, ErrAllSourcesFailed) {
			t.Fatalf("Refresh() error = %v, want ErrAllSourcesFailed", err)
		}
		status := requireStatus(t, statuses, "provider_live:openclaw")
		if status.RefreshState != RefreshStateFailed {
			t.Fatalf("RefreshState = %q, want failed", status.RefreshState)
		}
		if !strings.Contains(status.LastError, "no configured model discovery") {
			t.Fatalf("LastError = %q, want no configured discovery path", status.LastError)
		}
	})

	t.Run("Should discover Hermes models through a real ACP probe", func(t *testing.T) {
		t.Parallel()

		probe := &fakeACPModelProbe{options: []acp.SessionConfigOption{{
			ID:   "model",
			Kind: acp.SessionConfigOptionKindSelect,
			Values: []acp.SessionConfigOptionValue{{
				Value: "hermes-model",
				Label: "Hermes model",
			}},
		}}}
		provider := compozyconfig.ProviderConfig{
			Command: "hermes acp",
		}
		source := newLiveSourceForTest(t, "hermes", provider, &LiveProviderSourcesConfig{
			BaseEnv:  []string{"PATH=/bin"},
			ACPProbe: probe,
		})
		rows, err := source.ListModels(testutil.Context(t), ListOptions{
			ProviderID: "hermes",
			Now:        testTime(0),
		})
		if err != nil {
			t.Fatalf("ListModels(Hermes ACP) error = %v", err)
		}
		if got, want := requireSingleRow(t, rows).ModelID, "hermes-model"; got != want {
			t.Fatalf("ModelID = %q, want %q", got, want)
		}
		if !source.BootstrapOnList() {
			t.Fatal("BootstrapOnList() = false, want true")
		}
		req := probe.singleRequest(t)
		if req.Command != provider.Command {
			t.Fatalf("ACP discovery command = %q, want %q", req.Command, provider.Command)
		}

		disabled := false
		provider.Models.Discovery.Enabled = &disabled
		disabledSource := newLiveSourceForTest(t, "hermes", provider, &LiveProviderSourcesConfig{
			BaseEnv:  []string{"PATH=/bin"},
			ACPProbe: probe,
		})
		store := newMemoryStore()
		service := newTestService(t, store, []Source{disabledSource})
		statuses, err := service.Refresh(
			testutil.Context(t),
			RefreshOptions{ProviderID: "hermes", Force: true, Now: testTime(0)},
		)
		if err != nil {
			t.Fatalf("Refresh(disabled) error = %v", err)
		}
		if status := requireStatus(t, statuses, "provider_live:hermes"); status.RefreshState != RefreshStateDisabled {
			t.Fatalf("RefreshState = %q, want disabled", status.RefreshState)
		}
		if got := probe.requestCount(); got != 1 {
			t.Fatalf("ACP probe calls = %d, want 1 after disabled refresh", got)
		}
	})

	t.Run("Should use configured Pi endpoint only when enabled", func(t *testing.T) {
		t.Parallel()

		server := liveJSONServer(t, func(_ *http.Request) string {
			return `{"data":[{"id":"anthropic/claude-sonnet-4-6"}]}`
		})
		enabled := true
		provider := compozyconfig.ProviderConfig{}
		provider.Models.Discovery.Enabled = &enabled
		provider.Models.Discovery.Endpoint = server.URL
		rows, err := newLiveSourceForTest(t, "pi", provider, &LiveProviderSourcesConfig{
			BaseEnv: []string{"PATH=/bin"},
		}).ListModels(testutil.Context(t), ListOptions{ProviderID: "pi", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := requireSingleRow(t, rows).ModelID, "anthropic/claude-sonnet-4-6"; got != want {
			t.Fatalf("ModelID = %q, want %q", got, want)
		}
	})

	t.Run("Should record live discovery timeout without blocking indefinitely", func(t *testing.T) {
		t.Parallel()

		timeoutObserved := make(chan error, 1)
		provider := compozyconfig.ProviderConfig{}
		provider.Models.Discovery.Endpoint = "https://models.example.test/discovery"
		provider.Models.Discovery.Timeout = "20ms"
		source := newLiveSourceForTest(t, "vercel-ai-gateway", provider, &LiveProviderSourcesConfig{
			BaseEnv: []string{"PATH=/bin"},
			HTTPClient: &http.Client{
				Timeout: time.Second,
				Transport: modelCatalogRoundTripFunc(func(request *http.Request) (*http.Response, error) {
					<-request.Context().Done()
					err := request.Context().Err()
					timeoutObserved <- err
					return nil, err
				}),
			},
		})
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})

		statuses, err := service.Refresh(
			testutil.Context(t),
			RefreshOptions{ProviderID: "vercel-ai-gateway", Force: true, Now: testTime(0)},
		)
		if !errors.Is(err, ErrAllSourcesFailed) {
			t.Fatalf("Refresh() error = %v, want ErrAllSourcesFailed", err)
		}
		if timeoutErr := <-timeoutObserved; !errors.Is(timeoutErr, context.DeadlineExceeded) {
			t.Fatalf("request context error = %v, want context deadline exceeded", timeoutErr)
		}
		status := requireStatus(t, statuses, "provider_live:vercel-ai-gateway")
		if status.RefreshState != RefreshStateFailed {
			t.Fatalf("RefreshState = %q, want failed", status.RefreshState)
		}
	})
}

func TestLiveProviderRefreshCoalescing(t *testing.T) {
	t.Parallel()

	t.Run("Should coalesce concurrent refreshes for the same provider", func(t *testing.T) {
		t.Parallel()

		source := newBlockingProviderSource("provider_live:codex", "codex")
		service := newTestService(t, newMemoryStore(), []Source{source})
		waiterReached := make(chan struct{}, 1)
		service.onFlightWait = func(string) {
			select {
			case waiterReached <- struct{}{}:
			default:
			}
		}
		ctx := testutil.Context(t)
		var wg sync.WaitGroup
		results := make([][]SourceStatus, 2)
		errs := make([]error, 2)
		startRefresh := func(index int) {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				statuses, err := service.Refresh(ctx, RefreshOptions{
					ProviderID: "codex",
					Force:      true,
					Now:        testTime(0),
				})
				results[i] = statuses
				errs[i] = err
			}(index)
		}
		startRefresh(0)
		waitForBlockingProviderSourceStart(t, source.started, "first provider refresh")
		startRefresh(1)
		select {
		case <-waiterReached:
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for second refresh to register as flight waiter")
		}
		source.release()
		wg.Wait()

		for index, err := range errs {
			if err != nil {
				t.Fatalf("Refresh[%d]() error = %v", index, err)
			}
		}
		if got := source.callCount(); got != 1 {
			t.Fatalf("source calls = %d, want 1", got)
		}
		if len(results[0]) != 1 || len(results[1]) != 1 {
			t.Fatalf("statuses = %#v / %#v, want one status each", results[0], results[1])
		}
		if results[0][0].LastRefresh != results[1][0].LastRefresh {
			t.Fatalf("LastRefresh differs: %s vs %s", results[0][0].LastRefresh, results[1][0].LastRefresh)
		}
	})

	t.Run("Should serialize different source scopes without sharing statuses", func(t *testing.T) {
		t.Parallel()

		firstSource := newBlockingProviderSource("provider_live:codex_a", "codex")
		secondSource := newBlockingProviderSource("provider_live:codex_b", "codex")
		service := newTestService(t, newMemoryStore(), []Source{firstSource, secondSource})
		ctx := testutil.Context(t)

		firstDone := make(chan []SourceStatus, 1)
		firstErr := make(chan error, 1)
		go func() {
			statuses, err := service.Refresh(ctx, RefreshOptions{
				ProviderID: "codex",
				SourceID:   firstSource.ID(),
				Force:      true,
				Now:        testTime(0),
			})
			firstDone <- statuses
			firstErr <- err
		}()
		waitForBlockingProviderSourceStart(t, firstSource.started, "first source refresh")

		secondDone := make(chan []SourceStatus, 1)
		secondErr := make(chan error, 1)
		secondRefreshStarted := make(chan struct{})
		go func() {
			close(secondRefreshStarted)
			statuses, err := service.Refresh(ctx, RefreshOptions{
				ProviderID: "codex",
				SourceID:   secondSource.ID(),
				Force:      true,
				Now:        testTime(1),
			})
			secondDone <- statuses
			secondErr <- err
		}()
		waitForBlockingProviderSourceStart(t, secondRefreshStarted, "second source refresh launch")
		assertBlockingProviderSourceNotStarted(t, secondSource.started, "second source before first release")

		firstSource.release()
		waitForBlockingProviderSourceStart(t, secondSource.started, "second source after first release")
		secondSource.release()

		firstStatuses := <-firstDone
		if err := <-firstErr; err != nil {
			t.Fatalf("first Refresh() error = %v", err)
		}
		secondStatuses := <-secondDone
		if err := <-secondErr; err != nil {
			t.Fatalf("second Refresh() error = %v", err)
		}
		if got, want := requireStatus(t, firstStatuses, firstSource.ID()).SourceID, firstSource.ID(); got != want {
			t.Fatalf("first status source = %q, want %q", got, want)
		}
		if got, want := requireStatus(t, secondStatuses, secondSource.ID()).SourceID, secondSource.ID(); got != want {
			t.Fatalf("second status source = %q, want %q", got, want)
		}
		if got := firstSource.callCount(); got != 1 {
			t.Fatalf("first source calls = %d, want 1", got)
		}
		if got := secondSource.callCount(); got != 1 {
			t.Fatalf("second source calls = %d, want 1", got)
		}
	})

	t.Run("Should keep stale fallback error text stable and redact details separately", func(t *testing.T) {
		t.Parallel()

		err := &StaleFallbackError{
			SourceID: "provider_live:codex",
			Err:      errors.New("upstream failure sk-secret-value"),
		}
		if got := err.Error(); strings.Contains(got, "sk-secret-value") {
			t.Fatalf("Error() = %q, want stable redacted context", got)
		}
		if got := sourceErrorText(err); strings.Contains(got, "sk-secret-value") {
			t.Fatalf("sourceErrorText() = %q, want redacted upstream details", got)
		}
		if got := sourceErrorText(err); !strings.Contains(got, "[REDACTED]") {
			t.Fatalf("sourceErrorText() = %q, want redaction marker", got)
		}
	})
}

func TestLiveProviderSourceRegistration(t *testing.T) {
	t.Parallel()

	t.Run("Should register core live provider sources", func(t *testing.T) {
		t.Parallel()

		sources, err := NewLiveProviderSources(&LiveProviderSourcesConfig{
			Providers: map[string]compozyconfig.ProviderConfig{
				"ollama": {Command: "ollama serve"},
				"openai": {
					Command:  "openai",
					AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
					CredentialSlots: []compozyconfig.ProviderCredentialSlot{
						{Name: "api_key", TargetEnv: "OPENAI_API_KEY", SecretRef: "env:OPENAI_API_KEY", Required: true},
					},
				},
			},
			BaseEnv: []string{"PATH=/bin"},
		})
		if err != nil {
			t.Fatalf("NewLiveProviderSources() error = %v", err)
		}
		sourceIDs := make([]string, 0, len(sources))
		for _, source := range sources {
			sourceIDs = append(sourceIDs, source.ID())
		}
		for _, want := range []string{
			"provider_live:codex",
			"provider_live:cursor",
			"provider_live:claude",
			"provider_live:gemini",
			"provider_live:openrouter",
			"provider_live:vercel-ai-gateway",
			"provider_live:opencode",
			"provider_live:openclaw",
			"provider_live:hermes",
			"provider_live:pi",
			"provider_live:ollama",
			"provider_live:openai",
		} {
			if !slices.Contains(sourceIDs, want) {
				t.Fatalf("source ids = %#v, want %q registered", sourceIDs, want)
			}
		}
		var cursorSource *LiveProviderSource
		for _, source := range sources {
			if source.ID() == SourceKindProviderLiveID("cursor") {
				var ok bool
				cursorSource, ok = source.(*LiveProviderSource)
				if !ok {
					t.Fatalf("Cursor live source type = %T, want *LiveProviderSource", source)
				}
				break
			}
		}
		if cursorSource == nil {
			t.Fatal("Cursor live source = nil, want registered source")
		}
		if !cursorSource.BootstrapOnList() {
			t.Fatal("Cursor BootstrapOnList() = false, want pre-session discovery")
		}
		target, targetErr := cursorSource.discoveryTarget(cursorSource.providerSnapshot())
		if targetErr != nil {
			t.Fatalf("Cursor discoveryTarget() error = %v", targetErr)
		}
		if target.kind != liveDiscoveryCommand || target.command != cursorModelsCommand {
			t.Fatalf("Cursor discovery target = %#v, want cursor-agent models command", target)
		}
	})

	t.Run("Should derive default endpoint from versioned base URL", func(t *testing.T) {
		t.Parallel()

		provider := boundSecretProvider("OPENAI_API_KEY", "env:OPENAI_API_KEY")
		provider.BaseURL = "https://api.openai.test/v1"
		source := newLiveSourceForTest(t, "openai", provider, &LiveProviderSourcesConfig{
			BaseEnv:        []string{"PATH=/bin"},
			SecretResolver: mapSecretResolver{"env:OPENAI_API_KEY": "sk-test"},
		})
		target, err := source.discoveryTarget(source.providerSnapshot())
		if err != nil {
			t.Fatalf("discoveryTarget() error = %v", err)
		}
		if got, want := target.endpoint, "https://api.openai.test/v1/models"; got != want {
			t.Fatalf("endpoint = %q, want %q", got, want)
		}
		if got, want := source.ProviderIDs(), []string{"openai"}; !slices.Equal(got, want) {
			t.Fatalf("ProviderIDs() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should retain an owned provider config snapshot", func(t *testing.T) {
		t.Parallel()

		enabled := true
		hidden := false
		provider := boundSecretProvider("OPENAI_API_KEY", "env:OPENAI_API_KEY")
		provider.Models.Discovery.Enabled = &enabled
		provider.Models.Discovery.Endpoint = "https://api.openai.test/v1/models"
		provider.Models.Curated = []compozyconfig.ProviderModelConfig{{
			ID:     "gpt-test",
			Hidden: &hidden,
		}}
		source := newLiveSourceForTest(t, "openai", provider, &LiveProviderSourcesConfig{
			BaseEnv:        []string{"PATH=/bin"},
			SecretResolver: mapSecretResolver{"env:OPENAI_API_KEY": "sk-test"},
		})
		enabled = false
		hidden = true
		provider.Models.Discovery.Endpoint = "https://mutated.invalid/models"

		target, err := source.discoveryTarget(source.providerSnapshot())
		if err != nil {
			t.Fatalf("discoveryTarget() error = %v", err)
		}
		if got, want := target.endpoint, "https://api.openai.test/v1/models"; got != want {
			t.Fatalf("snapshot discovery endpoint = %q, want %q", got, want)
		}
		model := source.provider.Models.Curated[0]
		if model.Hidden == nil || *model.Hidden {
			t.Fatalf("snapshot curated hidden = %v, want false", model.Hidden)
		}
	})
}

func TestLiveProviderParsingHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should parse object map model payload", func(t *testing.T) {
		t.Parallel()

		rows, err := parseLiveModelPayload(
			"custom",
			[]byte(
				`{"model-a":{"display_name":"Model A","supports_tools":true,"pricing":{"cache_read":0.0000005,"cache_write":0.000003,"reasoning":0.000004}},`+
					`"model-b":{`+
					`"name":"Model B",`+
					`"reasoning_efforts":["minimal","unknown","high"],`+
					`"default_reasoning_effort":"high"`+
					`}}`,
			),
			testTime(0),
		)
		if err != nil {
			t.Fatalf("parseLiveModelPayload() error = %v", err)
		}
		if got, want := rowModelIDs(rows), []string{"model-a", "model-b"}; !slices.Equal(got, want) {
			t.Fatalf("row ids = %#v, want %#v", got, want)
		}
		if rows[0].SupportsTools == nil || !*rows[0].SupportsTools {
			t.Fatalf("SupportsTools = %v, want true", rows[0].SupportsTools)
		}
		if rows[0].CostCacheReadPerMillion == nil || *rows[0].CostCacheReadPerMillion != 0.5 ||
			rows[0].CostCacheWritePerMillion == nil || *rows[0].CostCacheWritePerMillion != 3 ||
			rows[0].CostReasoningPerMillion == nil || *rows[0].CostReasoningPerMillion != 4 {
			t.Fatalf("live five-rate pricing = %#v, want cache-read 0.5/cache-write 3/reasoning 4", rows[0])
		}
		if !slices.Equal(rows[1].ReasoningEfforts, []ReasoningEffort{ReasoningEffortMinimal, ReasoningEffortHigh}) {
			t.Fatalf("ReasoningEfforts = %#v, want minimal/high", rows[1].ReasoningEfforts)
		}
		if rows[1].DefaultReasoningEffort == nil || *rows[1].DefaultReasoningEffort != ReasoningEffortHigh {
			t.Fatalf("DefaultReasoningEffort = %v, want high", rows[1].DefaultReasoningEffort)
		}
	})

	t.Run("Should reject empty live payload", func(t *testing.T) {
		t.Parallel()

		_, err := parseLiveModelPayload("custom", []byte("  "), testTime(0))
		if err == nil {
			t.Fatal("parseLiveModelPayload(empty) error = nil, want error")
		}
	})

	t.Run("Should reject Cursor output without parseable model rows", func(t *testing.T) {
		t.Parallel()

		_, err := parseCursorModelRows("cursor", "Available models\n\nTip: use --model <id>", testTime(0))
		if err == nil || !strings.Contains(err.Error(), "cursor model command returned no model rows") {
			t.Fatalf("parseCursorModelRows() error = %v, want missing model rows", err)
		}
	})
}

func TestLiveDiscoverySupportTypes(t *testing.T) {
	t.Parallel()

	t.Run("Should apply the live refresh TTL without adding provider config state", func(t *testing.T) {
		t.Parallel()

		provider := compozyconfig.BuiltinProviders()["cursor"]
		defaultSource := newLiveSourceForTest(t, "cursor", provider, &LiveProviderSourcesConfig{})
		if got, want := defaultSource.TTL(), defaultLiveDiscoveryTTL; got != want {
			t.Fatalf("TTL() = %s, want %s", got, want)
		}

		customTTL := 37 * time.Second
		customSource := newLiveSourceForTest(t, "cursor", provider, &LiveProviderSourcesConfig{
			RefreshTTL: customTTL,
		})
		if got := customSource.TTL(); got != customTTL {
			t.Fatalf("TTL() = %s, want %s", got, customTTL)
		}
	})

	t.Run("Should resolve env secret refs", func(t *testing.T) {
		t.Parallel()

		resolver := EnvSecretResolver{LookupEnv: func(key string) (string, bool) {
			return map[string]string{"OPENAI_API_KEY": "sk-env"}[key], key == "OPENAI_API_KEY"
		}}
		value, err := resolver.ResolveRef(testutil.Context(t), "env:OPENAI_API_KEY")
		if err != nil {
			t.Fatalf("ResolveRef() error = %v", err)
		}
		if value != "sk-env" {
			t.Fatalf("value = %q, want sk-env", value)
		}
	})

	t.Run("Should reject unsupported secret refs", func(t *testing.T) {
		t.Parallel()

		_, err := (EnvSecretResolver{}).ResolveRef(testutil.Context(t), "vault:providers/openai/api_key")
		if !errors.Is(err, vault.ErrUnsupportedSecretRef) {
			t.Fatalf("ResolveRef(vault) error = %v, want ErrUnsupportedSecretRef", err)
		}
	})

	t.Run("Should run subprocess discovery command", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		result, err := ExecDiscoveryCommandExecutor{}.RunDiscoveryCommand(ctx, DiscoveryCommandRequest{
			ProviderID: "helper",
			Command:    os.Args[0],
			Args:       []string{"-test.run=TestLiveDiscoveryHelperProcess", "--", "ok"},
			Env:        append(os.Environ(), "COMPOZY_LIVE_DISCOVERY_HELPER=1"),
			Timeout:    time.Second,
		})
		if err != nil {
			t.Fatalf("RunDiscoveryCommand() error = %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
		}
		if strings.TrimSpace(result.Stdout) != `[{"id":"helper-model"}]` {
			t.Fatalf("Stdout = %q, want helper model JSON", result.Stdout)
		}
	})
}

func TestLiveDiscoveryHelperProcess(t *testing.T) {
	t.Run("Should emit the subprocess discovery fixture", func(t *testing.T) {
		if os.Getenv("COMPOZY_LIVE_DISCOVERY_HELPER") != "1" {
			return
		}
		if _, err := fmt.Fprint(os.Stdout, `[{"id":"helper-model"}]`); err != nil {
			t.Fatalf("Fprint(stdout) error = %v", err)
		}
		os.Exit(0)
	})
}

func liveJSONServer(t *testing.T, body func(*http.Request) string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprint(w, body(r))
		if err != nil {
			t.Errorf("Fprint(response) error = %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func boundSecretProvider(targetEnv string, secretRef string) compozyconfig.ProviderConfig {
	return compozyconfig.ProviderConfig{
		AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
		CredentialSlots: []compozyconfig.ProviderCredentialSlot{
			{Name: "api_key", TargetEnv: targetEnv, SecretRef: secretRef, Required: true},
		},
	}
}

func newLiveSourceForTest(
	t *testing.T,
	providerID string,
	provider compozyconfig.ProviderConfig,
	cfg *LiveProviderSourcesConfig,
) *LiveProviderSource {
	t.Helper()

	source, err := NewLiveProviderSource(providerID, provider, cfg)
	if err != nil {
		t.Fatalf("NewLiveProviderSource() error = %v", err)
	}
	return source
}

type mapSecretResolver map[string]string

func (r mapSecretResolver) ResolveRef(ctx context.Context, ref string) (string, error) {
	if ctx == nil {
		return "", errors.New("context required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized := vault.NormalizeRef(ref)
	value, ok := r[normalized]
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: %s", vault.ErrMissingSecret, normalized)
	}
	return value, nil
}

type fakeDiscoveryExecutor struct {
	mu       sync.Mutex
	result   DiscoveryCommandResult
	err      error
	requests []DiscoveryCommandRequest
}

type fakeACPModelProbe struct {
	mu       sync.Mutex
	options  []acp.SessionConfigOption
	err      error
	requests []ACPModelProbeRequest
}

func (p *fakeACPModelProbe) InspectModels(
	_ context.Context,
	req ACPModelProbeRequest,
) ([]acp.SessionConfigOption, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	return acp.CloneSessionConfigOptions(p.options), p.err
}

func (p *fakeACPModelProbe) singleRequest(t *testing.T) ACPModelProbeRequest {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.requests) != 1 {
		t.Fatalf("len(requests) = %d, want 1", len(p.requests))
	}
	return p.requests[0]
}

func (p *fakeACPModelProbe) requestCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

func (e *fakeDiscoveryExecutor) RunDiscoveryCommand(
	_ context.Context,
	req DiscoveryCommandRequest,
) (DiscoveryCommandResult, error) {
	e.mu.Lock()
	e.requests = append(e.requests, req)
	e.mu.Unlock()
	return e.result, e.err
}

func (e *fakeDiscoveryExecutor) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.requests)
}

func (e *fakeDiscoveryExecutor) singleRequest(t *testing.T) DiscoveryCommandRequest {
	t.Helper()

	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.requests) != 1 {
		t.Fatalf("len(requests) = %d, want 1", len(e.requests))
	}
	return e.requests[0]
}

type blockingProviderSource struct {
	id       string
	provider string
	started  chan struct{}
	released chan struct{}
	calls    atomic.Int64
	once     sync.Once
}

func newBlockingProviderSource(id string, provider string) *blockingProviderSource {
	return &blockingProviderSource{
		id:       id,
		provider: provider,
		started:  make(chan struct{}),
		released: make(chan struct{}),
	}
}

func (s *blockingProviderSource) ID() string {
	return s.id
}

func (s *blockingProviderSource) Kind() SourceKind {
	return SourceKindProviderLive
}

func (s *blockingProviderSource) Priority() int {
	return PriorityProviderLive
}

func (s *blockingProviderSource) CatalogExecutionFingerprint() (string, error) {
	return CatalogExecutionFingerprint(s.ID(), string(s.Kind())), nil
}

func (s *blockingProviderSource) ProviderIDs() []string {
	return []string{s.provider}
}

func (s *blockingProviderSource) ListModels(_ context.Context, _ ListOptions) ([]ModelRow, error) {
	s.calls.Add(1)
	s.once.Do(func() {
		close(s.started)
	})
	<-s.released
	available := true
	return []ModelRow{
		{
			ProviderID:  s.provider,
			ModelID:     "gpt-5.4",
			SourceID:    s.id,
			SourceKind:  SourceKindProviderLive,
			Priority:    PriorityProviderLive,
			Available:   &available,
			RefreshedAt: testTime(0),
		},
	}, nil
}

func (s *blockingProviderSource) release() {
	close(s.released)
}

func (s *blockingProviderSource) callCount() int64 {
	return s.calls.Load()
}

func waitForBlockingProviderSourceStart(t *testing.T, started <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for %s", label)
	}
}

func assertBlockingProviderSourceNotStarted(t *testing.T, started <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-started:
		t.Fatalf("%s started unexpectedly", label)
	default:
	}
}

func envValue(env []string, key string) string {
	for _, entry := range env {
		currentKey, value, ok := strings.Cut(entry, "=")
		if ok && currentKey == key {
			return value
		}
	}
	return ""
}

func requireModelRow(t *testing.T, rows []ModelRow, modelID string) ModelRow {
	t.Helper()

	for _, row := range rows {
		if row.ModelID == modelID {
			return row
		}
	}
	t.Fatalf("model rows = %#v, want model %q", rowModelIDs(rows), modelID)
	return ModelRow{}
}

func assertClaudeTransportBinding(t *testing.T, row ModelRow, transportModelID string) {
	t.Helper()

	for _, binding := range row.TransportBindings {
		if binding.TransportModelID == transportModelID {
			return
		}
	}
	t.Fatalf("row %q missing Claude transport binding %q", row.ModelID, transportModelID)
}

func assertCursorBinding(
	t *testing.T,
	row ModelRow,
	transportModelID string,
	effort ReasoningEffort,
	fast bool,
	thinking *bool,
) {
	t.Helper()

	for _, binding := range row.TransportBindings {
		if binding.TransportModelID != transportModelID {
			continue
		}
		if binding.ReasoningEffort == nil || *binding.ReasoningEffort != effort {
			t.Fatalf("binding %q reasoning = %v, want %q", transportModelID, binding.ReasoningEffort, effort)
		}
		if binding.Fast == nil || *binding.Fast != fast {
			t.Fatalf("binding %q fast = %v, want %t", transportModelID, binding.Fast, fast)
		}
		if (binding.Thinking == nil) != (thinking == nil) ||
			(binding.Thinking != nil && *binding.Thinking != *thinking) {
			t.Fatalf("binding %q thinking = %v, want %v", transportModelID, binding.Thinking, thinking)
		}
		if thinking != nil {
			if len(binding.OptionSelections) != 1 || binding.OptionSelections[0].ID != "thinking" ||
				binding.OptionSelections[0].BoolValue == nil ||
				*binding.OptionSelections[0].BoolValue != *thinking {
				t.Fatalf(
					"binding %q option selections = %#v, want thinking=%t",
					transportModelID,
					binding.OptionSelections,
					*thinking,
				)
			}
		}
		return
	}
	t.Fatalf("row %q missing transport binding %q", row.ModelID, transportModelID)
}
