package situation

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestRenderPromptPreservesSectionOrderAndOmitsUnavailableSections(t *testing.T) {
	t.Parallel()

	payload := contract.AgentContextPayload{
		Self: contract.AgentIdentityPayload{
			SessionID: "sess-1",
			AgentName: "coder",
			Provider:  "codex",
		},
		Workspace: contract.AgentWorkspacePayload{
			ID:      "ws-1",
			Name:    "agh",
			RootDir: "/work/agh",
		},
		Session: contract.AgentSessionPayload{
			ID:        "sess-1",
			Type:      session.SessionTypeUser,
			State:     session.StateActive,
			CreatedAt: fixedTime(),
			UpdatedAt: fixedTime(),
		},
		Capabilities: contract.AgentCapabilitySectionPayload{
			Section: contract.AgentContextSectionMetaPayload{Limit: 2},
		},
		Limits: contract.AgentLimitsPayload{
			MaxChildren:         5,
			MaxSpawnDepth:       1,
			MaxActiveTaskLeases: 1,
			ContextSectionLimit: 2,
		},
		Provenance: contract.AgentContextProvenancePayload{
			GeneratedAt: fixedTime(),
			Source:      ProvenanceSource,
		},
	}

	rendered, err := RenderPrompt(&payload)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}

	wantOrder := []string{
		`"self"`,
		`"workspace"`,
		`"session"`,
		`"capabilities"`,
		"\n  \"limits\"",
		`"provenance"`,
	}
	assertOrder(t, rendered, wantOrder)
	for _, omitted := range []string{`"task"`, `"coordination_channel"`, `"inbox_summary"`, `"peer_roster"`} {
		if strings.Contains(rendered, omitted) {
			t.Fatalf("RenderPrompt() included unavailable section %s: %s", omitted, rendered)
		}
	}
}

func TestRenderPromptProvenanceCacheStability(t *testing.T) {
	t.Parallel()

	t.Run("Should omit generated_at from prompt while preserving context payload provenance", func(t *testing.T) {
		t.Parallel()

		service := NewService(Deps{Now: fixedNow})
		payload, err := service.ContextForSession(context.Background(), &session.Info{
			ID:           "sess-1",
			AgentName:    "coder",
			Provider:     "codex",
			Type:         session.SessionTypeUser,
			State:        session.StateActive,
			ACPSessionID: "acp-1",
			CreatedAt:    fixedTime(),
			UpdatedAt:    fixedTime(),
		})
		if err != nil {
			t.Fatalf("ContextForSession() error = %v", err)
		}
		if got, want := payload.Provenance.GeneratedAt, fixedTime(); !got.Equal(want) {
			t.Fatalf("Provenance.GeneratedAt = %s, want %s", got, want)
		}
		if got, want := payload.Provenance.Source, ProvenanceSource; got != want {
			t.Fatalf("Provenance.Source = %q, want %q", got, want)
		}

		rendered, err := RenderPrompt(&payload)
		if err != nil {
			t.Fatalf("RenderPrompt() error = %v", err)
		}
		if strings.Contains(rendered, "generated_at") {
			t.Fatalf("RenderPrompt() = %s, want no generated_at", rendered)
		}
		if !strings.Contains(rendered, `"source":"daemon.situation"`) {
			t.Fatalf("RenderPrompt() = %s, want provenance source", rendered)
		}

		payload.Provenance.GeneratedAt = fixedTime().Add(time.Hour)
		renderedLater, err := RenderPrompt(&payload)
		if err != nil {
			t.Fatalf("RenderPrompt(later generated_at) error = %v", err)
		}
		if renderedLater != rendered {
			t.Fatalf(
				"RenderPrompt() changed after generated_at-only change\nbefore: %s\nafter:  %s",
				rendered,
				renderedLater,
			)
		}
	})
}

func TestContextForSessionBoundsListsAndIncludesTaskParticipationProvenance(t *testing.T) {
	t.Parallel()
	liveSpec := situationLiveSpec(t, "ws-1", "coord-structured")

	taskRecord := taskpkg.Task{
		ID:          "task-1",
		Identifier:  "AUTO-1",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "ws-1",
		Title:       "Implement context",
		Status:      taskpkg.TaskStatusInProgress,
		Priority:    taskpkg.PriorityHigh,
		Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindAgentSession, Ref: "sess-1"},
		UpdatedAt:   fixedTime().Add(time.Minute),
	}
	run := taskpkg.Run{
		ID:              "run-1",
		TaskID:          "task-1",
		Status:          taskpkg.TaskRunStatusRunning,
		Attempt:         1,
		ClaimedBy:       &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-1"},
		SessionID:       "sess-1",
		RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: liveSpec},
		Metadata:        jsonRaw(t, `{"workflow_id":"wf-1"}`),
		QueuedAt:        fixedTime(),
		StartedAt:       fixedTime().Add(time.Minute),
	}
	displayName := "Reviewer"
	service := NewService(Deps{
		Now:          fixedNow,
		SectionLimit: 2,
		WorkspaceResolver: workspaceResolverFunc(func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
			return workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{ID: "ws-1", Name: "AGH", RootDir: "/work/agh"},
				Config:    aghconfig.Config{Defaults: aghconfig.DefaultsConfig{Provider: "codex"}},
			}, nil
		}),
		AgentResolver: agentResolverFunc(func(string, *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error) {
			return aghconfig.AgentDef{
				Name:     "coder",
				Provider: "codex",
				Model:    "gpt-test",
				Capabilities: &aghconfig.CapabilityCatalog{Capabilities: []aghconfig.CapabilityDef{
					{ID: "build", Summary: "Build code"},
					{ID: "review", Summary: "Review code"},
				}},
			}, nil
		}),
		SkillRegistry: skillRegistryFunc(
			func(context.Context, *workspacepkg.ResolvedWorkspace) ([]*skillspkg.Skill, error) {
				return []*skillspkg.Skill{
					{Meta: skillspkg.SkillMeta{Name: "alpha", Description: "Alpha skill"}, Enabled: true},
					{Meta: skillspkg.SkillMeta{Name: "beta", Description: "Beta skill"}, Enabled: true},
				}, nil
			},
		),
		TaskStore: taskStoreStub{
			tasks: map[string]taskpkg.Task{"task-1": taskRecord},
			runs:  []taskpkg.Run{run},
		},
		Network: networkStub{
			envelopes: []network.Envelope{
				coordinationEnvelope(t, "msg-3", "coord-structured", "third", fixedTime().Add(3*time.Minute)),
				coordinationEnvelope(t, "msg-2", "coord-structured", "second", fixedTime().Add(2*time.Minute)),
				coordinationEnvelope(t, "msg-1", "coord-structured", "first", fixedTime().Add(time.Minute)),
			},
			peers: []network.PeerInfo{
				{
					PeerID:  "peer-c",
					Channel: "coord-structured",
					PeerCard: network.PeerCard{
						PeerID:       "peer-c",
						Capabilities: []string{"test"},
					},
				},
				{
					PeerID:  "peer-a",
					Channel: "coord-structured",
					PeerCard: network.PeerCard{
						PeerID:       "peer-a",
						DisplayName:  &displayName,
						Capabilities: []string{"review"},
					},
				},
				{
					PeerID:  "peer-b",
					Channel: "coord-structured",
					PeerCard: network.PeerCard{
						PeerID:       "peer-b",
						Capabilities: []string{"build"},
					},
				},
			},
		},
		CoordinatorRole: coordinatorResolverFunc(
			func(context.Context, string) (aghconfig.ResolvedCoordinatorRole, error) {
				return aghconfig.ResolvedCoordinatorRole{MaxChildren: 3}, nil
			},
		),
	})

	payload, err := service.ContextForSession(context.Background(), &session.Info{
		ID:          "sess-1",
		Name:        "Coding Session",
		AgentName:   "coder",
		Provider:    "codex",
		WorkspaceID: "ws-1",
		Workspace:   "/work/agh",
		Type:        session.SessionTypeUser,
		Lineage: &store.SessionLineage{
			ParentSessionID: "sess-parent",
			RootSessionID:   "sess-root",
			SpawnDepth:      1,
			SpawnRole:       "worker",
		},
		State:                session.StateActive,
		NetworkParticipation: liveSpec,
		CreatedAt:            fixedTime(),
		UpdatedAt:            fixedTime(),
	})
	if err != nil {
		t.Fatalf("ContextForSession() error = %v", err)
	}

	if got, want := payload.Self.Model, "gpt-test"; got != want {
		t.Fatalf("Self.Model = %q, want %q", got, want)
	}
	if payload.Session.Lineage == nil ||
		payload.Session.Lineage.ParentSessionID != "sess-parent" ||
		payload.Session.Lineage.RootSessionID != "sess-root" ||
		payload.Session.Lineage.SpawnDepth != 1 {
		t.Fatalf("Session.Lineage = %#v, want spawned lineage projection", payload.Session.Lineage)
	}
	if payload.Task.Task == nil || payload.Task.Task.ID != "task-1" {
		t.Fatalf("Task section = %#v, want task-1", payload.Task)
	}
	if payload.Task.Lease == nil || payload.Task.Lease.ResolvedNetworkParticipation == nil {
		t.Fatalf("Task lease = %#v, want complete resolved participation", payload.Task.Lease)
	}
	if got, want := *payload.Task.Lease.ResolvedNetworkParticipation, liveSpec; got != want {
		t.Fatalf("Task lease participation = %#v, want %#v", got, want)
	}
	if payload.Task.Bundle == nil ||
		payload.Task.Bundle.CurrentRun == nil ||
		payload.Task.Bundle.CurrentRun.ID != "run-1" {
		t.Fatalf("Task bundle = %#v, want current run context", payload.Task.Bundle)
	}
	if !payload.CoordinationChannel.Available ||
		payload.CoordinationChannel.Channel == nil ||
		payload.CoordinationChannel.Channel.ID != "coord-structured" ||
		payload.CoordinationChannel.Channel.WorkflowID != "wf-1" {
		t.Fatalf("CoordinationChannel = %#v, want workflow-bound channel", payload.CoordinationChannel)
	}
	if got := payload.InboxSummary.Section; got.Limit != 2 || got.Returned != 2 || !got.Truncated {
		t.Fatalf("Inbox section = %#v, want truncated limit 2", got)
	}
	if got, want := payload.InboxSummary.UnreadCount, 3; got != want {
		t.Fatalf("UnreadCount = %d, want %d", got, want)
	}
	if got := payload.PeerRoster.Section; got.Limit != 2 || got.Returned != 2 || !got.Truncated {
		t.Fatalf("Peer section = %#v, want truncated limit 2", got)
	}
	if got := payload.Capabilities.Section; got.Limit != 2 || got.Returned != 2 || !got.Truncated {
		t.Fatalf("Capability section = %#v, want truncated limit 2", got)
	}
	if got, want := payload.Limits.MaxChildren, 3; got != want {
		t.Fatalf("Limits.MaxChildren = %d, want %d", got, want)
	}
	if got, want := payload.Provenance.Source, ProvenanceSource; got != want {
		t.Fatalf("Provenance.Source = %q, want %q", got, want)
	}

	rendered, err := RenderPrompt(&payload)
	if err != nil {
		t.Fatalf("RenderPrompt(context) error = %v", err)
	}
	assertOrder(t, rendered, []string{
		`"self"`,
		`"workspace"`,
		`"session"`,
		`"task"`,
		`"coordination_channel"`,
		`"inbox_summary"`,
		`"peer_roster"`,
		`"capabilities"`,
		"\n  \"limits\"",
		`"provenance"`,
	})
	if strings.Contains(rendered, "claim_token") {
		t.Fatalf("RenderPrompt() leaked raw claim token field: %s", rendered)
	}
}

