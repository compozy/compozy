//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	"github.com/compozy/compozy/internal/providerexec"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/situation"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	skillbundled "github.com/compozy/compozy/skills"
)

func TestHarnessContextIntegrationStartupAndPromptShareResolverPolicy(t *testing.T) {
	t.Run(
		"Should share one resolver policy across startup and live prompts",
		testHarnessContextIntegrationStartupAndPromptShareResolverPolicy,
	)
}

func testHarnessContextIntegrationStartupAndPromptShareResolverPolicy(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	workspaceRoot := homePaths.HomeDir + "/workspace"
	resolvedWorkspace := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)
	writeDaemonMemoryIndex(t, cfg.Memory.GlobalDir, workspaceRoot)
	writeHarnessCheckpointSummary(t, workspaceRoot, "The prior session selected cobalt.")
	const gatedSkillName = "integration-platform-gated"
	writeDaemonFile(
		t,
		filepath.Join(homePaths.SkillsDir, gatedSkillName, "SKILL.md"),
		strings.Join([]string{
			"---",
			"name: " + gatedSkillName,
			"description: Must never be advertised by this integration fixture.",
			"metadata:",
			"  compozy:",
			"    when:",
			"      platforms: [compozy-integration-impossible]",
			"---",
			"body",
		}, "\n"),
	)

	daemonInstance, capturedDeps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	if daemonInstance.harnessResolver == nil {
		t.Fatal("boot() did not retain the harness resolver")
	}
	if _, ok := capturedDeps.PromptAssembler.(session.StartupPromptAssembler); !ok {
		t.Fatal("boot() did not inject a startup-aware prompt assembler")
	}
	if capturedDeps.StartupPromptOverlay == nil {
		t.Fatal("boot() did not inject the Compozy runtime startup prompt overlay")
	}
	if capturedDeps.PromptInputAugmenter == nil {
		t.Fatal("boot() did not inject the prompt input augmenter")
	}

	driver := newHarnessIntegrationDriver()
	workspaceResolver := &harnessIntegrationWorkspaceResolver{resolved: resolvedWorkspace}
	composite, err := newPromptInputCompositeAugmenter(
		discardLogger(),
		daemonInstance.harnessResolver,
		nil,
		defaultPromptInputAugmenterDescriptors(
			situation.WorkspaceKnowledgeAugmenter,
			memory.NewRecallAugmenter(daemonInstance.memoryStore),
			newSkillsCatalogAugmenter(daemonInstance.skillsRegistry, nil, func() promptSkillsWorkspaceResolver {
				return workspaceResolver
			}, nil),
			daemonInstance.situationContext.Augment,
		)...,
	)
	if err != nil {
		t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
	}
	capturedDeps.PromptInputAugmenter = composite
	manager := newHarnessIntegrationManager(t, homePaths, capturedDeps, resolvedWorkspace, driver)

	created, err := manager.Create(testutil.Context(t), session.CreateOpts{
		AgentName:                    resolvedWorkspace.Agents[0].Name,
		Name:                         "networked",
		Workspace:                    resolvedWorkspace.ID,
		ResolvedNetworkParticipation: daemonTestLiveParticipationPtr(resolvedWorkspace.ID, "builders"),
	})
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
		if info == nil || daemonInstance.skillsRegistry == nil {
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

	startupResolved, err := daemonInstance.harnessResolver.ResolveStartup(session.StartupPromptContext{
		SessionType:          created.Info().Type,
		NetworkParticipation: created.Info().NetworkParticipation,
		WorkspaceID:          created.Info().WorkspaceID,
		Workspace:            created.Info().Workspace,
		AgentName:            created.Info().AgentName,
	})
	if err != nil {
		t.Fatalf("ResolveStartup() error = %v", err)
	}
	if !containsHarnessSection(startupResolved.Policy.IncludeSections, HarnessPromptSectionNetwork) {
		t.Fatalf("startup IncludeSections = %#v, want network section", startupResolved.Policy.IncludeSections)
	}
	if !containsHarnessSection(startupResolved.Policy.IncludeSections, HarnessPromptSectionTools) {
		t.Fatalf("startup IncludeSections = %#v, want tools section", startupResolved.Policy.IncludeSections)
	}
	if !containsHarnessSection(startupResolved.Policy.IncludeSections, HarnessPromptSectionRuntimeIdentity) {
		t.Fatalf("startup IncludeSections = %#v, want runtime identity section", startupResolved.Policy.IncludeSections)
	}

	networkSkill, err := skillbundled.LoadResource(bundledCompozySkillName, bundledNetworkReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", bundledCompozySkillName, bundledNetworkReference, err)
	}
	networkSkill = strings.TrimSpace(networkSkill)
	toolsGuide, err := skillbundled.LoadResource(bundledCompozySkillName, bundledToolsReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", bundledCompozySkillName, bundledToolsReference, err)
	}
	toolsGuide = strings.TrimSpace(toolsGuide)
	nativeToolsGuide, err := skillbundled.LoadResource(bundledCompozySkillName, bundledNativeToolsReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", bundledCompozySkillName, bundledNativeToolsReference, err)
	}
	nativeToolsGuide = strings.TrimSpace(nativeToolsGuide)
	if got := driver.startCalls[0].SystemPrompt; !strings.Contains(got, "# Compozy Network Response Register") {
		t.Fatalf("start system prompt = %q, want compact network response register section", got)
	}
	if got := driver.startCalls[0].SystemPrompt; strings.Contains(got, networkSkill) {
		t.Fatalf("start system prompt = %q, want compact register, not full network skill", got)
	}
	if got := driver.startCalls[0].SystemPrompt; !strings.Contains(got, toolsGuide) {
		t.Fatalf("start system prompt = %q, want bundled tools guide content", got)
	}
	if got := driver.startCalls[0].SystemPrompt; !strings.Contains(got, nativeToolsGuide) {
		t.Fatalf("start system prompt = %q, want bundled native tools guide content", got)
	}
	if got := strings.Count(driver.startCalls[0].SystemPrompt, "# Compozy Network Response Register"); got != 1 {
		t.Fatalf("network response register occurrences = %d, want 1", got)
	}
	if got := strings.Count(driver.startCalls[0].SystemPrompt, toolsGuide); got != 1 {
		t.Fatalf("tools guide occurrences = %d, want 1", got)
	}
	if got := strings.Count(driver.startCalls[0].SystemPrompt, nativeToolsGuide); got != 1 {
		t.Fatalf("native tools guide occurrences = %d, want 1", got)
	}
	gatedCatalogEntry := `name="` + gatedSkillName + `"`
	if got := driver.startCalls[0].SystemPrompt; strings.Contains(got, gatedCatalogEntry) {
		t.Fatalf("start system prompt advertised gated skill %q", gatedSkillName)
	}
	// The startup prompt embeds the bundled tools guide, which documents the
	// "<compozy-situation-context>" open tag in prose; count the closing tag so the
	// assertion measures rendered sections, not documentation mentions.
	if got := strings.Count(driver.startCalls[0].SystemPrompt, "</compozy-situation-context>"); got != 1 {
		t.Fatalf("situation context section occurrences = %d, want 1", got)
	}
	if got := strings.Count(driver.startCalls[0].SystemPrompt, compozyRuntimeEnvelopeStart); got != 1 {
		t.Fatalf("Compozy runtime envelope occurrences = %d, want 1", got)
	}
	if got := driver.startCalls[0].SystemPrompt; !strings.Contains(got, "You are running inside Compozy") ||
		!strings.Contains(got, "Compozy is a local-first daemon") ||
		!strings.Contains(got, "- workspace_id: ws-harness") {
		t.Fatalf("start system prompt = %q, want Compozy runtime envelope with workspace facts", got)
	}
	if got := driver.startCalls[0].SystemPrompt; !strings.Contains(got, "<compozy_checkpoint_summary>") ||
		!strings.Contains(got, "The prior session selected cobalt.") {
		t.Fatalf("start system prompt = %q, want prior-session checkpoint summary", got)
	}
	assertPromptContainsInOrder(
		t,
		driver.startCalls[0].SystemPrompt,
		compozyRuntimeEnvelopeStart,
		"# Compozy Runtime",
		"canonical registry IDs",
		"<compozy-situation-context>",
		"# Persistent Memory",
		"<compozy_checkpoint_summary>",
		"The prior session selected cobalt.",
		"You are a coding assistant.",
		"<available-skills>",
		toolsGuide,
		nativeToolsGuide,
		"# Compozy Network Response Register",
	)

	userResolved, err := daemonInstance.harnessResolver.ResolvePrompt(
		created.Info(),
		session.TurnSourceUser,
		acp.PromptMeta{},
	)
	if err != nil {
		t.Fatalf("ResolvePrompt(user) error = %v", err)
	}
	if !slices.Equal(
		userResolved.Policy.EnableAugmenters,
		[]HarnessAugmenter{
			HarnessAugmenterWorkspaceKnowledge,
			HarnessAugmenterSkills,
			HarnessAugmenterSituation,
			HarnessAugmenterDurableMemory,
		},
	) {
		t.Fatalf(
			"user EnableAugmenters = %#v, want workspace knowledge, skills, situation, and durable memory",
			userResolved.Policy.EnableAugmenters,
		)
	}

	seedHarnessSituationTaskRun(t, daemonInstance, created.Info().WorkspaceID, created.Info().Workspace, created.ID)

	userEvents, err := manager.PromptWithOpts(testutil.Context(t), created.ID, session.PromptOpts{
		Message:    "workspace note",
		TurnSource: session.TurnSourceUser,
	})
	if err != nil {
		t.Fatalf("PromptWithOpts(user) error = %v", err)
	}
	drainHarnessIntegrationEvents(userEvents)
	if got := driver.promptCalls[0].Message; !strings.Contains(got, "Relevant durable memory for this turn:") {
		t.Fatalf("user prompt message = %q, want durable memory augmentation", got)
	}
	if got := driver.promptCalls[0].Message; !strings.Contains(got, "<current-available-skills>") {
		t.Fatalf("user prompt message = %q, want current skills augmentation", got)
	}
	if got := driver.promptCalls[0].Message; !strings.Contains(got, "<compozy-situation-context>") {
		t.Fatalf("user prompt message = %q, want situation context augmentation", got)
	}
	if got := driver.promptCalls[0].Message; !strings.Contains(got, `"channel_id":"coord-run-1"`) {
		t.Fatalf("user prompt message = %q, want resolved task participation channel", got)
	}
	if got := strings.Count(driver.promptCalls[0].Message, "<compozy-situation-context>"); got != 1 {
		t.Fatalf("user prompt situation context occurrences = %d, want 1", got)
	}
	if got := driver.promptCalls[0].Message; strings.Contains(got, gatedCatalogEntry) {
		t.Fatalf("user prompt advertised gated skill %q", gatedSkillName)
	}

	networkResolved, err := daemonInstance.harnessResolver.ResolvePrompt(
		created.Info(),
		session.TurnSourceNetwork,
		acp.PromptMeta{},
	)
	if err != nil {
		t.Fatalf("ResolvePrompt(network) error = %v", err)
	}
	if !slices.Equal(
		networkResolved.Policy.EnableAugmenters,
		[]HarnessAugmenter{HarnessAugmenterWorkspaceKnowledge, HarnessAugmenterSkills},
	) {
		t.Fatalf(
			"network EnableAugmenters = %#v, want workspace knowledge and skills",
			networkResolved.Policy.EnableAugmenters,
		)
	}

	networkEvents, err := manager.PromptNetwork(
		testutil.Context(t),
		created.ID,
		"workspace note",
		acp.PromptNetworkMeta{Channel: "builders", From: "ops.peer"},
	)
	if err != nil {
		t.Fatalf("PromptNetwork() error = %v", err)
	}
	drainHarnessIntegrationEvents(networkEvents)
	if got := driver.promptCalls[1].Message; !strings.Contains(got, "<current-available-skills>") {
		t.Fatalf("network prompt message = %q, want current skills augmentation", got)
	}
	if got := driver.promptCalls[1].Message; !strings.HasSuffix(got, "workspace note") {
		t.Fatalf("network prompt message = %q, want original network input preserved", got)
	}
	if got := driver.promptCalls[1].Meta.TurnSource; got != acp.PromptTurnSourceNetwork {
		t.Fatalf("network prompt turn source = %q, want %q", got, acp.PromptTurnSourceNetwork)
	}
	if got := driver.promptCalls[1].Message; strings.Contains(got, gatedCatalogEntry) {
		t.Fatalf("network prompt advertised gated skill %q", gatedSkillName)
	}
}

