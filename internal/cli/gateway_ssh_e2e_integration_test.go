//go:build integration && !windows

// Suite: remote SSH gateway end-to-end
// Invariant: A real SSH connection exposes a remote loopback daemon only through its local forward, persists its profile, and stops only a daemon it started.
// Boundary IN: Cobra connect ssh, system OpenSSH, a real CompozyOS binary, daemon lifecycle, and profile persistence.
// Boundary OUT: Gateway-provider publication and direct HTTPS gateway authentication, owned by gateway integration suites.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	sshE2EReadyTimeout = 15 * time.Second
	sshE2EPollInterval = 25 * time.Millisecond
)

func TestRemoteGatewayE2EConnectSSHUsesLoopbackAndPreservesOwnership(t *testing.T) {
	// This test changes HOME to isolate OpenSSH's configuration and known-host state.
	// not parallel: HOME is process-global state owned by the isolated OpenSSH client configuration.
	// Go forbids t.Setenv callers from running in parallel.
	t.Run("Should forward a real remote daemon and stop only the daemon it owns", func(t *testing.T) {
		clientHome := t.TempDir()
		remoteHome := newSSHGatewayE2ERemoteHome(t)
		remotePaths, err := compozyconfig.ResolveHomePathsFrom(remoteHome)
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom(remote) error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(remotePaths); err != nil {
			t.Fatalf("EnsureHomeLayout(remote) error = %v", err)
		}
		configureSSHGatewayE2ERemoteDaemon(t, remotePaths)
		clientPaths, err := compozyconfig.ResolveHomePathsFrom(clientHome)
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom(client) error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(clientPaths); err != nil {
			t.Fatalf("EnsureHomeLayout(client) error = %v", err)
		}

		remoteBinaryDir := filepath.Join(t.TempDir(), "bin")
		if err := os.MkdirAll(remoteBinaryDir, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(remote binary directory) error = %v", err)
		}
		compozyBinary := filepath.Join(remoteBinaryDir, "compozy")
		buildRemoteCompozyBinary(t, compozyBinary)

		server := startSSHGatewayE2EServer(t, remoteBinaryDir)
		t.Setenv("HOME", clientHome)
		configureSSHGatewayE2EClient(t, clientHome, server.clientKey, server.port)

		currentUser, err := user.Current()
		if err != nil {
			t.Fatalf("user.Current() error = %v", err)
		}
		if strings.TrimSpace(currentUser.Username) == "" {
			t.Fatal("user.Current() returned an empty username")
		}
		verifySSHGatewayE2EAuthentication(t, systemSSHExecutor{}, sshTarget{
			host: "127.0.0.1", user: currentUser.Username, port: server.port,
		}, server.stderr)
		verifySSHGatewayE2ERemoteBinary(t, systemSSHExecutor{}, sshTarget{
			host: "127.0.0.1", user: currentUser.Username, port: server.port,
		}, remoteBinaryDir, remoteHome)

		deps := commandDeps{
			resolveHome:  func() (compozyconfig.HomePaths, error) { return clientPaths, nil },
			loadConfig:   func() (compozyconfig.Config, error) { return compozyconfig.LoadForHome(clientPaths) },
			pollInterval: sshE2EPollInterval,
			startTimeout: sshE2EReadyTimeout,
			stopTimeout:  sshE2EReadyTimeout,
		}.withDefaults()
		target := sshTarget{host: "127.0.0.1", user: currentUser.Username, port: server.port}
		executor := systemSSHExecutor{}
		t.Cleanup(func() {
			stopRemoteDaemonForSSHGatewayE2E(t, executor, target, remoteHome)
		})

		owned := startSSHGatewayE2EConnect(t, deps, currentUser.Username, server.port, remoteHome, false)
		assertSSHGatewayE2EForward(t, owned.record.LocalURL)
		assertSSHGatewayE2EProfile(t, clientPaths, "ssh-e2e", server.port, remoteHome)
		ownedStatus := assertRemoteDaemonRunningForSSHGatewayE2E(t, executor, target, remoteHome)
		owned.stop(t)
		assertSSHGatewayE2EForwardClosed(t, owned.record.LocalURL)
		assertRemoteDaemonStoppedForSSHGatewayE2E(t, executor, target, ownedStatus, remotePaths.DaemonSocket)

		startRemoteDaemonForSSHGatewayE2E(t, executor, target, remoteHome)
		reused := startSSHGatewayE2EConnect(t, deps, currentUser.Username, server.port, remoteHome, true)
		if reused.record.Daemon != "reused" {
			t.Fatalf("reused connection daemon = %q, want %q", reused.record.Daemon, "reused")
		}
		assertSSHGatewayE2EForward(t, reused.record.LocalURL)
		reused.stop(t)
		assertSSHGatewayE2EForwardClosed(t, reused.record.LocalURL)
		assertRemoteDaemonRunningForSSHGatewayE2E(t, executor, target, remoteHome)

		stopRemoteDaemonForSSHGatewayE2E(t, executor, target, remoteHome)
		crashed := startSSHGatewayE2EConnectProcess(
			t, compozyBinary, clientHome, currentUser.Username, server.port, remoteHome,
		)
		assertSSHGatewayE2EForward(t, crashed.record.LocalURL)
		crashed.kill(t)
		waitForSSHGatewayE2EOwnershipRelease(t, executor, target, remoteHome)
		assertSSHGatewayE2EForwardClosed(t, crashed.record.LocalURL)
		assertRemoteDaemonRunningForSSHGatewayE2E(t, executor, target, remoteHome)

		afterCrash := startSSHGatewayE2EConnect(t, deps, currentUser.Username, server.port, remoteHome, true)
		if afterCrash.record.Daemon != "reused" {
			t.Fatalf("post-crash connection daemon = %q, want %q", afterCrash.record.Daemon, "reused")
		}
		afterCrash.stop(t)
		assertRemoteDaemonRunningForSSHGatewayE2E(t, executor, target, remoteHome)
	})
}