func TestContextBundleRedactsReviewContinuationAndRawClaimTokens(t *testing.T) {
	t.Parallel()

	t.Run("Should redact raw claim tokens from continuation text and event payloads", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskpkg.Task{
			ID:           "task-redact",
			Identifier:   "ORCH-23",
			Scope:        taskpkg.ScopeWorkspace,
			WorkspaceID:  "ws-redact",
			Title:        "Continue after review",
			Status:       taskpkg.TaskStatusInProgress,
			Priority:     taskpkg.PriorityHigh,
			MaxAttempts:  3,
			CurrentRunID: "run-cont",
		}
		review := taskpkg.RunReview{
			ReviewID:          "review-1",
			TaskID:            taskRecord.ID,
			RunID:             "run-reviewed",
			ReviewRound:       1,
			Attempt:           1,
			Status:            taskpkg.RunReviewStatusRecorded,
			Outcome:           taskpkg.RunReviewOutcomeRejected,
			Reason:            "Reviewer found agh_claim_REVIEW_SECRET in the output",
			MissingWork:       jsonRaw(t, `["remove agh_claim_MISSING_SECRET from docs","add regression coverage"]`),
			NextRoundGuidance: "Continue without agh_claim_GUIDANCE_SECRET",
			ReviewerAgentName: "reviewer",
			ReviewerSessionID: "sess-reviewer",
			ReviewedAt:        fixedTime().Add(10 * time.Minute),
		}
		currentRun := taskpkg.Run{
			ID:        "run-cont",
			TaskID:    taskRecord.ID,
			Status:    taskpkg.TaskRunStatusRunning,
			Attempt:   2,
			SessionID: "sess-worker",
			Review: &taskpkg.RunReviewLineage{
				ParentRunID:        review.RunID,
				ReviewID:           review.ReviewID,
				ReviewRound:        2,
				ContinuationReason: "review_rejected",
				MissingWork:        review.MissingWork,
				NextRoundGuidance:  review.NextRoundGuidance,
			},
			QueuedAt:  fixedTime().Add(20 * time.Minute),
			StartedAt: fixedTime().Add(21 * time.Minute),
			Error:     "do not leak agh_claim_RUN_SECRET",
		}
		priorRun := taskpkg.Run{
			ID:        review.RunID,
			TaskID:    taskRecord.ID,
			Status:    taskpkg.TaskRunStatusCompleted,
			Attempt:   1,
			SessionID: "sess-original",
			QueuedAt:  fixedTime(),
			StartedAt: fixedTime().Add(time.Minute),
			EndedAt:   fixedTime().Add(5 * time.Minute),
			Error:     "prior error had agh_claim_PRIOR_SECRET",
		}
		service := NewService(Deps{
			Now: fixedNow,
			WorkspaceResolver: workspaceResolverFunc(
				func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-redact", Name: "AGH", RootDir: "/work/agh"},
						Config:    workspaceConfigWithTaskDefaults(),
					}, nil
				},
			),
			TaskStore: taskStoreStub{
				tasks: map[string]taskpkg.Task{taskRecord.ID: taskRecord},
				runs:  []taskpkg.Run{priorRun, currentRun},
				events: []taskpkg.Event{
					{
						ID:        "evt-1",
						TaskID:    taskRecord.ID,
						RunID:     currentRun.ID,
						EventType: "task.run.continued",
						Payload: jsonRaw(
							t,
							`{"message":"saw agh_claim_EVENT_SECRET","claim_token":"raw","mcp_auth_token":"mcp-secret","oauth_code":"oauth-secret","pkce_verifier":"pkce-secret","nested":{"token":"agh_claim_NESTED_SECRET","secret_binding":"vault-ref","session_secret":"super-secret"}}`,
						),
						Timestamp: fixedTime().Add(22 * time.Minute),
					},
				},
				reviews: map[string]taskpkg.RunReview{review.ReviewID: review},
			},
		})

		bundle, err := service.BundleForActiveLease(context.Background(), taskpkg.ContextRequest{
			SessionID: currentRun.SessionID,
			RunID:     currentRun.ID,
			Now:       fixedTime().Add(30 * time.Minute),
		})
		if err != nil {
			t.Fatalf("BundleForActiveLease() error = %v", err)
		}
		if bundle.ReviewContinuation == nil {
			t.Fatalf("ReviewContinuation = nil, want rejected-review continuation")
		}
		if bundle.LatestEventSeq != 1 || bundle.Task.LatestEventSeq != 1 {
			t.Fatalf("LatestEventSeq bundle=%d task=%d, want 1", bundle.LatestEventSeq, bundle.Task.LatestEventSeq)
		}
		if len(bundle.RecentEvents) != 1 || bundle.RecentEvents[0].Sequence != 1 {
			t.Fatalf("RecentEvents = %#v, want one sequenced event", bundle.RecentEvents)
		}
		if got := bundle.ReviewContinuation.Reason; strings.Contains(got, "REVIEW_SECRET") ||
			!strings.Contains(got, "agh_claim_[REDACTED]") {
			t.Fatalf("ReviewContinuation.Reason = %q, want redacted reviewer reason", got)
		}
		encoded, err := json.Marshal(bundle)
		if err != nil {
			t.Fatalf("json.Marshal(bundle) error = %v", err)
		}
		encodedText := string(encoded)
		for _, leaked := range []string{
			"REVIEW_SECRET",
			"MISSING_SECRET",
			"GUIDANCE_SECRET",
			"RUN_SECRET",
			"PRIOR_SECRET",
			"EVENT_SECRET",
			"NESTED_SECRET",
			"mcp-secret",
			"oauth-secret",
			"pkce-secret",
			"vault-ref",
			"super-secret",
			`"claim_token"`,
			`"mcp_auth_token"`,
			`"oauth_code"`,
			`"pkce_verifier"`,
			`"secret_binding"`,
			`"session_secret"`,
		} {
			if strings.Contains(encodedText, leaked) {
				t.Fatalf("ContextBundle leaked %q: %s", leaked, encodedText)
			}
		}
		if !strings.Contains(encodedText, "agh_claim_[REDACTED]") {
			t.Fatalf("ContextBundle = %s, want redaction marker", encodedText)
		}
	})
}

