//go:build integration && linux

package api_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/sse"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
)

const (
	sshWorkspace = sshFixtureHome + "/code/api"
	sshAgent     = "ssh-fixture-agent"
	sshMarker    = "MARKER-7f3k"
)

func testSSHServiceAndTurn(t *testing.T, assets sshFixtureAssets) {
	t.Helper()
	// IT-020. Safety Invariants 1, 7, 11, 12, 15 and 19.
	j := newSSHJourney(t, assets)
	host := j.host(t, "")
	configureSSHFixtureACP(t, host)
	writeSSHFixtureFile(t, j.config, j.hostEntry("testhost", host))
	d, _ := j.controller(t)
	signInSSHController(t, j.cloud, d)
	events := d.stream(t, contract.ComputersPath+"/stream", "")
	j.cli(t, "computer", "add", "ssh", "testhost", "--trust-host-fingerprint", ssh.FingerprintSHA256(host.hostKey(t)))
	var steps []string
	events.until(t, func(frame sse.Event) bool {
		if frame.Event != "computer.setup" {
			return false
		}
		change := decodeDaemonAccountEvent[contract.ComputerEvent](t, frame).Data
		if len(steps) == 0 || steps[len(steps)-1] != change.Step {
			steps = append(steps, change.Step)
		}
		return change.State == sshsetup.StepDone
	})
	events.cancel()
	want := []string{"admit", "verify", "connect", "install", "service", "link", "forward", "done"}
	if !slices.Equal(steps, want) {
		t.Fatalf("setup steps = %v, want %v", steps, want)
	}
	assertSSHUserService(t, host)
	row := parseSSHJSON[contract.Computer](t, j.cli(t, "computer", "status", "testhost", "--json"))
	if row.Status != "connected" || row.RouteInUse == nil || row.RouteInUse.Kind != remoteproto.RouteSSH {
		t.Fatalf("SSH registry connection = %+v", row)
	}
	host.cli(t, "workspace", "add", sshWorkspace)
	session := parseSSHJSON[contract.SessionPayload](t, j.cli(t, "session", "new", "--computer", "testhost",
		"--workspace", sshWorkspace, "--agent", sshAgent, "--json"))
	if session.ID == "" {
		t.Fatal("target CLI did not return a session ID")
	}
	prompt := j.cli(t, "session", "prompt", session.ID, "--computer", "testhost", "reply with "+sshMarker)
	if !strings.Contains(prompt, session.ID) {
		t.Fatalf("prompt was not acknowledged for %s: %s", session.ID, prompt)
	}
	d.stop()
	killedAt := time.Now()
	testutil.Eventually(t, "remote turn after controller SIGKILL", 15*time.Second, func() bool {
		history := parseSSHJSON[[]contract.TurnHistoryPayload](
			t,
			host.cli(t, "session", "history", session.ID, "--json"),
		)
		return markerAfter(history, killedAt)
	})
	restarted, _ := j.controller(t)
	waitComputerRoute(t, restarted, row.ID, remoteproto.RouteSSH)
	history := parseSSHJSON[[]contract.TurnHistoryPayload](t,
		j.cli(t, "session", "history", session.ID, "--computer", "testhost", "--json"))
	if !markerAfter(history, killedAt) {
		t.Fatal("the restarted controller did not read the host's later assistant message")
	}
	state := parseSSHJSON[contract.SessionPayload](
		t,
		j.cli(t, "session", "status", session.ID, "--computer", "testhost", "--json"),
	)
	if string(state.State) != "idle" {
		t.Fatalf("completed session state = %s, want idle", state.State)
	}
}

func markerAfter(history []contract.TurnHistoryPayload, after time.Time) bool {
	for _, turn := range history {
		for _, event := range turn.Events {
			if event.Type == "agent_message" && event.Timestamp.After(after) &&
				bytes.Contains(event.Content, []byte(sshMarker)) {
				return true
			}
		}
	}
	return false
}

