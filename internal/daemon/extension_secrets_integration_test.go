//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestExtensionSecrets(t *testing.T) {
	t.Run("Should inject a global binding only after enable", testExtensionSecretBindingEnableInjection)
	t.Run("Should isolate a development binding from the global instance", testExtensionSecretDevBindingIsolation)
	t.Run("Should roll back a failed transport batch", testExtensionSecretTransportRollback)
	t.Run("Should retire global and development bindings by instance", testExtensionSecretBindingRetirement)
	t.Run("Should preserve remote header binding parity across HTTP and UDS", testExtensionRemoteHeaderBindingParity)
}

func testExtensionRemoteHeaderBindingParity(t *testing.T) {
	t.Parallel()

	const (
		extensionName = "remote-header-binding"
		secretValue   = "remote-header-secret-59f2"
	)
	harness := newExtensionSecretIntegrationHarness(t, extensionSecretIntegrationHarnessOptions{
		actorReason: "remote header transport parity", allowUnverified: true,
	})
	fixtureDir := writeRemoteHeaderExtensionFixture(t, t.TempDir(), extensionName)
	if _, err := harness.service.Install(t.Context(), contract.InstallExtensionRequest{
		Source: contract.InstallExtensionSourceLocalPath, Ref: fixtureDir, AllowUnverified: true,
	}, harness.actor); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	secretRef := vault.ExtensionSecretRef(extensionName, "", "DEPLOYMENT_KEY")
	if _, err := harness.vault.PutSecret(t.Context(), secretRef, "operator_remote_header", secretValue); err != nil {
		t.Fatalf("PutSecret(remote header) error = %v", err)
	}
	engines := []struct {
		name   string
		engine *gin.Engine
	}{
		{name: "HTTP", engine: harness.transport},
		{name: "UDS", engine: newExtensionTransportEngine(harness.service, harness.actor, "uds")},
	}
	for _, transport := range engines {
		setResponse := performExtensionTransportRequest(
			t,
			transport.engine,
			http.MethodPut,
			"/extensions/"+extensionName+"/secrets",
			mustExtensionTransportJSON(t, contract.SetExtensionSecretsRequest{
				Bindings: []contract.ExtensionSecretBindingInput{{
					EnvName: "DEPLOYMENT_KEY", VaultRef: &secretRef,
					MCPServer: "deployment-api", HeaderName: "X-Deployment-Key",
				}},
			}),
		)
		if setResponse.Code != http.StatusOK {
			t.Fatalf("%s set status = %d, want 200; body=%s", transport.name, setResponse.Code, setResponse.Body)
		}
		assertSecretsAbsent(
			t,
			transport.name+" set response",
			setResponse.Body.String(),
			[]string{secretValue, secretRef},
		)
		rows, err := harness.db.ExtensionEnvRepo.ListEnvBindings(t.Context(), extensionName, "")
		if err != nil {
			t.Fatalf("%s ListEnvBindings() error = %v", transport.name, err)
		}
		if len(rows) != 1 || rows[0].SecretRef != secretRef || rows[0].MCPServer != "deployment-api" ||
			rows[0].HeaderName != "X-Deployment-Key" {
			t.Fatalf("%s binding rows = %#v, want identical remote mapping", transport.name, rows)
		}
		listResponse := performExtensionTransportRequest(
			t, transport.engine, http.MethodGet, "/extensions/"+extensionName+"/secrets", nil,
		)
		if listResponse.Code != http.StatusOK {
			t.Fatalf("%s list status = %d, want 200; body=%s", transport.name, listResponse.Code, listResponse.Body)
		}
		var payload contract.ExtensionSecretsPayload
		if err := json.Unmarshal(listResponse.Body.Bytes(), &payload); err != nil {
			t.Fatalf("%s json.Unmarshal(list) error = %v", transport.name, err)
		}
		if len(payload.Bindings) != 1 || payload.Bindings[0].EnvName != "DEPLOYMENT_KEY" ||
			payload.Bindings[0].MCPServer != "deployment-api" ||
			payload.Bindings[0].HeaderName != "X-Deployment-Key" || payload.Bindings[0].Stale {
			t.Fatalf("%s presence-only payload = %#v", transport.name, payload)
		}
		assertSecretsAbsent(
			t,
			transport.name+" list response",
			listResponse.Body.String(),
			[]string{secretValue, secretRef},
		)
	}

	missingRef := vault.ExtensionSecretRef(extensionName, "", "MISSING_KEY")
	danglingResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPut,
		"/extensions/"+extensionName+"/secrets",
		mustExtensionTransportJSON(t, contract.SetExtensionSecretsRequest{
			Bindings: []contract.ExtensionSecretBindingInput{{
				EnvName: "MISSING_KEY", VaultRef: &missingRef,
				MCPServer: "deployment-api", HeaderName: "X-Missing-Key",
			}},
		}),
	)
	if danglingResponse.Code != http.StatusBadRequest ||
		!strings.Contains(danglingResponse.Body.String(), "extension_env_binding_dangling") {
		t.Fatalf(
			"dangling remote binding status = %d body=%s, want binding error",
			danglingResponse.Code,
			danglingResponse.Body,
		)
	}
}