func writeHarnessCheckpointSummary(t *testing.T, workspaceRoot string, fact string) {
	t.Helper()

	body := strings.Join([]string{
		"## Historical Task Snapshot", "None.",
		"## Goal", fact,
		"## Constraints & Preferences", "None.",
		"## Completed Actions", "1. Preserved the prior-session fact.",
		"## Active State", "Idle.",
		"## Historical In-Progress State", "None.",
		"## Blocked", "None.",
		"## Key Decisions", fact,
		"## Resolved Questions", "None.",
		"## Historical Pending User Asks", "None.",
		"## Relevant Files", "None.",
		"## Historical Remaining Work", "None.",
		"## Critical Context", fact,
	}, "\n\n")
	writeDaemonFile(
		t,
		filepath.Join(workspaceRoot, compozyconfig.DirName, "memory", memory.CheckpointSummaryFilename),
		memoryDocument(
			"Workspace Checkpoint Summary",
			"Continuity checkpoint updated from completed workspace sessions.",
			memcontract.TypeProject,
			body,
		),
	)
}

func TestHarnessContextIntegrationResolverStableAcrossResume(t *testing.T) {
	t.Run(
		"Should preserve resolver policy across session resume",
		testHarnessContextIntegrationResolverStableAcrossResume,
	)
}

