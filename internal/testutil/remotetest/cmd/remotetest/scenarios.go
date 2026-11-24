//go:build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	fixtureEmail    = "pedro@example.com"
	cliName         = "Linux CLI"
	testHostName    = "testhost"
	testHost2Name   = "testhost2"
	searchTitle     = "Remote release handoff"
	sshAdd          = "ssh-add"
	accountAdd      = "account-add"
	revokeDevice    = "revoke-device"
	remoteSession   = "remote-session"
	browserPurchase = "browser-purchase"
	hostedPurchase  = "hosted-purchase"
	projectFileName = "remote-notes.txt"
	projectFileBody = "hello from remote computer project\n"
	freeUpgrade     = "free-upgrade"
)

const echoFixture = `{
  "version":2,
  "agents":[{"name":"ssh-fixture-agent","provider":"acpmock","permissions":"approve-all",
  "turns":[{"name":"echo","match":{"user_text":"echo"},"steps":[{"kind":"assistant","text":"echo from testhost"}]}]}]
}`

type scenarioManifest struct {
	Scenario           string           `json:"scenario"`
	CloudURL           string           `json:"cloud_url"`
	Controller         daemonInstance   `json:"controller"`
	CLIController      *daemonInstance  `json:"cli_controller,omitempty"`
	Computers          []computerTarget `json:"computers"`
	Email              string           `json:"email"`
	SessionID          string           `json:"session_id,omitempty"`
	SessionTitle       string           `json:"session_title,omitempty"`
	WorkspaceID        string           `json:"workspace_id,omitempty"`
	Hostname           string           `json:"hostname,omitempty"`
	ProjectFile        string           `json:"project_file,omitempty"`
	ProjectFileContent string           `json:"project_file_content,omitempty"`
	EdgePort           int              `json:"edge_port,omitempty"`
	HostedURL          string           `json:"hosted_url,omitempty"`
	HostedProxy        string           `json:"hosted_proxy,omitempty"`
	StripeURL          string           `json:"stripe_url,omitempty"`
}

type computerTarget struct {
	ID        remoteproto.ComputerID `json:"id,omitempty"`
	Name      string                 `json:"name"`
	Socket    string                 `json:"socket"`
	Container string                 `json:"container,omitempty"`
}

func prepareScenario(f *fixture, repo, name string) scenarioManifest {
	f.Helper()
	if name != sshAdd && name != accountAdd && name != revokeDevice && name != remoteSession &&
		name != browserPurchase && name != hostedPurchase && name != hostedBrowser && name != freeUpgrade &&
		name != signInFirstRun && name != signInSignOut && name != accountDeletePro {
		f.Fatalf("unknown remotetest scenario %q", name)
	}
	const pgEnv = "COMPOZY_CLOUD_TEST_POSTGRES"
	dsn := os.Getenv(pgEnv)
	if dsn == "" {
		f.Fatalf("set %s to a disposable Postgres database", pgEnv)
	}
	if os.Getenv(daemonBinaryEnv) == "" {
		f.Fatalf("set %s to the CompozyOS CLI binary", daemonBinaryEnv)
	}
	if name == hostedBrowser {
		db := remotetest.OpenDatabase(f, dsn)
		db.Migrate(f)
		mock := filepath.Join(f.TempDir(), "echo.json")
		writeFile(f, mock, echoFixture)
		return prepareHostedBrowser(f, repo, db, mock)
	}
	listenHost := ""
	var assets remotetest.SSHAssets
	if name == sshAdd || name == freeUpgrade {
		assets = remotetest.PrepareSSHAssets(f, repo)
		listenHost = assets.Gateway
	}
	db := remotetest.OpenDatabase(f, dsn)
	db.Migrate(f)
	if name == browserPurchase || name == hostedPurchase {
		return preparePurchase(f, db, name)
	}
	if name == freeUpgrade {
		return prepareFreeUpgrade(f, db, assets)
	}
	if name == signInFirstRun {
		return prepareSignInFirstRun(f, db)
	}
	if name == signInSignOut {
		return prepareSignInSignOut(f, db)
	}
	if name == accountDeletePro {
		return prepareAccountDeletePro(f, db)
	}
	cloudURL := startCloud(f, db, listenHost, nil).Server.URL
	controller := prepareDaemon(f, false)
	mock := filepath.Join(f.TempDir(), "echo.json")
	writeFile(f, mock, echoFixture)
	result := scenarioManifest{Scenario: name, CloudURL: cloudURL, Controller: controller, Email: fixtureEmail}

	if name == sshAdd {
		host, releaseEnv := prepareSSH(f, controller, assets, cloudURL, mock, testHostName)
		result.Computers = []computerTarget{host}
		startDaemon(f, controller, cloudURL, releaseEnv...)
	} else {
		startDaemon(f, controller, cloudURL)
	}
	result.Controller.DeviceID = signIn(f, controller)
	cliJSON[contract.OnboardingStatusResponse](f, controller, cloudURL, "onboarding", "complete")
	if name != sshAdd {
		prepareAccountHost(f, db, cloudURL, mock, &result)
	}
	return result
}

