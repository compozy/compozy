//go:build integration && !windows

package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestExtensionSecretTransportAbsence(t *testing.T) {
	t.Run("Should keep extension secrets out of every transport", testExtensionSecretTransportAbsence)
	t.Run("Should inject a global binding only after enable", testExtensionSecretBindingEnableInjection)
	t.Run("Should isolate a development binding from the global instance", testExtensionSecretDevBindingIsolation)
	t.Run("Should roll back a failed transport batch", testExtensionSecretTransportRollback)
	t.Run("Should retire global and development bindings by instance", testExtensionSecretBindingRetirement)
}

func testExtensionSecretBindingRetirement(t *testing.T) {
	// This ordered lifecycle assertion owns one global and one workspace-local instance.
	const (
		extensionName = "binding-retirement"
		workspaceID   = "workspace-binding-retirement"
		ownedGlobal   = "owned-global-secret-705a"
		ownedDev      = "owned-dev-secret-a42f"
		foreignValue  = "foreign-owned-secret-4bd1"
	)
	homePaths := testHomePaths(t)
	db := openDaemonTestGlobalDB(t)
	registry := extensionpkg.NewRegistry(db.DB())
	secretVault, err := vault.NewService(
		db.VaultRepo,
		vault.NewFileKeyProvider(t.TempDir(), nil),
	)
	if err != nil {
		t.Fatalf("vault.NewService() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	workspaceResolver := &daemonExtensionWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID: workspaceID, Name: "binding-retirement", RootDir: workspaceRoot,
		},
		WorkspaceID: workspaceID,
	}}
	manager := extensionpkg.NewManager(
		registry,
		extensionpkg.WithLogger(discardLogger()),
		extensionpkg.WithWorkspaceResolver(workspaceResolver),
		extensionpkg.WithSecretResolver(secretVault),
		extensionpkg.WithEnvBindingStore(db.ExtensionEnvRepo),
	)
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Errorf("Manager.Stop() error = %v", err)
		}
	})
	service, ok := newDaemonExtensionService(
		registry,
		manager,
		nil,
		nil,
		nil,
		nil,
		homePaths,
		discardLogger(),
		time.Now,
		withDaemonExtensionMarketplace(
			compozyconfig.ExtensionsConfig{Trust: compozyconfig.ExtensionsTrustConfig{AllowUnverified: true}},
			nil,
		),
		withDaemonExtensionEventWriter(db),
		withDaemonExtensionWorkspaceResolver(workspaceResolver),
		withDaemonExtensionSecrets(db.ExtensionEnvRepo, secretVault),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	globalActor, err := taskpkg.DeriveHumanActorContext(
		"operator",
		taskpkg.OriginKindHTTP,
		"global binding retirement",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContext(global) error = %v", err)
	}
	devActor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		workspaceID,
		taskpkg.OriginKindHTTP,
		"development binding retirement",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	fixtureDir := writeBoundSecretExtensionFixtureWithEnv(
		t,
		t.TempDir(),
		extensionName,
		[]string{"BOUND_SECRET", "OTHER_SECRET"},
	)
	if _, err := service.Install(t.Context(), contract.InstallExtensionRequest{
		Source: contract.InstallExtensionSourceLocalPath, Ref: fixtureDir, AllowUnverified: true,
	}, globalActor); err != nil {
		t.Fatalf("Install(global) error = %v", err)
	}
	foreignRef := vault.ExtensionSecretRef(extensionName, "", "FOREIGN_SHARED")
	if _, err := secretVault.PutSecret(t.Context(), foreignRef, "foreign_token", foreignValue); err != nil {
		t.Fatalf("PutSecret(foreign) error = %v", err)
	}
	if _, err := service.SetExtensionSecrets(
		t.Context(),
		extensionName,
		contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
			"BOUND_SECRET": {Value: extensionSecretInputValue(ownedGlobal)},
			"OTHER_SECRET": {VaultRef: &foreignRef},
		}},
		globalActor,
	); err != nil {
		t.Fatalf("SetExtensionSecrets(global) error = %v", err)
	}
	globalOwnedRef := vault.ExtensionSecretRef(extensionName, "", "BOUND_SECRET")
	if _, err := service.Remove(t.Context(), extensionName, globalActor); err != nil {
		t.Fatalf("Remove(global) error = %v", err)
	}
	assertExtensionBindingsRetired(t, db, secretVault, extensionName, "", globalOwnedRef, foreignRef, foreignValue)
	if _, err := service.Install(t.Context(), contract.InstallExtensionRequest{
		Source: contract.InstallExtensionSourceLocalPath, Ref: fixtureDir, AllowUnverified: true,
	}, globalActor); err != nil {
		t.Fatalf("Install(global after removal) error = %v", err)
	}
	reinstalledSecrets, err := service.ListExtensionSecrets(t.Context(), extensionName, globalActor)
	if err != nil {
		t.Fatalf("ListExtensionSecrets(reinstalled global) error = %v", err)
	}
	if len(reinstalledSecrets.BoundEnvKeys) != 0 || len(reinstalledSecrets.Bindings) != 0 {
		t.Fatalf("reinstalled global bindings = %#v, want empty", reinstalledSecrets)
	}

	origin := filepath.Join(workspaceRoot, "binding-retirement-extension")
	firstGeneration := writeBoundSecretExtensionGenerationWithEnv(
		t,
		origin,
		extensionName,
		"2.0.0",
		[]string{"BOUND_SECRET", "OTHER_SECRET"},
	)
	if _, err := service.Dev(t.Context(), contract.DevLinkExtensionRequest{
		OriginPath: origin, GenerationHash: firstGeneration,
	}, devActor); err != nil {
		t.Fatalf("Dev() error = %v", err)
	}
	devForeignRef := vault.ExtensionSecretRef(extensionName, workspaceID, "FOREIGN_SHARED")
	if _, err := secretVault.PutSecret(t.Context(), devForeignRef, "foreign_token", foreignValue); err != nil {
		t.Fatalf("PutSecret(dev foreign) error = %v", err)
	}
	if _, err := service.SetExtensionSecrets(
		t.Context(),
		extensionName,
		contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
			"BOUND_SECRET": {Value: extensionSecretInputValue(ownedDev)},
			"OTHER_SECRET": {VaultRef: &devForeignRef},
		}},
		devActor,
	); err != nil {
		t.Fatalf("SetExtensionSecrets(dev) error = %v", err)
	}
	logsBeforeReload, err := service.ExtensionLogs(t.Context(), extensionName, 0, devActor)
	if err != nil {
		t.Fatalf("ExtensionLogs(before manifest drop) error = %v", err)
	}
	var sequenceBeforeReload int64
	if len(logsBeforeReload) > 0 {
		sequenceBeforeReload = logsBeforeReload[len(logsBeforeReload)-1].Sequence
	}
	secondGeneration := writeBoundSecretExtensionGenerationWithEnv(
		t,
		origin,
		extensionName,
		"2.1.0",
		[]string{"OTHER_SECRET"},
	)
	if _, err := service.ReloadDev(t.Context(), extensionName, contract.ReloadExtensionRequest{
		GenerationHash: secondGeneration,
	}, devActor); err != nil {
		t.Fatalf("ReloadDev(drop declared env) error = %v", err)
	}
	devSecrets, err := service.ListExtensionSecrets(t.Context(), extensionName, devActor)
	if err != nil {
		t.Fatalf("ListExtensionSecrets(dev after reload) error = %v", err)
	}
	if len(devSecrets.Bindings) != 2 || !devSecrets.Bindings[0].Stale ||
		devSecrets.Bindings[0].EnvName != "BOUND_SECRET" || devSecrets.Bindings[1].Stale {
		t.Fatalf("dev bindings after manifest drop = %#v, want stale BOUND_SECRET only", devSecrets.Bindings)
	}
	logs := waitForSecretExtensionLogsAfter(
		t,
		service,
		extensionName,
		devActor,
		sequenceBeforeReload,
		"runtime_secret=",
	)
	latest := logs[len(logs)-1]
	if strings.Contains(latest.Message, "runtime_secret=[REDACTED]") {
		t.Fatalf("latest dev log = %q, stale binding was injected", latest.Message)
	}
	devOwnedRef := vault.ExtensionSecretRef(extensionName, workspaceID, "BOUND_SECRET")
	if _, err := service.RemoveScoped(t.Context(), extensionName, devActor); err != nil {
		t.Fatalf("RemoveScoped(dev) error = %v", err)
	}
	assertExtensionBindingsRetired(
		t,
		db,
		secretVault,
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
	homePaths := testHomePaths(t)
	db := openDaemonTestGlobalDB(t)
	registry := extensionpkg.NewRegistry(db.DB())
	secretVault, err := vault.NewService(
		db.VaultRepo,
		vault.NewFileKeyProvider(t.TempDir(), nil),
	)
	if err != nil {
		t.Fatalf("vault.NewService() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	workspaceResolver := &daemonExtensionWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID: workspaceID, Name: "dev-binding", RootDir: workspaceRoot,
		},
		WorkspaceID: workspaceID,
	}}
	manager := extensionpkg.NewManager(
		registry,
		extensionpkg.WithLogger(discardLogger()),
		extensionpkg.WithWorkspaceResolver(workspaceResolver),
		extensionpkg.WithSecretResolver(secretVault),
		extensionpkg.WithEnvBindingStore(db.ExtensionEnvRepo),
	)
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Errorf("Manager.Stop() error = %v", err)
		}
	})
	service, ok := newDaemonExtensionService(
		registry,
		manager,
		nil,
		nil,
		nil,
		nil,
		homePaths,
		discardLogger(),
		time.Now,
		withDaemonExtensionEventWriter(db),
		withDaemonExtensionWorkspaceResolver(workspaceResolver),
		withDaemonExtensionSecrets(db.ExtensionEnvRepo, secretVault),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		workspaceID,
		taskpkg.OriginKindHTTP,
		"development binding isolation",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	engine := newExtensionSecretTransportEngine(service, actor, "http")
	origin := filepath.Join(workspaceRoot, "dev-binding-extension")
	firstGeneration := writeBoundSecretExtensionGeneration(t, origin, extensionName, "1.0.0")
	devResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPost,
		"/extensions/dev",
		mustSecretTransportJSON(t, contract.DevLinkExtensionRequest{
			OriginPath: origin, GenerationHash: firstGeneration,
		}),
	)
	if devResponse.Code != http.StatusCreated {
		t.Fatalf("dev link status = %d, want %d; body=%s", devResponse.Code, http.StatusCreated, devResponse.Body)
	}
	boundValue := devSecret
	setResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPut,
		"/extensions/"+extensionName+"/secrets",
		mustSecretTransportJSON(
			t,
			contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"BOUND_SECRET": {Value: &boundValue},
			}},
		),
	)
	if setResponse.Code != http.StatusOK {
		t.Fatalf("dev set status = %d, want %d; body=%s", setResponse.Code, http.StatusOK, setResponse.Body)
	}
	workspaceRows, err := db.ExtensionEnvRepo.ListEnvBindings(t.Context(), extensionName, workspaceID)
	if err != nil {
		t.Fatalf("ListEnvBindings(workspace) error = %v", err)
	}
	if len(workspaceRows) != 1 ||
		workspaceRows[0].SecretRef != vault.ExtensionSecretRef(extensionName, workspaceID, "BOUND_SECRET") {
		t.Fatalf("workspace bindings = %#v, want exact workspace-owned ref", workspaceRows)
	}
	globalRows, err := db.ExtensionEnvRepo.ListEnvBindings(t.Context(), extensionName, "")
	if err != nil {
		t.Fatalf("ListEnvBindings(global) error = %v", err)
	}
	if len(globalRows) != 0 {
		t.Fatalf("global bindings after dev set = %#v, want none", globalRows)
	}

	secondGeneration := writeBoundSecretExtensionGeneration(t, origin, extensionName, "1.1.0")
	reloadResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPost,
		"/extensions/"+extensionName+"/reload",
		mustSecretTransportJSON(t, contract.ReloadExtensionRequest{GenerationHash: secondGeneration}),
	)
	if reloadResponse.Code != http.StatusOK {
		t.Fatalf(
			"reload with workspace binding status = %d, want %d; body=%s",
			reloadResponse.Code,
			http.StatusOK,
			reloadResponse.Body,
		)
	}
	logs := waitForSecretExtensionLogs(t, service, extensionName, actor, "runtime_secret=")
	boundLogsJSON := string(mustSecretTransportJSON(t, logs))
	assertSecretsAbsent(t, "workspace-bound logs", boundLogsJSON, []string{devSecret})
	if !strings.Contains(boundLogsJSON, "runtime_secret=[REDACTED]") {
		t.Fatalf("workspace-bound logs = %s, want proof of injected and redacted binding", boundLogsJSON)
	}
	latestSequence := logs[len(logs)-1].Sequence

	globalRef := vault.ExtensionSecretRef(extensionName, "", "BOUND_SECRET")
	if _, err := secretVault.PutSecret(
		t.Context(),
		globalRef,
		extensionpkg.ExtensionEnvBindingKind,
		globalSecret,
	); err != nil {
		t.Fatalf("PutSecret(global sentinel) error = %v", err)
	}
	if err := db.ExtensionEnvRepo.PutEnvBinding(t.Context(), extensionpkg.EnvBinding{
		ExtensionName: extensionName,
		EnvName:       "BOUND_SECRET",
		SecretRef:     globalRef,
		Kind:          extensionpkg.ExtensionEnvBindingKind,
	}); err != nil {
		t.Fatalf("PutEnvBinding(global sentinel) error = %v", err)
	}
	deleteResponse := performSecretTransportRequest(
		t,
		engine,
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
	emptyReloadResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPost,
		"/extensions/"+extensionName+"/reload",
		mustSecretTransportJSON(t, contract.ReloadExtensionRequest{GenerationHash: thirdGeneration}),
	)
	if emptyReloadResponse.Code != http.StatusOK {
		t.Fatalf(
			"reload without workspace binding status = %d, want %d; body=%s",
			emptyReloadResponse.Code,
			http.StatusOK,
			emptyReloadResponse.Body,
		)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		newLogs, logsErr := service.ExtensionLogs(t.Context(), extensionName, latestSequence, actor)
		if logsErr != nil {
			t.Fatalf("ExtensionLogs(after workspace binding deletion) error = %v", logsErr)
		}
		if len(newLogs) == 0 {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		encoded := string(mustSecretTransportJSON(t, newLogs))
		assertSecretsAbsent(t, "dev logs without workspace binding", encoded, []string{devSecret, globalSecret})
		if strings.Contains(encoded, "runtime_secret=[REDACTED]") || !strings.Contains(encoded, "runtime_secret=") {
			t.Fatalf("dev logs after binding deletion = %s, want empty workspace value despite global row", encoded)
		}
		return
	}
	t.Fatal("dev logs did not record the binding-free replacement generation")
}

