//go:build integration

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/situation"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
)

func TestPromptInputCompositeIntegrationPreservesStoredMessagesAcrossUserAndNetworkTurns(t *testing.T) {
	t.Run("Should preserve stored messages while augmenting user and network turns", testPromptInputCompositeIntegrationPreservesStoredMessages)
}

func testPromptInputCompositeIntegrationPreservesStoredMessages(t *testing.T) {
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

	suffixAugmenter := HarnessAugmenter("suffix")
	workspaceResolver := &harnessIntegrationWorkspaceResolver{resolved: resolvedWorkspace}
	compositeResolver := &promptInputCompositeOverlayResolver{
		base: daemonInstance.harnessResolver,
		extra: map[TurnOrigin][]HarnessAugmenter{
			TurnOriginUser:    {suffixAugmenter},
			TurnOriginNetwork: {suffixAugmenter},
		},
	}

	composite, err := newPromptInputCompositeAugmenter(
		discardLogger(),
		compositeResolver,
		nil,
		append(
			defaultPromptInputAugmenterDescriptors(
				situation.WorkspaceKnowledgeAugmenter,
				memory.NewRecallAugmenter(daemonInstance.memoryStore),
				newSkillsCatalogAugmenter(daemonInstance.skillsRegistry, nil, func() promptSkillsWorkspaceResolver {
					return workspaceResolver
				}),
				daemonInstance.situationContext.Augment,
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

	if got := driver.promptCalls[0].Message; !strings.Contains(got, "Relevant durable memory for this turn:") {
		t.Fatalf("user prompt message = %q, want durable memory recall", got)
	} else if !strings.Contains(got, "<current-available-skills>") {
		t.Fatalf("user prompt message = %q, want current skills catalog", got)
	} else if !strings.Contains(got, "SUFFIX CONTEXT") {
		t.Fatalf("user prompt message = %q, want suffix augmenter output", got)
	}

	networkEvents, err := manager.PromptNetwork(
		testutil.Context(t),
		created.ID,
		"network note",
		acp.PromptNetworkMeta{
			MessageID:   "msg-1",
			Kind:        "say",
			Channel:     "builders",
			Surface:     "direct",
			DirectID:    "direct_0123456789abcdef0123456789abcdef",
			From:        "ops.peer",
			To:          "networked.peer",
			WorkID:      "work_patch_42",
			ReplyTo:     "msg-root",
			TraceID:     "trace_ops_patch_42",
			CausationID: "msg-root",
			Trust:       "untrusted",
		},
	)
	if err != nil {
		t.Fatalf("PromptNetwork() error = %v", err)
	}
	drainHarnessIntegrationEvents(networkEvents)

	if got := driver.promptCalls[1].Message; !strings.Contains(got, "<current-available-skills>") {
		t.Fatalf("network prompt message = %q, want current skills catalog", got)
	} else if !strings.HasSuffix(got, "network note\n\nSUFFIX CONTEXT") {
		t.Fatalf("network prompt message = %q, want augmented network dispatch with preserved suffix", got)
	}
	if got := driver.promptCalls[1].Meta.TurnSource; got != acp.PromptTurnSourceNetwork {
		t.Fatalf("network prompt turn source = %q, want %q", got, acp.PromptTurnSourceNetwork)
	}
	if got, want := len(compositeResolver.seenMeta), 2; got != want {
		t.Fatalf("len(resolver seen meta) after network prompt = %d, want %d", got, want)
	}
	if got := compositeResolver.seenMeta[1].Network; got == nil {
		t.Fatal("resolver network prompt meta = nil, want forwarded network metadata")
	} else {
		if got.MessageID != "msg-1" {
			t.Fatalf("resolver network message_id = %q, want %q", got.MessageID, "msg-1")
		}
		if got.Channel != "builders" {
			t.Fatalf("resolver network channel = %q, want %q", got.Channel, "builders")
		}
		if got.Surface != "direct" {
			t.Fatalf("resolver network surface = %q, want direct", got.Surface)
		}
		if got.DirectID != "direct_0123456789abcdef0123456789abcdef" {
			t.Fatalf("resolver network direct_id = %q, want final direct container", got.DirectID)
		}
		if got.WorkID != "work_patch_42" || got.ReplyTo != "msg-root" {
			t.Fatalf("resolver network work/reply = %q/%q, want work_patch_42/msg-root", got.WorkID, got.ReplyTo)
		}
		if got.TraceID != "trace_ops_patch_42" || got.CausationID != "msg-root" {
			t.Fatalf(
				"resolver network trace/causation = %q/%q, want final correlation ids",
				got.TraceID,
				got.CausationID,
			)
		}
		if got.Trust != "untrusted" {
			t.Fatalf("resolver network trust = %q, want untrusted", got.Trust)
		}
	}

	storedMessages := loadStoredPromptMessages(t, created)
	if got, want := len(storedMessages), 2; got != want {
		t.Fatalf("len(storedMessages) = %d, want %d", got, want)
	}
	if !strings.Contains(storedMessages[0], `"text":"workspace note"`) {
		t.Fatalf("stored user message = %q, want canonical user input", storedMessages[0])
	}
	if strings.Contains(storedMessages[0], "Relevant durable memory for this turn:") ||
		strings.Contains(storedMessages[0], "SUFFIX CONTEXT") {
		t.Fatalf("stored user message = %q, want no augmenter content", storedMessages[0])
	}
	if !strings.Contains(storedMessages[1], `"text":"network note"`) {
		t.Fatalf("stored network message = %q, want canonical network input", storedMessages[1])
	}
	if strings.Contains(storedMessages[1], "SUFFIX CONTEXT") {
		t.Fatalf("stored network message = %q, want no augmenter content", storedMessages[1])
	}
}

func TestPromptInputCompositeIntegrationRefreshesWorkspaceKnowledgeOnSyntheticWake(t *testing.T) {
	t.Run("Should deliver current confined knowledge bytes on every synthetic wake", func(t *testing.T) {
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		workspaceRoot := filepath.Join(homePaths.HomeDir, "workspace")
		resolvedWorkspace := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)
		knowledgePath := filepath.Join(workspaceRoot, "knowledge", "workspace", "bench-harness-status.md")
		writePromptKnowledgeFile(t, knowledgePath, "baseline_ms: 410\ncandidate_ms: 410\n")

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
			Name:      "knowledge-worker",
			Workspace: resolvedWorkspace.ID,
			Type:      session.SessionTypeSystem,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		firstEvents, err := manager.PromptSynthetic(
			testutil.Context(t),
			created.ID,
			session.SyntheticPromptOpts{
				Message: "inspect the current benchmark",
				Metadata: acp.PromptSyntheticMeta{
					TaskRunID: "run-bench-1",
					Reason:    "task_run_ready",
				},
			},
		)
		if err != nil {
			t.Fatalf("PromptSynthetic(first) error = %v", err)
		}
		drainHarnessIntegrationEvents(firstEvents)
		if got := driver.promptCalls[0].Message; !strings.Contains(got, `candidate_ms: 410\n`) {
			t.Fatalf("first synthetic prompt = %q, want initial knowledge bytes", got)
		}

		writePromptKnowledgeFile(t, knowledgePath, "baseline_ms: 410\ncandidate_ms: 500\n")
		outsideSecretPath := filepath.Join(t.TempDir(), "outside-secret.md")
		writePromptKnowledgeFile(t, outsideSecretPath, "OUTSIDE_WORKSPACE_SECRET")
		if err := os.Symlink(
			outsideSecretPath,
			filepath.Join(filepath.Dir(knowledgePath), "escaped.md"),
		); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}

		secondEvents, err := manager.PromptSynthetic(
			testutil.Context(t),
			created.ID,
			session.SyntheticPromptOpts{
				Message: "continue the benchmark review",
				Metadata: acp.PromptSyntheticMeta{
					TaskRunID: "run-bench-1",
					Reason:    "task_run_progress",
				},
			},
		)
		if err != nil {
			t.Fatalf("PromptSynthetic(second) error = %v", err)
		}
		drainHarnessIntegrationEvents(secondEvents)

		got := driver.promptCalls[1].Message
		if !strings.Contains(got, `candidate_ms: 500\n`) {
			t.Fatalf("second synthetic prompt = %q, want refreshed knowledge bytes", got)
		}
		if strings.Contains(got, `candidate_ms: 410\n`) {
			t.Fatalf("second synthetic prompt = %q, want no stale knowledge bytes", got)
		}
		if strings.Contains(got, "OUTSIDE_WORKSPACE_SECRET") {
			t.Fatalf("second synthetic prompt = %q, want symlink target excluded", got)
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
