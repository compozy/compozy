//go:build integration && !windows

package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// Public-entry coverage for the agy built-in: discovery without a custom
// provider block, explicit protocol-owned login, a remote readiness probe,
// and one ordinary durable prompt under the server-default model. The helper
// peer below stands in for Google's external ACP server only; discovery,
// authentication, probing, and session runtime are the real shared paths.

const (
	agyEntryPeerEnvKey = "COMPOZY_TEST_AGY_ENTRY_PEER"
	agyEntryStateKey   = "AGY_ENTRY_STATE"
	agyEntryPhaseKey   = "AGY_ENTRY_PHASE"
	agyEntryBinKey     = "AGY_ENTRY_PEER_BIN"

	agyEntryPhaseAuth    = "auth"
	agyEntryPhaseSession = "session"

	agyEntryPeerReply = "agy entry peer reply"
)

// TestAgyACPEntryPeer runs the fake Google ACP server as raw JSON-RPC. Raw
// frames keep full control over the initialize authMethods discriminator,
// which the pinned SDK agent side cannot express for a null-mcpServers auth
// probe. The peer advertises the omitted-type oauth-personal agent method,
// refuses session/new with auth-required until authenticate persists consent
// in AGY_ENTRY_STATE, then answers prompts only once the session phase
// starts.
func TestAgyACPEntryPeer(t *testing.T) {
	if os.Getenv(agyEntryPeerEnvKey) != "1" {
		return
	}
	state := strings.TrimSpace(os.Getenv(agyEntryStateKey))
	if state == "" {
		fmt.Fprintln(os.Stderr, "AGY_ENTRY_STATE is required")
		os.Exit(1)
	}
	peer := &agyEntryPeer{
		state:  state,
		phase:  strings.TrimSpace(os.Getenv(agyEntryPhaseKey)),
		out:    json.NewEncoder(os.Stdout),
		reader: bufio.NewReader(os.Stdin),
	}
	peer.serve()
	os.Exit(0)
}

type agyEntryEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type agyEntryPeer struct {
	state  string
	phase  string
	out    *json.Encoder
	reader *bufio.Reader
}