func TestTaskStoreStubListRunReviewsSortsBeforeApplyingLimit(t *testing.T) {
	t.Parallel()

	t.Run("Should sort reviews before applying the limit", func(t *testing.T) {
		t.Parallel()

		store := taskStoreStub{
			reviews: map[string]taskpkg.RunReview{
				"review-older": {
					ReviewID:  "review-older",
					TaskID:    "task-1",
					Status:    taskpkg.RunReviewStatusInReview,
					UpdatedAt: fixedTime().Add(time.Minute),
				},
				"review-newer": {
					ReviewID:  "review-newer",
					TaskID:    "task-1",
					Status:    taskpkg.RunReviewStatusInReview,
					UpdatedAt: fixedTime().Add(2 * time.Minute),
				},
			},
		}

		reviews, err := store.ListRunReviews(context.Background(), taskpkg.RunReviewQuery{
			TaskID: "task-1",
			Limit:  1,
		})
		if err != nil {
			t.Fatalf("ListRunReviews() error = %v", err)
		}
		if got, want := len(reviews), 1; got != want {
			t.Fatalf("len(reviews) = %d, want %d", got, want)
		}
		if got, want := reviews[0].ReviewID, "review-newer"; got != want {
			t.Fatalf("reviews[0].ReviewID = %q, want %q", got, want)
		}
	})
}

func TestTaskRunPromptOverlayByIDRejectsMismatchedRunTaskPair(t *testing.T) {
	t.Parallel()

	t.Run("Should reject run ids that belong to another task", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskpkg.Task{
			ID:          "task-overlay",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-overlay",
			Title:       "Overlay task",
			Status:      taskpkg.TaskStatusInProgress,
		}
		mismatchedRun := taskpkg.Run{
			ID:         "run-other-task",
			TaskID:     "task-other",
			Status:     taskpkg.TaskRunStatusRunning,
			SessionID:  "sess-overlay",
			QueuedAt:   fixedTime(),
			ClaimedAt:  fixedTime(),
			StartedAt:  fixedTime(),
			LeaseUntil: fixedTime().Add(time.Minute),
		}
		cfg := workspaceConfigWithTaskDefaults()
		service := NewService(Deps{
			Now: fixedNow,
			WorkspaceResolver: workspaceResolverFunc(
				func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-overlay", Name: "AGH", RootDir: "/work/agh"},
						Config:    cfg,
					}, nil
				},
			),
			TaskStore: taskStoreStub{
				tasks: map[string]taskpkg.Task{taskRecord.ID: taskRecord},
				runs:  []taskpkg.Run{mismatchedRun},
			},
		})

		_, err := service.TaskRunPromptOverlayByID(context.Background(), taskRecord.ID, mismatchedRun.ID)
		if !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("TaskRunPromptOverlayByID() error = %v, want %v", err, taskpkg.ErrValidation)
		}
		if err == nil || !strings.Contains(err.Error(), "belongs to task") {
			t.Fatalf("TaskRunPromptOverlayByID() error = %v, want mismatched task detail", err)
		}
	})
}

func TestBundleForOperatorTaskRejectsOversizedUntrimmableBundle(t *testing.T) {
	t.Parallel()

	taskRecord := taskpkg.Task{
		ID:          "task-oversized-bundle",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "ws-oversized-bundle",
		Title:       strings.Repeat("x", 256),
		Status:      taskpkg.TaskStatusReady,
	}
	cfg := workspaceConfigWithTaskDefaults()
	cfg.Task.Orchestration.ContextBodyMaxBytes = 64
	service := NewService(Deps{
		Now: fixedNow,
		WorkspaceResolver: workspaceResolverFunc(
			func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      taskRecord.WorkspaceID,
						Name:    "AGH",
						RootDir: "/work/agh",
					},
					Config: cfg,
				}, nil
			},
		),
		TaskStore: taskStoreStub{
			tasks: map[string]taskpkg.Task{taskRecord.ID: taskRecord},
		},
	})

	_, err := service.BundleForOperatorTask(context.Background(), taskpkg.OperatorTaskContextRequest{
		TaskID: taskRecord.ID,
		Now:    fixedTime(),
	})
	if !errors.Is(err, taskpkg.ErrPayloadTooLarge) {
		t.Fatalf("BundleForOperatorTask() error = %v, want %v", err, taskpkg.ErrPayloadTooLarge)
	}
	if err == nil || !strings.Contains(err.Error(), "exceeds 64 bytes after trimming") {
		t.Fatalf("BundleForOperatorTask() error = %v, want oversized bundle detail", err)
	}
}

func TestContextForSessionIncludesReviewerTaskBundleWithoutActiveLease(t *testing.T) {
	t.Parallel()

	t.Run("Should expose review-bound task context without a worker lease", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskpkg.Task{
			ID:          "task-review",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-review",
			Title:       "Review context",
			Status:      taskpkg.TaskStatusInProgress,
		}
		run := taskpkg.Run{
			ID:              "run-reviewed",
			TaskID:          taskRecord.ID,
			Status:          taskpkg.TaskRunStatusCompleted,
			SessionID:       "sess-worker",
			RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: situationLiveSpec(t, "ws-review", "reviews")},
			StartedAt:       fixedTime(),
			EndedAt:         fixedTime().Add(10 * time.Minute),
		}
		review := taskpkg.RunReview{
			ReviewID:          "review-bound",
			TaskID:            taskRecord.ID,
			RunID:             run.ID,
			Status:            taskpkg.RunReviewStatusInReview,
			Policy:            taskpkg.ReviewPolicyAlways,
			ReviewRound:       1,
			Attempt:           1,
			ReviewerSessionID: "sess-reviewer",
			ReviewerAgentName: "reviewer",
			ReviewerChannelID: "reviews",
		}
		service := NewService(Deps{
			Now: fixedNow,
			WorkspaceResolver: workspaceResolverFunc(
				func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-review", Name: "AGH", RootDir: "/work/agh"},
						Config:    workspaceConfigWithTaskDefaults(),
					}, nil
				},
			),
			AgentResolver: agentResolverFunc(func(string, *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error) {
				return aghconfig.AgentDef{Name: "reviewer", Provider: "codex", Model: "gpt-review"}, nil
			}),
			TaskStore: taskStoreStub{
				tasks:   map[string]taskpkg.Task{taskRecord.ID: taskRecord},
				runs:    []taskpkg.Run{run},
				reviews: map[string]taskpkg.RunReview{review.ReviewID: review},
			},
		})

		payload, err := service.ContextForSession(context.Background(), &session.Info{
			ID:                   "sess-reviewer",
			AgentName:            "reviewer",
			Provider:             "codex",
			WorkspaceID:          "ws-review",
			Workspace:            "/work/agh",
			NetworkParticipation: situationLiveSpec(t, "ws-review", "reviews"),
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			CreatedAt:            fixedTime(),
			UpdatedAt:            fixedTime(),
		})
		if err != nil {
			t.Fatalf("ContextForSession(reviewer) error = %v", err)
		}
		if !payload.Task.Available || payload.Task.Task == nil || payload.Task.Task.ID != taskRecord.ID {
			t.Fatalf("Task context = %#v, want review-bound task", payload.Task)
		}
		if payload.Task.Lease != nil {
			t.Fatalf("Task lease = %#v, want no worker lease for reviewer binding", payload.Task.Lease)
		}
		if payload.Task.Bundle == nil ||
			payload.Task.Bundle.CurrentRun == nil ||
			payload.Task.Bundle.CurrentRun.ID != run.ID {
			t.Fatalf("Task bundle = %#v, want reviewed run context", payload.Task.Bundle)
		}
		if !payload.CoordinationChannel.Available ||
			payload.CoordinationChannel.Channel == nil ||
			payload.CoordinationChannel.Channel.ID != "reviews" {
			t.Fatalf("CoordinationChannel = %#v, want reviewer channel", payload.CoordinationChannel)
		}
	})

	t.Run("Should skip review-bound context when the stored run belongs to another task", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskpkg.Task{
			ID:          "task-review",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-review",
			Title:       "Review context",
			Status:      taskpkg.TaskStatusInProgress,
		}
		run := taskpkg.Run{
			ID:              "run-mismatched",
			TaskID:          "task-other",
			Status:          taskpkg.TaskRunStatusCompleted,
			SessionID:       "sess-worker",
			RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: situationLiveSpec(t, "ws-review", "reviews")},
			StartedAt:       fixedTime(),
			EndedAt:         fixedTime().Add(10 * time.Minute),
		}
		review := taskpkg.RunReview{
			ReviewID:          "review-bound",
			TaskID:            taskRecord.ID,
			RunID:             run.ID,
			Status:            taskpkg.RunReviewStatusInReview,
			Policy:            taskpkg.ReviewPolicyAlways,
			ReviewRound:       1,
			Attempt:           1,
			ReviewerSessionID: "sess-reviewer",
			ReviewerAgentName: "reviewer",
			ReviewerChannelID: "reviews",
		}
		service := NewService(Deps{
			Now: fixedNow,
			WorkspaceResolver: workspaceResolverFunc(
				func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-review", Name: "AGH", RootDir: "/work/agh"},
						Config:    workspaceConfigWithTaskDefaults(),
					}, nil
				},
			),
			AgentResolver: agentResolverFunc(func(string, *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error) {
				return aghconfig.AgentDef{Name: "reviewer", Provider: "codex", Model: "gpt-review"}, nil
			}),
			TaskStore: taskStoreStub{
				tasks:   map[string]taskpkg.Task{taskRecord.ID: taskRecord},
				runs:    []taskpkg.Run{run},
				reviews: map[string]taskpkg.RunReview{review.ReviewID: review},
			},
		})

		payload, err := service.ContextForSession(context.Background(), &session.Info{
			ID:                   "sess-reviewer",
			AgentName:            "reviewer",
			Provider:             "codex",
			WorkspaceID:          "ws-review",
			Workspace:            "/work/agh",
			NetworkParticipation: situationLiveSpec(t, "ws-review", "reviews"),
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			CreatedAt:            fixedTime(),
			UpdatedAt:            fixedTime(),
		})
		if err != nil {
			t.Fatalf("ContextForSession(reviewer mismatch) error = %v", err)
		}
		if payload.Task.Available || payload.Task.Task != nil || payload.Task.Bundle != nil ||
			payload.Task.Lease != nil {
			t.Fatalf("Task context = %#v, want no mismatched review-bound task context", payload.Task)
		}
		if payload.CoordinationChannel.Available || payload.CoordinationChannel.Channel != nil {
			t.Fatalf(
				"CoordinationChannel = %#v, want no mismatched review channel context",
				payload.CoordinationChannel,
			)
		}
	})
}