func startCloud(
	f *fixture, db *remotetest.Database, host string, configure func(*cloud.Config, *remotetest.IdentityProvider),
) *remotetest.Replica {
	f.Helper()
	hostname := host
	if hostname == "" {
		hostname = "127.0.0.1"
	}
	profile := &remotetest.ProviderProfile{Subject: "remote-e2e", Name: "Pedro", Email: fixtureEmail, Verified: true}
	idp, endpoints := remotetest.StartIdentityProvider(f, time.Now, profile, hostname)
	oauth, err := auth.NewOAuth(
		auth.OAuthClient{ID: remotetest.GoogleClient, Secret: "google-secret"},
		auth.OAuthClient{ID: "github-client", Secret: "github-secret"}, auth.WithOAuthEndpoints(endpoints),
	)
	if err != nil {
		f.Fatal(err)
	}
	check, err := idp.Turnstile()
	if err != nil {
		f.Fatal(err)
	}
	cfg := cloud.Config{
		Postgres: db.Config(f, "web"),
		Auth: &auth.Config{
			OAuth: oauth, EncryptionKey: bytes.Repeat([]byte{7}, 32), Turnstile: check,
			TurnstileSiteKey: "test-site-key", HostedDomain: "compozy.run",
		},
	}
	if configure != nil {
		configure(&cfg, idp)
	}
	return remotetest.StartLocalReplicaOn(f, host, cfg, nil)
}

func prepareSSH(
	f *fixture, controller daemonInstance, assets remotetest.SSHAssets, cloudURL, mock, hostName string,
) (computerTarget, []string) {
	f.Helper()
	release := remotetest.StartReleaseOn(
		f, assets.Gateway, assets.ArchiveName, remotetest.SSHFixtureVersion, assets.Archive,
	)
	trust := filepath.Join(controller.Home, "trusted-root.json")
	writeFile(f, trust, string(release.TrustRoot))
	key := filepath.Join(controller.OperatorHome, ".ssh", "id_ed25519")
	ctx, cancel := context.WithTimeout(f.Context(), 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).
		CombinedOutput(); err != nil {
		f.Fatalf("generate fixture SSH key: %v\n%s", err, out)
	}
	public, err := os.ReadFile(key + ".pub")
	if err != nil {
		f.Fatal(err)
	}
	host := remotetest.StartSSHHost(f, assets, cloudURL, string(public), "")
	remotetest.ConfigureSSHACP(f, host, mock)
	writeFile(f, filepath.Join(controller.OperatorHome, ".ssh", "config"), fmt.Sprintf(
		"Host %s\n HostName 127.0.0.1\n User %s\n Port %d\n IdentityFile %s\n IdentitiesOnly yes\n",
		hostName, remotetest.SSHUser, host.Port, key,
	))
	return computerTarget{Name: hostName, Container: host.Container,
			Socket: remotetest.SSHHome + "/.compozy/daemon.sock"},
		[]string{releaseEnv + "=" + release.Server.URL, trustEnv + "=" + trust}
}