type extensionSecretIntegrationHarnessOptions struct {
	actorReason      string
	actorWorkspaceID string
	allowUnverified  bool
	failingVault     bool
	workspaceID      string
}

type extensionSecretIntegrationHarness struct {
	actor         taskpkg.ActorContext
	db            *globaldb.GlobalDB
	failingVault  *extensionSecretFailingVault
	service       *daemonExtensionService
	transport     *gin.Engine
	vault         *vault.Service
	workspaceRoot string
}

func newExtensionSecretIntegrationHarness(
	t *testing.T,
	opts extensionSecretIntegrationHarnessOptions,
) *extensionSecretIntegrationHarness {
	t.Helper()

	homePaths := testHomePaths(t)
	db := openDaemonTestGlobalDB(t)
	registry := extensionpkg.NewRegistry(db.DB())
	vaultService, err := vault.NewService(
		db.VaultRepo,
		vault.NewFileKeyProvider(t.TempDir(), nil),
	)
	if err != nil {
		t.Fatalf("vault.NewService() error = %v", err)
	}

	serviceVault := extensionSecretVault(vaultService)
	var failingVault *extensionSecretFailingVault
	if opts.failingVault {
		failingVault = &extensionSecretFailingVault{extensionSecretVault: vaultService}
		serviceVault = failingVault
	}

	managerOptions := []extensionpkg.Option{
		extensionpkg.WithLogger(discardLogger()),
		extensionpkg.WithSecretResolver(vaultService),
		extensionpkg.WithEnvBindingStore(db.ExtensionEnvRepo),
	}
	var workspaceResolver *daemonExtensionWorkspaceResolverStub
	var workspaceRoot string
	if opts.workspaceID != "" {
		workspaceRoot = t.TempDir()
		workspaceResolver = &daemonExtensionWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID: opts.workspaceID, Name: "extension-secret-integration", RootDir: workspaceRoot,
			},
			WorkspaceID: opts.workspaceID,
		}}
		managerOptions = append(managerOptions, extensionpkg.WithWorkspaceResolver(workspaceResolver))
	}

	manager := extensionpkg.NewManager(registry, managerOptions...)
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Errorf("Manager.Stop() error = %v", err)
		}
	})

	serviceOptions := []daemonExtensionServiceOption{
		withDaemonExtensionAutomation(&fakeAutomationManager{}),
		withDaemonExtensionEventWriter(db),
		withDaemonExtensionSecrets(db.ExtensionEnvRepo, serviceVault),
	}
	if workspaceResolver != nil {
		serviceOptions = append(serviceOptions, withDaemonExtensionWorkspaceResolver(workspaceResolver))
	}
	if opts.allowUnverified {
		serviceOptions = append(serviceOptions, withDaemonExtensionMarketplace(
			compozyconfig.ExtensionsConfig{
				Trust: compozyconfig.ExtensionsTrustConfig{AllowUnverified: true},
			},
			nil,
		))
	}
	service, ok := newDaemonExtensionService(daemonExtensionServiceDeps{
		Registry:  registry,
		Runtime:   manager,
		HomePaths: homePaths,
		Logger:    discardLogger(),
		Now:       time.Now,
	},
		serviceOptions...,
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	actor := newExtensionSecretIntegrationActor(t, opts.actorWorkspaceID, opts.actorReason)

	return &extensionSecretIntegrationHarness{
		actor:         actor,
		db:            db,
		failingVault:  failingVault,
		service:       service,
		transport:     newExtensionTransportEngine(service, actor, "http"),
		vault:         vaultService,
		workspaceRoot: workspaceRoot,
	}
}