func TestContextForSessionKeepsTaskChannelContextWhenBundleEnrichmentFails(t *testing.T) {
	t.Parallel()

	t.Run("Should keep active-lease task context when bundle enrichment fails", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskpkg.Task{
			ID:          "task-bundle-active",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-bundle-active",
			Title:       "Active lease context",
			Status:      taskpkg.TaskStatusInProgress,
		}
		run := taskpkg.Run{
			ID:        "run-bundle-active",
			TaskID:    taskRecord.ID,
			Status:    taskpkg.TaskRunStatusRunning,
			SessionID: "sess-active",
			RunNetworkState: &taskpkg.RunNetworkState{
				NetworkSpec: situationLiveSpec(t, "ws-bundle-active", "coord-active"),
			},
			QueuedAt:  fixedTime(),
			StartedAt: fixedTime().Add(time.Minute),
		}
		service := NewService(Deps{
			Now: fixedNow,
			WorkspaceResolver: workspaceResolverFunc(
				func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{
							ID:      taskRecord.WorkspaceID,
							Name:    "AGH",
							RootDir: "/work/agh",
						},
						Config: workspaceConfigWithTaskDefaults(),
					}, nil
				},
			),
			TaskStore: taskStoreStub{
				tasks:                   map[string]taskpkg.Task{taskRecord.ID: taskRecord},
				runs:                    []taskpkg.Run{run},
				listTaskEventRecordsErr: errors.New("historical event projection broke"),
			},
		})

		payload, err := service.ContextForSession(context.Background(), &session.Info{
			ID:                   "sess-active",
			AgentName:            "coder",
			Provider:             "codex",
			WorkspaceID:          taskRecord.WorkspaceID,
			Workspace:            "/work/agh",
			NetworkParticipation: situationLiveSpec(t, "ws-bundle-active", "coord-active"),
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			CreatedAt:            fixedTime(),
			UpdatedAt:            fixedTime(),
		})
		if err != nil {
			t.Fatalf("ContextForSession(active lease) error = %v", err)
		}
		if !payload.Task.Available || payload.Task.Task == nil || payload.Task.Task.ID != taskRecord.ID {
			t.Fatalf("Task context = %#v, want active task context", payload.Task)
		}
		if payload.Task.Bundle != nil {
			t.Fatalf("Task.Bundle = %#v, want nil optional bundle on enrichment failure", payload.Task.Bundle)
		}
		if payload.Task.Lease == nil || payload.Task.Lease.RunID != run.ID {
			t.Fatalf("Task.Lease = %#v, want active run lease", payload.Task.Lease)
		}
		if !payload.CoordinationChannel.Available ||
			payload.CoordinationChannel.Channel == nil ||
			payload.CoordinationChannel.Channel.ID != "coord-active" {
			t.Fatalf("CoordinationChannel = %#v, want preserved active channel", payload.CoordinationChannel)
		}
	})

	t.Run("Should keep review-bound task context when bundle enrichment fails", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskpkg.Task{
			ID:          "task-bundle-review",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-bundle-review",
			Title:       "Review context",
			Status:      taskpkg.TaskStatusInProgress,
		}
		run := taskpkg.Run{
			ID:        "run-bundle-review",
			TaskID:    taskRecord.ID,
			Status:    taskpkg.TaskRunStatusCompleted,
			SessionID: "sess-worker",
			RunNetworkState: &taskpkg.RunNetworkState{
				NetworkSpec: situationLiveSpec(t, "ws-bundle-review", "coord-run"),
			},
			StartedAt: fixedTime(),
			EndedAt:   fixedTime().Add(10 * time.Minute),
		}
		review := taskpkg.RunReview{
			ReviewID:          "review-bundle",
			TaskID:            taskRecord.ID,
			RunID:             run.ID,
			Status:            taskpkg.RunReviewStatusInReview,
			ReviewerSessionID: "sess-reviewer",
			ReviewerAgentName: "reviewer",
			ReviewerChannelID: "coord-review",
		}
		service := NewService(Deps{
			Now: fixedNow,
			WorkspaceResolver: workspaceResolverFunc(
				func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{
							ID:      taskRecord.WorkspaceID,
							Name:    "AGH",
							RootDir: "/work/agh",
						},
						Config: workspaceConfigWithTaskDefaults(),
					}, nil
				},
			),
			TaskStore: taskStoreStub{
				tasks:                   map[string]taskpkg.Task{taskRecord.ID: taskRecord},
				runs:                    []taskpkg.Run{run},
				reviews:                 map[string]taskpkg.RunReview{review.ReviewID: review},
				listTaskEventRecordsErr: errors.New("historical event projection broke"),
			},
		})

		payload, err := service.ContextForSession(context.Background(), &session.Info{
			ID:                   "sess-reviewer",
			AgentName:            "reviewer",
			Provider:             "codex",
			WorkspaceID:          taskRecord.WorkspaceID,
			Workspace:            "/work/agh",
			NetworkParticipation: situationLiveSpec(t, "ws-bundle-review", "coord-review"),
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			CreatedAt:            fixedTime(),
			UpdatedAt:            fixedTime(),
		})
		if err != nil {
			t.Fatalf("ContextForSession(review binding) error = %v", err)
		}
		if !payload.Task.Available || payload.Task.Task == nil || payload.Task.Task.ID != taskRecord.ID {
			t.Fatalf("Task context = %#v, want review-bound task context", payload.Task)
		}
		if payload.Task.Bundle != nil {
			t.Fatalf("Task.Bundle = %#v, want nil optional bundle on enrichment failure", payload.Task.Bundle)
		}
		if !payload.CoordinationChannel.Available ||
			payload.CoordinationChannel.Channel == nil ||
			payload.CoordinationChannel.Channel.ID != "coord-review" {
			t.Fatalf("CoordinationChannel = %#v, want preserved reviewer channel", payload.CoordinationChannel)
		}
	})
}

func TestContextForSessionIncludesCompactSoulProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should expose compact soul before task without full body", func(t *testing.T) {
		t.Parallel()

		snapshot := testSituationSoulSnapshot(t, "body-secret-marker")
		taskRecord := taskpkg.Task{
			ID:          "task-soul",
			Identifier:  "SOUL-1",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-1",
			Title:       "Use soul context",
			Status:      taskpkg.TaskStatusInProgress,
			Priority:    taskpkg.PriorityMedium,
			Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindAgentSession, Ref: "sess-1"},
			UpdatedAt:   fixedTime().Add(time.Minute),
		}
		run := taskpkg.Run{
			ID:        "run-soul",
			TaskID:    taskRecord.ID,
			Status:    taskpkg.TaskRunStatusRunning,
			SessionID: "sess-1",
			QueuedAt:  fixedTime(),
			StartedAt: fixedTime().Add(time.Minute),
		}
		service := NewService(Deps{
			Now:          fixedNow,
			SectionLimit: 3,
			WorkspaceResolver: workspaceResolverFunc(
				func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-1", Name: "AGH", RootDir: "/work/agh"},
						Config:    aghconfig.Config{Defaults: aghconfig.DefaultsConfig{Provider: "codex"}},
					}, nil
				},
			),
			AgentResolver: agentResolverFunc(func(string, *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error) {
				return aghconfig.AgentDef{Name: "coder", Provider: "codex", Model: "gpt-test"}, nil
			}),
			TaskStore: taskStoreStub{
				tasks: map[string]taskpkg.Task{taskRecord.ID: taskRecord},
				runs:  []taskpkg.Run{run},
			},
			SoulSnapshots: soulSnapshotStoreStub{
				snapshots: map[string]soul.Snapshot{snapshot.ID: snapshot},
			},
		})

		payload, err := service.ContextForSession(context.Background(), &session.Info{
			ID:             "sess-1",
			AgentName:      "coder",
			Provider:       "codex",
			WorkspaceID:    "ws-1",
			Workspace:      "/work/agh",
			Type:           session.SessionTypeUser,
			State:          session.StateActive,
			SoulSnapshotID: snapshot.ID,
			SoulDigest:     snapshot.Digest,
			CreatedAt:      fixedTime(),
			UpdatedAt:      fixedTime(),
		})
		if err != nil {
			t.Fatalf("ContextForSession() error = %v", err)
		}

		if !payload.Soul.Present || !payload.Soul.Active || !payload.Soul.Valid {
			t.Fatalf("Soul flags = %#v, want present active valid", payload.Soul)
		}
		if got, want := payload.Soul.SnapshotID, snapshot.ID; got != want {
			t.Fatalf("Soul.SnapshotID = %q, want %q", got, want)
		}
		if got, want := payload.Soul.Role, "Reviewer"; got != want {
			t.Fatalf("Soul.Role = %q, want %q", got, want)
		}
		if got, want := payload.Soul.Tone, []string{"direct"}; !slices.Equal(got, want) {
			t.Fatalf("Soul.Tone = %#v, want %#v", got, want)
		}
		if got, want := payload.Soul.Principles, []string{"protect correctness"}; !slices.Equal(got, want) {
			t.Fatalf("Soul.Principles = %#v, want %#v", got, want)
		}
		encodedSoul, err := json.Marshal(payload.Soul)
		if err != nil {
			t.Fatalf("json.Marshal(Soul) error = %v", err)
		}
		if strings.Contains(string(encodedSoul), "body-secret-marker") ||
			strings.Contains(string(encodedSoul), `"body":`) {
			t.Fatalf("Soul compact payload leaked full body data: %s", encodedSoul)
		}

		rendered, err := RenderPrompt(&payload)
		if err != nil {
			t.Fatalf("RenderPrompt(context) error = %v", err)
		}
		assertOrder(t, rendered, []string{
			`"self"`,
			`"workspace"`,
			`"session"`,
			`"soul"`,
			`"task"`,
			`"capabilities"`,
			"\n  \"limits\"",
			`"provenance"`,
		})
		if strings.Contains(rendered, "body-secret-marker") || strings.Contains(rendered, `"body":`) {
			t.Fatalf("RenderPrompt() leaked full soul body: %s", rendered)
		}
	})
}

func TestContextForSessionMissingOptionalServicesOmitsUnavailableSections(t *testing.T) {
	t.Parallel()

	service := NewService(Deps{
		Now:          fixedNow,
		SectionLimit: 3,
	})

	payload, err := service.ContextForSession(context.Background(), &session.Info{
		ID:          "sess-1",
		AgentName:   "coder",
		Provider:    "codex",
		WorkspaceID: "ws-1",
		Workspace:   "/work/agh",
		Type:        session.SessionTypeUser,
		State:       session.StateActive,
		CreatedAt:   fixedTime(),
		UpdatedAt:   fixedTime(),
	})
	if err != nil {
		t.Fatalf("ContextForSession() error = %v", err)
	}
	if payload.Task.Available {
		t.Fatalf("Task.Available = true, want false without task store")
	}
	if payload.CoordinationChannel.Available {
		t.Fatalf("CoordinationChannel.Available = true, want false without task store")
	}
	if payload.InboxSummary.Section.Limit != 0 {
		t.Fatalf("Inbox section = %#v, want omitted without network", payload.InboxSummary.Section)
	}
	if payload.PeerRoster.Section.Limit != 0 {
		t.Fatalf("Peer section = %#v, want omitted without network", payload.PeerRoster.Section)
	}
	if got := payload.Session.ResolvedNetworkParticipation; got == nil || *got != participation.LocalSpec() {
		t.Fatalf("ResolvedNetworkParticipation = %#v, want canonical Local", got)
	}

	rendered, err := RenderPrompt(&payload)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	for _, omitted := range []string{`"task"`, `"coordination_channel"`, `"inbox_summary"`, `"peer_roster"`} {
		if strings.Contains(rendered, omitted) {
			t.Fatalf("RenderPrompt() included unavailable section %s: %s", omitted, rendered)
		}
	}
}

func TestPromptStartupSectionIncludesStartupIdentity(t *testing.T) {
	t.Parallel()

	service := NewService(Deps{Now: fixedNow, SectionLimit: 4})
	workspace := &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "ws-1", Name: "AGH", RootDir: "/work/agh"},
		Config:    aghconfig.Config{Defaults: aghconfig.DefaultsConfig{Provider: "codex"}},
	}

	rendered, err := service.PromptStartupSection(
		context.Background(),
		session.StartupPromptContext{
			SessionID:   "sess-start",
			SessionName: "Startup",
			AgentName:   "coder",
			Provider:    "codex",
			WorkspaceID: "ws-1",
			Workspace:   "/work/agh",
			SessionType: session.SessionTypeUser,
			CreatedAt:   fixedTime(),
			UpdatedAt:   fixedTime(),
		},
		aghconfig.AgentDef{Name: "coder", Provider: "codex", Model: "gpt-test"},
		workspace,
	)
	if err != nil {
		t.Fatalf("PromptStartupSection() error = %v", err)
	}
	for _, want := range []string{
		`"session_id":"sess-start"`,
		`"agent_name":"coder"`,
		`"model":"gpt-test"`,
		`"mode":"local"`,
		`"source":"built_in_local"`,
		`"workspace"`,
		`"provenance"`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("PromptStartupSection() = %s, want substring %q", rendered, want)
		}
	}
}

func TestAugmentPrefixesFreshSituationWithoutRewritingMessage(t *testing.T) {
	t.Parallel()

	service := NewService(Deps{Now: fixedNow, SectionLimit: 2})
	augmented, err := service.Augment(
		context.Background(),
		&session.Session{
			ID:        "sess-1",
			AgentName: "coder",
			Provider:  "codex",
			Type:      session.SessionTypeUser,
			State:     session.StateActive,
			CreatedAt: fixedTime(),
			UpdatedAt: fixedTime(),
		},
		"original prompt",
	)
	if err != nil {
		t.Fatalf("Augment() error = %v", err)
	}
	if !strings.HasPrefix(augmented, promptContextOpen) {
		t.Fatalf("Augment() = %q, want situation context prefix", augmented)
	}
	if !strings.HasSuffix(augmented, "original prompt") {
		t.Fatalf("Augment() = %q, want original message suffix", augmented)
	}
	if got := strings.Count(augmented, promptContextOpen); got != 1 {
		t.Fatalf("situation context occurrences = %d, want 1", got)
	}
}