func (p *agyEntryPeer) logLine(entry string) {
	file, err := os.OpenFile(
		filepath.Join(p.state, "peer.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(file, entry)
	_ = file.Close()
}

func (p *agyEntryPeer) authed() bool {
	_, err := os.Stat(filepath.Join(p.state, "authed"))
	return err == nil
}

func (p *agyEntryPeer) respond(id json.RawMessage, result any) {
	_ = p.out.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (p *agyEntryPeer) respondError(id json.RawMessage, code int, message string) {
	_ = p.out.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": message},
	})
}

func (p *agyEntryPeer) notify(method string, params any) {
	_ = p.out.Encode(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (p *agyEntryPeer) serve() {
	for {
		line, err := p.reader.ReadBytes('\n')
		if err != nil {
			return
		}
		if strings.TrimSpace(string(line)) == "" {
			continue
		}
		var frame agyEntryEnvelope
		if err := json.Unmarshal(line, &frame); err != nil {
			continue
		}
		if len(frame.ID) == 0 {
			continue
		}
		p.dispatch(frame)
	}
}

func (p *agyEntryPeer) dispatch(frame agyEntryEnvelope) {
	switch frame.Method {
	case "initialize":
		p.logLine("initialize")
		p.respond(frame.ID, map[string]any{
			"protocolVersion": 1,
			"agentCapabilities": map[string]any{
				"loadSession":        false,
				"promptCapabilities": map[string]any{},
				"sessionCapabilities": map[string]any{
					"close": map[string]any{},
				},
			},
			"authMethods": []map[string]any{
				{"id": "oauth-personal", "name": "Personal"},
			},
		})
	case "session/new":
		if !p.authed() {
			p.logLine("session-new auth-required")
			p.respondError(frame.ID, -32000, "authentication required")
			return
		}
		p.logLine("session-new ok")
		p.respond(frame.ID, map[string]any{"sessionId": "agy-entry-sess"})
	case "authenticate":
		var params struct {
			MethodID string `json:"methodId"`
		}
		_ = json.Unmarshal(frame.Params, &params)
		p.logLine("authenticate methodId=" + params.MethodID)
		if strings.TrimSpace(params.MethodID) != "oauth-personal" {
			p.respondError(frame.ID, -32602, "unknown method")
			return
		}
		if err := os.WriteFile(filepath.Join(p.state, "authed"), []byte("1"), 0o600); err != nil {
			p.respondError(frame.ID, -32603, "persist consent")
			return
		}
		p.respond(frame.ID, map[string]any{})
	case "session/prompt":
		var params struct {
			SessionID string `json:"sessionId"`
		}
		_ = json.Unmarshal(frame.Params, &params)
		p.logLine("prompt")
		if p.phase != agyEntryPhaseSession {
			p.respondError(frame.ID, -32603, "prompt is not served during the auth phase")
			return
		}
		p.notify("session/update", map[string]any{
			"sessionId": params.SessionID,
			"update": map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{"type": "text", "text": agyEntryPeerReply},
			},
		})
		p.respond(frame.ID, map[string]any{"stopReason": "end_turn"})
	case "session/close":
		p.logLine("session-close")
		p.respond(frame.ID, map[string]any{})
	default:
		p.respondError(frame.ID, -32601, "method not found: "+frame.Method)
	}
}

type agyEntryFixture struct {
	homePaths compozyconfig.HomePaths
	cfg       compozyconfig.Config
	provider  compozyconfig.ProviderConfig
	env       []string
	stateDir  string
	cwd       string
}

func setupAgyEntryFixture(t *testing.T) *agyEntryFixture {
	t.Helper()

	stateDir := t.TempDir()
	binDir := t.TempDir()
	peerBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	shim := "#!/bin/sh\nexec \"" + peerBin + "\" -test.run=TestAgyACPEntryPeer\n"
	if err := os.WriteFile(filepath.Join(binDir, "agy_acp_server.par"), []byte(shim), 0o755); err != nil {
		t.Fatalf("write shim error = %v", err)
	}
	t.Setenv(agyEntryBinKey, peerBin)
	t.Setenv(agyEntryStateKey, stateDir)
	t.Setenv(agyEntryPhaseKey, agyEntryPhaseAuth)
	t.Setenv(agyEntryPeerEnvKey, "1")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	if _, ok := cfg.Providers["agy"]; ok {
		t.Fatal("test config must not carry a custom agy block")
	}
	provider, err := cfg.ResolveProvider("agy")
	if err != nil {
		t.Fatalf("ResolveProvider(agy) error = %v", err)
	}
	env, err := providerauth.CommandEnv(homePaths, "agy", provider, os.Environ())
	if err != nil {
		t.Fatalf("CommandEnv(agy) error = %v", err)
	}
	cwd := t.TempDir()
	return &agyEntryFixture{
		homePaths: homePaths,
		cfg:       cfg,
		provider:  provider,
		env:       env,
		stateDir:  stateDir,
		cwd:       cwd,
	}
}

func (f *agyEntryFixture) inspection() acp.SessionInspectionRequest {
	return acp.SessionInspectionRequest{
		AgentName: "agy",
		Command:   strings.TrimSpace(f.provider.Command),
		Cwd:       f.cwd,
		Env:       append([]string(nil), f.env...),
	}
}

func readAgyEntryPeerLog(t *testing.T, stateDir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stateDir, "peer.log"))
	if err != nil {
		t.Fatalf("read peer log error = %v", err)
	}
	lines := make([]string, 0)
	for line := range strings.SplitSeq(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func countAgyEntryLog(lines []string, prefix string) int {
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}

func TestAgyACPBuiltInEntry(t *testing.T) {
	// not parallel: phases share AGY_ENTRY_STATE/PATH via t.Setenv and must run in order.
	fixture := setupAgyEntryFixture(t)
	ctx := testutil.Context(t)

	t.Run("Should resolve the agy built-in without a custom block", func(t *testing.T) {
		if got := strings.TrimSpace(fixture.provider.Command); !strings.Contains(got, "agy_acp_server.par") {
			t.Fatalf("agy command = %q, want the official server executable", got)
		}
		if got := strings.TrimSpace(fixture.provider.AuthACPMethod); got != "oauth-personal" {
			t.Fatalf("agy auth method = %q, want oauth-personal", got)
		}
	})

	t.Run("Should login through shared ACP authentication", func(t *testing.T) {
		if err := acp.Authenticate(ctx, acp.AuthenticationRequest{
			SessionInspectionRequest: fixture.inspection(),
			MethodID:                 "oauth-personal",
		}); err != nil {
			t.Fatalf("Authenticate() error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(fixture.stateDir, "authed")); err != nil {
			t.Fatalf("login did not persist peer consent: %v", err)
		}
		lines := readAgyEntryPeerLog(t, fixture.stateDir)
		if countAgyEntryLog(lines, "authenticate methodId=oauth-personal") != 1 {
			t.Fatalf("peer log = %q, want exactly one oauth-personal authenticate", lines)
		}
		if countAgyEntryLog(lines, "prompt") != 0 {
			t.Fatalf("peer log = %q, want no prompt during login", lines)
		}
	})

	t.Run("Should verify readiness with a remote probe", func(t *testing.T) {
		before := countAgyEntryLog(readAgyEntryPeerLog(t, fixture.stateDir), "authenticate")
		if err := acp.ProbeAuthentication(ctx, fixture.inspection()); err != nil {
			t.Fatalf("ProbeAuthentication() error = %v", err)
		}
		lines := readAgyEntryPeerLog(t, fixture.stateDir)
		if got := countAgyEntryLog(lines, "authenticate"); got != before {
			t.Fatalf("probe sent authenticate (before=%d after=%d), want none", before, got)
		}
		if countAgyEntryLog(lines, "prompt") != 0 {
			t.Fatalf("peer log = %q, want no prompt during probe", lines)
		}
	})

	t.Run("Should persist an ordinary prompt under agy", func(t *testing.T) {
		t.Setenv(agyEntryPhaseKey, agyEntryPhaseSession)
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
		resolvedWorkspace := workspacepkg.ResolvedWorkspace{
			ID:      "ws-agy-entry",
			RootDir: workspaceRoot,
			Name:    "workspace",
			Config:  fixture.cfg,
			Agents: []compozyconfig.AgentDef{{
				Name:     "coder",
				Provider: "agy",
				Prompt:   "You are a coding assistant.",
			}},
		}
		driver := session.NewACPDriverAdapter(acp.New(acp.WithProviderPreStarter(daemonInstance.providerPreStarter)))
		manager := newHarnessIntegrationManager(
			t,
			fixture.homePaths,
			capturedDeps,
			resolvedWorkspace,
			driver,
		)
		created, err := manager.Create(testutil.Context(t), new(session.CreateOpts{
			AgentName: "coder",
			Name:      "agy entry session",
			Workspace: resolvedWorkspace.ID,
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
		var text strings.Builder
		timeout := time.After(60 * time.Second)
	collect:
		for {
			select {
			case event, ok := <-events:
				if !ok {
					break collect
				}
				text.WriteString(event.Text)
			case <-timeout:
				t.Fatal("Prompt() events did not complete")
			}
		}
		if got := text.String(); !strings.Contains(got, agyEntryPeerReply) {
			t.Fatalf("prompt text = %q, want peer reply", got)
		}
		if _, ok := manager.Get(created.ID); !ok {
			t.Fatal("created session is not retained after prompt")
		}
	})
}