func newExtensionSecretIntegrationActor(
	t *testing.T,
	workspaceID string,
	reason string,
) taskpkg.ActorContext {
	t.Helper()
	if workspaceID == "" {
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindHTTP, reason)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		return actor
	}
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		workspaceID,
		taskpkg.OriginKindHTTP,
		reason,
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	return actor
}

func testExtensionSecretBindingRetirement(t *testing.T) {
	t.Parallel()

	const (
		extensionName = "binding-retirement"
		workspaceID   = "workspace-binding-retirement"
		ownedGlobal   = "owned-global-secret-705a"
		ownedDev      = "owned-dev-secret-a42f"
		foreignValue  = "foreign-owned-secret-4bd1"
	)
	harness := newExtensionSecretIntegrationHarness(t, extensionSecretIntegrationHarnessOptions{
		actorReason:     "global binding retirement",
		allowUnverified: true,
		workspaceID:     workspaceID,
	})
	devActor := newExtensionSecretIntegrationActor(t, workspaceID, "development binding retirement")
	fixtureDir := writeBoundSecretExtensionFixtureWithEnv(
		t,
		t.TempDir(),
		extensionName,
		[]string{"BOUND_SECRET", "OTHER_SECRET"},
	)
	if _, err := harness.service.Install(t.Context(), contract.InstallExtensionRequest{
		Source: contract.InstallExtensionSourceLocalPath, Ref: fixtureDir, AllowUnverified: true,
	}, harness.actor); err != nil {
		t.Fatalf("Install(global) error = %v", err)
	}
	foreignRef := vault.ExtensionSecretRef(extensionName, "", "FOREIGN_SHARED")
	if _, err := harness.vault.PutSecret(t.Context(), foreignRef, "foreign_token", foreignValue); err != nil {
		t.Fatalf("PutSecret(foreign) error = %v", err)
	}
	if _, err := harness.service.SetExtensionSecrets(
		t.Context(),
		extensionName,
		contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
			{EnvName: "BOUND_SECRET", Value: extensionSecretInputValue(ownedGlobal)},
			{EnvName: "OTHER_SECRET", VaultRef: &foreignRef},
		}},
		harness.actor,
	); err != nil {
		t.Fatalf("SetExtensionSecrets(global) error = %v", err)
	}
	globalOwnedRef := vault.ExtensionSecretRef(extensionName, "", "BOUND_SECRET")
	if _, err := harness.service.Remove(t.Context(), extensionName, harness.actor); err != nil {
		t.Fatalf("Remove(global) error = %v", err)
	}
	assertExtensionBindingsRetired(
		t,
		harness.db,
		harness.vault,
		extensionName,
		"",
		globalOwnedRef,
		foreignRef,
		foreignValue,
	)
	if _, err := harness.service.Install(t.Context(), contract.InstallExtensionRequest{
		Source: contract.InstallExtensionSourceLocalPath, Ref: fixtureDir, AllowUnverified: true,
	}, harness.actor); err != nil {
		t.Fatalf("Install(global after removal) error = %v", err)
	}
	reinstalledSecrets, err := harness.service.ListExtensionSecrets(
		t.Context(),
		extensionName,
		harness.actor,
	)
	if err != nil {
		t.Fatalf("ListExtensionSecrets(reinstalled global) error = %v", err)
	}
	if len(reinstalledSecrets.BoundEnvKeys) != 0 || len(reinstalledSecrets.Bindings) != 0 {
		t.Fatalf("reinstalled global bindings = %#v, want empty", reinstalledSecrets)
	}

	origin := filepath.Join(harness.workspaceRoot, "binding-retirement-extension")
	firstGeneration := writeBoundSecretExtensionGenerationWithEnv(
		t,
		origin,
		extensionName,
		"2.0.0",
		[]string{"BOUND_SECRET", "OTHER_SECRET"},
	)
	if _, err := harness.service.Dev(t.Context(), contract.DevLinkExtensionRequest{
		OriginPath: origin, GenerationHash: firstGeneration,
	}, devActor); err != nil {
		t.Fatalf("Dev() error = %v", err)
	}
	devForeignRef := vault.ExtensionSecretRef(extensionName, workspaceID, "FOREIGN_SHARED")
	if _, err := harness.vault.PutSecret(t.Context(), devForeignRef, "foreign_token", foreignValue); err != nil {
		t.Fatalf("PutSecret(dev foreign) error = %v", err)
	}
	if _, err := harness.service.SetExtensionSecrets(
		t.Context(),
		extensionName,
		contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
			{EnvName: "BOUND_SECRET", Value: extensionSecretInputValue(ownedDev)},
			{EnvName: "OTHER_SECRET", VaultRef: &devForeignRef},
		}},
		devActor,
	); err != nil {
		t.Fatalf("SetExtensionSecrets(dev) error = %v", err)
	}
	logsBeforeReload, err := harness.service.ExtensionLogs(t.Context(), extensionName, 0, "", devActor)
	if err != nil {
		t.Fatalf("ExtensionLogs(before manifest drop) error = %v", err)
	}
	var sequenceBeforeReload int64
	if len(logsBeforeReload.Logs) > 0 {
		sequenceBeforeReload = logsBeforeReload.Logs[len(logsBeforeReload.Logs)-1].Sequence
	}
	secondGeneration := writeBoundSecretExtensionGenerationWithEnv(
		t,
		origin,
		extensionName,
		"2.1.0",
		[]string{"OTHER_SECRET"},
	)
	if _, err := harness.service.ReloadDev(t.Context(), extensionName, contract.ReloadExtensionRequest{
		GenerationHash: secondGeneration,
	}, devActor); err != nil {
		t.Fatalf("ReloadDev(drop declared env) error = %v", err)
	}
	devSecrets, err := harness.service.ListExtensionSecrets(t.Context(), extensionName, devActor)
	if err != nil {
		t.Fatalf("ListExtensionSecrets(dev after reload) error = %v", err)
	}
	if len(devSecrets.Bindings) != 2 || !devSecrets.Bindings[0].Stale ||
		devSecrets.Bindings[0].EnvName != "BOUND_SECRET" || devSecrets.Bindings[1].Stale {
		t.Fatalf("dev bindings after manifest drop = %#v, want stale BOUND_SECRET only", devSecrets.Bindings)
	}
	logMatch := waitForSecretExtensionLogsAfter(
		t,
		harness.service,
		extensionName,
		devActor,
		sequenceBeforeReload,
		logsBeforeReload.StreamEpoch,
		"runtime_secret=",
	)
	if strings.Contains(logMatch.matched.Message, "runtime_secret=[REDACTED]") {
		t.Fatalf("matched dev log = %q, stale binding was injected", logMatch.matched.Message)
	}
	devOwnedRef := vault.ExtensionSecretRef(extensionName, workspaceID, "BOUND_SECRET")
	if _, err := harness.service.RemoveScoped(t.Context(), extensionName, devActor); err != nil {
		t.Fatalf("RemoveScoped(dev) error = %v", err)
	}
	assertExtensionBindingsRetired(
		t,
		harness.db,
		harness.vault,
		extensionName,
		workspaceID,
		devOwnedRef,
		devForeignRef,
		foreignValue,
	)
}