func testExtensionSecretTransportRollback(t *testing.T) {
	t.Parallel()

	const extensionName = "binding-rollback"
	homePaths := testHomePaths(t)
	db := openDaemonTestGlobalDB(t)
	registry := extensionpkg.NewRegistry(db.DB())
	baseVault, err := vault.NewService(
		db.VaultRepo,
		vault.NewFileKeyProvider(t.TempDir(), nil),
	)
	if err != nil {
		t.Fatalf("vault.NewService() error = %v", err)
	}
	failingVault := &extensionSecretFailingVault{extensionSecretVault: baseVault}
	manager := extensionpkg.NewManager(
		registry,
		extensionpkg.WithLogger(discardLogger()),
		extensionpkg.WithSecretResolver(baseVault),
		extensionpkg.WithEnvBindingStore(db.ExtensionEnvRepo),
	)
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Errorf("Manager.Stop() error = %v", err)
		}
	})
	service := newDaemonExtensionService(
		registry,
		manager,
		nil,
		nil,
		nil,
		nil,
		homePaths,
		discardLogger(),
		time.Now,
		withDaemonExtensionMarketplace(
			compozyconfig.ExtensionsConfig{Trust: compozyconfig.ExtensionsTrustConfig{AllowUnverified: true}},
			nil,
		),
		withDaemonExtensionEventWriter(db),
		withDaemonExtensionSecrets(db.ExtensionEnvRepo, failingVault),
	)
	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindHTTP, "binding rollback")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	engine := newExtensionSecretTransportEngine(service, actor, "http")
	fixtureDir := writeBoundSecretExtensionFixtureWithEnv(
		t,
		t.TempDir(),
		extensionName,
		[]string{"A_KEY", "B_KEY"},
	)
	installResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPost,
		"/extensions",
		mustSecretTransportJSON(t, contract.InstallExtensionRequest{
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
	if _, err := baseVault.PutSecret(
		t.Context(),
		oldBRef,
		extensionpkg.ExtensionEnvBindingKind,
		oldB,
	); err != nil {
		t.Fatalf("PutSecret(ref-form seed) error = %v", err)
	}
	initialResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPut,
		"/extensions/"+extensionName+"/secrets",
		mustSecretTransportJSON(
			t,
			contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"A_KEY": {Value: &oldA},
				"B_KEY": {VaultRef: &oldBRef},
			}},
		),
	)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial set status = %d, want %d; body=%s", initialResponse.Code, http.StatusOK, initialResponse.Body)
	}
	initialRows, err := db.ExtensionEnvRepo.ListEnvBindings(t.Context(), extensionName, "")
	if err != nil {
		t.Fatalf("ListEnvBindings(initial) error = %v", err)
	}
	if len(initialRows) != 2 || initialRows[1].EnvName != "B_KEY" || initialRows[1].SecretRef != oldBRef {
		t.Fatalf("initial binding rows = %#v, want value-form A and reused ref-form B", initialRows)
	}
	failingVault.failSecondNextPut()
	newA, newB := "new-a-secret", "new-b-secret"
	failureResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPut,
		"/extensions/"+extensionName+"/secrets",
		mustSecretTransportJSON(
			t,
			contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"B_KEY": {Value: &newB},
				"A_KEY": {Value: &newA},
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
	assertSecretsAbsent(
		t,
		"failed set response",
		failureResponse.Body.String(),
		[]string{oldA, oldB, newA, newB},
	)

	listResponse := performSecretTransportRequest(
		t,
		engine,
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
		ref := state.ref
		got, resolveErr := baseVault.ResolveRef(t.Context(), ref)
		if resolveErr != nil {
			t.Fatalf("ResolveRef(%s, %q) error = %v", envName, ref, resolveErr)
		}
		if got != state.want {
			t.Fatalf("ResolveRef(%s, %q) = %q, want restored pre-call value", envName, ref, got)
		}
	}
}

