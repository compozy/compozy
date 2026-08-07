package cli

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/version"
)

type sshExecutorStub struct {
	run        func(context.Context, sshTarget, []string) ([]byte, error)
	startLease func(context.Context, sshTarget, string, *sshDaemonOwnership) (sshOwnershipLease, error)
}

type sshOwnershipLeaseStub struct {
	wait  func() error
	close func() error
}

type sshTunnelStub struct {
	wait func() error
}

func (sshTunnelStub) LocalPort() int { return 43123 }

func (s sshTunnelStub) Wait() error {
	if s.wait == nil {
		return nil
	}
	return s.wait()
}

func (sshTunnelStub) Close() error { return nil }

func (s sshOwnershipLeaseStub) Wait() error {
	if s.wait == nil {
		return nil
	}
	return s.wait()
}

func (s sshOwnershipLeaseStub) Close() error {
	if s.close == nil {
		return nil
	}
	return s.close()
}

func (s sshExecutorStub) StartOwnershipLease(
	ctx context.Context,
	target sshTarget,
	remoteHome string,
	ownership *sshDaemonOwnership,
) (sshOwnershipLease, error) {
	if s.startLease == nil {
		return nil, errors.New("unexpected SSH ownership lease")
	}
	return s.startLease(ctx, target, remoteHome, ownership)
}

func (s sshExecutorStub) Run(ctx context.Context, target sshTarget, command []string) ([]byte, error) {
	if s.run == nil {
		return nil, errors.New("unexpected SSH command")
	}
	return s.run(ctx, target, command)
}

func (sshExecutorStub) StartTunnel(context.Context, sshTarget, string, int) (sshTunnel, error) {
	return nil, errors.New("unexpected SSH tunnel")
}