func testHarnessContextIntegrationResolverStableAcrossResume(t *testing.T) {
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

	driver := newHarnessIntegrationDriver()
	manager := newHarnessIntegrationManager(t, homePaths, capturedDeps, resolvedWorkspace, driver)

	created, err := manager.Create(testutil.Context(t), session.CreateOpts{
		AgentName:                    resolvedWorkspace.Agents[0].Name,
		Name:                         "networked",
		Workspace:                    resolvedWorkspace.ID,
		ResolvedNetworkParticipation: daemonTestLiveParticipationPtr(resolvedWorkspace.ID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	beforeResume, err := daemonInstance.harnessResolver.ResolvePrompt(
		created.Info(),
		session.TurnSourceNetwork,
		acp.PromptMeta{},
	)
	if err != nil {
		t.Fatalf("ResolvePrompt(before resume) error = %v", err)
	}

	if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	resumed, err := manager.Resume(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t), resumed.ID); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})

	afterResume, err := daemonInstance.harnessResolver.ResolvePrompt(
		resumed.Info(),
		session.TurnSourceNetwork,
		acp.PromptMeta{},
	)
	if err != nil {
		t.Fatalf("ResolvePrompt(after resume) error = %v", err)
	}

	if !reflect.DeepEqual(beforeResume.Policy, afterResume.Policy) {
		t.Fatalf(
			"resolved policy changed across resume\nbefore=%#v\nafter=%#v",
			beforeResume.Policy,
			afterResume.Policy,
		)
	}

	networkSkill, err := skillbundled.LoadResource(bundledCompozySkillName, bundledNetworkReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", bundledCompozySkillName, bundledNetworkReference, err)
	}
	networkSkill = strings.TrimSpace(networkSkill)
	toolsGuide, err := skillbundled.LoadResource(bundledCompozySkillName, bundledToolsReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", bundledCompozySkillName, bundledToolsReference, err)
	}
	toolsGuide = strings.TrimSpace(toolsGuide)
	nativeToolsGuide, err := skillbundled.LoadResource(bundledCompozySkillName, bundledNativeToolsReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", bundledCompozySkillName, bundledNativeToolsReference, err)
	}
	nativeToolsGuide = strings.TrimSpace(nativeToolsGuide)
	if got := strings.Count(driver.startCalls[1].SystemPrompt, "# Compozy Network Response Register"); got != 1 {
		t.Fatalf("resume prompt network response register occurrences = %d, want 1", got)
	}
	if got := driver.startCalls[1].SystemPrompt; strings.Contains(got, networkSkill) {
		t.Fatalf("resume prompt = %q, want compact register, not full network skill", got)
	}
	if got := strings.Count(driver.startCalls[1].SystemPrompt, toolsGuide); got != 1 {
		t.Fatalf("resume prompt tools guide occurrences = %d, want 1", got)
	}
	if got := strings.Count(driver.startCalls[1].SystemPrompt, nativeToolsGuide); got != 1 {
		t.Fatalf("resume prompt native tools guide occurrences = %d, want 1", got)
	}
	if got := strings.Count(driver.startCalls[1].SystemPrompt, compozyRuntimeEnvelopeStart); got != 1 {
		t.Fatalf("resume prompt Compozy runtime envelope occurrences = %d, want 1", got)
	}
}