func prepareAccountHost(f *fixture, db *remotetest.Database, cloudURL, mock string, m *scenarioManifest) {
	f.Helper()
	host := prepareDaemon(f, true)
	configureLocalAgent(f, host, mock)
	startDaemon(f, host, cloudURL)
	signIn(f, host)
	linked := cliJSON[contract.RemoteLinkResponse](f, host, cloudURL, "remote", "link")
	statusClient := httpClientFor(host)
	defer statusClient.CloseIdleConnections()
	remotetest.Await(f, "linked computer's direct route and lease", 20*time.Second, func(ctx context.Context) bool {
		var status contract.RemoteStatusResponse
		return requestJSON(ctx, statusClient, http.MethodGet, "/api/remote", nil, &status) == nil &&
			status.Entitlement != nil && len(status.Direct.Addresses) > 0
	})
	target := computerTarget{ID: linked.ComputerID, Name: linked.Name, Socket: host.Socket}
	if m.Scenario == revokeDevice {
		target.Name = testHostName
		m.CLIController = prepareSecondController(f, db, cloudURL, target)
	} else {
		workspace := filepath.Join(host.OperatorHome, "project")
		if err := os.Mkdir(workspace, 0o700); err != nil {
			f.Fatal(err)
		}
		if m.Scenario == remoteSession {
			writeFile(f, filepath.Join(workspace, projectFileName), projectFileBody)
			hostname, err := os.Hostname()
			if err != nil {
				f.Fatal(err)
			}
			m.Hostname = hostname
			m.ProjectFile = projectFileName
			m.ProjectFileContent = strings.TrimSpace(projectFileBody)
		}
		session := cliJSON[contract.SessionPayload](f, host, cloudURL, "session", "new",
			"--cwd", workspace, "--agent", remotetest.SSHAgent, "--name", searchTitle)
		if session.ID == "" {
			f.Fatal("session fixture returned no ID")
		}
		m.SessionID, m.SessionTitle, m.WorkspaceID = session.ID, searchTitle, session.WorkspaceID
		if m.Scenario == remoteSession {
			cliJSON[contract.Computer](f, m.Controller, cloudURL, "computer", "add", "account", string(linked.ComputerID))
			controllerClient := httpClientFor(m.Controller)
			defer controllerClient.CloseIdleConnections()
			path := contract.ComputersPath + "/" + string(linked.ComputerID)
			remotetest.Await(f, "controller's direct connection to remote host", 20*time.Second, func(ctx context.Context) bool {
				var view contract.Computer
				return requestJSON(ctx, controllerClient, http.MethodGet, path, nil, &view) == nil &&
					view.Status == "connected"
			})
		}
	}
	m.Computers = []computerTarget{target}
}

func configureLocalAgent(f *fixture, host daemonInstance, mock string) {
	f.Helper()
	paths, err := config.ResolveHomePathsFrom(host.Home)
	if err != nil {
		f.Fatal(err)
	}
	registered, err := acpmock.Register(
		paths,
		acpmock.RegisterOptions{FixturePath: mock, FixtureAgent: remotetest.SSHAgent},
	)
	if err != nil {
		f.Fatal(err)
	}
	provider := acpmock.ProviderConfig(registered.DriverPath)
	path := filepath.Join(host.Home, "config.toml")
	original, err := os.ReadFile(path)
	if err != nil {
		f.Fatal(err)
	}
	writeFile(f, path, string(original)+fmt.Sprintf(`
[providers.acpmock]
command = %q
display_name = %q
harness = %q
auth_mode = %q
none_security = %q
`, provider.Command, provider.DisplayName, provider.Harness, provider.AuthMode, provider.NoneSecurity))
}

func prepareSecondController(
	f *fixture,
	db *remotetest.Database,
	cloudURL string,
	host computerTarget,
) *daemonInstance {
	f.Helper()
	d := prepareDaemon(f, false)
	startDaemon(f, d, cloudURL)
	d.DeviceID = signIn(f, d)
	// CLI and desktop currently share daemon enrollment. Give the seeded CLI device its own kind and label.
	db.Exec(f, "UPDATE %s.devices SET name = $1, kind = 'cli' WHERE id = $2", cliName, d.DeviceID)
	cliJSON[contract.Computer](f, d, cloudURL, "computer", "add", "account", string(host.ID))
	client := httpClientFor(d)
	defer client.CloseIdleConnections()
	path := contract.ComputersPath + "/" + string(host.ID)
	if err := requestJSON(
		f.Context(),
		client,
		http.MethodPatch,
		path,
		map[string]string{"name": host.Name},
		nil,
	); err != nil {
		f.Fatal(err)
	}
	remotetest.Await(f, "second controller's direct connection", 20*time.Second, func(ctx context.Context) bool {
		var view contract.Computer
		return requestJSON(ctx, client, http.MethodGet, path, nil, &view) == nil && view.Status == "connected" &&
			view.RouteInUse != nil && view.RouteInUse.Kind == remoteproto.RouteDirect
	})
	return &d
}
