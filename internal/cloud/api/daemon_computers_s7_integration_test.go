//go:build integration

package api_test

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	s7AccountAEmail = "s7-account-a@example.test"
	s7AccountBEmail = "s7-account-b@example.test"
	s7JobsPath      = "/api/automation/jobs"
)

func TestDaemonComputersS7(t *testing.T) {
	root := t.TempDir()

	t.Run("Should isolate keyring keys, computer lists, and bridge routes across an account switch", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)

		// IT-026. On one controller installation, sign in as A (first trusted device),
		// link and add C1, sign out, sign in as B (first trusted device), link and add C2.
		controllerHome := filepath.Join(root, "switch-controller")
		controller, ring := startConfiguredDaemonAccount(t, f, controllerHome, daemonAccountBoot{})
		progress := controller.stream(t, contract.AccountSignInEventsPath, "")
		defer progress.cancel()

		firstA := signInDaemonAccount(t, f, controller, progress, s7AccountAEmail)
		accountAID, jktA := assertEnrolledAccount(t, f, firstA, ring)
		hostA, _ := startSessionComputerForEmail(t, f, filepath.Join(root, "switch-host-a"), s7AccountAEmail, false, false)
		c1 := linkSessionComputer(t, hostA)
		addAccountComputer(t, controller, c1)
		waitComputerRoute(t, controller, c1, remoteproto.RouteDirect)

		// Voluntary sign-out clears active remote access for A while preserving A's device key.
		controller.uds(t, http.MethodPost, contract.AccountLogoutPath, map[string]bool{"unlink": false}).
			expect(t, http.StatusOK, "signed_out")

		firstB := signInDaemonAccount(t, f, controller, progress, s7AccountBEmail)
		accountBID, jktB := assertEnrolledAccount(t, f, firstB, ring)
		hostB, _ := startSessionComputerForEmail(t, f, filepath.Join(root, "switch-host-b"), s7AccountBEmail, false, false)
		c2 := linkSessionComputer(t, hostB)
		if jktA == jktB {
			t.Fatalf("accounts A and B shared device key thumbprint %s", jktA)
		}
		if firstA.Device.ID == firstB.Device.ID {
			t.Fatalf("accounts A and B shared device ID %s", firstA.Device.ID)
		}
		// Both device keys exist in the keyring under device-key:<accountID> with distinct thumbprints.
		jwkA, jwkB := ring.publicKey(t, accountAID), ring.publicKey(t, accountBID)
		thumbA, errA := jwkA.Thumbprint()
		thumbB, errB := jwkB.Thumbprint()
		if errA != nil || errB != nil || thumbA != jktA || thumbB != jktB || thumbA == thumbB {
			t.Fatalf("keyring thumbprints A=%q(%v) B=%q(%v), want distinct %q and %q", thumbA, errA, thumbB, errB, jktA, jktB)
		}
		// Cloud has one device per account on the controller (plus the linked host devices).
		assertAuthRow(t, f, "SELECT count(*) FROM devices WHERE account_id = '"+accountAID+"' AND id = '"+firstA.Device.ID+"' AND revoked_at IS NULL", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM devices WHERE account_id = '"+accountBID+"' AND id = '"+firstB.Device.ID+"' AND revoked_at IS NULL", 1)

		addAccountComputer(t, controller, c2)
		waitComputerRoute(t, controller, c2, remoteproto.RouteDirect)

		// While B is signed in, GET /api/computers lists only C2, and /c/C1/api/sessions returns 404 computer_not_found.
		listB := controller.uds(t, http.MethodGet, contract.ComputersPath, nil)
		listB.expect(t, http.StatusOK, string(c2))
		computersB := authJSON[contract.ComputerListResponse](t, listB).Computers
		if len(computersB) != 1 || computersB[0].ID != c2 {
			t.Fatalf("while signed in as B, /api/computers = %+v, want only %s", computersB, c2)
		}
		controller.request(t, http.MethodGet, bridgeAPI(c1, controllerSessions), nil).
			expect(t, http.StatusNotFound, "computer_not_found")

		// Revoking B's device leaves A's device untouched.
		controller.uds(t, http.MethodDelete, contract.AccountPath+"/devices/"+firstB.Device.ID, nil).
			expect(t, http.StatusNoContent, "")
		assertAuthRow(t, f, "SELECT count(*) FROM devices WHERE id = '"+firstA.Device.ID+"' AND revoked_at IS NULL", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM devices WHERE id = '"+firstB.Device.ID+"' AND revoked_at IS NOT NULL", 1)

		// Sign out of B and sign back in as A: reuses A's original device ID and key, lists C1.
		controller.uds(t, http.MethodPost, contract.AccountLogoutPath, map[string]bool{"unlink": false}).
			expect(t, http.StatusOK, "signed_out")
		reA := signInAccountDevice(t, f, controller, progress, s7AccountAEmail)
		if reA.Device.ID != firstA.Device.ID {
			t.Fatalf("re-signed-in A got device %s, want original %s", reA.Device.ID, firstA.Device.ID)
		}
		_, reJktA := assertEnrolledAccount(t, f, reA, ring)
		if reJktA != jktA {
			t.Fatalf("re-signed-in A thumbprint = %s, want original %s", reJktA, jktA)
		}
		listA := controller.uds(t, http.MethodGet, contract.ComputersPath, nil)
		listA.expect(t, http.StatusOK, string(c1))
		computersA := authJSON[contract.ComputerListResponse](t, listA).Computers
		if len(computersA) != 1 || computersA[0].ID != c1 {
			t.Fatalf("after signing back in as A, /api/computers = %+v, want only %s", computersA, c1)
		}
	})

	t.Run("Should store and run a remote automation job on devbox while the controller is stopped", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		devbox, _ := startSessionComputerForEmail(
			t, f, filepath.Join(root, "automation-devbox"), registryTestEmail, true, true,
		)
		devboxID := linkSessionComputer(t, devbox)

		controllerHome := filepath.Join(root, "automation-controller")
		controller, ring := startConfiguredDaemonAccount(t, f, controllerHome, daemonAccountBoot{automation: true})
		progress := controller.stream(t, contract.AccountSignInEventsPath, "")
		signInAccountDevice(t, f, controller, progress, registryTestEmail)
		progress.cancel()
		addAccountComputer(t, controller, devboxID)
		waitComputerRoute(t, controller, devboxID, remoteproto.RouteDirect)

		// IT-046. Create the automation job targeting devbox over /c/<devbox>/api/automation/jobs.
		createReply := controller.uds(
			t,
			http.MethodPost,
			bridgeAPI(devboxID, s7JobsPath),
			contract.CreateJobRequest{
				Scope:     automationpkg.AutomationScopeGlobal,
				Name:      "devbox-nightly-audit",
				AgentName: computerFixtureAgent,
				Prompt:    computerFixturePrompt,
				Schedule: automationpkg.ScheduleSpec{
					Mode:     automationpkg.ScheduleModeEvery,
					Interval: "2s",
				},
			},
		)
		createReply.expect(t, http.StatusCreated, "devbox-nightly-audit")
		createdJob := authJSON[contract.JobResponse](t, createReply).Job
		if createdJob.ID == "" {
			t.Fatal("created remote automation job lacked an ID")
		}

		// Assert stored in devbox's automation_jobs and NOT in the controller's.
		devboxJobs := authJSON[contract.JobsResponse](t, devbox.uds(t, http.MethodGet, s7JobsPath, nil)).Jobs
		if len(devboxJobs) != 1 || devboxJobs[0].ID != createdJob.ID {
			t.Fatalf("devbox jobs = %+v, want job %s", devboxJobs, createdJob.ID)
		}
		localJobs := authJSON[contract.JobsResponse](t, controller.uds(t, http.MethodGet, s7JobsPath, nil)).Jobs
		if len(localJobs) != 0 {
			t.Fatalf("controller local jobs = %+v, want empty", localJobs)
		}

		// Stop the controller; devbox's scheduler still fires the job independently.
		controller.stop()
		runsPath := s7JobsPath + "/" + createdJob.ID + "/runs"
		testutil.Eventually(t, "devbox scheduler fires remote job while controller is stopped", 12*time.Second, func() bool {
			reply := devbox.uds(t, http.MethodGet, runsPath, nil)
			if reply.StatusCode != http.StatusOK {
				return false
			}
			runs := authJSON[contract.RunsResponse](t, reply).Runs
			return len(runs) > 0
		})

		// Restart the controller and confirm the remote job and its run appear through the bridge.
		restarted, _ := startConfiguredDaemonAccount(
			t, f, controllerHome, daemonAccountBoot{keyring: ring, automation: true},
		)
		waitComputerRoute(t, restarted, devboxID, remoteproto.RouteDirect)

		remoteListed := authJSON[contract.JobsResponse](
			t, restarted.uds(t, http.MethodGet, bridgeAPI(devboxID, s7JobsPath), nil),
		).Jobs
		if len(remoteListed) != 1 || remoteListed[0].ID != createdJob.ID {
			t.Fatalf("merged remote jobs after restart = %+v, want %s", remoteListed, createdJob.ID)
		}
		localAfterRestart := authJSON[contract.JobsResponse](
			t, restarted.uds(t, http.MethodGet, s7JobsPath, nil),
		).Jobs
		if len(localAfterRestart) != 0 {
			t.Fatalf("controller local jobs after restart = %+v, want empty", localAfterRestart)
		}
		remoteRuns := authJSON[contract.RunsResponse](
			t, restarted.uds(t, http.MethodGet, bridgeAPI(devboxID, runsPath), nil),
		).Runs
		if len(remoteRuns) == 0 {
			t.Fatal("expected recorded scheduler run for remote automation job after controller restart")
		}
	})
}

func startSessionComputerForEmail(
	t *testing.T, f authFixture, home, email string, withACP, withAutomation bool,
) (daemonAccountFixture, *daemonAccountKeyring) {
	t.Helper()
	boot := daemonAccountBoot{direct: true, automation: withAutomation}
	if withACP {
		configureComputerACP(t, home)
	}
	host, ring := startConfiguredDaemonAccount(t, f, home, boot)
	progress := host.stream(t, contract.AccountSignInEventsPath, "")
	signInAccountDevice(t, f, host, progress, email)
	progress.cancel()
	return host, ring
}
