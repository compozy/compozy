package sshsetup

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const testVersion = "0.8.0"
const testArchive = "compozy_linux_x86_64.tar.gz"
const testAccountID = "acc_ssh"
const testComputerID = "cmp_ssh"

var testTarget = target{Alias: "devbox", Host: "10.0.0.42", User: "pedro", Port: 22}
var testRequest = Request{Host: "devbox"}

func TestSSHSetupReleaseAndArguments(t *testing.T) {
	t.Parallel()
	t.Run("Should wait for the SSH forward even when the local port accepts connections", func(t *testing.T) {
		t.Parallel()
		// RC-08. An unrelated listener is no evidence that OpenSSH bound its forward.
		port, err := new(net.ListenConfig).Listen(t.Context(), "tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := port.Close(); err != nil {
				t.Error(err)
			}
		})
		forwarded := make(chan struct{})
		done := make(chan error, 1)
		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()
		ready, _, err := awaitForward(ctx, forwarded, done)
		if ready || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unconfirmed forward at %s was published: ready=%t, error=%v", port.Addr(), ready, err)
		}
		close(forwarded)
		ready, _, err = awaitForward(t.Context(), forwarded, done)
		if !ready || err != nil {
			t.Fatalf("confirmed forward was refused: ready=%t, error=%v", ready, err)
		}
	})
	t.Run("Should pin every command and disable SSH connection sharing (UT-040)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariant 19.
		pins := filepath.Join(t.TempDir(), "ssh", "known_hosts")
		for _, args := range [][]string{
			commandArgs(testTarget, pins, "uname -sm"),
			forwardArgs(testTarget, pins, "127.0.0.1:40000", "127.0.0.1:47823", true),
		} {
			for _, want := range []string{
				"RemoteCommand=none", "RequestTTY=no", "ConnectTimeout=10", "StrictHostKeyChecking=yes",
				"UserKnownHostsFile=" + pins, "GlobalKnownHostsFile=none", "ControlMaster=no", "ControlPath=none",
				"ControlPersist=no",
			} {
				if !slices.Contains(args, want) {
					t.Fatalf("SSH arguments %v do not include %s", args, want)
				}
			}
		}
	})

	t.Run("Should install the named release with the controller digest (UT-041)", func(t *testing.T) {
		t.Parallel()
		manifest := releaseManifest{dir: t.TempDir(), url: releaseOrigin + "/v" + testVersion, version: testVersion}
		sum := strings.Repeat("ab12", 16)
		if err := os.WriteFile(filepath.Join(manifest.dir, checksumsName),
			[]byte(sum+"  "+testArchive+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		command, err := manifest.installCommand("linux_x86_64")
		if err != nil || !strings.Contains(command, releaseOrigin+"/v0.8.0/"+testArchive) ||
			!strings.Contains(command, sum) {
			t.Fatalf("remote install command = %q, %v", command, err)
		}
		if strings.Contains(installScript, checksumsName) || !strings.Contains(installScript, "$versions/$version") ||
			!strings.Contains(
				installScript,
				"runtime/versions",
			) || !strings.Contains(installScript, "$1\" = \"$digest") {
			t.Fatal("the installer must use the supplied digest and version directory")
		}
	})

	for _, tc := range []struct{ uname, platform, label string }{
		{"Linux aarch64", "linux_arm64", "Should detect Linux arm64 (UT-042)"},
		{"Darwin x86_64", "darwin_x86_64", "Should detect macOS x86_64 (UT-042)"},
		{"FreeBSD amd64", "", "Should refuse unsupported platforms (UT-045)"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()
			got, err := parseUname(tc.uname)
			if got != tc.platform || (tc.platform == "" && !errors.Is(err, errPlatform)) ||
				(tc.platform != "" && err != nil) {
				t.Fatalf("platform for %s = %q, %v", tc.uname, got, err)
			}
		})
	}

	t.Run("Should report a failed download with its safe last line (UT-046)", func(t *testing.T) {
		t.Parallel()
		err := describeFailure(installFailure(testTarget, commandResult{
			code: 22, stderr: []byte("previous line\ncurl: (22) 404\n"),
		}, errors.New("exit 22")))
		if err.Code != "ssh_install_download_failed" || err.Details["last_line"] != "curl: (22) 404" {
			t.Fatalf("download failure = %+v", err)
		}
	})

	t.Run("Should refuse the wrong signing identity and a missing archive digest (UT-053)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariant 19.
		fixture := remotetest.StartRelease(t, "another-platform.tar.gz", testVersion, []byte("archive"))
		source := releaseSource{fixture.Server.URL, fixture.Server.Client(), fixture.Verifier}
		fixture.BadSigner.Store(true)
		if _, err := source.verify(t.Context(), testVersion); !errors.Is(err, errProvenance) {
			t.Fatalf("wrong signer returned %v", err)
		}
		fixture.BadSigner.Store(false)
		manifest, err := source.verify(t.Context(), testVersion)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(manifest.dir); err != nil {
				t.Error(err)
			}
		})
		if _, err := manifest.installCommand("linux_x86_64"); !errors.Is(err, errChecksum) {
			t.Fatalf("missing digest returned %v", err)
		}
	})
}

