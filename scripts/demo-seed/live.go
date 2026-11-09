package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kballard/go-shellquote"

	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/fileutil"
)

const (
	liveRearmInterval = 20 * time.Second
	liveCommandLimit  = 60 * time.Second
	liveStopLimit     = 30 * time.Second
	liveLockName      = "demo-live.lock"
	liveOwnedName     = "demo-live-sessions.json"
)

// liveOptions locates the binaries the live companion drives.
type liveOptions struct {
	home, configFile, compozy, mockDriver, webDist string
}

// liveRun owns the daemon and prompt clients for one live session set.
type liveRun struct {
	options liveOptions
	output  io.Writer
	mu      sync.Mutex
	prompts map[*exec.Cmd]struct{}
	ids     map[string]string
	created []string
}

// runLive points the seeded home at the mock provider, starts the daemon, and holds the live
// states until ctx ends. The home's config is restored on the way out.
func runLive(ctx context.Context, options liveOptions, output io.Writer) (err error) {
	if err := requireSeededHome(options.home); err != nil {
		return err
	}
	lock, err := daemon.AcquireLock(filepath.Join(options.home, liveLockName), os.Getpid())
	if err != nil {
		return fmt.Errorf("demo seed: another --live run owns %s: %w", options.home, err)
	}
	defer func() { err = errors.Join(err, lock.Release()) }()
	run := &liveRun{
		options: options, output: output, ids: map[string]string{}, prompts: map[*exec.Cmd]struct{}{},
	}
	restore, err := run.configureMockProviders()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, restore()) }()
	stopCtx := context.WithoutCancel(ctx)
	// A home whose daemon is not running has nothing to stop.
	if _, err := run.compozyOutput(stopCtx, "daemon", "stop"); err != nil {
		fmt.Fprintln(output, "demo seed: no daemon to stop before starting")
	}
	// Armed before start, so an interrupted startup still stops a daemon on the mock config.
	defer run.shutdown(stopCtx)
	if _, err := run.compozyOutput(ctx, "daemon", "start"); err != nil {
		return fmt.Errorf("demo seed: start daemon: %w", err)
	}
	run.archivePreviousSessions(ctx)
	for _, scenario := range liveScenarios {
		if err := run.startScenario(ctx, scenario); err != nil {
			return err
		}
	}
	fmt.Fprintf(output, "Live states are running against %s. Press Ctrl-C to stop.\n", options.home)
	return run.holdUntilDone(ctx)
}

// requireSeededHome refuses a home the seed does not own, since live mode replaces its providers.
func requireSeededHome(home string) error {
	marker := filepath.Join(home, "workspaces", "northstar-pay", "launch-hq", ".compozy", "demo-seed.json")
	if _, err := os.Stat(marker); err != nil {
		return fmt.Errorf("demo seed: --live needs a seeded home; run demo-seed against %s first: %w", home, err)
	}
	return nil
}

func nestedTable(root map[string]any, keys ...string) map[string]any {
	current := root
	for _, key := range keys {
		next, ok := current[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[key] = next
		}
		current = next
	}
	return current
}

// configureMockProviders routes the seeded agents' providers to the mock and returns the undo.
func (r *liveRun) configureMockProviders() (func() error, error) {
	fixturePath := filepath.Join(r.options.home, "demo-live-fixture.json")
	fixture, err := mockFixture()
	if err != nil {
		return nil, fmt.Errorf("demo seed: build live fixture: %w", err)
	}
	// #nosec G703 -- the operator chose this seeded home on the command line.
	if err := os.WriteFile(fixturePath, fixture, 0o600); err != nil {
		return nil, fmt.Errorf("demo seed: write live fixture: %w", err)
	}
	overrides := []liveOverride{}
	for provider, agent := range map[string]string{"claude": mockClaudeAgent, "codex": mockCodexAgent} {
		overrides = append(overrides, liveOverride{
			table: []string{"providers", provider}, key: "command",
			injected: mockProviderCommand(r.options.mockDriver, fixturePath, agent, provider),
		})
	}
	return applyLiveConfig(r.options.configFile, append(overrides, liveDefaults(r.options.configFile)...))
}

// liveDefaults names a default provider for bundled agents unless the home already has one.
func liveDefaults(path string) []liveOverride {
	// #nosec G304,G703 -- the seeded home's own config file.
	raw, err := os.ReadFile(path)
	if err == nil {
		if values, decodeErr := decodeConfig(raw); decodeErr == nil {
			if table, ok := lookupTable(values, "defaults"); ok && table["provider"] != nil {
				return nil
			}
		}
	}
	return []liveOverride{
		{table: []string{"defaults"}, key: "provider", injected: "claude"},
		{table: []string{"defaults"}, key: "model", injected: mockClaudeModel},
	}
}

func mockProviderCommand(driver, fixture, agent, provider string) string {
	return shellquote.Join(driver, "--fixture", fixture, "--agent", agent, "--provider", provider)
}

// command builds a CLI call free of any session identity inherited from a calling agent.
func (r *liveRun) command(ctx context.Context, args ...string) *exec.Cmd {
	// #nosec G204 -- the operator names this binary on the command line.
	cmd := exec.CommandContext(ctx, r.options.compozy, args...)
	cmd.Env = []string{
		"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH"), "COMPOZY_HOME=" + r.options.home,
		"COMPOZY_WEB_DIST_DIR=" + r.options.webDist,
	}
	return cmd
}