func testExtensionSecretBindingEnableInjection(t *testing.T) {
	t.Parallel()

	const (
		extensionName = "binding-hygiene"
		secretValue   = "extension-binding-secret-f26a9c41"
	)
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
	manager := extensionpkg.NewManager(
		registry,
		extensionpkg.WithLogger(discardLogger()),
		extensionpkg.WithSecretResolver(vaultService),
		extensionpkg.WithEnvBindingStore(db.ExtensionEnvRepo),
	)
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Errorf("Manager.Stop() error = %v", err)
		}
	})
	service, ok := newDaemonExtensionService(
		registry,
		manager,
		nil,
		nil,
		nil,
		nil,
		homePaths,
		discardLogger(),
		time.Now,
		withDaemonExtensionMarketplace(
			compozyconfig.ExtensionsConfig{Trust: compozyconfig.ExtensionsTrustConfig{AllowUnverified: true}},
			nil,
		),
		withDaemonExtensionEventWriter(db),
		withDaemonExtensionSecrets(db.ExtensionEnvRepo, vaultService),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindHTTP, "binding hygiene")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	engine := newExtensionSecretTransportEngine(service, actor, "http")
	fixtureDir := writeBoundSecretExtensionFixture(t, t.TempDir(), extensionName)
	installResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPost,
		"/extensions",
		mustSecretTransportJSON(t, contract.InstallExtensionRequest{
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
	setResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPut,
		"/extensions/"+extensionName+"/secrets",
		mustSecretTransportJSON(t, contract.SetExtensionSecretsRequest{
			Secrets: map[string]contract.ExtensionSecretInput{
				"BOUND_SECRET": {Value: &boundValue},
			},
		}),
	)
	if setResponse.Code != http.StatusOK {
		t.Fatalf("set secrets status = %d, want %d; body=%s", setResponse.Code, http.StatusOK, setResponse.Body)
	}
	assertSecretsAbsent(t, "set secrets response", setResponse.Body.String(), []string{secretValue})
	rows, err := db.ExtensionEnvRepo.ListEnvBindings(t.Context(), extensionName, "")
	if err != nil {
		t.Fatalf("ListEnvBindings() error = %v", err)
	}
	if len(rows) != 1 || rows[0].EnvName != "BOUND_SECRET" ||
		!strings.HasPrefix(rows[0].SecretRef, "vault:extensions/global/") {
		t.Fatalf("global binding rows = %#v", rows)
	}

	enableResponse := performSecretTransportRequest(
		t,
		engine,
		http.MethodPost,
		"/extensions/"+extensionName+"/enable",
		mustSecretTransportJSON(t, contract.EnableExtensionRequest{}),
	)
	if enableResponse.Code != http.StatusOK {
		t.Fatalf("enable status = %d, want %d; body=%s", enableResponse.Code, http.StatusOK, enableResponse.Body)
	}
	assertSecretsAbsent(t, "enable response", enableResponse.Body.String(), []string{secretValue})
	logs := waitForSecretExtensionLogs(t, service, extensionName, actor, "runtime_secret=")
	logsJSON := mustSecretTransportJSON(t, logs)
	assertSecretsAbsent(t, "bound extension logs", string(logsJSON), []string{secretValue})
	if !bytes.Contains(logsJSON, []byte("[REDACTED]")) {
		t.Fatalf("bound extension logs = %s, want redaction marker proving injected value", logsJSON)
	}
	events, err := db.ListEventSummaries(t.Context(), store.EventSummaryQuery{Component: "extension"})
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	assertSecretsAbsent(
		t,
		"binding lifecycle events",
		string(mustSecretTransportJSON(t, events)),
		[]string{secretValue},
	)
}