func TestSSHSetupLifecycle(t *testing.T) {
	t.Parallel()
	t.Run("Should quarantine the key exchange and reject a changed pin (UT-043)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariant 19.
		f := newSetupFixture(t, false)
		key := newHostKey(t)
		calls := make(chan []string, 2)
		f.ssh.runCommand = func(_ context.Context, args []string, _ io.Reader) (commandResult, error) {
			calls <- slices.Clone(args)
			if !slices.Contains(args, "-N") {
				return commandResult{
					code:   255,
					stderr: []byte("REMOTE HOST IDENTIFICATION HAS CHANGED"),
				}, errors.New(
					"ssh",
				)
			}
			path := sshKnownHostFile(args)
			line := append([]byte(testTarget.knownHost()+" "), ssh.MarshalAuthorizedKey(key)...)
			return commandResult{}, os.WriteFile(path, line, 0o600)
		}
		id := f.start(t)
		status := awaitSetupState(t, f.manager, id, NeedsInput)
		args := <-calls
		for _, flag := range []string{
			"-N", "StrictHostKeyChecking=accept-new", "PreferredAuthentications=none", "BatchMode=yes",
			"ControlMaster=no", "ControlPath=none", "ControlPersist=no", testTarget.Alias,
		} {
			if !slices.Contains(args, flag) {
				t.Fatalf("missing %s in %v", flag, args)
			}
		}
		if status.Prompt == nil || status.Prompt.Fingerprint != ssh.FingerprintSHA256(key) || len(calls) != 0 {
			t.Fatalf("host trust prompt = %+v, extra calls = %d", status, len(calls))
		}
		if err := f.manager.Answer(t.Context(), id, Answer{Kind: TrustHost, Accept: true}, true); err != nil {
			t.Fatal(err)
		}
		mustFailSetup(t, f.manager, id, "ssh_host_key_changed", StepConnect)
		if _, err := os.Stat(sshKnownHostFile(args)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("quarantine file remains: %v", err)
		}
		pin, err := readHostKey(f.manager.pins, testTarget.knownHost())
		if err != nil || pin == nil || ssh.FingerprintSHA256(pin) != ssh.FingerprintSHA256(key) {
			t.Fatalf("pin = %v, %v", pin, err)
		}
	})

	t.Run("Should resume service without running install twice (UT-048)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariant 12.
		f := newSetupFixture(t, true)
		f.seed(t, StepService, "")
		f.ssh.runCommand = func(_ context.Context, args []string, _ io.Reader) (commandResult, error) {
			command := args[len(args)-1]
			if strings.Contains(command, "service install") || strings.Contains(command, "remote status") {
				return successCommand(command), nil
			}
			return commandResult{}, fmt.Errorf("unexpected command %s", command)
		}
		if id := f.start(t); id != "setup_resume" {
			t.Fatalf("resume created another setup: %s", id)
		}
		awaitSetupState(t, f.manager, "setup_resume", StepDone)
	})

	t.Run("Should refuse an existing SSH route (UT-049)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariant 12.
		f := newSetupFixture(t, false)
		f.registry.added = true
		_, err := f.manager.Start(t.Context(), testRequest)
		if err == nil || describeFailure(err).Code != "computer_already_added" {
			t.Fatalf("duplicate route = %v", err)
		}
	})

	t.Run("Should reserve and verify release before contacting SSH (UT-052)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariants 7 and 19.
		f := newSetupFixture(t, true)
		f.cloud.reserveErr = planLimitFailure()
		mustFailSetup(t, f.manager, f.start(t), "plan_limit_reached", StepAdmit)
		f.cloud.reserveErr = nil
		f.release.BadSigner.Store(true)
		mustFailSetup(t, f.manager, f.start(t), errProvenance.Code, StepVerify)
		if len(f.ssh.commands) != 0 {
			t.Fatalf("host contacted before admission and verification: %v", f.ssh.commands)
		}
		f.release.BadSigner.Store(false)
		awaitSetupState(t, f.manager, f.start(t), StepDone)
		if got := f.registry.steps(); !slices.Equal(got[len(got)-6:],
			[]string{StepVerify, StepConnect, StepInstall, StepService, StepLink, StepForward}) {
			t.Fatalf("unexpected setup order: %v", got)
		}
	})

	t.Run("Should wait for worker stop and release before cancel returns (UT-112)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariants 7, 12 and 19.
		f := newSetupFixture(t, true)
		stopped := make(chan struct{})
		f.ssh.runCommand = func(ctx context.Context, args []string, _ io.Reader) (commandResult, error) {
			command := args[len(args)-1]
			if strings.HasPrefix(command, "sh -s") {
				<-ctx.Done()
				close(stopped)
				return commandResult{}, ctx.Err()
			}
			return successCommand(command), nil
		}
		f.cloud.releaseCheck = func() error {
			select {
			case <-stopped:
				return nil
			default:
				return errors.New("reservation was released before the worker stopped")
			}
		}
		id := f.start(t)
		awaitCommand(t, f.ssh.commands, "sh -s")
		if err := f.manager.Cancel(t.Context(), id); err != nil {
			t.Fatal(err)
		}
		awaitSetupState(t, f.manager, id, Canceled)
		f.seed(t, StepLink, testComputerID)
		if err := f.manager.Cancel(t.Context(), "setup_resume"); !errors.Is(err, ErrLinked) {
			t.Fatalf("cancel after link = %v", err)
		}
	})

	t.Run("Should revalidate an unlinked checkpoint before resumed SSH (UT-112)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariants 7, 12 and 19.
		f := newSetupFixture(t, true)
		f.seed(t, StepService, "")
		f.cloud.reserveErr = planLimitFailure()
		mustFailSetup(t, f.manager, f.start(t), "plan_limit_reached", StepAdmit)
		if f.cloud.previous != "rsv_previous" || len(f.ssh.commands) != 0 {
			t.Fatalf("resume previous = %s, SSH calls = %v", f.cloud.previous, f.ssh.commands)
		}
	})
}

