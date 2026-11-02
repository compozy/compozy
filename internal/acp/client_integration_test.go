//go:build integration

package acp

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/workspacedb"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	terminaljournal "github.com/compozy/compozy/internal/terminal/journal"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	shellquote "github.com/kballard/go-shellquote"
)

func TestACPIntegrationReadTextFileRequest(t *testing.T) {
	t.Run("Should read file content through ACP file callback", func(t *testing.T) {
		t.Parallel()

		driver := New()

		root := t.TempDir()
		target := filepath.Join(root, "notes.txt")
		if err := os.WriteFile(target, []byte("from-disk"), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		proc := startHelperProcess(t, driver, "fs_read", target, StartOpts{
			Cwd:         root,
			Permissions: compozyconfig.PermissionModeApproveReads,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-integration-fs",
			Message: "read file",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if !containsEventText(events, "from-disk") {
			t.Fatalf("Prompt() events = %#v, want file content", events)
		}
	})
}

func TestACPIntegrationToolHostFileWriteReadAndTerminal(t *testing.T) {
	t.Run("Should write read and run terminal commands through the local tool host", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		driver, workspaceID := newIntegrationTerminalDriver(t, root)
		target := filepath.Join(root, "created.txt")
		proc := startHelperProcess(t, driver, "fs_write_terminal", target, StartOpts{
			WorkspaceID: workspaceID,
			ProfileID:   store.DefaultProfileID,
			Cwd:         root,
			Permissions: compozyconfig.PermissionModeApproveAll,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			RunID: "run-terminal-integration", Generation: 1,
			TurnID:  "turn-integration-toolhost",
			Message: "exercise tool host",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if !containsEventText(events, "from-write") {
			t.Fatalf("Prompt() events = %#v, want written file content", events)
		}
		if !containsEventText(events, "terminal-ok") {
			t.Fatalf("Prompt() events = %#v, want terminal output", events)
		}
		content, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", target, err)
		}
		if string(content) != "from-write" {
			t.Fatalf("written file content = %q, want %q", content, "from-write")
		}
	})
}

func TestACPIntegrationRequestPermissionPolicy(t *testing.T) {
	t.Run("Should allow an interactive edit permission request", func(t *testing.T) {
		t.Parallel()

		driver := New()

		root := t.TempDir()
		target := filepath.Join(root, "danger.txt")
		proc := startHelperProcess(t, driver, "permission", target, StartOpts{
			Cwd:         root,
			Permissions: compozyconfig.PermissionModeApproveReads,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-integration-permission",
			Message: "request permission",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := make([]AgentEvent, 0, 8)
		var pendingRequestID string
		timeout := time.NewTimer(5 * time.Second)
		defer timeout.Stop()

		for {
			select {
			case event, ok := <-eventsCh:
				if !ok {
					goto done
				}
				events = append(events, event)
				if event.Type == EventTypePermission && event.Decision == "" && pendingRequestID == "" {
					pendingRequestID = event.RequestIDValue()
					if pendingRequestID == "" {
						t.Fatal("permission request_id = empty, want non-empty")
					}
					if err := driver.ApprovePermission(testutil.Context(t), proc, ApproveRequest{
						RequestID: pendingRequestID,
						Decision:  string(decisionAllowAlways),
					}); err != nil {
						t.Fatalf("ApprovePermission() error = %v", err)
					}
				}
			case <-timeout.C:
				t.Fatalf("timed out waiting for prompt events; collected %#v", events)
			}
		}
	done:

		permissionEvents := make([]AgentEvent, 0, len(events))
		for _, event := range events {
			if event.Type == EventTypePermission {
				permissionEvents = append(permissionEvents, event)
			}
		}
		if len(permissionEvents) == 0 {
			t.Fatalf("Prompt() events = %#v, want permission event", events)
		}
		if permissionEvents[0].Decision != "" {
			t.Fatalf("initial permission event decision = %q, want empty", permissionEvents[0].Decision)
		}
		if permissionEvents[len(permissionEvents)-1].Decision != string(decisionAllowAlways) {
			t.Fatalf(
				"final permission event decision = %q, want %q",
				permissionEvents[len(permissionEvents)-1].Decision,
				decisionAllowAlways,
			)
		}
		if !containsEventText(events, "allow-always") {
			t.Fatalf("Prompt() events = %#v, want approved permission outcome", events)
		}
	})
}

func TestACPIntegrationRequestPermissionTimeout(t *testing.T) {
	t.Run("Should reject an interactive edit permission request after timeout", func(t *testing.T) {
		t.Parallel()

		driver := New(WithPermissionTimeout(25 * time.Millisecond))

		root := t.TempDir()
		target := filepath.Join(root, "danger.txt")
		proc := startHelperProcess(t, driver, "permission", target, StartOpts{
			Cwd:         root,
			Permissions: compozyconfig.PermissionModeApproveReads,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-integration-timeout",
			Message: "request permission",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if !containsEventText(events, "reject-once") {
			t.Fatalf("Prompt() events = %#v, want reject-once permission outcome", events)
		}

		permissionEvents := make([]AgentEvent, 0, len(events))
		for _, event := range events {
			if event.Type == EventTypePermission {
				permissionEvents = append(permissionEvents, event)
			}
		}
		if len(permissionEvents) < 2 {
			t.Fatalf("Prompt() permission events = %#v, want initial and final permission events", permissionEvents)
		}
		if permissionEvents[len(permissionEvents)-1].Decision != string(decisionRejectOnce) {
			t.Fatalf(
				"final permission event decision = %q, want %q",
				permissionEvents[len(permissionEvents)-1].Decision,
				decisionRejectOnce,
			)
		}
		if got := permissionEvents[len(permissionEvents)-1].ResolvedByValue(); got != "timeout" {
			t.Fatalf("final permission resolver = %q, want timeout", got)
		}
		if proc.HasPendingPermission() {
			t.Fatal("timed out permission remained pending")
		}
	})
}

func containsEventText(events []AgentEvent, want string) bool {
	for _, event := range events {
		if event.Text == want {
			return true
		}
	}
	return false
}

func newIntegrationTerminalDriver(t *testing.T, root string) (*Driver, string) {
	t.Helper()
	identity, err := workspacepkg.EnsureIdentity(t.Context(), root)
	if err != nil {
		t.Fatalf("EnsureIdentity() error = %v", err)
	}
	homeDir := t.TempDir()
	pool, err := workspacedb.NewPool(homeDir, func(context.Context, string) (workspacedb.ResolvedRoot, error) {
		return workspacedb.ResolvedRoot{RootDir: root, WorkspaceID: identity.WorkspaceID}, nil
	})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(context.Background()); err != nil {
			t.Errorf("Close(workspace pool): %v", err)
		}
	})
	journal, err := terminaljournal.New(t.Context(), terminaljournal.Options{Databases: pool, HomeDir: homeDir})
	if err != nil {
		t.Fatalf("NewJournal() error = %v", err)
	}
	manager, err := terminalpkg.NewManager(terminalpkg.WithJournal(journal))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := manager.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	return New(WithTerminalManager(manager)), identity.WorkspaceID
}

// Invariant: a real cold npm install and a canceled install recover before ACP initialize.
// Owner: ACP launcher boundary; opt-in external integration suite.
func TestACPIntegrationNPXColdCache(t *testing.T) {
	t.Run("Should recover an interrupted cold install and initialize the real adapter", func(t *testing.T) {
		t.Parallel()
		if os.Getenv("COMPOZY_TEST_NPX_COLD") != "1" {
			t.Skip("set COMPOZY_TEST_NPX_COLD=1 to allow an isolated npm registry install")
		}
		root := t.TempDir()
		env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root,
			"npm_config_cache=" + filepath.Join(root, "npm-cache"), "npm_config_update_notifier=false"}
		launcher := newLocalLauncher(testDiscardLogger(), time.Second, 0)
		spec, err := launcher.PrepareLaunch(t.Context(), LaunchSpec{
			Command: "npx -y @agentclientprotocol/codex-acp@latest", Cwd: root, Env: env,
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		finished := make(chan error, 1)
		go func() { finished <- launcher.prepareNPX(ctx, spec) }()
		deadline := time.NewTimer(time.Minute)
		defer deadline.Stop()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
	waitForLock:
		for {
			select {
			case err := <-finished:
				t.Fatalf("installer exited before its lock could be interrupted: %v", err)
			case <-deadline.C:
				cancel()
				err := <-finished
				t.Fatalf("npm install lock was not observed: %v", err)
			case <-ticker.C:
				locks, err := filepath.Glob(filepath.Join(root, "npm-cache", "_npx", "*", "concurrency.lock"))
				if err != nil {
					cancel()
					cleanupErr := <-finished
					t.Fatalf("lock lookup = %v; cleanup = %v", err, cleanupErr)
				}
				if len(locks) > 0 {
					break waitForLock
				}
			}
		}
		cancel()
		if err := <-finished; !errors.Is(err, context.Canceled) {
			t.Fatalf("interrupted install = %v, want caller cancellation", err)
		}
		t.Log("interrupted a real npm installation while its concurrency lock existed")
		driver := New(WithStopTimeout(time.Second))
		started := time.Now()
		proc, err := driver.launchAgentProcess(t.Context(), StartOpts{
			AgentName: "cold-cache", Command: spec.Command, Cwd: root, Env: env,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer stopProcess(t, driver, proc)
		t.Logf("recovered adapter launch after interrupted install in %s", time.Since(started))
		started = time.Now()
		if err := driver.initializeConnection(t.Context(), proc, "cold-cache"); err != nil {
			t.Fatal(err)
		}
		t.Logf("real ACP initialize completed in %s", time.Since(started))
	})
}

// Invariant: a local binary need not share the name of its supplying npm package.
// Owner: local launcher; canonical real npm integration suite.
func TestACPIntegrationNPXLocalBinary(t *testing.T) {
	t.Run("Should launch a differently named local package binary without resolution", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("Unix executable fixture; subprocess code is cross-built for Windows")
		}
		npx, err := exec.LookPath("npx")
		if err != nil {
			t.Skip("npx is unavailable")
		}
		root := t.TempDir()
		binDir := filepath.Join(root, "node_modules", ".bin")
		packageDir := filepath.Join(root, "node_modules", "different-package")
		if err := os.MkdirAll(binDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(packageDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(packageDir, "package.json"),
			[]byte(`{"name":"different-package","bin":{"my-adapter":"adapter"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		adapter := filepath.Join(packageDir, "adapter")
		if err := os.WriteFile(adapter, []byte("#!/bin/sh\nprintf local-adapter-ok\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(adapter, filepath.Join(binDir, "my-adapter")); err != nil {
			t.Fatal(err)
		}
		for _, flags := range [][]string{{"--no-install"}, {"--offline", "-y"}} {
			launcher := newLocalLauncher(testDiscardLogger(), time.Second, 0)
			args := append(flags, "my-adapter")
			handle, err := launcher.Launch(t.Context(), LaunchSpec{
				Command: shellquote.Join(append([]string{npx}, args...)...), Cwd: root,
				Env: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root,
					"npm_config_cache=" + filepath.Join(root, "cache")},
			})
			if err != nil {
				t.Fatal(err)
			}
			output, readErr := io.ReadAll(handle.Stdout())
			waitErr := handle.Wait()
			if readErr != nil || waitErr != nil || string(output) != "local-adapter-ok" {
				t.Fatalf("local launch = %q, read %v, wait %v", output, readErr, waitErr)
			}
		}
	})
}
