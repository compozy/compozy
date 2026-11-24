// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration && linux

// Suite: real OpenSSH computer setup through the controller and cloud.
// Invariants: admission, controller-side provenance and a confirmed key precede host commands;
// secrets stay at the operator boundary; resumable steps preserve the service and reservation.
package api_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/sse"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestDaemonSSHSetup(t *testing.T) {
	// not parallel: reuse one fixture image and release build across independent cloud/host cases.
	assets := prepareSSHFixtureAssets(t)
	t.Run("Should install the service and finish a turn through controller restart (IT-020)", func(t *testing.T) {
		testSSHServiceAndTurn(t, assets)
	})
	t.Run("Should confirm pins with an existing SSH master and through a bastion (IT-021)", func(t *testing.T) {
		testSSHTrust(t, assets)
	})
	t.Run("Should keep password answers on the operator pipe (IT-022)", func(t *testing.T) {
		testSSHPassword(t, assets)
	})
	t.Run("Should refuse an untrusted manifest or tampered archive (IT-023)", func(t *testing.T) {
		// Safety Invariant 19.
		j := newSSHJourney(t, assets)
		host := j.host(t, "")
		matchOutput := filepath.Join(j.operator, "match-executed")
		writeSSHFixtureFile(t, j.config, j.hostEntry("testhost", host)+
			fmt.Sprintf("Match originalhost testhost exec \"touch %s\"\n", matchOutput))
		d, _ := j.controller(t)
		signInSSHController(t, j.cloud, d)
		fingerprint := ssh.FingerprintSHA256(host.HostKey(t))
		before := sshConnections(host)
		j.release.BadSigner.Store(true)
		id := startSSHSetup(t, d, "testhost", fingerprint)
		expectSSHSetupFailure(t, d, id, sshsetup.StepVerify, "ssh_install_provenance_failed")
		if sshConnections(host) != before || j.release.Downloads.Load() != 0 {
			t.Fatal("the controller contacted SSH or downloaded an archive before verifying provenance")
		}
		if _, err := os.Stat(matchOutput); !os.IsNotExist(err) {
			t.Fatalf("preflight evaluated Match exec before verifying the release: %v", err)
		}
		host.Command(t, "test", "!", "-e", sshFixtureHome+"/.compozy/runtime/versions/"+sshFixtureVersion)
		j.release.BadSigner.Store(false)
		j.release.BadArchive.Store(true)
		if retry := startSSHSetup(t, d, "testhost", fingerprint); retry != id {
			t.Fatalf("release retry returned %q, want %q", retry, id)
		}
		expectSSHSetupFailure(t, d, id, sshsetup.StepInstall, "ssh_install_checksum_mismatch")
		if j.release.Downloads.Load() != 1 || sshConnections(host) <= before {
			t.Fatal("the host did not verify the tampered archive")
		}
		host.Command(t, "test", "!", "-e", sshFixtureHome+"/.compozy/runtime/versions/"+sshFixtureVersion)
	})
	t.Run("Should resume service only after revalidating the Free slot (IT-024)", func(t *testing.T) {
		for _, occupied := range []bool{false, true} {
			name := "Should resume when the released slot is still free"
			if occupied {
				name = "Should stop at admission when another setup took the slot"
			}
			t.Run(name, func(t *testing.T) { testSSHServiceResume(t, assets, occupied) })
		}
	})
	t.Run("Should verify an update before changing an incompatible host (IT-025)", func(t *testing.T) {
		testSSHUpdate(t, assets)
	})
	t.Run("Should free a canceled slot and refuse an add when Free is full (IT-027)", func(t *testing.T) {
		testSSHAdmissionAndCancel(t, assets)
	})
	t.Run("Should recover a committed link whose SSH response was lost (IT-028)", func(t *testing.T) {
		// Safety Invariants 7 and 12. Lose stdout only after the real link has committed.
		j := newSSHJourney(t, assets)
		host := j.host(t, "")
		host.Command(t, "touch", sshFixtureHome+"/fixture-drop-link")
		writeSSHFixtureFile(t, j.config, j.hostEntry("testhost", host))
		d, _ := j.controller(t)
		signInSSHController(t, j.cloud, d)
		fingerprint := ssh.FingerprintSHA256(host.HostKey(t))
		id := startSSHSetup(t, d, "testhost", fingerprint)
		expectSSHSetupFailure(t, d, id, sshsetup.StepLink, "computer_setup_interrupted")
		status := parseSSHJSON[contract.RemoteStatusResponse](t, host.CLI(t, "remote", "status", "--json"))
		if status.ComputerID == "" || !status.Enabled {
			t.Fatalf("remote link did not commit: %+v", status)
		}
		d.uds(t, http.MethodDelete, sshSetupPath(id), nil).expect(t, http.StatusConflict, "computer_setup_linked")
		if retry := startSSHSetup(t, d, "testhost", fingerprint); retry != id {
			t.Fatalf("ambiguous link created a second setup: %s", retry)
		}
		done := waitSSHSetup(t, d, id, sshsetup.StepDone)
		if done.ComputerID == nil || *done.ComputerID != status.ComputerID {
			t.Fatalf("resumed setup did not adopt %s: %+v", status.ComputerID, done)
		}
		if computers, consumed := j.consumedSlots(t); computers != 1 || consumed != 1 {
			t.Fatalf("after link recovery, computers = %d, consumed slots = %d", computers, consumed)
		}
	})
}