func testExtensionSecretTransportAbsence(t *testing.T) {
	t.Parallel()

	const (
		runtimeSecret     = "extension-runtime-secret-7b9e52f0"
		publishCredential = "extension-publish-token-a4c831d2"
		workspaceID       = "workspace-secret-hygiene"
		extensionName     = "secret-hygiene"
	)
	secrets := []string{runtimeSecret, publishCredential}

	homePaths := testHomePaths(t)
	db := openDaemonTestGlobalDB(t)
	extensionRegistry := extensionpkg.NewRegistry(db.DB())
	workspaceRoot := t.TempDir()
	workspaceResolver := &daemonExtensionWorkspaceResolverStub{
		resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      workspaceID,
				Name:    "secret-hygiene",
				RootDir: workspaceRoot,
			},
			WorkspaceID: workspaceID,
		},
	}
	secretResolver := nativeExtensionSecretResolver{values: map[string]string{
		"env:EXT_RUNTIME_SECRET": runtimeSecret,
		"env:GITHUB_TOKEN":       publishCredential,
	}}
	manager := extensionpkg.NewManager(
		extensionRegistry,
		extensionpkg.WithLogger(discardLogger()),
		extensionpkg.WithWorkspaceResolver(workspaceResolver),
		extensionpkg.WithSecretResolver(secretResolver),
	)
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Errorf("Manager.Stop() error = %v", err)
		}
	})

	service, ok := newDaemonExtensionService(
		extensionRegistry,
		manager,
		nil,
		nil,
		nil,
		nil,
		homePaths,
		discardLogger(),
		time.Now,
		withDaemonExtensionMarketplace(
			compozyconfig.ExtensionsConfig{Trust: compozyconfig.ExtensionsTrustConfig{AllowUnverified: true}},
			nil,
		),
		withDaemonExtensionEventWriter(db),
		withDaemonExtensionWorkspaceResolver(workspaceResolver),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		workspaceID,
		taskpkg.OriginKindHTTP,
		"extension secret hygiene",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}

	httpEngine := newExtensionSecretTransportEngine(service, actor, "http")
	udsEngine := newExtensionSecretTransportEngine(service, actor, "uds")
	fixtureDir := writeSecretExtensionFixture(t, t.TempDir(), extensionName, "1.0.0")
	installBody := mustSecretTransportJSON(t, contract.InstallExtensionRequest{
		Source:          contract.InstallExtensionSourceLocalPath,
		Ref:             fixtureDir,
		AllowUnverified: true,
	})
	installResponse := performSecretTransportRequest(
		t,
		httpEngine,
		http.MethodPost,
		"/extensions",
		installBody,
	)
	if installResponse.Code != http.StatusCreated {
		t.Fatalf(
			"HTTP install status = %d, want %d; body=%s",
			installResponse.Code,
			http.StatusCreated,
			installResponse.Body,
		)
	}
	assertSecretsAbsent(t, "HTTP install response", installResponse.Body.String(), secrets)

	origin := filepath.Join(workspaceRoot, "secret-extension")
	firstGeneration := writeSecretExtensionGeneration(t, origin, extensionName, "1.1.0")
	devResponse := performSecretTransportRequest(
		t,
		httpEngine,
		http.MethodPost,
		"/extensions/dev",
		mustSecretTransportJSON(t, contract.DevLinkExtensionRequest{
			OriginPath: origin, GenerationHash: firstGeneration,
		}),
	)
	if devResponse.Code != http.StatusCreated {
		t.Fatalf("HTTP dev status = %d, want %d; body=%s", devResponse.Code, http.StatusCreated, devResponse.Body)
	}
	assertSecretsAbsent(t, "HTTP dev response", devResponse.Body.String(), secrets)

	secondGeneration := writeSecretExtensionGeneration(t, origin, extensionName, "1.2.0")
	reloadResponse := performSecretTransportRequest(
		t,
		udsEngine,
		http.MethodPost,
		"/extensions/"+extensionName+"/reload",
		mustSecretTransportJSON(t, contract.ReloadExtensionRequest{GenerationHash: secondGeneration}),
	)
	if reloadResponse.Code != http.StatusOK {
		t.Fatalf("UDS reload status = %d, want %d; body=%s", reloadResponse.Code, http.StatusOK, reloadResponse.Body)
	}
	assertSecretsAbsent(t, "UDS reload response", reloadResponse.Body.String(), secrets)

	logs := waitForSecretExtensionLogs(t, service, extensionName, actor, "safe=visible")
	logsJSON := mustSecretTransportJSON(t, logs)
	assertSecretsAbsent(t, "extension log ring", string(logsJSON), secrets)
	if !bytes.Contains(logsJSON, []byte("[REDACTED]")) || !bytes.Contains(logsJSON, []byte("safe=visible")) {
		t.Fatalf("extension log ring = %s, want redaction marker and safe field", logsJSON)
	}

	for transport, engine := range map[string]*gin.Engine{"HTTP": httpEngine, "UDS": udsEngine} {
		response := performSecretTransportRequest(
			t,
			engine,
			http.MethodGet,
			"/extensions/"+extensionName+"/logs",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("%s logs status = %d, want %d; body=%s", transport, response.Code, http.StatusOK, response.Body)
		}
		assertSecretsAbsent(t, transport+" logs response", response.Body.String(), secrets)
	}

	sseFrame := readSecretExtensionSSEFrame(t, httpEngine, extensionName)
	assertSecretsAbsent(t, "HTTP SSE stream", sseFrame, secrets)
	if !strings.Contains(sseFrame, "[REDACTED]") {
		t.Fatalf("HTTP SSE stream = %q, want redaction marker", sseFrame)
	}

	publishServer, publishCapture := newNativeExtensionPublishServer(t, publishCredential)
	t.Cleanup(publishServer.Close)
	deps := &daemonNativeToolsDeps{
		ExtensionRegistry: extensionRegistry,
		Extensions: func() core.ExtensionService {
			return service
		},
		ExtensionRuntime: func() extensionRuntime { return manager },
		ExtensionConfig: compozyconfig.ExtensionsConfig{Sources: compozyconfig.ExtensionsSourcesConfig{
			GitHub: compozyconfig.ExtensionsGitHubSourceConfig{BaseURL: publishServer.URL},
		}},
		ExtensionEvents:   db,
		ExtensionSecrets:  secretResolver,
		HomePaths:         homePaths,
		WorkspaceResolver: workspaceResolver,
	}
	nativeRegistry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
	nativeScope := toolspkg.Scope{WorkspaceID: workspaceID, Operator: true}
	nativeLogs, err := nativeRegistry.Call(t.Context(), nativeScope, toolspkg.CallRequest{
		ToolID:      toolspkg.ToolIDExtensionsLogs,
		WorkspaceID: workspaceID,
		Input:       json.RawMessage(`{"name":"secret-hygiene","after":0}`),
	})
	if err != nil {
		t.Fatalf("Registry.Call(extensions_logs) error = %v", err)
	}
	assertSecretsAbsent(t, "native logs structured result", string(nativeLogs.Structured), secrets)
	assertSecretsAbsent(t, "native logs preview", nativeLogs.Preview, secrets)

	generationDir := filepath.Join(origin, "dist", "gen-"+secondGeneration)
	publishResult, err := nativeRegistry.Call(t.Context(), nativeScope, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsPublish,
		Input: json.RawMessage(fmt.Sprintf(
			`{"generation_dir":%q,"repository":"acme/native-dev","tag_name":"v0.2.0"}`,
			generationDir,
		)),
	})
	if err != nil {
		t.Fatalf("Registry.Call(extensions_publish) error = %v", err)
	}
	assertSecretsAbsent(t, "native publish structured result", string(publishResult.Structured), secrets)
	assertSecretsAbsent(t, "native publish preview", publishResult.Preview, secrets)
	publishCapture.requireComplete(t, publishCredential)

	failureServer := newSecretEchoingPublishFailureServer(t, secrets)
	t.Cleanup(failureServer.Close)
	deps.ExtensionConfig.Sources.GitHub.BaseURL = failureServer.URL
	failingNativeRegistry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
	_, publishErr := failingNativeRegistry.Call(t.Context(), nativeScope, toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDExtensionsPublish,
		Input: json.RawMessage(fmt.Sprintf(
			`{"generation_dir":%q,"repository":"acme/native-dev","tag_name":"v0.2.0"}`,
			generationDir,
		)),
	})
	if publishErr == nil {
		t.Fatal("Registry.Call(extensions_publish failure) error = nil, want structured error")
	}
	structuredError := mustSecretTransportJSON(t, core.ErrorPayloadForError(publishErr))
	assertSecretsAbsent(t, "structured publish error", string(structuredError), secrets)

	events, err := db.ListEventSummaries(t.Context(), store.EventSummaryQuery{})
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("persisted extension events = %d, want install/dev/reload", len(events))
	}
	assertSecretsAbsent(t, "persisted extension events", string(mustSecretTransportJSON(t, events)), secrets)
}