func TestAugmentCompactsRepeatedSituationSections(t *testing.T) {
	t.Parallel()

	t.Run("Should compact unchanged sections for the same ACP session and workspace", func(t *testing.T) {
		t.Parallel()

		service := NewService(Deps{Now: fixedNow, SectionLimit: 2})
		sess := newSituationCompactionSession()
		first, err := service.Augment(context.Background(), sess, "first prompt")
		if err != nil {
			t.Fatalf("Augment(first) error = %v", err)
		}
		if strings.Contains(first, `"unchanged":true`) {
			t.Fatalf("Augment(first) = %s, want full sections", first)
		}
		if !strings.Contains(first, `"agent_name":"coder"`) {
			t.Fatalf("Augment(first) = %s, want full self section", first)
		}

		second, err := service.Augment(context.Background(), sess, "second prompt")
		if err != nil {
			t.Fatalf("Augment(second) error = %v", err)
		}
		for _, want := range []string{
			`"unchanged":true`,
			`"section":"self"`,
			`"reuse":"previous_full_section_same_acp_session"`,
		} {
			if !strings.Contains(second, want) {
				t.Fatalf("Augment(second) = %s, want %q", second, want)
			}
		}
		if strings.Contains(second, `"agent_name":"coder"`) {
			t.Fatalf("Augment(second) = %s, want compact self section", second)
		}
		if !strings.HasSuffix(second, "second prompt") {
			t.Fatalf("Augment(second) = %q, want second prompt suffix", second)
		}
	})

	t.Run("Should refresh changed sections while compacting stable peers", func(t *testing.T) {
		t.Parallel()

		service := NewService(Deps{Now: fixedNow, SectionLimit: 2})
		sess := newSituationCompactionSession()
		if _, err := service.Augment(context.Background(), sess, "first prompt"); err != nil {
			t.Fatalf("Augment(first) error = %v", err)
		}
		sess.UpdatedAt = fixedTime().Add(time.Minute)

		second, err := service.Augment(context.Background(), sess, "second prompt")
		if err != nil {
			t.Fatalf("Augment(second) error = %v", err)
		}
		if !strings.Contains(second, `"section":"self"`) {
			t.Fatalf("Augment(second) = %s, want stable self section compacted", second)
		}
		if strings.Contains(second, `"section":"session"`) {
			t.Fatalf("Augment(second) = %s, want changed session section refreshed", second)
		}
		if !strings.Contains(second, `"updated_at":"2026-04-26T12:01:00Z"`) {
			t.Fatalf("Augment(second) = %s, want refreshed session updated_at", second)
		}
	})

	t.Run("Should reset compaction when ACP session changes", func(t *testing.T) {
		t.Parallel()

		service := NewService(Deps{Now: fixedNow, SectionLimit: 2})
		sess := newSituationCompactionSession()
		if _, err := service.Augment(context.Background(), sess, "first prompt"); err != nil {
			t.Fatalf("Augment(first) error = %v", err)
		}
		sess.ACPSessionID = "acp-2"

		second, err := service.Augment(context.Background(), sess, "second prompt")
		if err != nil {
			t.Fatalf("Augment(second) error = %v", err)
		}
		if strings.Contains(second, `"unchanged":true`) {
			t.Fatalf("Augment(second after ACP reset) = %s, want full sections", second)
		}
		if !strings.Contains(second, `"agent_name":"coder"`) {
			t.Fatalf("Augment(second after ACP reset) = %s, want full self section", second)
		}
	})
}

func TestPromptSectionAndHelperBranches(t *testing.T) {
	t.Parallel()

	service := NewService(Deps{Now: fixedNow})
	rendered, err := service.PromptSection(context.Background(), &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: "/work/agh", Name: "AGH"},
	})
	if err != nil {
		t.Fatalf("PromptSection() error = %v", err)
	}
	if !strings.Contains(rendered, `"workspace"`) || !strings.Contains(rendered, `"limits"`) {
		t.Fatalf("PromptSection() = %s, want workspace and limits sections", rendered)
	}

	if got, err := service.Augment(context.Background(), nil, "message"); err != nil || got != "message" {
		t.Fatalf("Augment(nil session) = %q, %v; want original message", got, err)
	}
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ContextForSession(canceledCtx, &session.Info{ID: "sess-1"}); err == nil {
		t.Fatal("ContextForSession(canceled ctx) error = nil, want validation error")
	}
	if !isContextError(context.Canceled) {
		t.Fatal("isContextError(context.Canceled) = false, want true")
	}
	if !isContextError(context.DeadlineExceeded) {
		t.Fatal("isContextError(context.DeadlineExceeded) = false, want true")
	}
}

func TestSelectionPreviewAndBoundingHelpers(t *testing.T) {
	t.Parallel()

	queuedAt := fixedTime()
	runs := []taskpkg.Run{
		{ID: "done", Status: taskpkg.TaskRunStatusCompleted, QueuedAt: queuedAt.Add(4 * time.Minute)},
		{ID: "queued", Status: taskpkg.TaskRunStatusQueued, QueuedAt: queuedAt.Add(3 * time.Minute)},
		{ID: "claimed", Status: taskpkg.TaskRunStatusClaimed, QueuedAt: queuedAt.Add(2 * time.Minute)},
		{ID: "starting", Status: taskpkg.TaskRunStatusStarting, QueuedAt: queuedAt.Add(time.Minute)},
		{ID: "running", Status: taskpkg.TaskRunStatusRunning, StartedAt: queuedAt.Add(5 * time.Minute)},
	}
	selected, ok := selectActiveRun(runs)
	if !ok || selected.ID != "running" {
		t.Fatalf("selectActiveRun() = %#v, %v; want running", selected, ok)
	}
	if _, ok := selectActiveRun([]taskpkg.Run{{ID: "done", Status: taskpkg.TaskRunStatusCompleted}}); ok {
		t.Fatal("selectActiveRun(terminal) ok = true, want false")
	}
	if got := activeRunRank(taskpkg.TaskRunStatusCanceled); got != -1 {
		t.Fatalf("activeRunRank(canceled) = %d, want -1", got)
	}
	if got := runActivityTime(taskpkg.Run{QueuedAt: queuedAt, StartedAt: queuedAt.Add(time.Minute)}); !got.Equal(
		queuedAt.Add(time.Minute),
	) {
		t.Fatalf("runActivityTime() = %s, want latest start", got)
	}

	direct := envelopeWithBody(t, network.KindSay, network.SayBody{Text: "direct message"})
	if got, want := envelopePreview(direct), "direct message"; got != want {
		t.Fatalf("envelopePreview(direct) = %q, want %q", got, want)
	}
	trace := envelopeWithBody(t, network.KindTrace, network.TraceBody{
		State:   network.WorkStateWorking,
		Message: "trace message",
	})
	if got, want := envelopePreview(trace), "trace message"; got != want {
		t.Fatalf("envelopePreview(trace) = %q, want %q", got, want)
	}
	capability := envelopeWithBody(t, network.KindCapability, network.CapabilityBody{
		Capability: capabilityPayload(t, "cap", "capability summary", "done"),
	})
	if got, want := envelopePreview(capability), "capability summary"; got != want {
		t.Fatalf("envelopePreview(capability) = %q, want %q", got, want)
	}
	detail := "receipt detail"
	receipt := envelopeWithBody(t, network.KindReceipt, network.ReceiptBody{
		ForID:  "msg-1",
		Status: network.ReceiptStatusAccepted,
		Detail: &detail,
	})
	if got, want := envelopePreview(receipt), "receipt detail"; got != want {
		t.Fatalf("envelopePreview(receipt) = %q, want %q", got, want)
	}
	greet := envelopeWithBody(t, network.KindGreet, network.GreetBody{
		PeerCard: network.PeerCard{
			PeerID:              "peer",
			ProfilesSupported:   []string{network.ProtocolV0},
			Capabilities:        []string{},
			ArtifactsSupported:  []string{},
			TrustModesSupported: []string{},
		},
		Summary: "hello peer",
	})
	if got, want := envelopePreview(greet), "hello peer"; got != want {
		t.Fatalf("envelopePreview(greet) = %q, want %q", got, want)
	}
	if got := envelopePreview(network.Envelope{Kind: network.KindSay, Body: json.RawMessage(`{`)}); got != "" {
		t.Fatalf("envelopePreview(invalid) = %q, want empty", got)
	}
	if got := envelopeTimestamp(network.Envelope{}); !got.IsZero() {
		t.Fatalf("envelopeTimestamp(zero) = %s, want zero", got)
	}

	long := strings.Repeat("x", inboxPreviewLimit+10)
	if got := truncateRunes(long, 12); utf8.RuneCountInString(got) != 12 || !strings.HasSuffix(got, "...") {
		t.Fatalf("truncateRunes() = %q, want 12-rune ellipsized value", got)
	}
	if got, want := truncateRunes("abcdef", 2), ".."; got != want {
		t.Fatalf("truncateRunes(short limit) = %q, want %q", got, want)
	}
}