func TestHarnessContextIntegrationStartupOmitsNetworkSectionForNonChannelSession(t *testing.T) {
	t.Run(
		"Should omit the startup network section for a non-channel session",
		testHarnessContextIntegrationStartupOmitsNetworkSectionForNonChannelSession,
	)
}

func testHarnessContextIntegrationStartupOmitsNetworkSectionForNonChannelSession(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	workspaceRoot := homePaths.HomeDir + "/workspace"
	resolvedWorkspace := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)
	writeDaemonMemoryIndex(t, cfg.Memory.GlobalDir, workspaceRoot)

	daemonInstance, capturedDeps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	driver := newHarnessIntegrationDriver()
	manager := newHarnessIntegrationManager(t, homePaths, capturedDeps, resolvedWorkspace, driver)

	created, err := manager.Create(testutil.Context(t), session.CreateOpts{
		AgentName: resolvedWorkspace.Agents[0].Name,
		Name:      "interactive",
		Workspace: resolvedWorkspace.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})

	networkSkill, err := skillbundled.LoadResource(bundledCompozySkillName, bundledNetworkReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", bundledCompozySkillName, bundledNetworkReference, err)
	}
	networkSkill = strings.TrimSpace(networkSkill)
	if strings.Contains(driver.startCalls[0].SystemPrompt, networkSkill) {
		t.Fatalf("start system prompt unexpectedly contains bundled network skill")
	}
	toolsGuide, err := skillbundled.LoadResource(bundledCompozySkillName, bundledToolsReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", bundledCompozySkillName, bundledToolsReference, err)
	}
	toolsGuide = strings.TrimSpace(toolsGuide)
	nativeToolsGuide, err := skillbundled.LoadResource(bundledCompozySkillName, bundledNativeToolsReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", bundledCompozySkillName, bundledNativeToolsReference, err)
	}
	nativeToolsGuide = strings.TrimSpace(nativeToolsGuide)
	if !strings.Contains(driver.startCalls[0].SystemPrompt, toolsGuide) {
		t.Fatalf("start system prompt missing bundled tools guide")
	}
	if !strings.Contains(driver.startCalls[0].SystemPrompt, nativeToolsGuide) {
		t.Fatalf("start system prompt missing bundled native tools guide")
	}
	assertPromptContainsInOrder(
		t,
		driver.startCalls[0].SystemPrompt,
		compozyRuntimeEnvelopeStart,
		"# Compozy Runtime",
		"canonical registry IDs",
		"<compozy-situation-context>",
		"# Persistent Memory",
		"You are a coding assistant.",
		"<available-skills>",
		toolsGuide,
		nativeToolsGuide,
	)
}

