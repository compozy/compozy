//go:build integration

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/agentmemory"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/compozy/compozy/internal/testutil/acpmock"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestPromptInputCompositeIntegrationPreservesStoredMessagesAcrossUserTurns(t *testing.T) {
	t.Run(
		"Should preserve stored messages while augmenting user turns",
		testPromptInputCompositeIntegrationPreservesStoredMessages,
	)
}

func testPromptInputCompositeIntegrationPreservesStoredMessages(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	workspaceRoot := homePaths.HomeDir + "/workspace"
	resolvedWorkspace := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)

	daemonInstance, capturedDeps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	suffixAugmenter := HarnessAugmenter("suffix")
	workspaceResolver := &harnessIntegrationWorkspaceResolver{resolved: resolvedWorkspace}
	compositeResolver := &promptInputCompositeOverlayResolver{
		base: daemonInstance.harnessResolver,
		extra: map[TurnOrigin][]HarnessAugmenter{
			TurnOriginUser: {suffixAugmenter},
		},
	}

	composite, err := newPromptInputCompositeAugmenter(
		discardLogger(),
		compositeResolver,
		nil,
		append(
			defaultPromptInputAugmenterDescriptors(
				newSkillsCatalogAugmenter(daemonInstance.skillsRegistry, nil, func() promptSkillsWorkspaceResolver {
					return workspaceResolver
				}, nil),
				daemonInstance.situationContext.Augment,
				daemonInstance.standingContext.Augment,
			),
			promptInputAugmenterDescriptor{
				Name:   suffixAugmenter,
				Order:  200,
				Budget: 64,
				Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
					return message + "\n\nSUFFIX CONTEXT", nil
				},
			},
		)...,
	)
	if err != nil {
		t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
	}
	capturedDeps.PromptInputAugmenter = composite

	driver := newHarnessIntegrationDriver()
	manager := newHarnessIntegrationManager(t, homePaths, capturedDeps, resolvedWorkspace, driver)

	created, err := manager.Create(testutil.Context(t), new(session.CreateOpts{
		AgentName: resolvedWorkspace.Agents[0].Name,
		Name:      "worker",
		Workspace: resolvedWorkspace.ID,
	}))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})

	waitForCondition(t, "current skills catalog ready", func() bool {
		info := created.Info()
		if info == nil {
			return false
		}
		resolved, err := workspaceResolver.Resolve(testutil.Context(t), info.WorkspaceID)
		if err != nil {
			return false
		}
		projectedSkills, err := daemonInstance.skillsRegistry.ForAgent(
			testutil.Context(t),
			&resolved,
			info.AgentName,
		)
		if err != nil {
			return false
		}
		return strings.Contains(skillspkg.BuildCurrentCatalog(projectedSkills), "<current-available-skills>")
	})

	userEvents, err := manager.Prompt(testutil.Context(t), created.ID, "workspace note")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	drainHarnessIntegrationEvents(userEvents)

	if got := driver.promptCalls[0].Message; !strings.Contains(got, "<current-available-skills>") {
		t.Fatalf("user prompt message = %q, want current skills catalog", got)
	} else if !strings.Contains(got, "SUFFIX CONTEXT") {
		t.Fatalf("user prompt message = %q, want suffix augmenter output", got)
	}

	followupEvents, err := manager.Prompt(testutil.Context(t), created.ID, "follow-up note")
	if err != nil {
		t.Fatalf("Prompt(follow-up) error = %v", err)
	}
	drainHarnessIntegrationEvents(followupEvents)
	if got := driver.promptCalls[1].Message; !strings.Contains(got, "<current-available-skills>") ||
		!strings.Contains(got, "follow-up note") || !strings.Contains(got, "SUFFIX CONTEXT") {
		t.Fatalf("follow-up prompt message = %q, want current skills and augmented input", got)
	}
	if got, want := len(compositeResolver.seenMeta), 2; got != want {
		t.Fatalf("len(resolver seen meta) = %d, want %d", got, want)
	}

	storedMessages := loadStoredPromptMessages(t, created)
	if got, want := len(storedMessages), 2; got != want {
		t.Fatalf("len(storedMessages) = %d, want %d", got, want)
	}
	if !strings.Contains(storedMessages[0], `"text":"workspace note"`) {
		t.Fatalf("stored user message = %q, want canonical user input", storedMessages[0])
	}
	if strings.Contains(storedMessages[0], "SUFFIX CONTEXT") {
		t.Fatalf("stored user message = %q, want no augmenter content", storedMessages[0])
	}
	if !strings.Contains(storedMessages[1], `"text":"follow-up note"`) {
		t.Fatalf("stored follow-up message = %q, want canonical follow-up input", storedMessages[1])
	}
	if strings.Contains(storedMessages[1], "SUFFIX CONTEXT") {
		t.Fatalf("stored follow-up message = %q, want no augmenter content", storedMessages[1])
	}
}