func newSSHGatewayE2ERemoteHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp(os.TempDir(), "compozy-ssh-")
	if err != nil {
		t.Fatalf("os.MkdirTemp(remote CompozyOS home) error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(home); err != nil {
			t.Errorf("os.RemoveAll(remote CompozyOS home) error = %v", err)
		}
	})
	return home
}

func configureSSHGatewayE2ERemoteDaemon(t *testing.T, homePaths compozyconfig.HomePaths) {
	t.Helper()
	port := reserveSSHGatewayE2EPort(t)
	config := fmt.Sprintf("[http]\nhost = \"127.0.0.1\"\nport = %d\n", port)
	if err := os.WriteFile(homePaths.ConfigFile, []byte(config), 0o600); err != nil {
		t.Fatalf("os.WriteFile(remote daemon config) error = %v", err)
	}
}

type sshGatewayE2EServer struct {
	clientKey string
	port      int
	command   *exec.Cmd
	cancel    context.CancelFunc
	stderr    *lockedSSHGatewayE2EBuffer
}

func startSSHGatewayE2EServer(t *testing.T, remoteBinaryDir string) *sshGatewayE2EServer {
	t.Helper()

	const sshdPath = "/usr/sbin/sshd"
	if _, err := os.Stat(sshdPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v", sshdPath, err)
	}

	fixtureDir := t.TempDir()
	hostKey := filepath.Join(fixtureDir, "host_ed25519")
	clientKey := filepath.Join(fixtureDir, "client_ed25519")
	generateSSHGatewayE2EKey(t, hostKey)
	generateSSHGatewayE2EKey(t, clientKey)
	clientPublicKey, err := os.ReadFile(clientKey + ".pub")
	if err != nil {
		t.Fatalf("os.ReadFile(client public key) error = %v", err)
	}
	port := reserveSSHGatewayE2EPort(t)
	authorizedKeys := filepath.Join(fixtureDir, "authorized_keys")
	pathValue := remoteBinaryDir + ":/usr/local/bin:/usr/bin:/bin"
	authorizedLine := strings.TrimSpace(string(clientPublicKey)) + "\n"
	if err := os.WriteFile(authorizedKeys, []byte(authorizedLine), 0o600); err != nil {
		t.Fatalf("os.WriteFile(authorized_keys) error = %v", err)
	}
	commandWrapper := filepath.Join(fixtureDir, "command-wrapper")
	wrapperScript := strings.Join([]string{
		"#!/bin/sh",
		"export PATH=" + shellQuote(pathValue),
		"test -n \"${SSH_ORIGINAL_COMMAND:-}\" || exit 0",
		"exec /bin/sh -c \"$SSH_ORIGINAL_COMMAND\"",
		"",
	}, "\n")
	if err := os.WriteFile(commandWrapper, []byte(wrapperScript), 0o700); err != nil {
		t.Fatalf("os.WriteFile(command wrapper) error = %v", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current() error = %v", err)
	}
	configPath := filepath.Join(fixtureDir, "sshd_config")
	config := strings.Join([]string{
		"Port " + strconv.Itoa(port),
		"ListenAddress 127.0.0.1",
		"HostKey " + hostKey,
		"PidFile " + filepath.Join(fixtureDir, "sshd.pid"),
		"AuthorizedKeysFile " + authorizedKeys,
		"AllowUsers " + currentUser.Username,
		"PasswordAuthentication no",
		"KbdInteractiveAuthentication no",
		"ChallengeResponseAuthentication no",
		"UsePAM no",
		"StrictModes no",
		"PermitRootLogin no",
		"ForceCommand " + commandWrapper,
		"UseDNS no",
		"PubkeyAuthentication yes",
		"LogLevel QUIET",
		"Subsystem sftp internal-sftp",
		"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("os.WriteFile(sshd config) error = %v", err)
	}

	// Keep sshd alive through registered cleanup; testing cancels t.Context
	// before cleanup callbacks run.
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, sshdPath, "-D", "-e", "-f", configPath)
	server := &sshGatewayE2EServer{
		clientKey: clientKey,
		port:      port,
		command:   command,
		cancel:    cancel,
		stderr:    &lockedSSHGatewayE2EBuffer{},
	}
	command.Stderr = server.stderr
	if err := command.Start(); err != nil {
		cancel()
		t.Fatalf("start sshd error = %v", err)
	}
	t.Cleanup(func() {
		server.cancel()
		if err := server.command.Wait(); err != nil && !isExpectedSSHGatewayE2EServerShutdown(err) {
			t.Errorf("wait for sshd error = %v; stderr: %s", err, server.stderr.String())
		}
	})
	waitForSSHGatewayE2EPort(t, port)
	return server
}

func isExpectedSSHGatewayE2EServerShutdown(err error) bool {
	var exitErr *exec.ExitError
	return errors.Is(err, context.Canceled) || errors.As(err, &exitErr)
}

func generateSSHGatewayE2EKey(t *testing.T, path string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "/usr/bin/ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", path)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("ssh-keygen %q error = %v; output: %s", path, err, output)
	}
}