func seedHarnessSituationTaskRun(
	t *testing.T,
	daemonInstance *Daemon,
	workspaceID string,
	workspaceRoot string,
	sessionID string,
) {
	t.Helper()

	if daemonInstance.tasks == nil || daemonInstance.tasks.store == nil {
		t.Fatal("daemon task store is unavailable")
	}
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	if err := daemonInstance.registry.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
		ID:        workspaceID,
		RootDir:   workspaceRoot,
		Name:      "workspace",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	taskRecord := taskpkg.Task{
		ID:          "task-run-context",
		Identifier:  "AUTO-CTX",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: workspaceID,
		Title:       "Render situation context",
		Status:      taskpkg.TaskStatusInProgress,
		Priority:    taskpkg.PriorityHigh,
		CreatedBy:   taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "test"},
		Origin:      taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "test"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := daemonInstance.tasks.store.CreateTask(testutil.Context(t), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run := taskpkg.Run{
		ID:        "run-context",
		TaskID:    taskRecord.ID,
		Status:    taskpkg.TaskRunStatusRunning,
		Attempt:   1,
		SessionID: sessionID,
		Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "test"},
		RunNetworkState: &taskpkg.RunNetworkState{
			NetworkSpec: daemonTestLiveParticipation(workspaceID, "coord-run-1"),
		},
		Metadata:  json.RawMessage(`{"workflow_id":"wf-run-1"}`),
		QueuedAt:  now,
		StartedAt: now.Add(time.Minute),
	}
	seedDaemonTaskRunLifecycle(t, daemonInstance.tasks.store, run)
}