func testSSHTrust(t *testing.T, assets remotetest.SSHAssets) {
	t.Helper()
	// Safety Invariant 19. A configured live master must never stand in for the target key exchange.
	j := newSSHJourney(t, assets)
	host := j.host(t, "")
	bastion := j.host(t, "")
	private := j.host(t, bastion.Container)
	privateKey := ssh.FingerprintSHA256(private.HostKey(t))
	socket := filepath.Join(filepath.Dir(j.config), "user-master")
	userPins := filepath.Join(filepath.Dir(j.config), "user-known-hosts")
	writeSSHFixtureFile(
		t,
		userPins,
		fmt.Sprintf("[127.0.0.1]:%d %s", bastion.Port, ssh.MarshalAuthorizedKey(bastion.HostKey(t))),
	)
	writeSSHFixtureFile(t, j.config, j.hostEntry("testhost", host,
		" StrictHostKeyChecking accept-new", " ControlMaster auto", " ControlPath "+socket,
		" UserKnownHostsFile "+userPins)+j.hostEntry("fixture-bastion", bastion,
		" StrictHostKeyChecking yes", " UserKnownHostsFile "+userPins)+
		j.hostEntry("private-host", private, " StrictHostKeyChecking accept-new", " ProxyJump fixture-bastion"))
	openUserSSHMaster(t, j.config, socket)
	d, _ := j.controller(t)
	signInSSHController(t, j.cloud, d)
	beforeConn, beforeAuth := sshConnections(host), sshAuthentications(host)
	id := startSSHSetup(t, d, "testhost", "")
	fingerprint := ssh.FingerprintSHA256(host.HostKey(t))
	expectSSHTrust(t, d, id, fingerprint)
	if sshConnections(host) <= beforeConn || sshAuthentications(host) != beforeAuth {
		t.Fatal("the first-contact probe reused the user master or authenticated to the target")
	}
	answerSSHTrust(t, d, id, false)
	waitSSHSetup(t, d, id, sshsetup.Canceled)
	host.Command(t, "test", "!", "-e", sshFixtureHome+"/.compozy/runtime")
	host.Command(t, "test", "!", "-e", sshFixtureHome+"/fixture-commands")

	host.Command(t, "touch", sshFixtureHome+"/fixture-drop-service")
	id = startSSHSetup(t, d, "testhost", "")
	expectSSHTrust(t, d, id, fingerprint)
	answerSSHTrust(t, d, id, true)
	expectSSHSetupFailure(t, d, id, sshsetup.StepService, "computer_setup_interrupted")
	host.RotateKey(t)
	if retry := startSSHSetup(t, d, "testhost", ""); retry != id {
		t.Fatalf("retry after key rotation returned %s, want %s", retry, id)
	}
	expectSSHSetupFailure(t, d, id, sshsetup.StepService, "ssh_host_key_changed")

	id = startSSHSetup(t, d, "private-host", "")
	expectSSHTrust(t, d, id, privateKey)
	if sshAuthentications(private) != 0 {
		t.Fatal("the host behind the bastion authenticated before confirmation")
	}
	answerSSHTrust(t, d, id, true)
	assertSSHSetupConnected(t, d, id)
}

