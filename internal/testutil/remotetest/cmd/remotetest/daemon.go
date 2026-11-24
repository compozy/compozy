//go:build integration

package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	"github.com/compozy/compozy/internal/version"
)

const (
	daemonBinaryEnv = "COMPOZY_TEST_DAEMON_BIN"
	releaseEnv      = "COMPOZY_TEST_SSH_RELEASE"
	trustEnv        = "COMPOZY_TEST_SSH_TRUST_ROOT"
)

type daemonInstance struct {
	WebURL       string `json:"web_url"`
	Socket       string `json:"socket"`
	Home         string `json:"home"`
	OperatorHome string `json:"operator_home"`
	DeviceID     string `json:"device_id,omitempty"`
}

func (d daemonInstance) env(cloudURL string) []string {
	return append(os.Environ(), "HOME="+d.OperatorHome, "COMPOZY_HOME="+d.Home,
		"XDG_CONFIG_HOME="+filepath.Join(d.OperatorHome, ".config"),
		"XDG_RUNTIME_DIR="+filepath.Join(d.OperatorHome, "run"),
		"DBUS_SESSION_BUS_ADDRESS=unix:path="+filepath.Join(d.OperatorHome, "no-session-bus"),
		"COMPOZY_CLOUD_URL="+cloudURL, "COMPOZY_ACCOUNT_TOKEN=", "COMPOZY_CONFIG=",
		"COMPOZY_AGENT=", "COMPOZY_SESSION_ID=", "COMPOZY_PROFILE=", "COMPOZY_WORKSPACE=")
}

func prepareDaemon(f *fixture, direct bool) daemonInstance {
	f.Helper()
	root := f.TempDir()
	d := daemonInstance{
		Home: filepath.Join(root, "data"), OperatorHome: filepath.Join(root, "user"),
		Socket: filepath.Join(root, "d.sock"),
	}
	for _, dir := range []string{d.Home, filepath.Join(d.OperatorHome, ".ssh"), filepath.Join(d.OperatorHome, "run")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			f.Fatal(err)
		}
	}
	listener, err := new(net.ListenConfig).Listen(f.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		f.Fatal(err)
	}
	d.WebURL = "http://" + listener.Addr().String()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		f.Fatal(err)
	}
	cfg := fmt.Sprintf(
		"[http]\nhost = '127.0.0.1'\nport = %d\n[daemon]\nsocket = %q\n[automation]\nenabled = false\n",
		port, d.Socket)
	if direct {
		cfg += "[remote]\nlistener = '127.0.0.1:0'\n[remote.direct]\nenabled = true\n"
	}
	writeFile(f, filepath.Join(d.Home, "config.toml"), cfg)
	return d
}

func startDaemon(f *fixture, d daemonInstance, cloudURL string, extraEnv ...string) {
	f.Helper()
	executable, err := os.Executable()
	if err != nil {
		f.Fatal(err)
	}
	logFile, err := os.Create(filepath.Join(d.Home, "daemon.log"))
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() {
		if err := logFile.Close(); err != nil {
			f.Error(err)
		}
		if f.Failed() {
			if data, err := os.ReadFile(logFile.Name()); err == nil {
				f.Logf("daemon log:\n%s", data)
			}
		}
	})
	cmd := exec.Command(executable, "serve-daemon")
	cmd.Dir, cmd.Env = d.OperatorHome, append(d.env(cloudURL), extraEnv...)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	done, err := procutil.StartParentOwnedCommand(cmd)
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() {
		if err := procutil.StopParentOwnedCommand(cmd, done); err != nil {
			f.Errorf("stop daemon %s: %v", d.Socket, err)
		}
	})
	waitForDaemon(f, d, cmd, done)
}

func waitForDaemon(f *fixture, d daemonInstance, cmd *exec.Cmd, done <-chan error) {
	f.Helper()
	client := httpClientFor(d)
	defer client.CloseIdleConnections()
	var identity contract.RuntimeIdentityPayload
	var probeErr error
	defer func() {
		if f.Failed() {
			f.Logf("last readiness probe for %s: %+v (error: %v)", d.Socket, identity.Daemon.Readiness, probeErr)
		}
	}()
	remotetest.Await(f, "daemon readiness", 45*time.Second, func(ctx context.Context) bool {
		select {
		case err := <-done:
			f.Fatalf("daemon exited before readiness (%v): %v", cmd.ProcessState, err)
		default:
		}
		probeErr = requestJSON(ctx, client, http.MethodGet, "/api/status/identity", nil, &identity)
		return probeErr == nil && identity.Daemon.Readiness != nil &&
			identity.Daemon.Readiness.Phase == contract.DaemonReadinessReady
	})
}

func runDaemon(ctx context.Context) error {
	defer version.OverrideVersionForTesting(remotetest.SSHFixtureVersion)()
	options := []daemon.Option{daemon.WithCompozyExecutableResolver(func() (string, error) {
		return exec.LookPath(os.Getenv(daemonBinaryEnv))
	})}
	if release := os.Getenv(releaseEnv); release != "" {
		root, err := os.ReadFile(os.Getenv(trustEnv))
		if err != nil {
			return err
		}
		verifier, err := remotetest.ReleaseVerifier(root)
		if err != nil {
			return err
		}
		options = append(options, daemon.WithSSHSetupOptions(sshsetup.Options{
			SSHConfig:   filepath.Join(os.Getenv("HOME"), ".ssh", "config"),
			ReleaseBase: release, Verifier: verifier,
		}))
	}
	d, err := daemon.New(options...)
	if err != nil {
		return err
	}
	return d.Run(ctx)
}

func cliJSON[T any](f *fixture, d daemonInstance, cloudURL string, args ...string) T {
	f.Helper()
	ctx, cancel := context.WithTimeout(f.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Getenv(daemonBinaryEnv), append(args, "--json")...)
	cmd.Env, cmd.Dir = d.env(cloudURL), d.OperatorHome
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			f.Fatalf("compozy %s: %v\n%s\n%s", args[0], err, out, exit.Stderr)
		}
		f.Fatalf("compozy %s: %v", args[0], err)
	}
	var result T
	if err := json.Unmarshal(out, &result); err != nil {
		f.Fatalf("decode compozy %s: %v\n%s", args[0], err, out)
	}
	return result
}

func writeFile(f *fixture, path, body string) {
	f.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		f.Fatal(err)
	}
}