func assertExtensionBindingsRetired(
	t *testing.T,
	db *globaldb.GlobalDB,
	secretVault *vault.Service,
	extensionName string,
	workspaceID string,
	ownedRef string,
	foreignRef string,
	foreignValue string,
) {
	t.Helper()
	rows, err := db.ExtensionEnvRepo.ListEnvBindings(t.Context(), extensionName, workspaceID)
	if err != nil {
		t.Fatalf("ListEnvBindings(%q) error = %v", workspaceID, err)
	}
	if len(rows) != 0 {
		t.Fatalf("bindings after retirement for workspace %q = %#v, want none", workspaceID, rows)
	}
	if _, err := secretVault.ResolveRef(t.Context(), ownedRef); !errors.Is(err, vault.ErrSecretNotFound) {
		t.Fatalf("ResolveRef(owned %q) error = %v, want secret removed", ownedRef, err)
	}
	gotForeign, err := secretVault.ResolveRef(t.Context(), foreignRef)
	if err != nil {
		t.Fatalf("ResolveRef(foreign %q) error = %v", foreignRef, err)
	}
	if gotForeign != foreignValue {
		t.Fatalf("foreign ref %q = %q, want preserved", foreignRef, gotForeign)
	}
}

func testExtensionSecretDevBindingIsolation(t *testing.T) {
	t.Parallel()

	const (
		extensionName = "dev-binding-isolation"
		workspaceID   = "workspace-dev-binding"
		devSecret     = "workspace-only-secret-f18a"
		globalSecret  = "global-only-secret-c53b"
	)
	harness := newExtensionSecretIntegrationHarness(t, extensionSecretIntegrationHarnessOptions{
		actorReason:      "development binding isolation",
		actorWorkspaceID: workspaceID,
		workspaceID:      workspaceID,
	})
	origin := filepath.Join(harness.workspaceRoot, "dev-binding-extension")
	firstGeneration := writeBoundSecretExtensionGeneration(t, origin, extensionName, "1.0.0")
	devResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPost,
		"/extensions/dev",
		mustExtensionTransportJSON(t, contract.DevLinkExtensionRequest{
			OriginPath: origin, GenerationHash: firstGeneration,
		}),
	)
	if devResponse.Code != http.StatusCreated {
		t.Fatalf("dev link status = %d, want %d; body=%s", devResponse.Code, http.StatusCreated, devResponse.Body)
	}
	boundValue := devSecret
	setResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPut,
		"/extensions/"+extensionName+"/secrets",
		mustExtensionTransportJSON(
			t,
			contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "BOUND_SECRET", Value: &boundValue},
			}},
		),
	)
	if setResponse.Code != http.StatusOK {
		t.Fatalf("dev set status = %d, want %d; body=%s", setResponse.Code, http.StatusOK, setResponse.Body)
	}
	workspaceRows, err := harness.db.ExtensionEnvRepo.ListEnvBindings(
		t.Context(),
		extensionName,
		workspaceID,
	)
	if err != nil {
		t.Fatalf("ListEnvBindings(workspace) error = %v", err)
	}
	if len(workspaceRows) != 1 ||
		workspaceRows[0].SecretRef != vault.ExtensionSecretRef(extensionName, workspaceID, "BOUND_SECRET") {
		t.Fatalf("workspace bindings = %#v, want exact workspace-owned ref", workspaceRows)
	}
	globalRows, err := harness.db.ExtensionEnvRepo.ListEnvBindings(t.Context(), extensionName, "")
	if err != nil {
		t.Fatalf("ListEnvBindings(global) error = %v", err)
	}
	if len(globalRows) != 0 {
		t.Fatalf("global bindings after dev set = %#v, want none", globalRows)
	}

	secondGeneration := writeBoundSecretExtensionGeneration(t, origin, extensionName, "1.1.0")
	reloadResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPost,
		"/extensions/"+extensionName+"/reload",
		mustExtensionTransportJSON(t, contract.ReloadExtensionRequest{GenerationHash: secondGeneration}),
	)
	if reloadResponse.Code != http.StatusOK {
		t.Fatalf(
			"reload with workspace binding status = %d, want %d; body=%s",
			reloadResponse.Code,
			http.StatusOK,
			reloadResponse.Body,
		)
	}
	logs := waitForSecretExtensionLogs(t, harness.service, extensionName, harness.actor, "runtime_secret=")
	boundLogsJSON := string(mustExtensionTransportJSON(t, logs))
	assertSecretsAbsent(t, "workspace-bound logs", boundLogsJSON, []string{devSecret})
	if !strings.Contains(boundLogsJSON, "runtime_secret=[REDACTED]") {
		t.Fatalf("workspace-bound logs = %s, want proof of injected and redacted binding", boundLogsJSON)
	}
	latestSequence := logs[len(logs)-1].Sequence

	globalRef := vault.ExtensionSecretRef(extensionName, "", "BOUND_SECRET")
	if _, err := harness.vault.PutSecret(
		t.Context(),
		globalRef,
		extensionpkg.ExtensionEnvBindingKind,
		globalSecret,
	); err != nil {
		t.Fatalf("PutSecret(global sentinel) error = %v", err)
	}
	if err := harness.db.ExtensionEnvRepo.PutEnvBinding(t.Context(), extensionpkg.EnvBinding{
		ExtensionName: extensionName,
		EnvName:       "BOUND_SECRET",
		SecretRef:     globalRef,
		Kind:          extensionpkg.ExtensionEnvBindingKind,
	}); err != nil {
		t.Fatalf("PutEnvBinding(global sentinel) error = %v", err)
	}
	deleteResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodDelete,
		"/extensions/"+extensionName+"/secrets/BOUND_SECRET",
		nil,
	)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf(
			"delete dev binding status = %d, want %d; body=%s",
			deleteResponse.Code,
			http.StatusNoContent,
			deleteResponse.Body,
		)
	}
	thirdGeneration := writeBoundSecretExtensionGeneration(t, origin, extensionName, "1.2.0")
	emptyReloadResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPost,
		"/extensions/"+extensionName+"/reload",
		mustExtensionTransportJSON(t, contract.ReloadExtensionRequest{GenerationHash: thirdGeneration}),
	)
	if emptyReloadResponse.Code != http.StatusOK {
		t.Fatalf(
			"reload without workspace binding status = %d, want %d; body=%s",
			emptyReloadResponse.Code,
			http.StatusOK,
			emptyReloadResponse.Body,
		)
	}
	logMatch := waitForSecretExtensionLogsAfter(
		t,
		harness.service,
		extensionName,
		harness.actor,
		latestSequence,
		logs[len(logs)-1].StreamEpoch,
		"runtime_secret=",
	)
	encoded := string(mustExtensionTransportJSON(t, logMatch.logs))
	assertSecretsAbsent(t, "dev logs without workspace binding", encoded, []string{devSecret, globalSecret})
	if strings.Contains(logMatch.matched.Message, "runtime_secret=[REDACTED]") {
		t.Fatalf(
			"matched dev log after binding deletion = %q, want empty workspace value despite global row",
			logMatch.matched.Message,
		)
	}
}