func bootHarnessPolicyDaemon(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	cfg *compozyconfig.Config,
) (*Daemon, SessionManagerDeps) {
	t.Helper()

	var capturedDeps SessionManagerDeps

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
		capturedDeps = deps
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	return d, capturedDeps
}

func newHarnessIntegrationManager(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	deps SessionManagerDeps,
	resolvedWorkspace workspacepkg.ResolvedWorkspace,
	driver *harnessIntegrationDriver,
	extraOpts ...session.Option,
) *session.Manager {
	t.Helper()

	opts := []session.Option{
		session.WithHomePaths(homePaths),
		session.WithDriver(driver),
		session.WithWorkspaceResolver(&harnessIntegrationWorkspaceResolver{resolved: resolvedWorkspace}),
		session.WithStore(
			func(ctx context.Context, owner store.SessionDBOwner, path string) (session.EventRecorder, error) {
				return sessiondb.OpenSessionDB(ctx, owner, path)
			},
		),
		session.WithLogger(discardLogger()),
		session.WithSandboxRegistry(deps.SandboxRegistry),
		session.WithPromptAssembler(deps.PromptAssembler),
		session.WithStartupPromptOverlay(deps.StartupPromptOverlay),
		session.WithPromptInputAugmenter(deps.PromptInputAugmenter),
		session.WithSkillRegistry(deps.SkillRegistry),
		session.WithMCPResolver(deps.MCPResolver),
	}
	opts = append(opts, extraOpts...)
	manager, err := session.NewManager(opts...)
	if err != nil {
		t.Fatalf("session.NewManager() error = %v", err)
	}
	return manager
}

func drainHarnessIntegrationEvents(events <-chan acp.AgentEvent) {
	for range events {
	}
}

func newHarnessIntegrationWorkspace(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	cfg compozyconfig.Config,
	root string,
) workspacepkg.ResolvedWorkspace {
	t.Helper()
	cfg.Providers = compozyconfig.CloneProviderConfigs(cfg.Providers)
	builtinProviders := compozyconfig.BuiltinProviders()
	for name := range builtinProviders {
		provider, err := cfg.ResolveProvider(name)
		if err != nil {
			t.Fatalf("ResolveProvider(%q) error = %v", name, err)
		}
		if providerexec.StrategyFor(provider).Kind != providerexec.StrategyNativeCLIBridge {
			continue
		}
		provider.Command = "test-acp-driver"
		cfg.Providers[name] = provider
	}
	for name, provider := range cfg.Providers {
		if _, builtin := builtinProviders[name]; builtin {
			continue
		}
		if providerexec.StrategyFor(provider).Kind != providerexec.StrategyNativeCLIBridge {
			continue
		}
		provider.Command = "test-acp-driver"
		cfg.Providers[name] = provider
	}

	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", root, err)
	}

	resolvedSandbox, err := cfg.ResolveSandbox(cfg.Defaults.Sandbox)
	if err != nil {
		t.Fatalf("ResolveSandbox() error = %v", err)
	}

	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      "ws-harness",
			RootDir: root,
			Name:    "workspace",
		},
		Config: cfg,
		Agents: []compozyconfig.AgentDef{
			{
				Name:     "coder",
				Provider: "claude",
				Model:    "test-model",
				Prompt:   "You are a coding assistant.",
			},
		},
		Sandbox: resolvedSandbox,
	}
}

type harnessIntegrationWorkspaceResolver struct {
	resolved workspacepkg.ResolvedWorkspace
}