func newExtensionSecretTransportEngine(
	service core.ExtensionService,
	actor taskpkg.ActorContext,
	transport string,
) *gin.Engine {
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: transport,
		Extensions:    service,
		TaskActorContextResolver: func(*gin.Context, string) (taskpkg.ActorContext, error) {
			return actor, nil
		},
	})
	engine := gin.New()
	engine.POST("/extensions", handlers.InstallExtension)
	engine.POST("/extensions/:name/enable", handlers.EnableExtension)
	engine.POST("/extensions/:name/disable", handlers.DisableExtension)
	engine.POST("/extensions/dev", handlers.DevExtension)
	engine.POST("/extensions/:name/reload", handlers.ReloadDevExtension)
	engine.GET("/extensions/:name/secrets", handlers.ListExtensionSecrets)
	engine.PUT("/extensions/:name/secrets", handlers.SetExtensionSecrets)
	engine.DELETE("/extensions/:name/secrets/:env_name", handlers.DeleteExtensionSecret)
	engine.GET("/extensions/:name/logs", handlers.ExtensionLogs)
	return engine
}

func writeSecretExtensionFixture(t *testing.T, root, name, version string) string {
	t.Helper()
	dir := filepath.Join(root, name+"-"+version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
	}
	manifest := daemonTestExtensionManifest(name, daemonTestExtensionOptions{
		runtimeCommand: daemonExtensionHelperCommand(t),
		runtimeArgs:    daemonExtensionHelperArgs(),
		runtimeEnv:     daemonExtensionHelperScenarioEnv("secret_hygiene", ""),
		runtimeSecretEnv: map[string]string{
			"BOUND_PROVIDER_TOKEN": "env:GITHUB_TOKEN",
			"BOUND_SECRET":         "env:EXT_RUNTIME_SECRET",
		},
	})
	manifest = strings.Replace(manifest, `version = "0.2.1"`, fmt.Sprintf("version = %q", version), 1)
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	return dir
}