func configureSSHGatewayE2EClient(t *testing.T, home, clientKey string, port int) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(client SSH directory) error = %v", err)
	}
	config := strings.Join([]string{
		"Host 127.0.0.1",
		"  IdentityFile " + clientKey,
		"  IdentitiesOnly yes",
		"  UserKnownHostsFile " + filepath.Join(sshDir, "known_hosts"),
		"  GlobalKnownHostsFile /dev/null",
		"  ControlPath none",
		"  LogLevel QUIET",
		"  Port " + strconv.Itoa(port),
		"",
	}, "\n")
	configPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("os.WriteFile(client SSH config) error = %v", err)
	}
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	keyscan := exec.CommandContext(
		t.Context(),
		"/usr/bin/ssh-keyscan",
		"-p", strconv.Itoa(port),
		"127.0.0.1",
	)
	knownHost, err := keyscan.Output()
	if err != nil {
		t.Fatalf("ssh-keyscan fixture host error = %v", err)
	}
	if err := os.WriteFile(knownHostsPath, knownHost, 0o600); err != nil {
		t.Fatalf("os.WriteFile(known_hosts) error = %v", err)
	}
	shimDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(shimDir, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(SSH shim directory) error = %v", err)
	}
	shimPath := filepath.Join(shimDir, "ssh")
	shim := "#!/bin/sh\nexec /usr/bin/ssh -F " + shellJoin([]string{configPath}) + " \"$@\"\n"
	if err := os.WriteFile(shimPath, []byte(shim), 0o700); err != nil {
		t.Fatalf("os.WriteFile(SSH client shim) error = %v", err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func verifySSHGatewayE2EAuthentication(
	t *testing.T,
	executor sshExecutor,
	target sshTarget,
	serverStderr *lockedSSHGatewayE2EBuffer,
) {
	t.Helper()
	if _, err := executor.Run(t.Context(), target, []string{"true"}); err != nil {
		var commandErr *sshCommandError
		if errors.As(err, &commandErr) {
			t.Fatalf(
				"authenticate SSH gateway fixture error = %v; output: %s; sshd: %s",
				err,
				commandErr.output,
				serverStderr.String(),
			)
		}
		t.Fatalf("authenticate SSH gateway fixture error = %v", err)
	}
}

func verifySSHGatewayE2ERemoteBinary(
	t *testing.T,
	executor sshExecutor,
	target sshTarget,
	remoteBinaryDir string,
	remoteHome string,
) {
	t.Helper()
	resolvedPath, err := executor.Run(t.Context(), target, remoteCompozyLookupCommand())
	if err != nil {
		t.Fatalf("resolve remote compozy binary error = %v", err)
	}
	wantPath := filepath.Join(remoteBinaryDir, "compozy")
	if gotPath := strings.TrimSpace(string(resolvedPath)); gotPath != wantPath {
		t.Fatalf("remote compozy path = %q, want %q", gotPath, wantPath)
	}
	output, err := executor.Run(t.Context(), target, remoteCompozyCommand(remoteHome, "version", "-o", "json"))
	if err != nil {
		t.Fatalf("run remote compozy binary error = %v", err)
	}
	var version struct {
		Version string
	}
	if err := json.Unmarshal(output, &version); err != nil {
		t.Fatalf("decode remote compozy version error = %v; output: %s", err, output)
	}
	if version.Version == "" {
		t.Fatalf("remote compozy version = %#v, want a version", version)
	}
}

func buildRemoteCompozyBinary(t *testing.T, outputPath string) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs(repository root) error = %v", err)
	}
	command := exec.CommandContext(t.Context(), "go", "build", "-o", outputPath, "./cmd/compozy")
	command.Dir = repoRoot
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build remote compozy binary error = %v; output: %s", err, output)
	}
}