func testSSHPassword(t *testing.T, assets remotetest.SSHAssets) {
	t.Helper()
	// Safety Invariant 17. Exercise the helper with a real OpenSSH password prompt.
	j := newSSHJourney(t, assets)
	host := j.host(t, "")
	writeSSHFixtureFile(t, j.config, j.hostEntry("testhost", host,
		" PubkeyAuthentication no", " PreferredAuthentications password"))
	d, logPath := j.controller(t)
	signInSSHController(t, j.cloud, d)
	stream := d.stream(t, contract.ComputersPath+"/stream", "")
	id := startSSHSetup(t, d, "testhost", ssh.FingerprintSHA256(host.HostKey(t)))
	const secret = "wrong-ssh-password-for-IT-022"
	answer := contract.ComputerSetupAnswer{Kind: sshsetup.Secret, Value: secret}
	for attempt := range 3 {
		prompt := waitSSHSetup(t, d, id, sshsetup.NeedsOperatorSecret)
		if prompt.PromptKind != "password" || prompt.Prompt == nil {
			t.Fatalf("password prompt %d = %+v", attempt, prompt)
		}
		if attempt == 0 {
			body, err := json.Marshal(answer)
			if err != nil {
				t.Fatal(err)
			}
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"http://localhost"+sshSetupPath(id)+"/answer",
				bytes.NewReader(body),
			)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(agentidentity.HeaderSessionID, "sess-ssh-fixture")
			res, err := d.udsClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(res.Body)
			if closeErr := res.Body.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil || res.StatusCode != http.StatusForbidden ||
				!bytes.Contains(data, []byte("secret_answer_operator_only")) {
				t.Fatalf("agent password answer status = %d, error = %v", res.StatusCode, err)
			}
		}
		d.uds(t, http.MethodPost, sshSetupPath(id)+"/answer", answer).expect(t, http.StatusNoContent, "")
	}
	expectSSHSetupFailure(t, d, id, sshsetup.StepConnect, "ssh_auth_failed")
	for _, path := range []string{logPath, filepath.Join(j.home, "compozy.db"), filepath.Join(j.home, "compozy.db-wal")} {
		if data, err := os.ReadFile(path); err == nil && bytes.Contains(data, []byte(secret)) {
			t.Fatalf("SSH password leaked into %s", filepath.Base(path))
		}
	}
	stream.until(t, func(frame sse.Event) bool {
		if bytes.Contains(frame.Data, []byte(secret)) {
			t.Fatal("SSH password leaked into a computers event")
		}
		if frame.Event != "computer.setup" {
			return false
		}
		change := decodeDaemonAccountEvent[contract.ComputerEvent](t, frame).Data
		return change.SetupID == id && change.State == sshsetup.Failed
	})
}

func testSSHServiceResume(t *testing.T, assets remotetest.SSHAssets, occupied bool) {
	t.Helper()
	// Safety Invariants 7, 12 and 19.
	j := newSSHJourney(t, assets)
	host := j.host(t, "")
	config := j.hostEntry("testhost", host, " StrictHostKeyChecking accept-new")
	var other remotetest.SSHHost
	if occupied {
		other = j.host(t, "")
		config += j.hostEntry("testhost2", other)
	}
	writeSSHFixtureFile(t, j.config, config)
	d, _ := j.controller(t)
	signInSSHController(t, j.cloud, d)
	host.Command(t, "touch", sshFixtureHome+"/fixture-drop-service")
	fingerprint := ssh.FingerprintSHA256(host.HostKey(t))
	id := startSSHSetup(t, d, "testhost", fingerprint)
	expectSSHSetupFailure(t, d, id, sshsetup.StepService, "computer_setup_interrupted")
	if j.activeSlots(t) != 0 || j.release.Downloads.Load() != 1 {
		t.Fatal("interrupted service did not release its slot after installing the archive once")
	}
	if occupied {
		second := startSSHSetup(t, d, "testhost2", ssh.FingerprintSHA256(other.HostKey(t)))
		assertSSHSetupConnected(t, d, second)
	}
	before := sshConnections(host)
	if retry := startSSHSetup(t, d, "testhost", fingerprint); retry != id {
		t.Fatalf("service retry returned %s, want %s", retry, id)
	}
	if occupied {
		expectSSHSetupFailure(t, d, id, sshsetup.StepAdmit, "plan_limit_reached")
		if before != sshConnections(host) {
			t.Fatal("a retried setup contacted the host after losing the Free slot")
		}
		return
	}
	assertSSHSetupConnected(t, d, id)
	if j.release.Downloads.Load() != 1 {
		t.Fatal("resuming service repeated the archive download")
	}
	assertSSHUserService(t, host)
}

func testSSHAdmissionAndCancel(t *testing.T, assets remotetest.SSHAssets) {
	t.Helper()
	// Safety Invariants 7 and 19. Hold the release response to stop immediately after admission.
	j := newSSHJourney(t, assets)
	first, second := j.host(t, ""), j.host(t, "")
	writeSSHFixtureFile(t, j.config, j.hostEntry("testhost", first)+j.hostEntry("testhost2", second))
	d, _ := j.controller(t)
	signInSSHController(t, j.cloud, d)
	request, resume := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(resume) })
	j.release.ManifestGate.Store(&remotetest.ManifestGate{Requested: request, Resume: resume})
	id := startSSHSetup(t, d, "testhost", "")
	select {
	case <-request:
	case <-time.After(10 * time.Second):
		t.Fatal("the setup did not pass cloud admission")
	}
	if j.activeSlots(t) != 1 || sshConnections(first) != 0 {
		t.Fatal("the reservation and host contact were out of order")
	}
	canceled := parseSSHJSON[contract.ComputerSetupCanceled](t, j.cli(t, "computer", "setup", "cancel", id, "--json"))
	if canceled.State != sshsetup.Canceled || j.activeSlots(t) != 0 {
		t.Fatal("cancel returned before releasing the Free slot")
	}
	id = startSSHSetup(t, d, "testhost2", ssh.FingerprintSHA256(second.HostKey(t)))
	assertSSHSetupConnected(t, d, id)
	d.uds(t, http.MethodDelete, sshSetupPath(id), nil).expect(t, http.StatusConflict, "computer_setup_linked")
	refused := startSSHSetup(t, d, "testhost", "")
	expectSSHSetupFailure(t, d, refused, sshsetup.StepAdmit, "plan_limit_reached")
	if sshConnections(first) != 0 || sshAuthentications(first) != 0 {
		t.Fatal("a refused Free reservation contacted the host")
	}
}