func extensionSecretInputValue(value string) *string {
	return &value
}

func writeBoundSecretExtensionFixture(t *testing.T, root, name string) string {
	t.Helper()
	return writeBoundSecretExtensionFixtureWithEnv(t, root, name, []string{"BOUND_SECRET"})
}

func writeBoundSecretExtensionFixtureWithEnv(
	t *testing.T,
	root string,
	name string,
	requiredEnv []string,
) string {
	t.Helper()
	dir := writeSecretExtensionFixture(t, root, name, "1.0.0")
	manifestPath := filepath.Join(dir, "extension.toml")
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("os.ReadFile(extension.toml) error = %v", err)
	}
	manifest := strings.Replace(
		string(payload),
		`min_compozy_version = "0.5.0"`,
		fmt.Sprintf(
			"min_compozy_version = \"0.5.0\"\nrequires_env = [%s]",
			quotedSecretEnvNames(requiredEnv),
		),
		1,
	)
	manifest = strings.Replace(
		manifest,
		`[subprocess.secret_env]
BOUND_PROVIDER_TOKEN = "env:GITHUB_TOKEN"
BOUND_SECRET = "env:EXT_RUNTIME_SECRET"
`,
		"",
		1,
	)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	return dir
}

func quotedSecretEnvNames(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}
	return strings.Join(quoted, ", ")
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