func reserveSSHGatewayE2EPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(reserve SSH port) error = %v", err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			t.Errorf("close SSH port reservation error = %v", closeErr)
		}
	}()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.Port <= 0 {
		t.Fatalf("SSH port reservation address = %#v, want loopback TCP port", listener.Addr())
	}
	return address.Port
}

func waitForSSHGatewayE2EPort(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(sshE2EReadyTimeout)
	for {
		connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), sshE2EPollInterval)
		if err == nil {
			if closeErr := connection.Close(); closeErr != nil {
				t.Fatalf("close SSH readiness connection error = %v", closeErr)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("sshd did not listen on port %d before deadline: %v", port, err)
		}
		timer := time.NewTimer(sshE2EPollInterval)
		select {
		case <-t.Context().Done():
			t.Fatalf("wait for sshd listener canceled: %v", t.Context().Err())
		case <-timer.C:
		}
	}
}

type sshGatewayE2EConnect struct {
	cancel   context.CancelFunc
	complete chan struct{}
	output   *lockedSSHGatewayE2EBuffer
	stderr   *lockedSSHGatewayE2EBuffer
	mu       sync.Mutex
	err      error
	record   sshConnectionRecord
}

type sshGatewayE2EConnectProcess struct {
	command *exec.Cmd
	stdout  *lockedSSHGatewayE2EBuffer
	stderr  *lockedSSHGatewayE2EBuffer
	record  sshConnectionRecord
}