func testExtensionSecretTransportRollback(t *testing.T) {
	t.Parallel()

	const extensionName = "binding-rollback"
	harness := newExtensionSecretIntegrationHarness(t, extensionSecretIntegrationHarnessOptions{
		actorReason:     "binding rollback",
		allowUnverified: true,
		failingVault:    true,
	})
	fixtureDir := writeBoundSecretExtensionFixtureWithEnv(
		t,
		t.TempDir(),
		extensionName,
		[]string{"A_KEY", "B_KEY"},
	)
	installResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPost,
		"/extensions",
		mustExtensionTransportJSON(t, contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceLocalPath, Ref: fixtureDir, AllowUnverified: true,
		}),
	)
	if installResponse.Code != http.StatusCreated {
		t.Fatalf(
			"install status = %d, want %d; body=%s",
			installResponse.Code,
			http.StatusCreated,
			installResponse.Body,
		)
	}

	oldA, oldB := "old-a-secret", "old-b-secret"
	oldBRef := vault.ExtensionSecretRef(extensionName, "", "B_SHARED")
	if _, err := harness.vault.PutSecret(
		t.Context(),
		oldBRef,
		extensionpkg.ExtensionEnvBindingKind,
		oldB,
	); err != nil {
		t.Fatalf("PutSecret(ref-form seed) error = %v", err)
	}
	initialResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPut,
		"/extensions/"+extensionName+"/secrets",
		mustExtensionTransportJSON(
			t,
			contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "A_KEY", Value: &oldA},
				{EnvName: "B_KEY", VaultRef: &oldBRef},
			}},
		),
	)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial set status = %d, want %d; body=%s", initialResponse.Code, http.StatusOK, initialResponse.Body)
	}
	initialRows, err := harness.db.ExtensionEnvRepo.ListEnvBindings(t.Context(), extensionName, "")
	if err != nil {
		t.Fatalf("ListEnvBindings(initial) error = %v", err)
	}
	if len(initialRows) != 2 || initialRows[1].EnvName != "B_KEY" || initialRows[1].SecretRef != oldBRef {
		t.Fatalf("initial binding rows = %#v, want value-form A and reused ref-form B", initialRows)
	}
	harness.failingVault.failSecondNextPut()
	newA, newB := "new-a-secret", "new-b-secret"
	failureResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPut,
		"/extensions/"+extensionName+"/secrets",
		mustExtensionTransportJSON(
			t,
			contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "B_KEY", Value: &newB},
				{EnvName: "A_KEY", Value: &newA},
			}},
		),
	)
	if failureResponse.Code != http.StatusInternalServerError {
		t.Fatalf(
			"failed set status = %d, want %d; body=%s",
			failureResponse.Code,
			http.StatusInternalServerError,
			failureResponse.Body,
		)
	}
	var errorPayload contract.ErrorPayload
	if err := json.Unmarshal(failureResponse.Body.Bytes(), &errorPayload); err != nil {
		t.Fatalf("json.Unmarshal(failed set response) error = %v", err)
	}
	if !strings.Contains(errorPayload.Error, "injected transport vault failure") {
		t.Fatalf("failed set error = %q, want injected transport vault failure", errorPayload.Error)
	}
	assertSecretsAbsent(
		t,
		"failed set response",
		failureResponse.Body.String(),
		[]string{oldA, oldB, newA, newB},
	)

	listResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodGet,
		"/extensions/"+extensionName+"/secrets",
		nil,
	)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listResponse.Code, http.StatusOK, listResponse.Body)
	}
	var payload contract.ExtensionSecretsPayload
	if err := json.Unmarshal(listResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(list response) error = %v", err)
	}
	if strings.Join(payload.BoundEnvKeys, ",") != "A_KEY,B_KEY" {
		t.Fatalf("bound env keys = %#v, want pre-call A_KEY and B_KEY", payload.BoundEnvKeys)
	}
	for envName, state := range map[string]struct {
		ref  string
		want string
	}{
		"A_KEY": {ref: vault.ExtensionSecretRef(extensionName, "", "A_KEY"), want: oldA},
		"B_KEY": {ref: oldBRef, want: oldB},
	} {
		got, resolveErr := harness.vault.ResolveRef(t.Context(), state.ref)
		if resolveErr != nil {
			t.Fatalf("ResolveRef(%s, %q) error = %v", envName, state.ref, resolveErr)
		}
		if got != state.want {
			t.Fatalf("ResolveRef(%s, %q) = %q, want restored pre-call value", envName, state.ref, got)
		}
	}
}