func TestSSHAskpass(t *testing.T) {
	t.Parallel()
	t.Run("Should relay only operator answers and keep the value private (UT-044)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariant 17.
		f := newSetupFixture(t, true)
		const secret = "distinct-test-password"
		received := make(chan string, 1)
		f.ssh.runWithEnv = func(ctx context.Context, env []string) error {
			value, err := readAskpassAnswer(ctx, env)
			if err == nil {
				received <- value
			}
			return errors.New("SSH authentication ended")
		}
		id := f.start(t)
		status := awaitSetupState(t, f.manager, id, NeedsOperatorSecret)
		if status.Prompt == nil || status.Prompt.Kind != "password" {
			t.Fatalf("askpass state = %+v", status)
		}
		input := Answer{Kind: Secret, Value: secret}
		if err := f.manager.Answer(t.Context(), id, input, false); !errors.Is(err, ErrSecretOperatorOnly) {
			t.Fatalf("agent answer = %v", err)
		}
		if err := f.manager.Answer(t.Context(), id, input, true); err != nil {
			t.Fatal(err)
		}
		if got := awaitResult(t, received); got != secret {
			t.Fatal("askpass pipe lost the operator answer")
		}
		awaitSetupState(t, f.manager, id, Failed)
		row, err := f.db.GetComputerSSHSetup(t.Context(), testAccountID, id)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal([]any{row, f.registry.events})
		if err != nil || bytes.Contains(data, []byte(secret)) || strings.Contains(f.log.String(), secret) {
			t.Fatal("an SSH answer escaped the relay")
		}
	})
}