func startSSHGatewayE2EConnectProcess(
	t *testing.T,
	binary string,
	clientHome string,
	username string,
	port int,
	remoteHome string,
) *sshGatewayE2EConnectProcess {
	t.Helper()
	process := &sshGatewayE2EConnectProcess{
		stdout: &lockedSSHGatewayE2EBuffer{},
		stderr: &lockedSSHGatewayE2EBuffer{},
	}
	process.command = exec.Command(
		binary,
		"connect", "ssh", "127.0.0.1",
		"--user", username,
		"--port", strconv.Itoa(port),
		"--name", "ssh-e2e",
		"--remote-home", remoteHome,
		"--overwrite",
		"-o", "json",
	)
	process.command.Env = sshGatewayE2EProcessEnvironment(clientHome)
	process.command.Stdout = process.stdout
	process.command.Stderr = process.stderr
	if err := process.command.Start(); err != nil {
		t.Fatalf("start crash-test connect process error = %v", err)
	}
	t.Cleanup(func() {
		if process.command.ProcessState == nil {
			if err := process.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Errorf("kill crash-test connect process cleanup error = %v", err)
			}
			if err := process.command.Wait(); err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Errorf("wait crash-test connect process cleanup error = %v", err)
				}
			}
		}
	})
	process.record = waitForSSHGatewayE2EProcessConnection(t, process)
	return process
}

func sshGatewayE2EProcessEnvironment(clientHome string) []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "COMPOZY_HOME=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "HOME="+clientHome, "COMPOZY_HOME="+clientHome)
}

func waitForSSHGatewayE2EProcessConnection(
	t *testing.T,
	process *sshGatewayE2EConnectProcess,
) sshConnectionRecord {
	t.Helper()
	deadline := time.Now().Add(sshE2EReadyTimeout)
	for {
		output := process.stdout.String()
		if json.Valid([]byte(output)) {
			var record sshConnectionRecord
			if err := json.Unmarshal([]byte(output), &record); err != nil {
				t.Fatalf("decode crash-test connect output error = %v; output: %s", err, output)
			}
			return record
		}
		if process.command.ProcessState != nil {
			t.Fatalf(
				"crash-test connect exited before readiness: %v; stdout: %s; stderr: %s",
				process.command.ProcessState, output, process.stderr.String(),
			)
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"crash-test connect did not become ready; stdout: %s; stderr: %s",
				output, process.stderr.String(),
			)
		}
		timer := time.NewTimer(sshE2EPollInterval)
		select {
		case <-t.Context().Done():
			timer.Stop()
			t.Fatalf("wait for crash-test connect canceled: %v", t.Context().Err())
		case <-timer.C:
		}
	}
}

func (p *sshGatewayE2EConnectProcess) kill(t *testing.T) {
	t.Helper()
	if err := p.command.Process.Kill(); err != nil {
		t.Fatalf("SIGKILL crash-test connect process error = %v", err)
	}
	if err := p.command.Wait(); err == nil {
		t.Fatal("crash-test connect process exited cleanly after SIGKILL")
	} else {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("wait crash-test connect process error = %v", err)
		}
	}
}

func waitForSSHGatewayE2EOwnershipRelease(
	t *testing.T,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
) {
	t.Helper()
	deadline := time.Now().Add(sshE2EReadyTimeout)
	for {
		owned, err := inspectSSHDaemonOwnership(t.Context(), executor, target, remoteHome)
		if err == nil && !owned {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("SSH ownership marker remained after parent SIGKILL: owned=%v err=%v", owned, err)
		}
		timer := time.NewTimer(sshE2EPollInterval)
		select {
		case <-t.Context().Done():
			timer.Stop()
			t.Fatalf("wait for SSH ownership release canceled: %v", t.Context().Err())
		case <-timer.C:
		}
	}
}

func startSSHGatewayE2EConnect(
	t *testing.T,
	deps commandDeps,
	username string,
	port int,
	remoteHome string,
	overwrite bool,
) *sshGatewayE2EConnect {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	connect := &sshGatewayE2EConnect{
		cancel:   cancel,
		complete: make(chan struct{}),
		output:   &lockedSSHGatewayE2EBuffer{},
		stderr:   &lockedSSHGatewayE2EBuffer{},
	}
	command := newRootCommand(deps)
	command.SetOut(connect.output)
	command.SetErr(connect.stderr)
	arguments := []string{
		"connect", "ssh", "127.0.0.1",
		"--user", username,
		"--port", strconv.Itoa(port),
		"--name", "ssh-e2e",
		"--remote-home", remoteHome,
		"-o", "json",
	}
	if overwrite {
		arguments = append(arguments, "--overwrite")
	}
	command.SetArgs(arguments)
	go func() {
		connect.mu.Lock()
		connect.err = command.ExecuteContext(ctx)
		connect.mu.Unlock()
		close(connect.complete)
	}()
	t.Cleanup(func() {
		connect.cancel()
		if err := connect.wait(); err != nil {
			t.Errorf("finish SSH connection cleanup error = %v; stderr: %s", err, connect.stderr.String())
		}
	})
	connect.record = waitForSSHGatewayE2EConnection(t, connect)
	return connect
}