// Invariant: workspace files stay uninjected in both user and synthetic deliveries.
// Owner: daemon prompt composition; canonical suite: prompt_input_composite integration.
func TestPromptInputCompositeIntegrationWorkspaceFilesRemainUserOwned(t *testing.T) {
	t.Run("Should deliver user and synthetic inputs without workspace knowledge [IT-010]", func(t *testing.T) {
		driverPath := acpmock.RequireDriver(t)
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		workspaceRoot := filepath.Join(homePaths.HomeDir, "workspace")
		resolvedWorkspace := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)
		writePromptKnowledgeFile(t, filepath.Join(workspaceRoot, "knowledge", "notes.md"), "WORKSPACE_NOTE_SENTINEL")
		daemonInstance, capturedDeps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
		t.Cleanup(func() {
			if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown(): %v", err)
			}
		})
		if err := daemonInstance.registry.InsertWorkspace(
			testutil.Context(t),
			resolvedWorkspace.Workspace,
		); err != nil {
			t.Fatal(err)
		}
		diagnostics := filepath.Join(t.TempDir(), "prompts.jsonl")
		command := acpmock.BuildCommand(
			driverPath,
			mockFixturePath(t, "agent_roles_fixture.json"),
			"role-agent",
			diagnostics,
		)
		resolvedWorkspace.Config.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)
		resolvedWorkspace.Agents[0].Provider = acpmock.ProviderName
		resolvedWorkspace.Agents[0].Model = "auto-title-model-v1"
		driver := session.NewACPDriverAdapter(acp.New(acp.WithProviderPreStarter(daemonInstance.providerPreStarter)))
		manager := newHarnessIntegrationManager(
			t,
			homePaths,
			capturedDeps,
			resolvedWorkspace,
			driver,
			session.WithSessionCatalog(capturedDeps.SessionCatalog),
			session.WithSessionInputQueueStore(capturedDeps.SessionInputQueue),
			session.WithSessionPromptAdmissionStore(capturedDeps.SessionPromptAdmission),
		)
		created, err := manager.Create(
			testutil.Context(t),
			new(session.CreateOpts{AgentName: resolvedWorkspace.Agents[0].Name, Workspace: resolvedWorkspace.ID}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Errorf("Stop(): %v", err)
			}
		})
		user, err := manager.Prompt(testutil.Context(t), created.ID, "inspect the workspace")
		if err != nil {
			t.Fatal(err)
		}
		drainHarnessIntegrationEvents(user)
		synthetic, err := manager.PromptSynthetic(
			testutil.Context(t),
			created.ID,
			session.SyntheticPromptOpts{
				Message:  "continue the task",
				Metadata: acp.PromptSyntheticMeta{TaskRunID: "run-1", Reason: "task_run_ready"},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		drainHarnessIntegrationEvents(synthetic)
		records, err := acpmock.ReadDiagnostics(diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		prompts := acpmock.PromptDiagnostics(records)
		if len(prompts) != 2 {
			t.Fatalf("prompt calls = %d, want user and synthetic", len(prompts))
		}
		for _, prompt := range prompts {
			if strings.Contains(prompt.Prompt, "<workspace-knowledge-snapshot>") ||
				strings.Contains(prompt.Prompt, "WORKSPACE_NOTE_SENTINEL") {
				t.Fatalf("delivered prompt = %q", prompt.Prompt)
			}
		}
		deliveries, err := manager.Deliveries(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(deliveries) != 2 {
			t.Fatalf("delivery ledger = %#v, want two receipts", deliveries)
		}
		for _, delivery := range deliveries {
			for _, span := range delivery.Manifest.Spans {
				if span.Key == "knowledge" {
					t.Fatalf("knowledge delivery span = %#v", span)
				}
			}
		}
	})
}

// Invariant: retired SOUL metadata never prevents session startup or hides active persona.
// Owner: daemon session startup; canonical suite: prompt_input_composite integration.
func TestPromptInputCompositeIntegrationSoulRetiredPolicy(t *testing.T) {
	t.Run("Should start with retained soul content and no retired policy diagnostic [IT-005]", func(t *testing.T) {
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		workspace := newHarnessIntegrationWorkspace(t, homePaths, cfg, filepath.Join(homePaths.HomeDir, "workspace"))
		agentPath := filepath.Join(workspace.RootDir, ".compozy", "agents", "coder", "AGENT.md")
		writePromptKnowledgeFile(t, agentPath, "# Coder")
		writePromptKnowledgeFile(
			t,
			filepath.Join(filepath.Dir(agentPath), "SOUL.md"),
			"---\nrole: Reviewer\nprinciples: [protect correctness]\nmemory_policy: [keep notes]\n---\nKeep the persona.",
		)
		workspace.Agents[0].SourcePath = agentPath
		daemonInstance, deps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
		t.Cleanup(func() {
			if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown(): %v", err)
			}
		})
		if err := daemonInstance.registry.InsertWorkspace(testutil.Context(t), workspace.Workspace); err != nil {
			t.Fatal(err)
		}
		driver := newHarnessIntegrationDriver()
		manager := newHarnessIntegrationManager(
			t,
			homePaths,
			deps,
			workspace,
			driver,
			session.WithSoulSnapshotStore(deps.SoulStore),
		)
		created, err := manager.Create(
			testutil.Context(t),
			new(session.CreateOpts{AgentName: "coder", Workspace: workspace.ID}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Errorf("Stop(): %v", err)
			}
		})
		output, err := manager.Prompt(testutil.Context(t), created.ID, "review the change")
		if err != nil {
			t.Fatal(err)
		}
		drainHarnessIntegrationEvents(output)
		prompt := driver.startCalls[0].SystemPrompt
		for _, content := range []string{"<compozy-agent-soul>", "Role: Reviewer", "protect correctness", "Keep the persona."} {
			if !strings.Contains(prompt, content) {
				t.Fatalf("startup prompt = %q, want %q", prompt, content)
			}
		}
		if strings.Contains(prompt, "Memory policy") || strings.Contains(prompt, "keep notes") {
			t.Fatalf("startup prompt = %q", prompt)
		}
		snapshot, err := deps.SoulStore.GetSoulSnapshot(testutil.Context(t), created.Info().SoulSnapshotID)
		if err != nil {
			t.Fatal(err)
		}
		profile, err := snapshot.ProfileEnvelope()
		if err != nil {
			t.Fatal(err)
		}
		if len(profile.Diagnostics) != 0 {
			t.Fatalf("SOUL diagnostics = %#v", profile.Diagnostics)
		}
	})
}

func writePromptKnowledgeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

type promptInputCompositeOverlayResolver struct {
	base     promptInputAugmenterResolver
	extra    map[TurnOrigin][]HarnessAugmenter
	seenMeta []acp.PromptMeta
}

func (r *promptInputCompositeOverlayResolver) ResolvePrompt(
	info *session.Info,
	source session.TurnSource,
	meta acp.PromptMeta,
) (ResolvedHarnessContext, error) {
	r.seenMeta = append(r.seenMeta, meta.Normalize())
	resolved, err := r.base.ResolvePrompt(info, source, meta)
	if err != nil {
		return ResolvedHarnessContext{}, err
	}
	additional := r.extra[resolved.Policy.TurnOrigin]
	if len(additional) == 0 {
		return resolved, nil
	}
	resolved.Policy.EnableAugmenters = append(
		append([]HarnessAugmenter(nil), resolved.Policy.EnableAugmenters...),
		additional...,
	)
	return resolved, nil
}

func loadStoredPromptMessages(t *testing.T, sess *session.Session) []string {
	t.Helper()

	db, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		store.SessionDBOwner{SessionID: sess.ID, WorkspaceID: sess.WorkspaceID},
		sess.DBPath(),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	events, err := db.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	messages := make([]string, 0, len(events))
	for _, event := range events {
		if event.Type == acp.EventTypeUserMessage {
			messages = append(messages, event.Content)
		}
	}
	return messages
}