func startSSHSetup(t *testing.T, d daemonAccountFixture, alias, pin string) string {
	t.Helper()
	reply := d.uds(t, http.MethodPost, contract.ComputersPath, contract.AddComputerRequest{
		Kind: contract.ComputerAddKindSSH, Host: alias, TrustHostFingerprint: pin,
	})
	reply.expect(t, http.StatusAccepted, "setup_id")
	return authJSON[contract.ComputerSetupStarted](t, reply).SetupID
}

func waitSSHSetup(t *testing.T, d daemonAccountFixture, id, target string) contract.ComputerSetupStatus {
	t.Helper()
	deadline := time.Now().Add(85 * time.Second)
	for time.Now().Before(deadline) {
		reply := d.uds(t, http.MethodGet, sshSetupPath(id)+"?wait_ms=500", nil)
		reply.expect(t, http.StatusOK, id)
		status := authJSON[contract.ComputerSetupStatus](t, reply)
		if status.State == target {
			return status
		}
		if slices.Contains([]string{sshsetup.Failed, sshsetup.StepDone, sshsetup.Canceled}, status.State) {
			t.Fatalf("setup %s returned %s while waiting for %s: %s", id, status.State, target, reply.body)
		}
	}
	t.Fatalf("setup %s did not reach %s", id, target)
	return contract.ComputerSetupStatus{}
}

func expectSSHSetupFailure(t *testing.T, d daemonAccountFixture, id, step, code string) {
	t.Helper()
	if out := waitSSHSetup(t, d, id, sshsetup.Failed); out.Error == nil || out.Error.Code != code || out.Step != step {
		t.Fatalf("setup %s failed at %s with %+v, want %s at %s", id, out.Step, out.Error, code, step)
	}
}

func assertSSHSetupConnected(t *testing.T, d daemonAccountFixture, id string) remoteproto.ComputerID {
	t.Helper()
	status := waitSSHSetup(t, d, id, sshsetup.StepDone)
	if status.ComputerID == nil || *status.ComputerID == "" {
		t.Fatalf("finished setup %s has no computer ID", id)
	}
	waitComputerRoute(t, d, *status.ComputerID, remoteproto.RouteSSH)
	return *status.ComputerID
}

func expectSSHTrust(t *testing.T, d daemonAccountFixture, id, fingerprint string) {
	t.Helper()
	status := waitSSHSetup(t, d, id, sshsetup.NeedsInput)
	if status.Prompt == nil || status.Prompt.Kind != sshsetup.TrustHost || status.Prompt.Fingerprint != fingerprint {
		t.Fatalf("SSH trust prompt = %+v, want %s", status.Prompt, fingerprint)
	}
}

func answerSSHTrust(t *testing.T, d daemonAccountFixture, id string, accept bool) {
	t.Helper()
	d.uds(t, http.MethodPost, sshSetupPath(id)+"/answer", contract.ComputerSetupAnswer{
		Kind: sshsetup.TrustHost, Accept: &accept,
	}).expect(t, http.StatusNoContent, "")
}

func sshSetupPath(id string) string { return contract.ComputerSetupsPath + "/" + id }

func parseSSHJSON[T any](t *testing.T, output string) T {
	t.Helper()
	var decoded T
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode fixture result: %v\n%s", err, output)
	}
	return decoded
}

func openUserSSHMaster(t *testing.T, config, socket string) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cmd := exec.CommandContext(ctx, "ssh", "-F", config, "-M", "-S", socket, "-N", "testhost")
	cmd.Stderr = t.Output()
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("open the user's SSH master: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})
	testutil.Eventually(t, "user SSH control socket", 10*time.Second, func() bool {
		info, err := os.Stat(socket)
		return err == nil && info.Mode()&os.ModeSocket != 0
	})
}

func sshConnections(h remotetest.SSHHost) int     { return strings.Count(h.ReadLog(), "Connection from ") }
func sshAuthentications(h remotetest.SSHHost) int { return strings.Count(h.ReadLog(), "Accepted ") }