func readAskpassAnswer(ctx context.Context, env []string) (string, error) {
	const prefix = promptEndpointEnv + "="
	index := slices.IndexFunc(env, func(entry string) bool { return strings.HasPrefix(entry, prefix) })
	if index < 0 {
		return "", errors.New("missing askpass environment")
	}
	addr, token, _ := strings.Cut(strings.TrimPrefix(env[index], prefix), "/")
	conn, err := new(net.Dialer).DialContext(ctx, "tcp4", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "%s %s\n", token, classifyPrompt("Password:")); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	return strings.TrimSpace(line), err
}

type setupFixture struct {
	manager  *Manager
	db       *globaldb.GlobalDB
	ssh      *fixtureSSH
	cloud    *fixtureCloud
	registry *fixtureRegistry
	release  *remotetest.Release
	log      *bytes.Buffer
}

func newSetupFixture(t *testing.T, pin bool) setupFixture {
	t.Helper()
	home := t.TempDir()
	db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(home, "compozy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	release := remotetest.StartRelease(t, testArchive, testVersion, []byte("archive"))
	accounts := &fixtureAccounts{lifetime: t.Context(), secrets: make(map[string]string)}
	cloud, registry := &fixtureCloud{}, &fixtureRegistry{}
	var log bytes.Buffer
	manager := New(t.Context(), accounts, cloud, db, registry, Options{
		Home: home, OperatorHome: home, Version: testVersion, ReleaseBase: release.Server.URL,
		Verifier: release.Verifier, Logger: slog.New(slog.NewTextHandler(&log, nil)),
	})
	t.Cleanup(manager.Close)
	shell := &fixtureSSH{commands: make(chan string, 32)}
	manager.ssh = shell
	if pin {
		if err := manager.ensurePins(); err != nil {
			t.Fatal(err)
		}
		if err := manager.pinHostKey(testTarget, newHostKey(t)); err != nil {
			t.Fatal(err)
		}
	}
	return setupFixture{manager, db, shell, cloud, registry, release, &log}
}

func (f setupFixture) start(t *testing.T) string {
	t.Helper()
	id, err := f.manager.Start(t.Context(), testRequest)
	if err != nil || id == "" {
		t.Fatalf("start setup = %s, %v", id, err)
	}
	return id
}

func (f setupFixture) seed(t *testing.T, step, computer string) {
	t.Helper()
	row := store.ComputerSSHSetup{
		ID: "setup_resume", HostKey: testTarget.address(), AccountID: testAccountID, TargetAlias: testTarget.Alias,
		Kind: KindAdd, Step: step, Name: testTarget.Alias, ReleaseVersion: testVersion,
		Platform: "linux_x86_64", ReservationID: "rsv_previous", ComputerID: computer, UpdatedAt: setupTime(),
	}
	if err := f.db.CreateComputerSSHSetup(t.Context(), row); err != nil {
		t.Fatal(err)
	}
	if err := f.db.WriteComputerSSHSetup(t.Context(), row); err != nil {
		t.Fatal(err)
	}
}

func mustFailSetup(t *testing.T, manager *Manager, id, code, step string) {
	t.Helper()
	status := awaitSetupState(t, manager, id, Failed)
	if status.Error == nil || status.Error.Code != code || status.Step != step {
		t.Fatalf("setup returned %+v, want %s at %s", status, code, step)
	}
}

func awaitSetupState(t *testing.T, manager *Manager, id, state string) Status {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Second)
	defer cancel()
	for {
		status, err := manager.Status(ctx, id, 250*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		if status.State == state {
			return status
		}
		if status.State == Failed || status.State == Canceled || status.State == StepDone {
			t.Fatalf("setup returned %+v, want %s", status, state)
		}
	}
}

func awaitCommand(t *testing.T, calls <-chan string, expected string) {
	t.Helper()
	for {
		if strings.HasPrefix(awaitResult(t, calls), expected) {
			return
		}
	}
}

func awaitResult[T any](t *testing.T, results <-chan T) T {
	t.Helper()
	select {
	case entry := <-results:
		return entry
	case <-time.After(6 * time.Second):
		t.Fatal("fixture did not receive the expected command")
		var zero T
		return zero
	}
}

func newHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func sshKnownHostFile(args []string) string {
	for _, arg := range args {
		if path, ok := strings.CutPrefix(arg, "UserKnownHostsFile="); ok {
			return path
		}
	}
	return ""
}

type fixtureSSH struct {
	commands   chan string
	runWithEnv func(context.Context, []string) error
	runCommand func(context.Context, []string, io.Reader) (commandResult, error)
}

func (*fixtureSSH) resolve(context.Context, Request) (target, error) { return testTarget, nil }

func (f *fixtureSSH) run(ctx context.Context, args []string, stdin io.Reader, env []string) (commandResult, error) {
	cmd := args[len(args)-1]
	f.commands <- cmd
	if f.runWithEnv != nil {
		return commandResult{}, f.runWithEnv(ctx, env)
	}
	if f.runCommand != nil {
		return f.runCommand(ctx, args, stdin)
	}
	return successCommand(cmd), nil
}

func successCommand(command string) commandResult {
	switch {
	case command == "uname -sm":
		return commandResult{stdout: []byte("Linux x86_64")}
	case strings.Contains(command, "service install"):
		return commandResult{stdout: []byte("{\"manager\":\"systemd --user\",\"running\":true}")}
	case strings.Contains(command, "remote status"):
		return commandResult{stdout: []byte(
			"{\"computer_id\":\"cmp_ssh\",\"enabled\":true,\"listener\":\"127.0.0.1:47823\"}",
		)}
	default:
		return commandResult{}
	}
}

type fixtureAccounts struct {
	lifetime context.Context
	secrets  map[string]string
}

func (a *fixtureAccounts) IdentityLifetime() (context.Context, account.Installation) {
	return a.lifetime, account.Installation{AccountID: testAccountID, State: account.SignedIn}
}

func (*fixtureAccounts) UseDevice(
	ctx context.Context,
	call func(context.Context, cloudclient.Credentials) error,
) error {
	return call(ctx, cloudclient.Credentials{})
}

func (a *fixtureAccounts) GetHostSecret(key string) (string, bool, error) {
	value, found := a.secrets[key]
	return value, found, nil
}
func (a *fixtureAccounts) PutHostSecret(key, value string) error { a.secrets[key] = value; return nil }
func (a *fixtureAccounts) DeleteHostSecret(key string) error     { delete(a.secrets, key); return nil }

type fixtureCloud struct {
	reserveErr   error
	releaseCheck func() error
	previous     string
}

func (f *fixtureCloud) ReserveComputer(
	_ context.Context, _ cloudclient.Credentials, prev string,
) (cloudclient.ComputerReservation, error) {
	f.previous = prev
	return cloudclient.ComputerReservation{ID: "rsv_new", ExpiresAt: time.Now().Add(15 * time.Minute)}, f.reserveErr
}

func (f *fixtureCloud) ReleaseComputerReservation(context.Context, cloudclient.Credentials, string) error {
	if f.releaseCheck != nil {
		return f.releaseCheck()
	}
	return nil
}

func (*fixtureCloud) NewComputerLinkToken(
	context.Context, cloudclient.Credentials, string,
) (cloudclient.ComputerLinkToken, error) {
	return cloudclient.ComputerLinkToken{}, errors.New("unexpected link token request")
}

func (*fixtureCloud) Computers(context.Context, cloudclient.Credentials) ([]remoteproto.AccountComputer, error) {
	return []remoteproto.AccountComputer{{ID: testComputerID, AccountID: testAccountID}}, nil
}

func planLimitFailure() error {
	return &cloudclient.Failure{
		Status:  http.StatusPaymentRequired,
		Code:    "plan_limit_reached",
		Message: "Free slot in use",
	}
}

type fixtureRegistry struct {
	mu     sync.Mutex
	events []Status
	added  bool
}

func (f *fixtureRegistry) HasSSHRoute(context.Context, string, string) (bool, error) {
	return f.added, nil
}
func (*fixtureRegistry) SSHUpdateTarget(context.Context, string, remoteproto.ComputerID) (store.ComputerRoute, error) {
	return testTarget.route(), nil
}
func (*fixtureRegistry) RegisterSSHComputer(
	context.Context, string, string, remoteproto.ComputerID, store.ComputerRoute,
) error {
	return nil
}
func (*fixtureRegistry) WaitForSSHComputer(context.Context, string, remoteproto.ComputerID) error {
	return nil
}
func (f *fixtureRegistry) PublishSSHSetup(_ string, _ int64, status Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, status)
}

func (f *fixtureRegistry) steps() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	steps := make([]string, 0, len(f.events))
	for _, event := range f.events {
		if event.State == Running && event.Step != StepDone {
			steps = append(steps, event.Step)
		}
	}
	return slices.Compact(steps)
}