func (c *sshGatewayE2EConnect) stop(t *testing.T) {
	t.Helper()
	c.cancel()
	if err := c.wait(); err != nil {
		t.Fatalf("connect ssh completion error = %v; stderr: %s", err, c.stderr.String())
	}
}

func (c *sshGatewayE2EConnect) wait() error {
	select {
	case <-c.complete:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.err
	case <-time.After(sshE2EReadyTimeout):
		return errors.New("connect ssh did not stop before the deadline")
	}
}

func waitForSSHGatewayE2EConnection(t *testing.T, connect *sshGatewayE2EConnect) sshConnectionRecord {
	t.Helper()
	deadline := time.Now().Add(sshE2EReadyTimeout)
	for {
		output := connect.output.String()
		if json.Valid([]byte(output)) {
			var record sshConnectionRecord
			if err := json.Unmarshal([]byte(output), &record); err != nil {
				t.Fatalf("decode connect ssh output error = %v; output: %s", err, output)
			}
			if record.LocalURL == "" || record.RemoteHTTP == "" {
				t.Fatalf("connect ssh output = %#v, want local and remote URLs", record)
			}
			return record
		}
		select {
		case <-connect.complete:
			connect.mu.Lock()
			err := connect.err
			connect.mu.Unlock()
			t.Fatalf("connect ssh returned before a connection record: %v; stdout: %s; stderr: %s", err, output, connect.stderr.String())
		case <-t.Context().Done():
			t.Fatalf("wait for connect ssh output canceled: %v", t.Context().Err())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("connect ssh did not write a connection record before deadline; stdout: %s; stderr: %s", output, connect.stderr.String())
		}
		timer := time.NewTimer(sshE2EPollInterval)
		select {
		case <-t.Context().Done():
			timer.Stop()
			t.Fatalf("wait for connect ssh output canceled: %v", t.Context().Err())
		case <-timer.C:
		}
	}
}

type lockedSSHGatewayE2EBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedSSHGatewayE2EBuffer) Write(content []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(content)
}