func (r *liveRun) compozyOutput(ctx context.Context, args ...string) ([]byte, error) {
	limited, cancel := context.WithTimeout(ctx, liveCommandLimit)
	defer cancel()
	output, err := r.command(limited, args...).CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("compozy %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(output))
	}
	return output, nil
}

func (r *liveRun) startScenario(ctx context.Context, scenario liveScenario) error {
	id := scenario.existing
	if id == "" {
		args := []string{"session", "new", "--agent", scenario.agent, "--workspace", scenario.project,
			"--name", scenario.name, "--json"}
		if scenario.parent != "" {
			args = append(args, "--parent", r.ids[scenario.parent])
		}
		output, err := r.compozyOutput(ctx, args...)
		if err != nil {
			return fmt.Errorf("demo seed: create %q: %w", scenario.key, err)
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(output, &created); err != nil || created.ID == "" {
			return fmt.Errorf("demo seed: read created session %q: %w: %s", scenario.key, err, output)
		}
		id = created.ID
		if err := r.recordCreated(id); err != nil {
			return err
		}
	}
	r.ids[scenario.key] = id
	return r.prompt(ctx, id, scenario.prompt)
}

// prompt leaves the CLI client attached in the background; the turn belongs to the daemon.
func (r *liveRun) prompt(ctx context.Context, id, text string) error {
	cmd := r.command(ctx, "session", "prompt", id, text)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("demo seed: prompt session %s: %w", id, err)
	}
	r.mu.Lock()
	r.prompts[cmd] = struct{}{}
	r.mu.Unlock()
	go func() {
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			fmt.Fprintln(r.output, "demo seed: prompt client ended:", err)
		}
		r.mu.Lock()
		delete(r.prompts, cmd)
		r.mu.Unlock()
	}()
	return nil
}

// recordCreated remembers a session this driver made, the only ones a later run may archive.
func (r *liveRun) recordCreated(id string) error {
	r.created = append(r.created, id)
	encoded, err := json.Marshal(r.created)
	if err != nil {
		return fmt.Errorf("demo seed: encode live sessions: %w", err)
	}
	if err := fileutil.AtomicWritePrivateFile(filepath.Join(r.options.home, liveOwnedName), encoded); err != nil {
		return fmt.Errorf("demo seed: record live session: %w", err)
	}
	return nil
}

// archivePreviousSessions clears the last run's sessions so a restart does not pile up copies.
func (r *liveRun) archivePreviousSessions(ctx context.Context) {
	raw, err := os.ReadFile(filepath.Join(r.options.home, liveOwnedName))
	if err != nil {
		return
	}
	var previous []string
	if err := json.Unmarshal(raw, &previous); err != nil {
		fmt.Fprintln(r.output, "demo seed: ignore unreadable live session list:", err)
		return
	}
	for _, id := range previous {
		if _, err := r.compozyOutput(ctx, "session", "archive", id, "--stop"); err != nil {
			// Kept in the owned list, so the next run tries this session again.
			r.created = append(r.created, id)
			fmt.Fprintln(r.output, "demo seed: archive previous live session:", err)
		}
	}
}

// expectedBadge is the state a re-armable scenario holds; waits and delegations are never re-sent.
func expectedBadge(behavior string) string {
	switch behavior {
	case liveHold:
		return "running"
	case liveAsk:
		return "waiting-for-auth"
	case liveClarify:
		return "waiting-for-input"
	default:
		return ""
	}
}

func (r *liveRun) badges(ctx context.Context) (map[string]string, error) {
	output, err := r.compozyOutput(ctx, "session", "list", "--all-workspaces", "--json")
	if err != nil {
		return nil, err
	}
	var listed struct {
		Sessions []struct {
			ID    string `json:"id"`
			Badge string `json:"badge"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(output, &listed); err != nil {
		return nil, fmt.Errorf("demo seed: read session list: %w", err)
	}
	badges := make(map[string]string, len(listed.Sessions))
	for _, session := range listed.Sessions {
		badges[session.ID] = session.Badge
	}
	return badges, nil
}

// holdUntilDone re-sends a prompt when its state ended; permission and clarify asks time out.
func (r *liveRun) holdUntilDone(ctx context.Context) error {
	ticker := time.NewTicker(liveRearmInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		badges, err := r.badges(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			fmt.Fprintln(r.output, "demo seed: live check failed:", err)
			continue
		}
		for _, scenario := range liveScenarios {
			expected := expectedBadge(scenario.behavior)
			if expected == "" || badges[r.ids[scenario.key]] == expected {
				continue
			}
			if err := r.prompt(ctx, r.ids[scenario.key], scenario.prompt); err != nil {
				fmt.Fprintln(r.output, err)
			}
		}
	}
}

func (r *liveRun) shutdown(ctx context.Context) {
	r.mu.Lock()
	for cmd := range r.prompts {
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				fmt.Fprintln(r.output, "demo seed: stop prompt client:", err)
			}
		}
	}
	r.mu.Unlock()
	limited, cancel := context.WithTimeout(ctx, liveStopLimit)
	defer cancel()
	if output, err := r.command(limited, "daemon", "stop").CombinedOutput(); err != nil {
		fmt.Fprintf(r.output, "demo seed: stop daemon: %v: %s\n", err, output)
	}
}
