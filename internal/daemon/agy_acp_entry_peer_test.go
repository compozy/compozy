//go:build integration && !windows

package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
	shellquote "github.com/kballard/go-shellquote"
)

// Fixture for the agy native-entry integration test: the helper peer stands in
// for Google's installed CLI only. It advertises the CLI 1.5.9 shape (logical
// model values, model-specific thought levels, and the four native modes), and
// the PATH shim below records the exact argv it is launched with. Discovery,
// authentication, probing, and session runtime stay real.

const (
	agyEntryPeerEnvKey = "COMPOZY_TEST_AGY_ENTRY_PEER"
	agyEntryStateKey   = "AGY_ENTRY_STATE"
	agyEntryPhaseKey   = "AGY_ENTRY_PHASE"

	agyEntryPhaseAuth    = "auth"
	agyEntryPhaseSession = "session"

	agyEntryPeerReply   = "agy entry peer reply"
	agyEntryHydrogenID  = "gemini-4-hydrogen"
	agyEntryXHighEffort = "xhigh"
)

// agyEntryModes mirrors the CLI 1.5.9 advertisement: no `yolo` exists.
var agyEntryModes = []string{"default", "accept-edits", "auto", "plan"}

// agyEntryModels mirrors the CLI 1.5.9 logical `model` config values and their
// model-specific thought levels.
var agyEntryModels = []struct {
	ID      string
	Label   string
	Efforts []string
	Current string
}{
	{"gemini-4-argon", "Gemini 4 Argon", []string{"low", "medium", "high"}, "medium"},
	{"gemini-4-hydrogen", "Gemini 4 Hydrogen", []string{"low", "medium", "high", "xhigh"}, "medium"},
	{"gemini-pro-orchestrator-g1-pro", "Gemini Pro Orchestrator", []string{"low", "medium", "high"}, "medium"},
	{"gemini-3.8-flash", "Gemini 3.8 Flash", []string{"low", "medium", "high"}, "high"},
	{"claude-opus-5-5", "Claude Opus 5.5", []string{"low", "medium", "high"}, "medium"},
	{"claude-sonnet-5-5", "Claude Sonnet 5.5", []string{"low", "high"}, "high"},
	{"gpt-oss-120b-medium", "GPT-OSS 120B", []string{"low", "medium", "high"}, "medium"},
}

// TestAgyACPEntryPeer runs the fake Antigravity CLI as raw JSON-RPC. Raw frames
// keep full control over the initialize authMethods discriminator and the
// model-specific thought levels. The peer advertises the omitted-type
// oauth-personal agent method, refuses session/new with auth-required until
// authenticate persists consent in AGY_ENTRY_STATE, refuses a Hydrogen turn
// whose fourth level was not confirmed, and answers prompts only once the
// session phase starts.
//
// This is a helper-process entry, not a behavior case: the integration test
// re-executes it as the CLI subprocess through the `agy` shim, and every
// assertion lives in TestAgyACPBuiltInEntry. It deliberately has no t.Run
// subtests because the harness needs one process that serves the whole session.
func TestAgyACPEntryPeer(_ *testing.T) {
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
		model:  agyEntryModels[0].ID,
		effort: agyEntryModels[0].Current,
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
	model  string
	effort string
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
		p.abort(err)
	}
	if _, err := fmt.Fprintln(file, entry); err != nil {
		p.abort(err)
	}
	if err := file.Close(); err != nil {
		p.abort(err)
	}
}

// abort fails the launched CLI loudly. A dropped log line or frame must surface
// as a broken fixture instead of silently faking success.
func (p *agyEntryPeer) abort(err error) {
	fmt.Fprintln(os.Stderr, "agy entry peer:", err)
	os.Exit(1)
}

func (p *agyEntryPeer) encode(payload map[string]any) {
	if err := p.out.Encode(payload); err != nil {
		p.abort(err)
	}
}

func (p *agyEntryPeer) respond(id json.RawMessage, result any) {
	p.encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (p *agyEntryPeer) respondError(id json.RawMessage, code int, message string) {
	p.encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": message},
	})
}

