//go:build integration

package daemon

import (
	"context"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/session"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/testutil"
)

func TestPromptInputCompositeIntegrationPreservesStoredMessagesAcrossUserAndNetworkTurns(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	workspaceRoot := homePaths.HomeDir + "/workspace"
	resolvedWorkspace := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)
	writeDaemonMemoryIndex(t, cfg.Memory.GlobalDir, workspaceRoot)

	daemonInstance, capturedDeps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
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
				memory.NewRecallAugmenter(daemonInstance.memoryStore),
				newSkillsCatalogAugmenter(daemonInstance.skillsRegistry, func() promptSkillsWorkspaceResolver {
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
		_ = manager.Stop(testutil.Context(t), created.ID)
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

	syntheticEvents, err := manager.PromptSynthetic(testutil.Context(t), created.ID, session.SyntheticPromptOpts{
		Message: "daemon wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-1",
			Reason:    "task_run_completed",
			Summary:   "background work finished",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}
	drainHarnessIntegrationEvents(syntheticEvents)

	if got := driver.promptCalls[2].Message; got != "daemon wake-up" {
		t.Fatalf("synthetic prompt message = %q, want canonical synthetic dispatch", got)
	}
	if got := driver.promptCalls[2].Meta.TurnSource; got != acp.PromptTurnSourceSynthetic {
		t.Fatalf("synthetic prompt turn source = %q, want %q", got, acp.PromptTurnSourceSynthetic)
	}
	if got, want := len(compositeResolver.seenMeta), 2; got != want {
		t.Fatalf("len(resolver seen meta) after synthetic prompt = %d, want %d", got, want)
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

	db, err := sessiondb.OpenSessionDB(testutil.Context(t), sess.ID, sess.DBPath())
	if err != nil {
		t.Fatalf("OpenSessionDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
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
