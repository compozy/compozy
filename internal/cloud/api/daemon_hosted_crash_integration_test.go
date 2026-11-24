//go:build integration && linux

package api_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/testutil"
)

const hostedCrashChildTime = "COMPOZY_TEST_HOSTED_CRASH_CHILD_TIME"
const hostedCrashKeyringFile = "fixture-keyring.json"

func TestDaemonHostedCrash(t *testing.T) {
	if raw := os.Getenv(hostedCrashChildTime); raw != "" {
		runHostedCrashDaemon(t, raw)
		return
	}
	// IT-079. Exercise SIGKILL and marker-based recovery through real daemon restarts.
	f, _ := newHostedAuthFixture(t)
	home := t.TempDir()
	host, ring, id := startHostedDaemon(t, f, home, "")
	host.uds(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{CloudRoute: new(true)}).
		expect(t, http.StatusOK, string(id))
	waitHostedRouteReady(t, host)
	host.stop()
	saveCrashDaemonKeyring(t, home, ring)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 6*time.Second)
		defer cancel()
		if err := procutil.ReapRemoteConnectors(ctx, home, filepath.Dir(hostedConnectorDir(home))); err != nil {
			t.Error(err)
		}
	})
	t.Run("Should stop the connector when Linux kills its daemon", func(t *testing.T) {
		prior, err := readHostedConnector(home)
		if err != nil {
			t.Fatal(err)
		}
		child := startHostedCrashDaemon(t, f.clock.Now())
		connector := waitHostedConnector(t, home, prior.PID)
		waitForFixtureConnectorPIDs(t, home, []int{connector.PID})
		child.kill(t)
		waitForFixtureConnectorPIDs(t, home, nil)
	})
	t.Run("Should reap a detached connector before starting its replacement", func(t *testing.T) {
		assertHostedCrashRecovery(t, home, f.clock.Now(), "detached")
	})
	t.Run("Should reap a detached connector from a crash during launch", func(t *testing.T) {
		assertHostedCrashRecovery(t, home, f.clock.Now(), "crash-before-ready")
	})
}

func assertHostedCrashRecovery(t *testing.T, home string, now time.Time, mode string) {
	t.Helper()
	prior, err := readHostedConnector(home)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(hostedConnectorDir(home), connectorFixtureMode)
	if err := os.WriteFile(path, []byte(mode), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	child := startHostedCrashDaemon(t, now)
	var orphan hostedConnectorState
	if mode == "crash-before-ready" {
		// The stand-in kills the daemon before making /ready or its PID available.
		child.wait(t)
		orphan = waitHostedConnector(t, home, prior.PID)
	} else {
		orphan = waitHostedConnector(t, home, prior.PID)
		child.kill(t)
	}
	waitForFixtureConnectorPIDs(t, home, []int{orphan.PID})
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	restarted := startHostedCrashDaemon(t, now)
	replacement := waitHostedConnector(t, home, orphan.PID)
	waitForFixtureConnectorPIDs(t, home, []int{replacement.PID})
	if replacement.Hostname != orphan.Hostname {
		t.Fatalf("restart changed the hosted hostname: %q to %q", orphan.Hostname, replacement.Hostname)
	}
	restarted.kill(t)
	waitForFixtureConnectorPIDs(t, home, nil)
}

func waitForFixtureConnectorPIDs(t *testing.T, home string, want []int) {
	t.Helper()
	var got []int
	testutil.Eventually(t, fmt.Sprintf("connector PIDs become %v", want), 8*time.Second, func() bool {
		state, err := procutil.ScanProcesses(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		got = got[:0]
		for _, process := range state.Processes {
			if process.RemoteConnector && process.Home == home && !process.Zombie {
				got = append(got, process.PID)
			}
		}
		return slices.Equal(got, want)
	})
}

type hostedCrashDaemon struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

func startHostedCrashDaemon(t *testing.T, now time.Time) *hostedCrashDaemon {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestDaemonHostedCrash$")
	cmd.Env = append(os.Environ(), hostedCrashChildTime+"="+now.Format(time.RFC3339Nano))
	cmd.Stdout, cmd.Stderr = t.Output(), t.Output()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	child := &hostedCrashDaemon{cmd: cmd, done: make(chan struct{})}
	go func() {
		defer close(child.done)
		child.err = cmd.Wait()
	}()
	t.Cleanup(func() {
		_ = child.cmd.Process.Kill()
		select {
		case <-child.done:
		case <-time.After(6 * time.Second):
			t.Error("fixture daemon did not exit during cleanup")
		}
	})
	return child
}

func (d *hostedCrashDaemon) kill(t *testing.T) {
	t.Helper()
	if err := d.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	d.wait(t)
}

func (d *hostedCrashDaemon) wait(t *testing.T) {
	t.Helper()
	select {
	case <-d.done:
		failure, ok := errors.AsType[*exec.ExitError](d.err)
		if !ok || failure.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
			t.Fatalf("fixture daemon did not exit from SIGKILL: %v", d.err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("fixture daemon did not exit")
	}
}

func runHostedCrashDaemon(t *testing.T, rawTime string) {
	t.Helper()
	now, err := time.Parse(time.RFC3339Nano, rawTime)
	if err != nil {
		t.Fatal(err)
	}
	home := os.Getenv("COMPOZY_HOME")
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadForHome(paths)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Automation.Enabled = false
	cfg.HTTP.Host, cfg.HTTP.Port = "127.0.0.1", testutil.FreeTCPPort(t)
	raw, err := os.ReadFile(filepath.Join(home, hostedCrashKeyringFile))
	if err != nil {
		t.Fatal(err)
	}
	var entries [][3]string
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	ring := &daemonAccountKeyring{}
	for _, entry := range entries {
		_ = ring.Set(entry[0], entry[1], entry[2])
	}
	clock := clockwork.NewFakeClockAt(now)
	instance, err := daemon.New(
		daemon.WithHomePaths(paths), daemon.WithConfig(&cfg), daemon.WithNow(clock.Now),
		daemon.WithCloudflaredInstaller(hostedFixtureInstaller),
		daemon.WithAccountOptions(account.WithKeyring(ring), account.WithClock(clock)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func saveCrashDaemonKeyring(t *testing.T, home string, ring *daemonAccountKeyring) {
	t.Helper()
	var entries [][3]string
	ring.entries.Range(func(key, value any) bool {
		pair := key.([2]string)
		entries = append(entries, [3]string{pair[0], pair[1], value.(string)})
		return true
	})
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, hostedCrashKeyringFile), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