func configureSSHFixtureACP(t *testing.T, host sshFixtureHost) {
	t.Helper()
	fixture := filepath.Join(t.TempDir(), "ssh-marker.json")
	writeSSHFixtureFile(t, fixture, `{"version":2,"agents":[{"name":"ssh-fixture-agent","provider":"acpmock",
"permissions":"approve-all","turns":[{"name":"marker","match":{"user_text":"reply with MARKER-7f3k"},"steps":[
{"kind":"driver_control","driver_control":{"action":"delay","delay_ms":5000}},
{"kind":"assistant","text":"MARKER-7f3k"}]}]}]}`)
	paths, err := config.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registration, err := acpmock.Register(paths, acpmock.RegisterOptions{
		FixturePath: fixture, FixtureAgent: sshAgent, AgentName: sshAgent, ProviderName: acpmock.ProviderName,
	})
	if err != nil {
		t.Fatal(err)
	}
	definition, err := os.ReadFile(registration.AgentDefPath)
	if err != nil {
		t.Fatal(err)
	}
	const driver = sshFixtureHome + "/acpmock-driver"
	const script = sshFixtureHome + "/ssh-marker.json"
	host.copy(t, registration.DriverPath, driver)
	host.copy(t, fixture, script)
	host.command(t, "mkdir", "-p", sshWorkspace, sshFixtureHome+"/.compozy/agents/"+sshAgent)
	declared := strings.NewReplacer(registration.DriverPath, driver, registration.FixturePath, script,
		registration.DiagnosticsPath, sshFixtureHome+"/acpmock-diagnostics.jsonl").Replace(string(definition))
	host.write(t, sshFixtureHome+"/.compozy/agents/"+sshAgent+"/AGENT.md", declared)
	provider := acpmock.ProviderConfig(driver)
	cfg := host.command(t, "cat", sshFixtureHome+"/.compozy/config.toml") + fmt.Sprintf(`
[providers.acpmock]
command = %q
display_name = %q
harness = %q
auth_mode = %q
none_security = %q
`, provider.Command, provider.DisplayName, provider.Harness, provider.AuthMode, provider.NoneSecurity)
	host.write(t, sshFixtureHome+"/.compozy/config.toml", cfg)
	host.command(t, "chown", "-R", sshFixtureUser+":"+sshFixtureUser, sshFixtureHome)
}

func assertSSHUserService(t *testing.T, host sshFixtureHost) {
	t.Helper()
	const systemdDir = sshFixtureHome + "/.config/systemd/user"
	host.command(t, "test", "-f", systemdDir+"/compozy.service")
	host.command(t, "test", "-L", systemdDir+"/default.target.wants/compozy.service")
	host.command(t, "runuser", "-u", sshFixtureUser, "--", "env", "XDG_RUNTIME_DIR=/run/user/1500",
		"systemctl", "--user", "is-active", "--quiet", "compozy.service")
}

func testSSHUpdate(t *testing.T, assets sshFixtureAssets) {
	t.Helper()
	// IT-025. Safety Invariant 19 applies to an update as well as the initial install.
	j := newSSHJourney(t, assets)
	host := j.host(t, "")
	writeSSHFixtureFile(t, j.config, j.hostEntry("testhost", host))
	d, _ := j.controller(t)
	signInSSHController(t, j.cloud, d)
	id := assertSSHSetupConnected(t, d, startSSHSetup(t, d, "testhost", ssh.FingerprintSHA256(host.hostKey(t))))
	legacy, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const oldPath = sshFixtureHome + "/" + sshLegacyBinary
	const activePath = sshFixtureHome + "/.compozy/bin/compozy"
	host.copy(t, legacy, oldPath)
	host.command(t, "ln", "-sfn", oldPath, activePath)
	host.command(t, "runuser", "-u", sshFixtureUser, "--", "env", "XDG_RUNTIME_DIR=/run/user/1500",
		"systemctl", "--user", "restart", "compozy.service")
	testutil.Eventually(t, "protocol two excluded from the controller", 30*time.Second, func() bool {
		return readControllerComputer(t, d, id).Status == "incompatible"
	})

	before := host.command(t, "cat", sshFixtureHome+"/fixture-commands")
	downloads := j.release.Downloads.Load()
	j.release.BadSigner.Store(true)
	failed := parseSSHJSON[contract.ComputerUpdateStarted](t, j.cli(t, "computer", "update", "testhost", "--json"))
	expectSSHSetupFailure(t, d, failed.SetupID, sshsetup.StepVerify, "ssh_install_provenance_failed")
	if host.command(t, "cat", sshFixtureHome+"/fixture-commands") != before ||
		j.release.Downloads.Load() != downloads ||
		host.command(t, "readlink", activePath) != oldPath {
		t.Fatal("an unverified update contacted the host or replaced the old install")
	}
	j.release.BadSigner.Store(false)
	j.cli(t, "computer", "update", "testhost")
	waitComputerRoute(t, d, id, remoteproto.RouteSSH)
	if j.release.Downloads.Load() != downloads+1 || host.command(t, "readlink", activePath) == oldPath {
		t.Fatal("verified update did not replace the incompatible release")
	}
	d.uds(t, http.MethodGet, computerAPI(id), nil).expect(t, http.StatusOK, sshFixtureVersion)
}