func TestConnectSSHFailureTaxonomy(t *testing.T) {
	t.Run("Should require a remote Compozy installation without bootstrapping [UT-110]", func(t *testing.T) {
		runs := 0
		executor := sshExecutorStub{run: func(context.Context, sshTarget, []string) ([]byte, error) {
			runs++
			return nil, &sshCommandError{cause: errors.New("exit status 1"), output: "compozy not found"}
		}}
		err := verifyRemoteCompozy(t.Context(), executor, sshTarget{host: "remote", port: 22}, "")
		assertGatewayClientErrorCode(t, err, sshInstallRequiredCode)
		if runs != 1 {
			t.Fatalf("SSH runs = %d, want only install probe", runs)
		}
	})

	t.Run("Should refuse incompatible remote and local versions [UT-111]", func(t *testing.T) {
		restore := version.OverrideVersionForTesting("1.2.3")
		defer restore()
		executor := sshExecutorStub{run: func(_ context.Context, _ sshTarget, command []string) ([]byte, error) {
			if slices.Contains(command, "version") {
				return []byte(`{"Version":"2.0.0"}`), nil
			}
			return []byte("/usr/local/bin/compozy\n"), nil
		}}
		err := verifyRemoteCompozy(t.Context(), executor, sshTarget{host: "remote", port: 22}, "")
		assertGatewayClientErrorCode(t, err, sshVersionMismatchCode)
		if !strings.Contains(err.Error(), "2.0.0") || !strings.Contains(err.Error(), "1.2.3") {
			t.Fatalf("version mismatch error = %q, want both versions", err)
		}
	})

	t.Run("Should fail closed when OpenSSH reports a changed host key [UT-112]", func(t *testing.T) {
		executor := sshExecutorStub{run: func(context.Context, sshTarget, []string) ([]byte, error) {
			return nil, &sshCommandError{
				cause:  errors.New("exit status 255"),
				output: "WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!\nHost key verification failed.",
			}
		}}
		err := verifyRemoteCompozy(t.Context(), executor, sshTarget{host: "remote", port: 22}, "")
		assertGatewayClientErrorCode(t, err, sshHostKeyChangedCode)
	})

	t.Run("Should reject option-shaped SSH hosts and users", func(t *testing.T) {
		for _, testCase := range []struct {
			name    string
			host    string
			options connectSSHOptions
		}{
			{name: "host", host: "-oProxyCommand=malicious", options: connectSSHOptions{port: 22}},
			{name: "user", host: "remote.example", options: connectSSHOptions{port: 22, user: "-Fbad"}},
		} {
			t.Run("Should reject option-shaped "+testCase.name, func(t *testing.T) {
				if err := validateSSHTarget(testCase.host, testCase.options); err == nil {
					t.Fatal("validateSSHTarget() error = nil, want rejection")
				}
			})
		}
	})

	t.Run("Should reject hostname aliases and public listeners for SSH forwarding", func(t *testing.T) {
		for _, host := range []string{"localhost", "0.0.0.0", "gateway.example"} {
			if err := validateRemoteDaemonListener(DaemonStatus{HTTPHost: host, HTTPPort: 2123}); err == nil {
				t.Fatalf("validateRemoteDaemonListener(%q) error = nil, want loopback IP rejection", host)
			}
		}
		if err := validateRemoteDaemonListener(DaemonStatus{HTTPHost: "127.0.0.1", HTTPPort: 65536}); err == nil {
			t.Fatal("validateRemoteDaemonListener() error = nil, want invalid port rejection")
		}
	})

	t.Run("Should build complete OpenSSH commands with an option boundary [UT-110]", func(t *testing.T) {
		target := sshTarget{host: "remote.example", user: "operator", port: 2222}
		runArgs := sshRunArgs(
			target,
			remoteCompozyCommand("/srv/compozy operator", "status", "-o", "json"),
		)
		wantRun := []string{
			"-o", "StrictHostKeyChecking=yes",
			"-o", "ControlMaster=auto",
			"-o", "ControlPersist=60",
			"-p", "2222",
			"--", "operator@remote.example",
			"'env' 'COMPOZY_HOME=/srv/compozy operator' 'compozy' 'status' '-o' 'json'",
		}
		if !slices.Equal(runArgs, wantRun) {
			t.Fatalf("sshRunArgs() = %#v, want %#v", runArgs, wantRun)
		}
		tunnelArgs := sshTunnelArgs(target, 43123, "127.0.0.1", 2123)
		wantTunnel := append(
			sshBaseArgs(target),
			"-o", "ExitOnForwardFailure=yes", "-N", "-L",
			"127.0.0.1:43123:127.0.0.1:2123", "--", "operator@remote.example",
		)
		if !slices.Equal(tunnelArgs, wantTunnel) {
			t.Fatalf("sshTunnelArgs() = %#v, want %#v", tunnelArgs, wantTunnel)
		}
		leaseArgs := sshOwnershipLeaseArgs(target, "/srv/compozy operator", "owner-token")
		wantLeasePrefix := []string{
			"-o", "StrictHostKeyChecking=yes",
			"-o", "ControlMaster=no",
			"-o", "ControlPath=none",
			"-o", "ControlPersist=no",
			"-p", "2222",
			"--", "operator@remote.example",
		}
		if len(leaseArgs) != len(wantLeasePrefix)+1 ||
			!slices.Equal(leaseArgs[:len(wantLeasePrefix)], wantLeasePrefix) {
			t.Fatalf("sshOwnershipLeaseArgs() = %#v, want prefix %#v and one command", leaseArgs, wantLeasePrefix)
		}
	})

	t.Run("Should leave the remote daemon running after ownership lease loss", func(t *testing.T) {
		leaseResult := make(chan error, 1)
		tunnelResult := make(chan error)
		stopRemote := true
		resolution := remoteDaemonResolution{
			ownership: &sshDaemonOwnership{token: "owner-token"},
			lease: sshOwnershipLeaseStub{wait: func() error {
				return <-leaseResult
			}},
		}
		leaseResult <- errors.New("lease disconnected")
		err := waitForSSHTunnel(
			t.Context(), resolution,
			sshTunnelStub{wait: func() error { return <-tunnelResult }},
			&stopRemote,
		)
		assertGatewayClientErrorCode(t, err, sshTunnelLostCode)
		if stopRemote {
			t.Fatal("stopRemote = true after ownership lease loss, want false")
		}
	})

	t.Run("Should stop a newly-started daemon when its listener is unsafe", func(t *testing.T) {
		startedAt := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		status := DaemonStatus{
			Status: "running", PID: 4101, StartedAt: startedAt,
			HTTPHost: "0.0.0.0", HTTPPort: 2123,
		}
		statusCalls := 0
		stopCalls := 0
		executor := sshExecutorStub{
			startLease: func(
				context.Context,
				sshTarget,
				string,
				*sshDaemonOwnership,
			) (sshOwnershipLease, error) {
				return sshOwnershipLeaseStub{}, nil
			},
			run: func(_ context.Context, _ sshTarget, command []string) ([]byte, error) {
				joined := strings.Join(command, " ")
				switch {
				case strings.HasSuffix(joined, "compozy status -o json"):
					statusCalls++
					if statusCalls == 1 {
						return nil, errors.New("daemon unavailable")
					}
					return jsonMarshalForTest(t, StatusRecord{Daemon: status}), nil
				case strings.Contains(joined, "daemon start"):
					return jsonMarshalForTest(t, status), nil
				case strings.Contains(joined, "daemon stop"):
					stopCalls++
					return []byte(`{"status":"stopped"}`), nil
				default:
					return nil, errors.New("unexpected command")
				}
			},
		}

		resolution, err := resolveRemoteDaemon(
			t.Context(), executor, sshTarget{host: "remote.example", port: 22},
			connectSSHOptions{port: 22, remoteStart: true, ownerToken: strings.Repeat("o", 24)},
			commandDeps{}.withDefaults(),
		)
		if err == nil {
			t.Fatalf("resolveRemoteDaemon() = %#v err:nil, want cleanup failure path", resolution)
		}
		if stopCalls != 1 {
			t.Fatalf("daemon stop calls = %d, want 1", stopCalls)
		}
	})

	t.Run("Should not stop a daemon whose process identity changed", func(t *testing.T) {
		startedAt := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		current := DaemonStatus{PID: 4202, StartedAt: startedAt.Add(time.Second)}
		stopCalls := 0
		executor := sshExecutorStub{run: func(_ context.Context, _ sshTarget, command []string) ([]byte, error) {
			if strings.Contains(strings.Join(command, " "), "daemon stop") {
				stopCalls++
			}
			return jsonMarshalForTest(t, StatusRecord{Daemon: current}), nil
		}}
		err := stopOwnedRemoteDaemon(
			t.Context(), executor, sshTarget{host: "remote.example", port: 22}, "",
			DaemonStatus{PID: 4101, StartedAt: startedAt}, &sshDaemonOwnership{token: "owner"},
		)
		if err != nil {
			t.Fatalf("stopOwnedRemoteDaemon() error = %v", err)
		}
		if stopCalls != 0 {
			t.Fatalf("daemon stop calls = %d, want 0", stopCalls)
		}
	})

	t.Run("Should refuse to stop when the ownership token changed", func(t *testing.T) {
		startedAt := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		status := DaemonStatus{PID: 4203, StartedAt: startedAt}
		stopGuardCalls := 0
		executor := sshExecutorStub{run: func(_ context.Context, _ sshTarget, command []string) ([]byte, error) {
			if len(command) >= 3 && command[2] == sshOwnershipStopScript {
				stopGuardCalls++
				if got := command[len(command)-1]; got != "old-owner" {
					t.Fatalf("ownership token = %q, want old-owner", got)
				}
				return []byte(sshOwnershipNotOwner), nil
			}
			return jsonMarshalForTest(t, StatusRecord{Daemon: status}), nil
		}}
		if err := stopOwnedRemoteDaemon(
			t.Context(), executor, sshTarget{host: "remote.example", port: 22}, "",
			status, &sshDaemonOwnership{token: "old-owner"},
		); err != nil {
			t.Fatalf("stopOwnedRemoteDaemon() error = %v", err)
		}
		if stopGuardCalls != 1 {
			t.Fatalf("guarded stop calls = %d, want 1", stopGuardCalls)
		}
	})

	t.Run("Should return busy while another SSH connection owns the running daemon", func(t *testing.T) {
		startedAt := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		status := DaemonStatus{
			Status: "running", PID: 4303, StartedAt: startedAt,
			HTTPHost: "127.0.0.1", HTTPPort: 2123,
		}
		executor := sshExecutorStub{run: func(_ context.Context, _ sshTarget, command []string) ([]byte, error) {
			if len(command) >= 3 && command[2] == sshOwnershipInspectScript {
				return []byte(sshOwnershipMarkerState), nil
			}
			joined := strings.Join(command, " ")
			if strings.HasSuffix(joined, "compozy status -o json") {
				return jsonMarshalForTest(t, StatusRecord{Daemon: status}), nil
			}
			return nil, errors.New("unexpected command")
		}}

		resolution, err := resolveRemoteDaemon(
			t.Context(), executor, sshTarget{host: "remote.example", port: 22},
			connectSSHOptions{port: 22, remoteStart: true}, commandDeps{}.withDefaults(),
		)
		if resolution.started() {
			t.Fatalf("resolveRemoteDaemon() = %#v, want no ownership", resolution)
		}
		assertGatewayClientErrorCode(t, err, sshBusyCode)
	})

	t.Run("Should serialize simultaneous starts with remote ownership", func(t *testing.T) {
		var ownerMu sync.Mutex
		ownerToken := ""
		startRelease := make(chan struct{})
		firstAcquired := make(chan struct{})
		var startCalls atomic.Int32
		status := DaemonStatus{
			Status: "running", PID: 4404, StartedAt: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
			HTTPHost: "127.0.0.1", HTTPPort: 2123,
		}
		executor := sshExecutorStub{
			startLease: func(
				_ context.Context,
				_ sshTarget,
				_ string,
				ownership *sshDaemonOwnership,
			) (sshOwnershipLease, error) {
				ownerMu.Lock()
				defer ownerMu.Unlock()
				if ownerToken != "" {
					return nil, newSSHBusyError()
				}
				ownerToken = ownership.token
				close(firstAcquired)
				return sshOwnershipLeaseStub{close: func() error {
					ownerMu.Lock()
					ownerToken = ""
					ownerMu.Unlock()
					return nil
				}}, nil
			},
			run: func(_ context.Context, _ sshTarget, command []string) ([]byte, error) {
				joined := strings.Join(command, " ")
				switch {
				case strings.HasSuffix(joined, "compozy status -o json"):
					return nil, errors.New("daemon unavailable")
				case strings.Contains(joined, "daemon start"):
					startCalls.Add(1)
					<-startRelease
					return jsonMarshalForTest(t, status), nil
				default:
					return nil, errors.New("unexpected command")
				}
			},
		}
		deps := commandDeps{}.withDefaults()
		firstResult := make(chan remoteDaemonResolution, 1)
		firstErr := make(chan error, 1)
		go func() {
			resolution, err := resolveRemoteDaemon(
				t.Context(), executor, sshTarget{host: "remote.example", port: 22},
				connectSSHOptions{port: 22, remoteStart: true, ownerToken: "first-owner"}, deps,
			)
			firstResult <- resolution
			firstErr <- err
		}()
		<-firstAcquired

		second, err := resolveRemoteDaemon(
			t.Context(), executor, sshTarget{host: "remote.example", port: 22},
			connectSSHOptions{port: 22, remoteStart: true, ownerToken: "second-owner"}, deps,
		)
		if second.started() {
			t.Fatalf("second resolution = %#v, want no ownership", second)
		}
		assertGatewayClientErrorCode(t, err, sshBusyCode)
		close(startRelease)
		first := <-firstResult
		if err := <-firstErr; err != nil || !first.started() {
			t.Fatalf("first resolution = %#v, %v, want owned daemon", first, err)
		}
		if startCalls.Load() != 1 {
			t.Fatalf("daemon start calls = %d, want one", startCalls.Load())
		}
	})
}

