//go:build integration

package daytona

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox"
)

func TestDaytonaProviderIntegrationFullLifecycle(t *testing.T) {
	apiKey := os.Getenv("DAYTONA_API_KEY")
	if apiKey == "" {
		t.Skip("DAYTONA_API_KEY is required for Daytona provider integration tests")
	}
	snapshot := os.Getenv("DAYTONA_SNAPSHOT")
	image := os.Getenv("DAYTONA_IMAGE")
	if snapshot == "" && image == "" {
		t.Skip("DAYTONA_SNAPSHOT or DAYTONA_IMAGE is required for Daytona provider integration tests")
	}
	if err := seedKnownHosts(t, daytonaSSHHost()); err != nil {
		t.Fatalf("seedKnownHosts() error = %v", err)
	}

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "input.txt"), "hello from local")
	provider := NewProvider(WithLogger(nil))
	startupSource := sandbox.DaytonaStartupSourceImage
	startupRef := image
	if snapshot != "" {
		startupSource = sandbox.DaytonaStartupSourceSnapshot
		startupRef = snapshot
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	prepared, err := provider.Prepare(ctx, sandbox.PrepareRequest{
		SessionID:    "integration-daytona",
		WorkspaceID:  "workspace-integration",
		SandboxID:    "env-integration",
		LocalRootDir: root,
		Sandbox: sandbox.Resolved{
			Profile:        "daytona-integration",
			Backend:        sandbox.BackendDaytona,
			SyncMode:       sandbox.SyncModeSessionBidirectional,
			Persistence:    sandbox.PersistenceTransient,
			RuntimeRootDir: "/home/daytona/agh-integration",
			Daytona: &sandbox.DaytonaConfig{
				APIURL:        os.Getenv("DAYTONA_API_URL"),
				Image:         image,
				Snapshot:      snapshot,
				StartupSource: startupSource,
				StartupRef:    startupRef,
			},
		},
		AgentCommand: "cat",
		AgentEnv:     []string{"AGH_SESSION_ID=integration-daytona"},
		Permissions:  string(config.PermissionModeApproveAll),
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		if err := provider.Destroy(cleanupCtx, prepared.State); err != nil {
			t.Logf("Destroy() cleanup error = %v", err)
		}
	})

	if _, err := provider.SyncToRuntime(ctx, prepared.State, sandbox.SyncOptions{
		Reason: sandbox.SyncReasonStart,
	}); err != nil {
		t.Fatalf("SyncToRuntime() error = %v", err)
	}
	remoteContent, err := prepared.ToolHost.ReadTextFile(ctx, "input.txt")
	if err != nil {
		t.Fatalf("ToolHost.ReadTextFile() error = %v", err)
	}
	if remoteContent != "hello from local" {
		t.Fatalf("remote content = %q, want local content", remoteContent)
	}
	handle, err := prepared.Launcher.Launch(ctx, sandbox.LaunchSpec{
		Command: "cat",
		Cwd:     prepared.RuntimeRootDir,
		Env:     []string{"AGH_SESSION_ID=integration-daytona"},
	})
	if err != nil {
		t.Fatalf("Launch(cat) error = %v", err)
	}
	if _, err := handle.Stdin().Write([]byte("echo test")); err != nil {
		t.Fatalf("handle.Stdin().Write() error = %v", err)
	}
	if err := handle.Stdin().Close(); err != nil {
		t.Fatalf("handle.Stdin().Close() error = %v", err)
	}
	output := make([]byte, len("echo test"))
	if _, err := io.ReadFull(handle.Stdout(), output); err != nil {
		t.Fatalf("ReadFull(handle.Stdout()) error = %v", err)
	}
	if err := handle.Stop(ctx); err != nil {
		t.Fatalf("handle.Stop() error = %v", err)
	}
	if string(output) != "echo test" {
		t.Fatalf("SSH cat output = %q, want echo test", string(output))
	}
	if err := prepared.ToolHost.WriteTextFile(ctx, "output.txt", "hello from runtime"); err != nil {
		t.Fatalf("ToolHost.WriteTextFile() error = %v", err)
	}
	if _, err := provider.SyncFromRuntime(ctx, prepared.State, sandbox.SyncOptions{
		Reason: sandbox.SyncReasonStop,
	}); err != nil {
		t.Fatalf("SyncFromRuntime() error = %v", err)
	}
	assertFileContent(t, filepath.Join(root, "output.txt"), "hello from runtime")
}

func seedKnownHosts(t *testing.T, host string) error {
	t.Helper()

	if _, err := exec.LookPath("ssh-keyscan"); err != nil {
		t.Skipf("ssh-keyscan is required for Daytona provider integration tests: %v", err)
	}

	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("create ssh dir: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh-keyscan", "-t", "ed25519", host)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ssh-keyscan %q: %w", host, err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "known_hosts"), output, 0o600); err != nil {
		return fmt.Errorf("write known_hosts: %w", err)
	}
	t.Setenv("HOME", home)
	return nil
}