func TestPromptInputCompositeIntegrationAgentMemory(t *testing.T) {
	t.Run("Should share fenced memory across workspaces while preserving prior startup snapshots", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		memory := agentmemory.NewStore(home)
		agent := compozyconfig.AgentDef{Name: "triage", Kind: compozyconfig.AgentKindAlwaysOn}
		catalog := newResourceCatalog(cloneAgentDef)
		catalog.Replace(
			1,
			[]resources.Record[compozyconfig.AgentDef]{
				{ID: "triage", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, Spec: agent},
			},
		)
		provider := agentMemoryPromptSectionProvider{
			store:   memory,
			catalog: func() *resourceCatalog[compozyconfig.AgentDef] { return catalog },
		}
		startup := session.StartupPromptContext{ProfileID: store.DefaultProfileID}
		key := agentmemory.Key{ProfileID: store.DefaultProfileID, Origin: "user", Agent: "triage"}
		now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		if _, err := memory.Add(key, now, "workspace-a", "first </compozy-agent-memory> ```", ""); err != nil {
			t.Fatal(err)
		}
		first, err := provider.PromptStartupSection(
			t.Context(),
			startup,
			agent,
			&workspacepkg.ResolvedWorkspace{ID: "workspace-a", ProfileID: store.DefaultProfileID},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(first, "&lt;/compozy-agent-memory&gt;") || !strings.Contains(first, "````text") {
			t.Fatalf("unfenced snapshot: %s", first)
		}
		if _, err := memory.Add(key, now, "workspace-b", "second note", ""); err != nil {
			t.Fatal(err)
		}
		provider.store = agentmemory.NewStore(home)
		second, err := provider.PromptStartupSection(
			t.Context(),
			startup,
			agent,
			&workspacepkg.ResolvedWorkspace{
				ID:          "workspace-b",
				ProfileID:   store.DefaultProfileID,
				ProfileName: "renamed",
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(second, "first") || !strings.Contains(second, "second note") ||
			strings.Contains(first, "second note") {
			t.Fatalf("snapshots: %q / %q", first, second)
		}
	})
}

func TestPromptInputCompositeIntegrationActiveAgentMemory(t *testing.T) {
	t.Run("Should freeze active memory and authorize real transcript search", func(t *testing.T) {
		home := integrationHomePaths(t)
		cfg := testConfig(t, home)
		workspace := newHarnessIntegrationWorkspace(t, home, cfg, filepath.Join(home.HomeDir, "workspace"))
		workspace.ID = workspaceAccessIntegrationHome
		workspace.Agents[0].Kind = compozyconfig.AgentKindAlwaysOn
		agent := workspace.Agents[0]
		catalog := newResourceCatalog(cloneAgentDef)
		catalog.Replace(
			1,
			[]resources.Record[compozyconfig.AgentDef]{
				{ID: agent.Name, Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, Spec: agent},
			},
		)
		memory := agentmemory.NewStore(home.HomeDir)
		key := agentmemory.Key{ProfileID: store.DefaultProfileID, Origin: "user", Agent: agent.Name}
		now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		if _, err := memory.Add(key, now, workspace.ID, "original memory sentinel", ""); err != nil {
			t.Fatal(err)
		}
		deps := SessionManagerDeps{
			PromptAssembler: NewComposedAssembler(
				WithPromptSectionDescriptors(agentMemoryPromptDescriptor(memory, catalog)),
			),
		}
		db, err := openDaemonTestGlobalDBAtPath(t.Context(), filepath.Join(t.TempDir(), store.GlobalDatabaseName))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := db.InsertWorkspace(t.Context(), workspace.Workspace); err != nil {
			t.Fatal(err)
		}
		driver := newHarnessIntegrationDriver()
		manager := newHarnessIntegrationManager(t, home, deps, workspace, driver, session.WithSessionCatalog(db))
		first, err := manager.Create(
			t.Context(),
			new(session.CreateOpts{AgentName: agent.Name, Workspace: workspace.ID}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(context.Background(), first.ID); err != nil {
				t.Error(err)
			}
		})
		events, err := manager.Prompt(t.Context(), first.ID, "searchable memory transcript")
		if err != nil {
			t.Fatal(err)
		}
		drainHarnessIntegrationEvents(events)
		if _, err := memory.Add(key, now, workspace.ID, "updated memory sentinel", ""); err != nil {
			t.Fatal(err)
		}
		events, err = manager.Prompt(t.Context(), first.ID, "continue")
		if err != nil {
			t.Fatal(err)
		}
		drainHarnessIntegrationEvents(events)
		driver.mu.Lock()
		starts := append([]acp.StartOpts(nil), driver.startCalls...)
		driver.mu.Unlock()
		if len(starts) != 1 || !strings.Contains(starts[0].SystemPrompt, "original memory sentinel") ||
			strings.Contains(starts[0].SystemPrompt, "updated memory sentinel") {
			t.Fatalf("active startup: %+v", starts)
		}
		second, err := manager.Create(
			t.Context(),
			new(session.CreateOpts{AgentName: agent.Name, Workspace: workspace.ID}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(context.Background(), second.ID); err != nil {
				t.Error(err)
			}
		})
		events, err = manager.Prompt(t.Context(), second.ID, "new session")
		if err != nil {
			t.Fatal(err)
		}
		drainHarnessIntegrationEvents(events)
		driver.mu.Lock()
		latest := driver.startCalls[len(driver.startCalls)-1].SystemPrompt
		driver.mu.Unlock()
		if !strings.Contains(latest, "updated memory sentinel") {
			t.Fatalf("new startup: %s", latest)
		}
		audit, err := newWorkspaceAccessAuditEmitter(db, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		facts, err := newWorkspaceAccessModeSource(manager)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := workspaceaccess.New(
			workspaceaccess.Deps{
				Facts:   facts,
				Consent: newWorkspaceAccessConsentCache(),
				Audit:   audit,
				Log:     discardLogger(),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		actor := workspaceaccess.ActorRef{
			Kind:        workspaceaccess.ActorAgentSession,
			SessionID:   first.ID,
			AgentName:   agent.Name,
			WorkspaceID: workspace.ID,
		}
		matches, err := core.SearchAgentMemorySessions(
			t.Context(),
			manager,
			policy,
			actor,
			store.DefaultProfileID,
			"searchable memory transcript",
		)
		if err != nil || len(matches) != 1 || matches[0].SessionID != first.ID {
			t.Fatalf("authorized search: %+v, %v", matches, err)
		}
		otherWorkspace := workspace
		otherWorkspace.ID = workspaceAccessIntegrationTarget
		otherWorkspace.RootDir = t.TempDir()
		otherWorkspace.Name = "other-workspace"
		otherWorkspace.Agents = append([]compozyconfig.AgentDef(nil), workspace.Agents...)
		otherWorkspace.Agents[0].Permissions = string(compozyconfig.PermissionModeDenyAll)
		if err := db.InsertWorkspace(t.Context(), otherWorkspace.Workspace); err != nil {
			t.Fatal(err)
		}
		otherManager := newHarnessIntegrationManager(
			t,
			home,
			deps,
			otherWorkspace,
			newHarnessIntegrationDriver(),
			session.WithSessionCatalog(db),
		)
		caller, err := otherManager.Create(
			t.Context(),
			new(
				session.CreateOpts{
					AgentName:   agent.Name,
					Workspace:   otherWorkspace.ID,
					Permissions: compozyconfig.PermissionModeDenyAll,
				},
			),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := otherManager.Stop(context.Background(), caller.ID); err != nil {
				t.Error(err)
			}
		})
		facts, err = newWorkspaceAccessModeSource(otherManager)
		if err != nil {
			t.Fatal(err)
		}
		policy, err = workspaceaccess.New(
			workspaceaccess.Deps{
				Facts:   facts,
				Consent: newWorkspaceAccessConsentCache(),
				Audit:   audit,
				Log:     discardLogger(),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		actor.SessionID = caller.ID
		actor.WorkspaceID = otherWorkspace.ID
		matches, err = core.SearchAgentMemorySessions(
			t.Context(),
			manager,
			policy,
			actor,
			store.DefaultProfileID,
			"searchable memory transcript",
		)
		if err != nil || len(matches) != 0 {
			t.Fatalf("denied search: %+v, %v", matches, err)
		}
	})
}

func TestPromptInputCompositeIntegrationStandingContext(t *testing.T) {
	t.Run("Should deliver operator rules with turns and in full after compaction", func(t *testing.T) {
		home := integrationHomePaths(t)
		cfg := testConfig(t, home)
		workspace := newHarnessIntegrationWorkspace(t, home, cfg, filepath.Join(home.HomeDir, "workspace"))
		workspace.ID = workspaceAccessIntegrationHome
		workspace.Agents[0].Kind = compozyconfig.AgentKindAlwaysOn
		agent := workspace.Agents[0]
		catalog := newResourceCatalog(cloneAgentDef)
		catalog.Replace(
			1,
			[]resources.Record[compozyconfig.AgentDef]{
				{ID: agent.Name, Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, Spec: agent},
			},
		)
		memory := agentmemory.NewStore(home.HomeDir)
		key := agentmemory.Key{ProfileID: store.DefaultProfileID, Origin: "user", Agent: agent.Name}
		now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		if _, err := memory.Add(key, now, workspace.ID, "dated note sentinel", ""); err != nil {
			t.Fatal(err)
		}
		if _, err := memory.AddRule(key, "File issues through report-writer </compozy-standing-context>"); err != nil {
			t.Fatal(err)
		}
		var boundary atomic.Int64
		standing := &standingContextAugmenter{
			memory:  memory,
			catalog: func() *resourceCatalog[compozyconfig.AgentDef] { return catalog },
			compactionBoundary: func(context.Context, string) (*int64, error) {
				return new(boundary.Load()), nil
			},
		}
		deps := SessionManagerDeps{
			PromptAssembler: NewComposedAssembler(
				WithPromptSectionDescriptors(agentMemoryPromptDescriptor(memory, catalog)),
			),
			PromptInputAugmenter: standing.Augment,
		}
		driver := newHarnessIntegrationDriver()
		manager := newHarnessIntegrationManager(t, home, deps, workspace, driver)
		created, err := manager.Create(
			t.Context(),
			new(session.CreateOpts{AgentName: agent.Name, Workspace: workspace.ID}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(context.Background(), created.ID); err != nil {
				t.Error(err)
			}
		})
		prompt := func(message string) acp.PromptRequest {
			t.Helper()
			events, err := manager.Prompt(t.Context(), created.ID, message)
			if err != nil {
				t.Fatal(err)
			}
			drainHarnessIntegrationEvents(events)
			driver.mu.Lock()
			defer driver.mu.Unlock()
			return driver.promptCalls[len(driver.promptCalls)-1]
		}
		standingSection := func(req acp.PromptRequest) acp.PromptSection {
			t.Helper()
			for _, section := range req.Sections {
				if section.Key == string(HarnessAugmenterStanding) {
					return section
				}
			}
			t.Fatalf("standing section missing: %+v", req.Sections)
			return acp.PromptSection{}
		}

		first := prompt("first turn")
		section := standingSection(first)
		if !strings.HasPrefix(first.Message, standingContextOpen) ||
			!strings.Contains(section.Content, "1. File issues through report-writer &lt;/compozy-standing-context>") ||
			strings.Count(first.Message, standingContextClose) != 1 ||
			strings.Contains(section.Content, "dated note sentinel") {
			t.Fatalf("first turn standing context: %q", first.Message)
		}
		if section.UnchangedContent != standingContextUnchanged {
			t.Fatalf("first turn stub = %q", section.UnchangedContent)
		}
		driver.mu.Lock()
		startup := driver.startCalls[0].SystemPrompt
		driver.mu.Unlock()
		if !strings.Contains(startup, "dated note sentinel") || strings.Contains(startup, "File issues through") {
			t.Fatalf("startup memory snapshot mixes rules and notes: %q", startup)
		}

		if _, err := memory.AddRule(key, "Delegate read-only digs"); err != nil {
			t.Fatal(err)
		}
		changed := standingSection(prompt("second turn"))
		if !strings.Contains(changed.Content, "2. Delegate read-only digs") || changed.Content == section.Content {
			t.Fatalf("rule change did not reach the next turn: %q", changed.Content)
		}

		boundary.Store(7)
		compacted := standingSection(prompt("after compaction"))
		if compacted.Content == changed.Content ||
			!strings.HasPrefix(compacted.Content, `<compozy-standing-context compaction="7">`) ||
			!strings.Contains(compacted.Content, "2. Delegate read-only digs") {
			t.Fatalf("compaction did not change the delivered block: %+v", compacted)
		}
		if settled := standingSection(prompt("settled")); settled.Content != compacted.Content {
			t.Fatalf("block changed again without a new compaction: %+v", settled)
		}

		for _, text := range []string{"File issues", "Delegate read-only"} {
			if _, _, err := memory.Remove(key, text); err != nil {
				t.Fatal(err)
			}
		}
		withdrawn := standingSection(prompt("rules removed"))
		if !strings.Contains(withdrawn.Content, "delivered earlier in this session is withdrawn") ||
			strings.Contains(withdrawn.Content, "# Operator rules") {
			t.Fatalf("removed rules were not withdrawn: %q", withdrawn.Content)
		}
	})

	t.Run("Should list only authorized peers and reuse the roster until it can have changed", func(t *testing.T) {
		t.Parallel()
		db, err := openDaemonTestGlobalDBAtPath(t.Context(), filepath.Join(t.TempDir(), store.GlobalDatabaseName))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		stamp := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
		for _, id := range []string{"allowed", "denied"} {
			if err := db.InsertWorkspace(t.Context(), workspacepkg.Workspace{
				ID: id, Name: id, RootDir: filepath.Join(t.TempDir(), id), CreatedAt: stamp, UpdatedAt: stamp,
			}); err != nil {
				t.Fatal(err)
			}
		}
		repo := db.Instances()
		admit := func(workspace, agent string) {
			t.Helper()
			if _, _, err := repo.AdmitCreate(t.Context(), agentinstance.Instance{
				ProfileID: store.DefaultProfileID, WorkspaceID: workspace, AgentName: agent,
				PermissionCeiling: "{}", CreatedByKind: "human",
			}, 10); err != nil {
				t.Fatal(err)
			}
		}
		admit("allowed", "lead")
		admit("allowed", "writer")
		admit("denied", "hidden")
		reads := 0
		api := &core.AgentInstanceAPI{
			Store: repo,
			Authority: agentinstance.Authority{Callers: &instanceAdmissionInputs{
				caller: agentinstance.Caller{
					InstanceID: "lead-instance", AgentName: "lead", ProfileID: store.DefaultProfileID,
				},
			}},
			ReadPeer: func(_ context.Context, row agentinstance.Instance) (contract.AgentInstancePeer, error) {
				reads++
				return contract.AgentInstancePeer{
					Name: row.AgentName, Scope: row.WorkspaceID, State: row.State, Accepts: []string{"intake: <text>"},
				}, nil
			},
		}
		now := stamp
		standing := &standingContextAugmenter{
			instances: func() *core.AgentInstanceAPI { return api },
			access:    func() workspaceaccess.Policy { return standingAllowedWorkspacePolicy{} },
			now:       func() time.Time { return now },
		}
		info := &session.Info{
			ID: "sess-lead", InstanceID: "lead-instance", AgentName: "lead",
			ProfileID: store.DefaultProfileID, WorkspaceID: "allowed",
		}
		peers, more, _ := standing.peers(t.Context(), info, "")
		if more || len(peers) != 1 || peers[0].Name != "writer" || peers[0].Accepts[0] != "intake: <text>" {
			t.Fatalf("roster = %+v more=%t", peers, more)
		}
		admit("allowed", "cleaner")
		firstReads := reads
		if peers, _, _ = standing.peers(t.Context(), info, ""); len(peers) != 1 || reads != firstReads {
			t.Fatalf("cached roster = %+v reads=%d->%d", peers, firstReads, reads)
		}
		now = now.Add(standingRosterTTL)
		if peers, _, _ = standing.peers(t.Context(), info, ""); len(peers) != 2 {
			t.Fatalf("refreshed roster = %+v", peers)
		}
		admit("allowed", "watcher")
		standing.forgetRosters()
		if peers, _, _ = standing.peers(t.Context(), info, ""); len(peers) != 3 {
			t.Fatalf("roster after an instance change = %+v", peers)
		}
		standing.forgetRosters()
		if peers, _, _ = standing.peers(t.Context(), info, "other-placement"); len(peers) != 3 {
			t.Fatalf("own placement listed as a peer: %+v", peers)
		}
		standing.forgetRosters()
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		_, _, unavailable := standing.peers(canceled, info, "")
		failed := renderStandingContext(standingView{peersUnavailable: unavailable})
		if !unavailable || !strings.Contains(failed, "could not be read") || strings.Contains(failed, "withdrawn") {
			t.Fatalf("failed roster read rendered as %q", failed)
		}
		outsider := &session.Info{ID: "sess-plain", AgentName: "lead", ProfileID: store.DefaultProfileID}
		if peers, _, _ = standing.peers(t.Context(), outsider, ""); len(peers) != 0 {
			t.Fatalf("session outside an instance got a roster: %+v", peers)
		}
	})

	t.Run("Should render a bounded roster with input forms and skip on-demand agents", func(t *testing.T) {
		t.Parallel()
		peers := []contract.AgentInstancePeer{
			{
				Name: "report-writer", Scope: "compozy", State: "active",
				Description: "Turns raw reports into issues.", Accepts: []string{"intake: <text>", "sweep:"},
			},
			{Name: "cleaner", Scope: "compozy", State: "paused", Description: strings.Repeat("long ", 80)},
		}
		rendered := renderStandingContext(standingView{peers: peers, more: true})
		for _, want := range []string{
			"- report-writer (compozy): Turns raw reports into issues. Accepts: `intake: <text>` | `sweep:`",
			"- cleaner (compozy) [paused]: long ",
			"compozy__agent_instance_list",
			"`agent` set to the peer name",
		} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("roster missing %q in %q", want, rendered)
			}
		}
		if strings.Contains(rendered, "# Operator rules") {
			t.Fatalf("rules heading rendered without rules: %q", rendered)
		}
		rules := make([]string, 0, 60)
		for range 60 {
			rules = append(rules, strings.Repeat("r", 900))
		}
		bounded := renderStandingContext(standingView{rules: rules, peers: peers})
		if size := utf8.RuneCountInString(bounded); size+2 > standingAugmenterBudget {
			t.Fatalf("standing context is %d runes, budget %d", size, standingAugmenterBudget)
		}
		if !strings.HasSuffix(bounded, standingContextClose) || !strings.Contains(bounded, "More rules exist") ||
			!strings.Contains(bounded, "More peers exist") {
			t.Fatalf("bounded context lost its envelope or an overflow pointer")
		}
		unavailable := renderStandingContext(standingView{rulesUnavailable: true, peers: peers})
		if !strings.Contains(unavailable, "could not be loaded") || !strings.Contains(unavailable, "- report-writer") {
			t.Fatalf("unreadable rules hid the failure or the roster: %q", unavailable)
		}
		if empty := renderStandingContext(standingView{}); !strings.Contains(empty, "is withdrawn") {
			t.Fatalf("empty standing context = %q", empty)
		}
		onDemand := &standingContextAugmenter{}
		if got, err := onDemand.Augment(t.Context(), nil, "message"); err != nil || got != "message" {
			t.Fatalf("nil session = %q, %v", got, err)
		}
	})
}

type standingAllowedWorkspacePolicy struct{}

func (standingAllowedWorkspacePolicy) Authorize(
	_ context.Context, req workspaceaccess.Request,
) (workspaceaccess.Decision, error) {
	return workspaceaccess.Decision{
		Allowed: req.Target.WorkspaceID == "allowed", Source: workspaceaccess.SourceAgentGrant,
	}, nil
}