func TestConnectSSHRecordsNonDefaultRemoteHome(t *testing.T) {
	t.Parallel()

	t.Run("Should persist and apply a non-default remote home [UT-113]", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		profile := compozyconfig.GatewayConnectionConfig{
			Name: "ssh-lab", Scheme: "ssh", Host: "lab.example", Port: 2222,
			RemoteHome: "/srv/compozy/operator-a",
		}
		if err := persistSSHProfile(t.Context(), commandDeps{}, homePaths, profile, false); err != nil {
			t.Fatalf("persistSSHProfile() error = %v", err)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		stored, exists := findGatewayProfile(loaded.Gateway.Connections, profile.Name)
		if !exists || stored.RemoteHome != profile.RemoteHome {
			t.Fatalf("stored SSH profile = %#v, want remote_home %q [UT-113]", stored, profile.RemoteHome)
		}
		for _, arguments := range [][]string{
			{"version", "-o", "json"},
			{"status", "-o", "json"},
			{"daemon", "start", "-o", "json"},
			{"daemon", "stop", "-o", "json"},
		} {
			command := remoteCompozyCommand(profile.RemoteHome, arguments...)
			if !slices.Contains(command, "COMPOZY_HOME="+profile.RemoteHome) {
				t.Fatalf("remote command = %#v, want non-default COMPOZY_HOME", command)
			}
		}
	})
}

func jsonMarshalForTest(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return payload
}