func writeSecretExtensionGeneration(t *testing.T, origin, name, version string) string {
	t.Helper()
	fixture := writeSecretExtensionFixture(t, t.TempDir(), name, version)
	return publishSecretExtensionGeneration(t, origin, fixture)
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
	fixture := writeBoundSecretExtensionFixtureWithEnv(t, t.TempDir(), name, requiredEnv)
	manifestPath := filepath.Join(fixture, "extension.toml")
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("os.ReadFile(extension.toml) error = %v", err)
	}
	manifest := strings.Replace(string(payload), `version = "1.0.0"`, fmt.Sprintf("version = %q", version), 1)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	return publishSecretExtensionGeneration(t, origin, fixture)
}

func publishSecretExtensionGeneration(t *testing.T, origin, fixture string) string {
	t.Helper()
	manifest, err := os.ReadFile(filepath.Join(fixture, "extension.toml"))
	if err != nil {
		t.Fatalf("os.ReadFile(extension.toml) error = %v", err)
	}
	dist := filepath.Join(origin, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dist, err)
	}
	staging, err := os.MkdirTemp(dist, ".secret-generation-")
	if err != nil {
		t.Fatalf("os.MkdirTemp(%q) error = %v", dist, err)
	}
	if err := os.WriteFile(filepath.Join(staging, "extension.toml"), manifest, 0o644); err != nil {
		t.Fatalf("os.WriteFile(staged extension.toml) error = %v", err)
	}
	hash, err := extensionpkg.ComputeDirectoryChecksum(staging)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", staging, err)
	}
	generationDir := filepath.Join(dist, "gen-"+hash)
	if err := os.Rename(staging, generationDir); err != nil {
		t.Fatalf("os.Rename(%q, %q) error = %v", staging, generationDir, err)
	}
	return hash
}