func TestCoordinationMetadataAndPeerHelpers(t *testing.T) {
	t.Parallel()

	rawMetadata := jsonRaw(t, `{
		"task_id":"task-1",
		"run_id":"run-1",
		"channel_id":"coord-1",
		"message_kind":"status",
		"correlation_id":"corr-1"
	}`)
	direct := network.Envelope{
		Ext: network.ExtensionMap{
			"task_id":        jsonRaw(t, `"task-1"`),
			"run_id":         jsonRaw(t, `"run-1"`),
			"channel_id":     jsonRaw(t, `"coord-1"`),
			"message_kind":   jsonRaw(t, `"status"`),
			"correlation_id": jsonRaw(t, `"corr-1"`),
		},
	}
	if metadata, ok := coordinationMetadataFromEnvelope(direct); !ok || metadata.TaskID != "task-1" {
		t.Fatalf("coordinationMetadataFromEnvelope(direct) = %#v, %v; want task metadata", metadata, ok)
	}
	nested := network.Envelope{Ext: network.ExtensionMap{"metadata": rawMetadata}}
	if metadata, ok := coordinationMetadataFromEnvelope(nested); !ok || metadata.CorrelationID != "corr-1" {
		t.Fatalf("coordinationMetadataFromEnvelope(nested) = %#v, %v; want nested metadata", metadata, ok)
	}
	if _, ok := coordinationMetadataFromEnvelope(network.Envelope{}); ok {
		t.Fatal("coordinationMetadataFromEnvelope(empty) ok = true, want false")
	}
	if _, ok := decodeCoordinationMetadata(json.RawMessage(`{"claim_token":"raw"}`)); ok {
		t.Fatal("decodeCoordinationMetadata(raw claim token) ok = true, want false")
	}

	selfSession := "sess-self"
	display := "Peer A"
	roster := peerRoster([]network.PeerInfo{
		{
			SessionID: &selfSession,
			PeerID:    "self",
			Channel:   "coord",
		},
		{
			PeerID:  "peer-a",
			Channel: "coord",
			PeerCard: network.PeerCard{
				PeerID:       "peer-a",
				DisplayName:  &display,
				Capabilities: []string{"review", "review"},
			},
			CapabilityCatalogKnown: true,
			CapabilityCatalog: []session.NetworkPeerCapability{
				{ID: "build"},
				{ID: "review"},
			},
		},
	}, selfSession, 4)
	if got, want := len(roster.Peers), 1; got != want {
		t.Fatalf("peerRoster() peers = %d, want %d", got, want)
	}
	if got, want := roster.Peers[0].Capabilities, []string{"build", "review"}; !slices.Equal(got, want) {
		t.Fatalf("peer capabilities = %#v, want %#v", got, want)
	}
}

func assertOrder(t *testing.T, value string, tokens []string) {
	t.Helper()

	last := -1
	for _, token := range tokens {
		index := strings.Index(value, token)
		if index < 0 {
			t.Fatalf("missing token %q in %s", token, value)
		}
		if index <= last {
			t.Fatalf("token %q appeared out of order in %s", token, value)
		}
		last = index
	}
}

func fixedNow() time.Time {
	return fixedTime()
}

func fixedTime() time.Time {
	return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
}

func situationLiveSpec(t *testing.T, workspaceID string, channelID string) participation.Spec {
	t.Helper()
	spec := participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       channelID,
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         1,
			MaxWakeWallTime:  "1m",
			MaxTotalWallTime: "5m",
			MaxInputTokens:   1024,
			MaxOutputTokens:  512,
			MaxWakeDepth:     1,
			CoalesceWindow:   "500ms",
		},
	}
	if err := participation.ValidateSpec(spec); err != nil {
		t.Fatalf("ValidateSpec() error = %v", err)
	}
	return spec
}

func newSituationCompactionSession() *session.Session {
	return &session.Session{
		ID:           "sess-compact",
		Name:         "Compaction Session",
		AgentName:    "coder",
		Provider:     "codex",
		WorkspaceID:  "ws-compact",
		Workspace:    "/work/agh",
		Type:         session.SessionTypeUser,
		State:        session.StateActive,
		ACPSessionID: "acp-1",
		CreatedAt:    fixedTime(),
		UpdatedAt:    fixedTime(),
	}
}

func jsonRaw(t *testing.T, value string) json.RawMessage {
	t.Helper()

	if !json.Valid([]byte(value)) {
		t.Fatalf("invalid JSON fixture: %s", value)
	}
	return json.RawMessage(value)
}

func workspaceConfigWithTaskDefaults() aghconfig.Config {
	return aghconfig.Config{
		Defaults: aghconfig.DefaultsConfig{Provider: "codex"},
		Task:     aghconfig.DefaultTaskConfig(),
	}
}

func coordinationEnvelope(
	t *testing.T,
	id string,
	channel string,
	text string,
	timestamp time.Time,
) network.Envelope {
	t.Helper()

	body, err := json.Marshal(network.SayBody{Text: text})
	if err != nil {
		t.Fatalf("marshal say body: %v", err)
	}
	metadata, err := json.Marshal(contract.CoordinationMessageMetadataPayload{
		TaskID:        "task-1",
		RunID:         "run-1",
		ChannelID:     channel,
		MessageKind:   contract.CoordinationMessageStatus,
		CorrelationID: id + "-corr",
	})
	if err != nil {
		t.Fatalf("marshal coordination metadata: %v", err)
	}
	return network.Envelope{
		Protocol: network.ProtocolV0,
		ID:       id,
		Kind:     network.KindSay,
		Channel:  channel,
		From:     "peer-a",
		TS:       timestamp.Unix(),
		Body:     body,
		Ext:      network.ExtensionMap{"coordination": metadata},
	}
}

func envelopeWithBody(t *testing.T, kind network.Kind, bodyValue network.Body) network.Envelope {
	t.Helper()

	body, err := json.Marshal(bodyValue)
	if err != nil {
		t.Fatalf("marshal %s body: %v", kind, err)
	}
	return network.Envelope{
		Protocol: network.ProtocolV0,
		ID:       "preview",
		Kind:     kind,
		Channel:  "coord",
		From:     "peer",
		TS:       fixedTime().Unix(),
		Body:     body,
	}
}

func capabilityPayload(
	t *testing.T,
	id string,
	summary string,
	outcome string,
) network.CapabilityEnvelopePayload {
	t.Helper()

	digest, err := aghconfig.CanonicalCapabilityDigest(aghconfig.CapabilityDef{
		ID:      id,
		Summary: summary,
		Outcome: outcome,
	})
	if err != nil {
		t.Fatalf("CanonicalCapabilityDigest() error = %v", err)
	}
	return network.CapabilityEnvelopePayload{
		ID:      id,
		Summary: summary,
		Outcome: outcome,
		Digest:  digest,
	}
}

type workspaceResolverFunc func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)

func (fn workspaceResolverFunc) Resolve(
	ctx context.Context,
	idOrPath string,
) (workspacepkg.ResolvedWorkspace, error) {
	return fn(ctx, idOrPath)
}

type agentResolverFunc func(string, *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error)

func (fn agentResolverFunc) ResolveAgent(
	name string,
	workspace *workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, error) {
	return fn(name, workspace)
}

type skillRegistryFunc func(context.Context, *workspacepkg.ResolvedWorkspace) ([]*skillspkg.Skill, error)

func (fn skillRegistryFunc) ForWorkspace(
	ctx context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
) ([]*skillspkg.Skill, error) {
	return fn(ctx, workspace)
}

func (fn skillRegistryFunc) ForAgent(
	ctx context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
	_ string,
) ([]*skillspkg.Skill, error) {
	return fn(ctx, workspace)
}

type coordinatorResolverFunc func(context.Context, string) (aghconfig.ResolvedCoordinatorRole, error)

func (fn coordinatorResolverFunc) ResolveCoordinatorRole(
	ctx context.Context,
	workspaceID string,
) (aghconfig.ResolvedCoordinatorRole, error) {
	return fn(ctx, workspaceID)
}

type taskStoreStub struct {
	tasks                   map[string]taskpkg.Task
	runs                    []taskpkg.Run
	events                  []taskpkg.Event
	profiles                map[string]taskpkg.ExecutionProfile
	reviews                 map[string]taskpkg.RunReview
	getTaskErr              error
	listTaskRunsErr         error
	getTaskRunErr           error
	listTaskEventsErr       error
	listTaskEventRecordsErr error
	getExecutionProfileErr  error
	getRunReviewErr         error
	lookupRunReviewErr      error
	listRunReviewsErr       error
}

func (s taskStoreStub) GetTask(_ context.Context, id string) (taskpkg.Task, error) {
	if s.getTaskErr != nil {
		return taskpkg.Task{}, s.getTaskErr
	}
	taskRecord, ok := s.tasks[id]
	if !ok {
		return taskpkg.Task{}, errors.New("missing task")
	}
	taskRecord.LatestEventSeq = s.latestEventSeq(taskRecord.ID)
	return taskRecord, nil
}

