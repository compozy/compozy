//go:build integration && linux

package api_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cli"
	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	"github.com/compozy/compozy/internal/version"
)

const (
	sshFixtureEmail   = "ssh-controller@example.test"
	sshFixtureRelease = "COMPOZY_TEST_SSH_RELEASE"
	sshFixtureTrust   = "COMPOZY_TEST_SSH_TRUST_ROOT"
	sshLegacyBinary   = "compozy-protocol-two"
)

func TestMain(m *testing.M) {
	os.Exit(runHostedFixtureProcess(func() int {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runSSHFixtureProcess(ctx, m.Run)
	}))
}

func runSSHFixtureProcess(ctx context.Context, runTests func() int) int {
	if used, err := sshsetup.RunAskpass(ctx, os.Args[1:], os.Stdout); used {
		if err != nil {
			return 1
		}
		return 0
	}
	exe, _ := os.Executable()
	if filepath.Base(exe) == sshLegacyBinary {
		remoteproto.CurrentProtocol = remoteproto.ProtocolRange{Min: 2, Max: 2}
		defer version.OverrideVersionForTesting("0.7.9")()
		return cli.ExecuteContext(ctx, os.Args[1:], os.Stdout, os.Stderr)
	}
	if base := os.Getenv(sshFixtureRelease); base != "" {
		if err := runSSHFixtureDaemon(ctx, base); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	return runTests()
}

func runSSHFixtureDaemon(ctx context.Context, releaseBase string) error {
	root, err := os.ReadFile(os.Getenv(sshFixtureTrust))
	if err != nil {
		return err
	}
	verifier, err := remotetest.ReleaseVerifier(root)
	if err != nil {
		return err
	}
	defer version.OverrideVersionForTesting(sshFixtureVersion)()
	d, err := daemon.New(daemon.WithSSHSetupOptions(sshsetup.Options{
		SSHConfig:   filepath.Join(os.Getenv("HOME"), ".ssh", "config"),
		ReleaseBase: releaseBase, Verifier: verifier,
	}))
	if err != nil {
		return err
	}
	return d.Run(ctx)
}

type sshJourney struct {
	assets                           sshFixtureAssets
	cloud                            authFixture
	release                          *remotetest.Release
	home, operator, socket, trust    string
	key, publicKey, config, httpBase string
}

func newSSHJourney(t *testing.T, assets sshFixtureAssets) sshJourney {
	t.Helper()
	f := newAuthFixtureWithTime(t, func(t *testing.T, cfg cloud.Config, register func(*cloud.App)) *remotetest.Replica {
		t.Helper()
		return remotetest.StartLocalReplicaOn(t, assets.gateway, cfg, register)
	}, time.Now)
	release := remotetest.StartReleaseOn(t, assets.gateway, assets.archiveName, sshFixtureVersion, assets.archive)
	root, err := os.MkdirTemp("", "rcssh-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	home, operator := filepath.Join(root, "data"), filepath.Join(root, "user")
	sshDir := filepath.Join(operator, ".ssh")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	port := testutil.FreeTCPPort(t)
	socket := filepath.Join(root, "d.sock")
	config := fmt.Sprintf(
		"[http]\nhost = '127.0.0.1'\nport = %d\n[daemon]\nsocket = %q\n[automation]\nenabled = false\n",
		port,
		socket,
	)
	writeSSHFixtureFile(t, filepath.Join(home, "config.toml"), config)
	trust := filepath.Join(root, "trusted_root.json")
	writeSSHFixtureFile(t, trust, string(release.TrustRoot))
	key := filepath.Join(sshDir, "id_ed25519")
	if out, err := exec.CommandContext(t.Context(), "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).
		CombinedOutput(); err != nil {
		t.Fatalf("generate fixture SSH key: %v\n%s", err, out)
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	return sshJourney{assets: assets, cloud: f, release: release, home: home, operator: operator,
		socket: socket, trust: trust, key: key, publicKey: string(pub), config: filepath.Join(sshDir, "config"),
		httpBase: "http://127.0.0.1:" + strconv.Itoa(port)}
}

func (j sshJourney) clientEnv() []string {
	return append(os.Environ(), "HOME="+j.operator, "COMPOZY_HOME="+j.home,
		"COMPOZY_CLOUD_URL="+j.cloud.replica.Server.URL, "COMPOZY_SESSION_ID=", "COMPOZY_AGENT=")
}

func (j sshJourney) controller(t *testing.T) (daemonAccountFixture, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.CreateTemp(t.TempDir(), "controller-*.log")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), executable, "-test.run=^$")
	cmd.Env = append(j.clientEnv(), sshFixtureRelease+"="+j.release.Server.URL, sshFixtureTrust+"="+j.trust)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	stop := sync.OnceFunc(func() {
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}
		if err := cmd.Wait(); err != nil {
			if _, ok := errors.AsType[*exec.ExitError](err); !ok {
				t.Error(err)
			}
		}
		if err := log.Close(); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(func() {
		stop()
		if t.Failed() {
			if data, err := os.ReadFile(log.Name()); err == nil {
				t.Logf("SSH controller process log:\n%s", data)
			}
		}
	})
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return new(net.Dialer).DialContext(ctx, "unix", j.socket)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	controller := daemonAccountFixture{httpBase: j.httpBase, socket: j.socket, stop: stop,
		httpClient: &http.Client{Timeout: 2 * time.Minute},
		udsClient:  &http.Client{Timeout: 10 * time.Second, Transport: transport}}
	testutil.Eventually(t, "SSH controller daemon ready", 25*time.Second, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/api/status", nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := controller.udsClient.Do(req)
		if err != nil {
			return false
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		return err == nil && res.StatusCode == http.StatusOK && strings.Contains(string(data), `"phase":"ready"`)
	})
	return controller, log.Name()
}

func signInSSHController(t *testing.T, f authFixture, d daemonAccountFixture) {
	t.Helper()
	login := d.uds(
		t,
		http.MethodPost,
		contract.AccountLoginPath,
		contract.AccountLoginRequest{Provider: new(auth.Google)},
	)
	login.expect(t, http.StatusOK, "verification_uri_complete")
	started := authJSON[contract.AccountLoginResponse](t, login)
	grant := remoteproto.DeviceFlowResponse{UserCode: started.UserCode}
	cookie := f.openActivation(t, grant, auth.Google)
	redirect := f.continueGrant(t, grant, auth.Google, cookie, false)
	path := f.callbackPath(t, auth.Google, redirect, providerProfile{
		email: sshFixtureEmail, subject: sshFixtureEmail, verified: true,
	})
	f.request(t, http.MethodGet, path, nil, cookie).expect(t, http.StatusOK, "Return to CompozyOS.")
	testutil.Eventually(t, "controller's Google device sign-in", 15*time.Second, func() bool {
		reply := d.uds(t, http.MethodGet, contract.AccountPath, nil)
		return reply.StatusCode == http.StatusOK && authJSON[contract.AccountStatusResponse](t, reply).SignedIn
	})
}

func (j sshJourney) host(t *testing.T, bastion string) sshFixtureHost {
	t.Helper()
	return startSSHFixtureHost(t, j.assets, j.cloud.replica.Server.URL, j.publicKey, bastion)
}

func (j sshJourney) cli(t *testing.T, args ...string) string {
	t.Helper()
	out, err := j.callCLI(t, args...)
	if err != nil {
		t.Fatalf("compozy %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func (j sshJourney) callCLI(t *testing.T, args ...string) (out string, err error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, j.assets.binary, args...)
	cmd.Env = j.clientEnv()
	result, err := cmd.CombinedOutput()
	return string(result), err
}

func (j sshJourney) hostEntry(alias string, h sshFixtureHost, settings ...string) string {
	return fmt.Sprintf("Host %s\n HostName 127.0.0.1\n User %s\n Port %d\n IdentityFile %s\n IdentitiesOnly yes\n%s\n",
		alias, sshFixtureUser, h.port, j.key, strings.Join(settings, "\n"))
}

func writeSSHFixtureFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (j sshJourney) consumedSlots(t *testing.T) (computers, consumed int) {
	t.Helper()
	if err := j.cloud.conn.QueryRow(t.Context(), `
		SELECT (SELECT count(*) FROM computers WHERE unlinked_at IS NULL),
		       (SELECT count(*) FROM slot_reservations WHERE consumed_at IS NOT NULL)
	`).Scan(&computers, &consumed); err != nil {
		t.Fatal(err)
	}
	return computers, consumed
}

func (j sshJourney) activeSlots(t *testing.T) int {
	t.Helper()
	var count int
	if err := j.cloud.conn.QueryRow(t.Context(), `
		SELECT count(*) FROM slot_reservations
		WHERE expires_at > now() AND consumed_at IS NULL AND released_at IS NULL
	`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