func testExtensionSecretBindingEnableInjection(t *testing.T) {
	t.Parallel()

	const (
		extensionName = "binding-hygiene"
		secretValue   = "extension-binding-secret-f26a9c41"
	)
	harness := newExtensionSecretIntegrationHarness(t, extensionSecretIntegrationHarnessOptions{
		actorReason:     "binding hygiene",
		allowUnverified: true,
	})
	fixtureDir := writeBoundSecretExtensionFixture(t, t.TempDir(), extensionName)
	installResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPost,
		"/extensions",
		mustExtensionTransportJSON(t, contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceLocalPath, Ref: fixtureDir, AllowUnverified: true,
		}),
	)
	if installResponse.Code != http.StatusCreated {
		t.Fatalf(
			"install status = %d, want %d; body=%s",
			installResponse.Code,
			http.StatusCreated,
			installResponse.Body,
		)
	}
	assertSecretsAbsent(t, "install response", installResponse.Body.String(), []string{secretValue})
	var installed contract.ExtensionPayload
	if err := json.Unmarshal(installResponse.Body.Bytes(), &installed); err != nil {
		t.Fatalf("json.Unmarshal(install response) error = %v", err)
	}
	if installed.Enabled || installed.PID != 0 {
		t.Fatalf("installed extension = %#v, want inert before secret binding and enable", installed)
	}

	boundValue := secretValue
	setResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPut,
		"/extensions/"+extensionName+"/secrets",
		mustExtensionTransportJSON(t, contract.SetExtensionSecretsRequest{
			Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "BOUND_SECRET", Value: &boundValue},
			},
		}),
	)
	if setResponse.Code != http.StatusOK {
		t.Fatalf("set secrets status = %d, want %d; body=%s", setResponse.Code, http.StatusOK, setResponse.Body)
	}
	assertSecretsAbsent(t, "set secrets response", setResponse.Body.String(), []string{secretValue})
	rows, err := harness.db.ExtensionEnvRepo.ListEnvBindings(t.Context(), extensionName, "")
	if err != nil {
		t.Fatalf("ListEnvBindings() error = %v", err)
	}
	if len(rows) != 1 || rows[0].EnvName != "BOUND_SECRET" ||
		!strings.HasPrefix(rows[0].SecretRef, "vault:extensions/global/") {
		t.Fatalf("global binding rows = %#v", rows)
	}

	enableResponse := performExtensionTransportRequest(
		t,
		harness.transport,
		http.MethodPost,
		"/extensions/"+extensionName+"/enable",
		mustExtensionTransportJSON(t, contract.EnableExtensionRequest{}),
	)
	if enableResponse.Code != http.StatusOK {
		t.Fatalf("enable status = %d, want %d; body=%s", enableResponse.Code, http.StatusOK, enableResponse.Body)
	}
	assertSecretsAbsent(t, "enable response", enableResponse.Body.String(), []string{secretValue})
	logs := waitForSecretExtensionLogs(t, harness.service, extensionName, harness.actor, "runtime_secret=")
	logsJSON := mustExtensionTransportJSON(t, logs)
	assertSecretsAbsent(t, "bound extension logs", string(logsJSON), []string{secretValue})
	if !bytes.Contains(logsJSON, []byte("[REDACTED]")) {
		t.Fatalf("bound extension logs = %s, want redaction marker proving injected value", logsJSON)
	}
	events, err := harness.db.ListEventSummaries(t.Context(), store.EventSummaryQuery{Component: "extension"})
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	assertSecretsAbsent(
		t,
		"binding lifecycle events",
		string(mustExtensionTransportJSON(t, events)),
		[]string{secretValue},
	)
}