func (p *agyEntryPeer) notify(method string, params any) {
	p.encode(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// decodeParams answers malformed client params with a JSON-RPC error so the
// caller observes the failure instead of a silently zeroed request.
func (p *agyEntryPeer) decodeParams(frame agyEntryEnvelope, target any) bool {
	if err := json.Unmarshal(frame.Params, target); err != nil {
		p.logLine("invalid-params " + frame.Method)
		p.respondError(frame.ID, -32602, "invalid params: "+err.Error())
		return false
	}
	return true
}

func (p *agyEntryPeer) authed() bool {
	_, err := os.Stat(filepath.Join(p.state, "authed"))
	return err == nil
}

func (p *agyEntryPeer) serve() {
	for {
		line, err := p.reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			// Clean shutdown: the client closed stdin.
			return
		}
		if err != nil {
			p.abort(fmt.Errorf("read frame: %w", err))
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
		p.sessionNew(frame)
	case "authenticate":
		var params struct {
			MethodID string `json:"methodId"`
		}
		if !p.decodeParams(frame, &params) {
			return
		}
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
	case "session/set_mode":
		var params struct {
			ModeID string `json:"modeId"`
		}
		if !p.decodeParams(frame, &params) {
			return
		}
		if !slices.Contains(agyEntryModes, params.ModeID) {
			p.logLine("mode-rejected " + params.ModeID)
			p.respondError(frame.ID, -32602, "unsupported mode "+params.ModeID)
			return
		}
		p.logLine("mode=" + params.ModeID)
		p.respond(frame.ID, map[string]any{})
	case "session/set_config_option":
		var params struct {
			ConfigID string `json:"configId"`
			Value    string `json:"value"`
		}
		if !p.decodeParams(frame, &params) {
			return
		}
		p.setConfigOption(frame, params.ConfigID, params.Value)
	case "session/prompt":
		var params struct {
			SessionID string `json:"sessionId"`
		}
		if !p.decodeParams(frame, &params) {
			return
		}
		if !p.authed() {
			p.logLine("prompt auth-required")
			p.respondError(frame.ID, -32000, "authentication required")
			return
		}
		if p.phase != agyEntryPhaseSession {
			p.logLine("prompt auth-phase-rejected")
			p.respondError(frame.ID, -32603, "prompt is not served during the auth phase")
			return
		}
		// The selected model's advertised fourth level must be confirmed before
		// the turn runs; a silent downgrade is refused by the peer itself.
		if p.model == agyEntryHydrogenID && p.effort != agyEntryXHighEffort {
			p.logLine("prompt-unconfirmed")
			p.respondError(frame.ID, -32603, "prompt refused: Hydrogen xhigh was not confirmed")
			return
		}
		p.logLine("prompt")
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

func (p *agyEntryPeer) sessionNew(frame agyEntryEnvelope) {
	if !p.authed() {
		p.logLine("session-new auth-required")
		p.respondError(frame.ID, -32000, "authentication required")
		return
	}
	p.logLine("session-new ok")
	availableModes := make([]map[string]string, 0, len(agyEntryModes))
	for _, mode := range agyEntryModes {
		availableModes = append(availableModes, map[string]string{"id": mode, "name": mode})
	}
	p.respond(frame.ID, map[string]any{
		"sessionId": "agy-entry-sess",
		"modes": map[string]any{
			"currentModeId":  agyEntryModes[0],
			"availableModes": availableModes,
		},
		"configOptions": p.configOptions(),
	})
}

func (p *agyEntryPeer) configOptions() []map[string]any {
	models := make([]map[string]string, 0, len(agyEntryModels))
	for _, model := range agyEntryModels {
		models = append(models, map[string]string{"value": model.ID, "name": model.Label})
	}
	levels := agyEntryEfforts(p.model)
	thoughts := make([]map[string]string, 0, len(levels))
	for _, level := range levels {
		thoughts = append(thoughts, map[string]string{"value": level, "name": level})
	}
	return []map[string]any{
		{
			"type": "select", "id": "model", "name": "Model", "category": "model",
			"currentValue": p.model, "options": models,
		},
		{
			"type": "select", "id": "thought_level", "name": "Thinking", "category": "thought_level",
			"currentValue": p.effort, "options": thoughts,
		},
	}
}

func (p *agyEntryPeer) setConfigOption(frame agyEntryEnvelope, configID, value string) {
	switch configID {
	case "model":
		entry, ok := agyEntryModel(value)
		if !ok {
			p.logLine("model-rejected " + value)
			p.respondError(frame.ID, -32602, "unsupported model "+value)
			return
		}
		p.model, p.effort = entry.ID, entry.Current
		p.logLine("model=" + value)
	case "thought_level":
		if !slices.Contains(agyEntryEfforts(p.model), value) {
			p.logLine("thought-level-rejected " + value)
			p.respondError(
				frame.ID,
				-32602,
				fmt.Sprintf("Thinking level %q is not offered for model %q", value, p.model),
			)
			return
		}
		p.effort = value
		p.logLine("thought_level=" + value)
	default:
		p.respondError(frame.ID, -32602, "unsupported config option "+configID)
		return
	}
	p.respond(frame.ID, map[string]any{"configOptions": p.configOptions()})
}

func agyEntryModel(modelID string) (struct {
	ID      string
	Label   string
	Efforts []string
	Current string
}, bool) {
	for _, model := range agyEntryModels {
		if model.ID == modelID {
			return model, true
		}
	}
	return agyEntryModels[0], false
}

func agyEntryEfforts(modelID string) []string {
	model, ok := agyEntryModel(modelID)
	if !ok {
		return nil
	}
	return model.Efforts
}

type agyEntryFixture struct {
	homePaths compozyconfig.HomePaths
	cfg       compozyconfig.Config
	provider  compozyconfig.ProviderConfig
	env       []string
	stateDir  string
	binDir    string
	cwd       string
}

// not parallel: the fixture rewrites PATH and the launched CLI environment for
// the single integration journey that starts it.
func setupAgyEntryFixture(t *testing.T) *agyEntryFixture {
	t.Helper()

	stateDir := t.TempDir()
	binDir := t.TempDir()
	peerBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	// The PATH shim must be named `agy` and accept exactly the single `acp`
	// argument, so a standalone server, an extra flag, or a planted `--acp`
	// spelling cannot pass as the native entry. Every invocation is recorded in
	// launch.log before validation, so a rejected launch never reads as no launch.
	shim := strings.Join([]string{
		"#!/bin/sh",
		`printf 'argc=%s argv=%s\n' "$#" "$*" >> ` + shellquote.Join(filepath.Join(stateDir, "launch.log")),
		`if [ "$#" -ne 1 ] || [ "$1" != "acp" ]; then`,
		`  echo "agy shim: unexpected argv: $*" >&2`,
		"  exit 64",
		"fi",
		`printf '%s\n' "$*" >> ` + shellquote.Join(filepath.Join(stateDir, "argv.log")),
		"exec " + shellquote.Join(peerBin, "-test.run=^TestAgyACPEntryPeer$"),
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(binDir, "agy"), []byte(shim), 0o755); err != nil {
		t.Fatalf("write agy shim error = %v", err)
	}
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
	if got := strings.TrimSpace(provider.Command); got != "agy acp" {
		t.Fatalf("agy command = %q, want the native CLI ACP entry", got)
	}
	env, err := providerauth.CommandEnv(homePaths, "agy", provider, os.Environ())
	if err != nil {
		t.Fatalf("CommandEnv(agy) error = %v", err)
	}
	return &agyEntryFixture{
		homePaths: homePaths,
		cfg:       cfg,
		provider:  provider,
		env:       env,
		stateDir:  stateDir,
		binDir:    binDir,
		cwd:       t.TempDir(),
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

func setAgyEntryAuthed(t *testing.T, stateDir string, authed bool) {
	t.Helper()

	path := filepath.Join(stateDir, "authed")
	if authed {
		if err := os.WriteFile(path, []byte("1"), 0o600); err != nil {
			t.Fatalf("write consent marker error = %v", err)
		}
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove consent marker error = %v", err)
	}
}

func readAgyEntryLog(t *testing.T, stateDir string, name string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(stateDir, name))
	if err != nil {
		t.Fatalf("read %s error = %v", name, err)
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

func agyEntryLogIndex(t *testing.T, lines []string, prefix string) int {
	t.Helper()

	for index, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return index
		}
	}
	t.Fatalf("peer log = %q, want an entry starting with %q", lines, prefix)
	return -1
}