func performSecretTransportRequest(
	t *testing.T,
	engine *gin.Engine,
	method string,
	path string,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	reader := io.Reader(http.NoBody)
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func waitForSecretExtensionLogs(
	t *testing.T,
	service *daemonExtensionService,
	name string,
	actor taskpkg.ActorContext,
	marker string,
) []contract.ExtensionLogPayload {
	t.Helper()
	return waitForSecretExtensionLogsAfter(t, service, name, actor, 0, marker)
}

func waitForSecretExtensionLogsAfter(
	t *testing.T,
	service *daemonExtensionService,
	name string,
	actor taskpkg.ActorContext,
	after int64,
	marker string,
) []contract.ExtensionLogPayload {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		logs, err := service.ExtensionLogs(t.Context(), name, after, actor)
		if err != nil {
			t.Fatalf("ExtensionLogs() error = %v", err)
		}
		for _, entry := range logs {
			if strings.Contains(entry.Message, marker) {
				return logs
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	status, statusErr := service.Status(t.Context(), name)
	logs, logsErr := service.ExtensionLogs(t.Context(), name, after, actor)
	t.Fatalf(
		"extension log ring did not receive marker %q; status=%#v status_error=%v logs=%#v logs_error=%v",
		marker,
		status,
		statusErr,
		logs,
		logsErr,
	)
	return nil
}

func readSecretExtensionSSEFrame(t *testing.T, engine *gin.Engine, name string) string {
	t.Helper()
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(t.Context())
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		server.URL+"/extensions/"+name+"/logs?follow=1",
		http.NoBody,
	)
	if err != nil {
		cancel()
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		cancel()
		t.Fatalf("client.Do(SSE) error = %v", err)
	}
	defer func() {
		cancel()
		if err := response.Body.Close(); err != nil {
			t.Errorf("SSE response body Close() error = %v", err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			t.Fatalf("io.ReadAll(SSE error body) error = %v", readErr)
		}
		t.Fatalf("SSE status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, body)
	}
	reader := bufio.NewReader(response.Body)
	var frame strings.Builder
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("ReadString(SSE) error = %v", readErr)
		}
		frame.WriteString(line)
		if line == "\n" {
			return frame.String()
		}
	}
}

func newSecretEchoingPublishFailureServer(t *testing.T, secrets []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		if _, err := fmt.Fprintf(writer, "upstream failure %s %s", secrets[0], secrets[1]); err != nil {
			t.Errorf("fmt.Fprintf(publish failure response) error = %v", err)
		}
	}))
}

func mustSecretTransportJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v", value, err)
	}
	return payload
}

func assertSecretsAbsent(t *testing.T, surface, value string, secrets []string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(value, secret) {
			t.Fatalf("%s leaked secret %q: %s", surface, secret, value)
		}
	}
}