func extensionSecretInputValue(value string) *string {
	return &value
}

func writeBoundSecretExtensionFixture(t *testing.T, root, name string) string {
	t.Helper()
	return writeBoundSecretExtensionFixtureWithEnv(t, root, name, []string{"BOUND_SECRET"})
}

func writeRemoteHeaderExtensionFixture(t *testing.T, root, name string) string {
	t.Helper()
	dir := writeBoundSecretExtensionFixtureWithEnv(t, root, name, nil)
	manifestPath := filepath.Join(dir, "extension.toml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", manifestPath, err)
	}
	manifest = append(manifest, []byte(`

[resources.mcp_servers.deployment-api]
transport = "http"
url = "https://deployment.example.test/mcp"
headers = { "X-Tenant" = "acme" }
`)...)
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", manifestPath, err)
	}
	return dir
}

func writeBoundSecretExtensionFixtureWithEnv(
	t *testing.T,
	root string,
	name string,
	requiredEnv []string,
) string {
	t.Helper()
	return writeBoundSecretExtensionFixtureVersion(t, root, name, "1.0.0", requiredEnv)
}

func writeBoundSecretExtensionFixtureVersion(
	t *testing.T,
	root string,
	name string,
	version string,
	requiredEnv []string,
) string {
	t.Helper()
	dir := filepath.Join(root, name+"-"+version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
	}
	manifest := daemonTestExtensionManifest(name, daemonTestExtensionOptions{
		version:        version,
		requiredEnv:    requiredEnv,
		runtimeCommand: daemonExtensionHelperCommand(t),
		runtimeArgs:    daemonExtensionHelperArgs(),
		runtimeEnv:     daemonExtensionHelperScenarioEnv("secret_hygiene", ""),
	})
	manifestPath := filepath.Join(dir, "extension.toml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", manifestPath, err)
	}
	return dir
}