func (s taskStoreStub) ListTaskRuns(_ context.Context, query taskpkg.RunQuery) ([]taskpkg.Run, error) {
	if s.listTaskRunsErr != nil {
		return nil, s.listTaskRunsErr
	}
	runs := make([]taskpkg.Run, 0, len(s.runs))
	for _, run := range s.runs {
		if strings.TrimSpace(query.TaskID) != "" &&
			strings.TrimSpace(run.TaskID) != strings.TrimSpace(query.TaskID) {
			continue
		}
		if query.Status != taskpkg.TaskRunStatusUnknown && run.Status.Normalize() != query.Status.Normalize() {
			continue
		}
		if strings.TrimSpace(query.SessionID) != "" &&
			strings.TrimSpace(run.SessionID) != strings.TrimSpace(query.SessionID) {
			continue
		}
		runs = append(runs, run)
	}
	if query.Limit > 0 && len(runs) > query.Limit {
		runs = runs[len(runs)-query.Limit:]
	}
	return runs, nil
}

func (s taskStoreStub) GetTaskRun(_ context.Context, id string) (taskpkg.Run, error) {
	if s.getTaskRunErr != nil {
		return taskpkg.Run{}, s.getTaskRunErr
	}
	for _, run := range s.runs {
		if strings.TrimSpace(run.ID) == strings.TrimSpace(id) {
			return run, nil
		}
	}
	return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
}

func (s taskStoreStub) ListTaskEvents(_ context.Context, query taskpkg.EventQuery) ([]taskpkg.Event, error) {
	if s.listTaskEventsErr != nil {
		return nil, s.listTaskEventsErr
	}
	events := make([]taskpkg.Event, 0, len(s.events))
	for _, event := range s.events {
		if strings.TrimSpace(query.TaskID) != "" &&
			strings.TrimSpace(event.TaskID) != strings.TrimSpace(query.TaskID) {
			continue
		}
		if strings.TrimSpace(query.RunID) != "" &&
			strings.TrimSpace(event.RunID) != strings.TrimSpace(query.RunID) {
			continue
		}
		if strings.TrimSpace(query.EventType) != "" &&
			strings.TrimSpace(event.EventType) != strings.TrimSpace(query.EventType) {
			continue
		}
		events = append(events, event)
	}
	if query.Limit > 0 && len(events) > query.Limit {
		events = events[len(events)-query.Limit:]
	}
	return events, nil
}

func (s taskStoreStub) TaskWakeEventExists(context.Context, string, string) (bool, error) {
	return false, nil
}

func (s taskStoreStub) ListTaskEventRecords(
	_ context.Context,
	query taskpkg.EventRecordQuery,
) ([]taskpkg.EventRecord, error) {
	if err := query.Validate("task_event_record_query"); err != nil {
		return nil, err
	}
	if s.listTaskEventRecordsErr != nil {
		return nil, s.listTaskEventRecordsErr
	}
	records := make([]taskpkg.EventRecord, 0, len(s.events))
	for idx, event := range s.events {
		if strings.TrimSpace(event.TaskID) != strings.TrimSpace(query.TaskID) {
			continue
		}
		sequence := int64(idx + 1)
		if sequence <= query.AfterSequence {
			continue
		}
		records = append(records, taskpkg.EventRecord{
			Sequence: sequence,
			Event:    event,
		})
	}
	slices.SortStableFunc(records, func(left, right taskpkg.EventRecord) int {
		if query.Descending {
			if left.Sequence > right.Sequence {
				return -1
			}
			if left.Sequence < right.Sequence {
				return 1
			}
			return 0
		}
		if left.Sequence < right.Sequence {
			return -1
		}
		if left.Sequence > right.Sequence {
			return 1
		}
		return 0
	})
	if query.Limit > 0 && len(records) > query.Limit {
		return append([]taskpkg.EventRecord(nil), records[:query.Limit]...), nil
	}
	return records, nil
}

func (s taskStoreStub) latestEventSeq(taskID string) int64 {
	var latest int64
	trimmedTaskID := strings.TrimSpace(taskID)
	for idx, event := range s.events {
		if strings.TrimSpace(event.TaskID) != trimmedTaskID {
			continue
		}
		latest = int64(idx + 1)
	}
	return latest
}

func (s taskStoreStub) GetExecutionProfile(_ context.Context, taskID string) (taskpkg.ExecutionProfile, error) {
	if s.getExecutionProfileErr != nil {
		return taskpkg.ExecutionProfile{}, s.getExecutionProfileErr
	}
	profile, ok := s.profiles[strings.TrimSpace(taskID)]
	if !ok {
		return taskpkg.ExecutionProfile{}, taskpkg.ErrExecutionProfileNotFound
	}
	return profile, nil
}

func (s taskStoreStub) GetRunReview(_ context.Context, reviewID string) (taskpkg.RunReview, error) {
	if s.getRunReviewErr != nil {
		return taskpkg.RunReview{}, s.getRunReviewErr
	}
	review, ok := s.reviews[strings.TrimSpace(reviewID)]
	if !ok {
		return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
	}
	return review, nil
}

func (s taskStoreStub) LookupRunReviewBySession(_ context.Context, sessionID string) (taskpkg.RunReview, error) {
	if s.lookupRunReviewErr != nil {
		return taskpkg.RunReview{}, s.lookupRunReviewErr
	}
	for _, review := range s.reviews {
		if strings.TrimSpace(review.ReviewerSessionID) == strings.TrimSpace(sessionID) {
			return review, nil
		}
	}
	return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
}

func (s taskStoreStub) ListRunReviews(_ context.Context, query taskpkg.RunReviewQuery) ([]taskpkg.RunReview, error) {
	if s.listRunReviewsErr != nil {
		return nil, s.listRunReviewsErr
	}
	reviews := make([]taskpkg.RunReview, 0, len(s.reviews))
	for _, review := range s.reviews {
		if strings.TrimSpace(query.TaskID) != "" &&
			strings.TrimSpace(review.TaskID) != strings.TrimSpace(query.TaskID) {
			continue
		}
		if strings.TrimSpace(query.RunID) != "" &&
			strings.TrimSpace(review.RunID) != strings.TrimSpace(query.RunID) {
			continue
		}
		if query.Status != "" && review.Status.Normalize() != query.Status.Normalize() {
			continue
		}
		if strings.TrimSpace(query.ReviewerSessionID) != "" &&
			strings.TrimSpace(review.ReviewerSessionID) != strings.TrimSpace(query.ReviewerSessionID) {
			continue
		}
		reviews = append(reviews, review)
	}
	slices.SortFunc(reviews, func(left, right taskpkg.RunReview) int {
		switch {
		case left.UpdatedAt.After(right.UpdatedAt):
			return -1
		case left.UpdatedAt.Before(right.UpdatedAt):
			return 1
		default:
			return strings.Compare(right.ReviewID, left.ReviewID)
		}
	})
	if query.Limit > 0 && len(reviews) > query.Limit {
		reviews = reviews[:query.Limit]
	}
	return reviews, nil
}

type soulSnapshotStoreStub struct {
	snapshots map[string]soul.Snapshot
}

func (s soulSnapshotStoreStub) GetSoulSnapshot(_ context.Context, id string) (soul.Snapshot, error) {
	snapshot, ok := s.snapshots[strings.TrimSpace(id)]
	if !ok {
		return soul.Snapshot{}, soul.ErrSnapshotNotFound
	}
	return snapshot, nil
}

type networkStub struct {
	envelopes []network.Envelope
	peers     []network.PeerInfo
}

func (s networkStub) Inbox(_ context.Context, _ string) ([]network.Envelope, error) {
	return slices.Clone(s.envelopes), nil
}

func (s networkStub) ListPeers(_ context.Context, _ string, _ string) ([]network.PeerInfo, error) {
	return slices.Clone(s.peers), nil
}

func testSituationSoulSnapshot(t *testing.T, body string) soul.Snapshot {
	t.Helper()

	cfg := aghconfig.DefaultSoulConfig()
	resolved, err := soul.Parse(context.Background(), soul.ParseRequest{
		SourcePath:    "/work/agh/.agh/agents/coder/SOUL.md",
		WorkspaceRoot: "/work/agh",
		Content: []byte(strings.Join([]string{
			"---",
			"role: Reviewer",
			"tone:",
			"  - direct",
			"principles:",
			"  - protect correctness",
			"---",
			body,
		}, "\n")),
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("soul.Parse() error = %v", err)
	}
	provenance, err := soul.NewConfigProvenance(cfg, "test")
	if err != nil {
		t.Fatalf("NewConfigProvenance() error = %v", err)
	}
	snapshot, err := soul.SnapshotFromResolved(
		"soul-situation",
		"ws-1",
		"coder",
		&resolved,
		provenance,
		fixedTime(),
	)
	if err != nil {
		t.Fatalf("SnapshotFromResolved() error = %v", err)
	}
	return snapshot
}