func (b *lockedSSHGatewayE2EBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func assertSSHGatewayE2EForward(t *testing.T, localURL string) {
	t.Helper()
	target, err := parseSSHGatewayE2EForwardTarget(localURL)
	if err != nil {
		t.Fatalf("parse SSH forward target error = %v", err)
	}
	client, err := NewClient(target)
	if err != nil {
		t.Fatalf("NewClient(SSH forward) error = %v", err)
	}
	status, err := client.DaemonStatus(t.Context())
	if err != nil {
		t.Fatalf("DaemonStatus(SSH forward) error = %v", err)
	}
	if status.Status != "running" || status.HTTPHost != "127.0.0.1" || status.HTTPPort <= 0 {
		t.Fatalf("DaemonStatus(SSH forward) = %#v, want running loopback daemon", status)
	}
}

func assertSSHGatewayE2EForwardClosed(t *testing.T, localURL string) {
	t.Helper()
	target, err := parseSSHGatewayE2EForwardTarget(localURL)
	if err != nil {
		t.Fatalf("parse SSH forward target error = %v", err)
	}
	client, err := NewClient(target)
	if err != nil {
		t.Fatalf("NewClient(closed SSH forward) error = %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := client.DaemonStatus(ctx); err == nil {
		t.Fatal("DaemonStatus(closed SSH forward) error = nil, want unavailable forward")
	}
}

func parseSSHGatewayE2EForwardTarget(localURL string) (ClientTarget, error) {
	address := strings.TrimPrefix(strings.TrimSpace(localURL), "http://")
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return ClientTarget{}, fmt.Errorf("split SSH forward URL: %w", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return ClientTarget{}, fmt.Errorf("parse SSH forward port: %w", err)
	}
	return SSHForwardClientTarget(host, port)
}

func assertSSHGatewayE2EProfile(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	name string,
	port int,
	remoteHome string,
) {
	t.Helper()
	config, err := compozyconfig.LoadForHome(homePaths)
	if err != nil {
		t.Fatalf("LoadForHome(client) error = %v", err)
	}
	profile, exists := findGatewayProfile(config.Gateway.Connections, name)
	if !exists || profile.Scheme != "ssh" || profile.Host != "127.0.0.1" ||
		profile.Port != port || profile.RemoteHome != remoteHome {
		t.Fatalf("persisted SSH profile = %#v, want SSH profile %q for the remote daemon", profile, name)
	}
}

func startRemoteDaemonForSSHGatewayE2E(t *testing.T, executor sshExecutor, target sshTarget, remoteHome string) {
	t.Helper()
	output, err := executor.Run(t.Context(), target, remoteCompozyCommand(remoteHome, "daemon", "start", "-o", "json"))
	if err != nil {
		t.Fatalf("start remote daemon error = %v", err)
	}
	var status DaemonStatus
	if err := json.Unmarshal(output, &status); err != nil {
		t.Fatalf("decode remote daemon start output error = %v; output: %s", err, output)
	}
	if status.Status != "running" || status.HTTPHost != "127.0.0.1" || status.HTTPPort <= 0 {
		t.Fatalf("remote daemon start status = %#v, want running loopback daemon", status)
	}
}

func assertRemoteDaemonStoppedForSSHGatewayE2E(
	t *testing.T,
	executor sshExecutor,
	target sshTarget,
	expected DaemonStatus,
	socketPath string,
) {
	t.Helper()
	command := []string{
		"sh", "-c",
		`if kill -0 "$1" 2>/dev/null; then exit 1; fi; test ! -S "$2"`,
		"compozy-ssh-stopped", strconv.Itoa(expected.PID), socketPath,
	}
	if output, err := executor.Run(t.Context(), target, command); err != nil {
		t.Fatalf("remote daemon still reachable after owned cleanup: %v; output: %s", err, output)
	}
}

func assertRemoteDaemonRunningForSSHGatewayE2E(
	t *testing.T,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
) DaemonStatus {
	t.Helper()
	status := remoteDaemonStatusForSSHGatewayE2E(t, executor, target, remoteHome)
	if status.Status != "running" || status.HTTPHost != "127.0.0.1" || status.HTTPPort <= 0 {
		t.Fatalf("remote daemon after reused connection cleanup = %#v, want running loopback daemon", status)
	}
	return status
}

func remoteDaemonStatusForSSHGatewayE2E(t *testing.T, executor sshExecutor, target sshTarget, remoteHome string) DaemonStatus {
	t.Helper()
	output, err := executor.Run(t.Context(), target, remoteCompozyCommand(remoteHome, "status", "-o", "json"))
	if err != nil {
		t.Fatalf("read remote daemon status error = %v", err)
	}
	var status StatusRecord
	if err := json.Unmarshal(output, &status); err != nil {
		t.Fatalf("decode remote daemon status error = %v; output: %s", err, output)
	}
	return status.Daemon
}

func stopRemoteDaemonForSSHGatewayE2E(t *testing.T, executor sshExecutor, target sshTarget, remoteHome string) {
	t.Helper()
	cleanupCtx, cancel := context.WithTimeout(context.Background(), sshE2EReadyTimeout)
	defer cancel()
	output, err := executor.Run(
		cleanupCtx,
		target,
		remoteCompozyCommand(remoteHome, "daemon", "stop", "-o", "json"),
	)
	if err != nil {
		var commandErr *sshCommandError
		if errors.As(err, &commandErr) && commandErr.exitCode() == 1 {
			return
		}
		t.Errorf("stop remote daemon cleanup error = %v", err)
		return
	}
	if len(output) == 0 {
		t.Error("remote daemon stop returned an empty response")
	}
}

var _ io.Writer = (*lockedSSHGatewayE2EBuffer)(nil)