type extensionSecretFailingVault struct {
	extensionSecretVault

	mu        sync.Mutex
	putCalls  int
	failPutAt int
}

func (v *extensionSecretFailingVault) PutSecret(
	ctx context.Context,
	ref string,
	kind string,
	value string,
) (vault.Metadata, error) {
	v.mu.Lock()
	v.putCalls++
	shouldFail := v.failPutAt > 0 && v.putCalls == v.failPutAt
	v.mu.Unlock()
	if shouldFail {
		return vault.Metadata{}, errors.New("injected transport vault failure")
	}
	return v.extensionSecretVault.PutSecret(ctx, ref, kind, value)
}

func (v *extensionSecretFailingVault) failSecondNextPut() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.failPutAt = v.putCalls + 2
}

func writeBoundSecretExtensionGeneration(t *testing.T, origin, name, version string) string {
	t.Helper()
	return writeBoundSecretExtensionGenerationWithEnv(
		t,
		origin,
		name,
		version,
		[]string{"BOUND_SECRET"},
	)
}

func writeBoundSecretExtensionGenerationWithEnv(
	t *testing.T,
	origin string,
	name string,
	version string,
	requiredEnv []string,
) string {
	t.Helper()
	fixture := writeBoundSecretExtensionFixtureVersion(t, t.TempDir(), name, version, requiredEnv)
	return publishSecretExtensionGeneration(t, origin, fixture)
}