func (r *harnessIntegrationWorkspaceResolver) Resolve(
	_ context.Context,
	idOrPath string,
) (workspacepkg.ResolvedWorkspace, error) {
	target := strings.TrimSpace(idOrPath)
	switch target {
	case r.resolved.ID, r.resolved.Name, r.resolved.RootDir:
		return r.resolved, nil
	default:
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
}

func (r *harnessIntegrationWorkspaceResolver) ResolveOrRegister(
	_ context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	if strings.TrimSpace(path) == strings.TrimSpace(r.resolved.RootDir) {
		return r.resolved, nil
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

type harnessIntegrationDriver struct {
	mu          sync.Mutex
	startCalls  []acp.StartOpts
	promptCalls []acp.PromptRequest
	processes   map[*session.AgentProcess]*harnessIntegrationProcess
	sequence    int
	startHook   func(acp.StartOpts, int) error
	promptHook  func(context.Context, *session.AgentProcess, acp.PromptRequest) (<-chan acp.AgentEvent, error)
	cancelHook  func(context.Context, *session.AgentProcess) error
}

type harnessIntegrationProcess struct {
	done   chan struct{}
	closed bool
	handle *session.AgentProcess
}

func newHarnessIntegrationDriver() *harnessIntegrationDriver {
	return &harnessIntegrationDriver{
		processes: make(map[*session.AgentProcess]*harnessIntegrationProcess),
	}
}

func (d *harnessIntegrationDriver) Start(_ context.Context, opts acp.StartOpts) (*session.AgentProcess, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.sequence++
	copied := opts
	copied.AdditionalDirs = append([]string(nil), opts.AdditionalDirs...)
	copied.Env = append([]string(nil), opts.Env...)
	copied.MCPServers = append([]compozyconfig.MCPServer(nil), opts.MCPServers...)
	d.startCalls = append(d.startCalls, copied)
	if d.startHook != nil {
		if err := d.startHook(copied, d.sequence); err != nil {
			return nil, err
		}
	}

	sessionID := fmt.Sprintf("acp-%d", d.sequence)
	if copied.ResumeSessionID != "" {
		sessionID = copied.ResumeSessionID
	}

	proc := newHarnessIntegrationProcess(copied.AgentName, copied.Command, copied.Cwd, sessionID)
	d.processes[proc.handle] = proc
	return proc.handle, nil
}

func (d *harnessIntegrationDriver) Prompt(
	ctx context.Context,
	proc *session.AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	if d.processes[proc] == nil {
		d.mu.Unlock()
		return nil, fmt.Errorf("test: unknown process")
	}
	d.promptCalls = append(d.promptCalls, req)
	hook := d.promptHook
	d.mu.Unlock()
	if hook != nil {
		return hook(ctx, proc, req)
	}

	events := make(chan acp.AgentEvent, 2)
	go func() {
		defer close(events)
		ts := time.Now().UTC()
		totalTokens := int64(3)
		events <- acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: proc.SessionID,
			TurnID:    req.TurnID,
			Timestamp: ts,
			Text:      "reply",
		}
		events <- acp.AgentEvent{
			Type:             acp.EventTypeDone,
			SessionID:        proc.SessionID,
			TurnID:           req.TurnID,
			Timestamp:        ts,
			StopReason:       string(acp.PromptStopReasonEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
			Usage: &acp.TokenUsage{
				TurnID:      req.TurnID,
				TotalTokens: &totalTokens,
				Timestamp:   ts,
			},
		}
	}()
	return events, nil
}

func (d *harnessIntegrationDriver) Cancel(ctx context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	hook := d.cancelHook
	d.mu.Unlock()
	if hook != nil {
		return hook(ctx, proc)
	}
	return nil
}

func (d *harnessIntegrationDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	fakeProc := d.processes[proc]
	if fakeProc == nil {
		return nil
	}
	fakeProc.exit()
	return nil
}

func newHarnessIntegrationProcess(
	agentName string,
	command string,
	cwd string,
	sessionID string,
) *harnessIntegrationProcess {
	proc := &harnessIntegrationProcess{
		done: make(chan struct{}),
	}
	proc.handle = session.NewAgentProcess(session.AgentProcessOptions{
		PID:       1,
		AgentName: agentName,
		Command:   command,
		Cwd:       cwd,
		SessionID: sessionID,
		Caps: acp.Caps{
			SupportsLoadSession: true,
			SupportedModes:      []string{"chat"},
		},
		StartedAt: time.Now().UTC(),
		Done:      proc.done,
		Wait: func() error {
			<-proc.done
			return nil
		},
		ConfigureRuntime: func(func() session.TurnSource) {},
	})
	return proc
}

func (p *harnessIntegrationProcess) exit() {
	if p == nil || p.closed {
		return
	}
	p.closed = true
	close(p.done)
}
