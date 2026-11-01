package session

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
)

type recordingSpawnWakeNotifier struct {
	mu      sync.Mutex
	events  []SpawnWakeEvent
	parents []string
	err     error
}

func (notifier *recordingSpawnWakeNotifier) WakeSpawnCreator(
	_ context.Context,
	creatorSessionID string,
	event SpawnWakeEvent,
) error {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	notifier.parents = append(notifier.parents, creatorSessionID)
	notifier.events = append(notifier.events, event)
	return notifier.err
}

func (notifier *recordingSpawnWakeNotifier) calls() ([]string, []SpawnWakeEvent) {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	return append([]string(nil), notifier.parents...), append([]SpawnWakeEvent(nil), notifier.events...)
}

func TestSpawnWakeEventContract(t *testing.T) {
	t.Parallel()

	t.Run("Should redact and bound wake detail before validation", func(t *testing.T) {
		t.Parallel()

		rawToken := "compozy_claim_secret-value"
		event := SpawnWakeEvent{
			ChildSessionID: " child-1 ",
			ChildAgentName: " researcher ",
			Reason:         " NEEDS_ATTENTION ",
			Badge:          " WAITING-FOR-INPUT ",
			Detail:         rawToken + strings.Repeat("界", 300),
			WakeEventID:    " wake-1 ",
		}.Normalize()

		if err := event.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if strings.Contains(event.Detail, rawToken) || !strings.Contains(event.Detail, "compozy_claim_[REDACTED]") {
			t.Fatalf("Normalize().Detail = %q, want claim token redacted", event.Detail)
		}
		if got := len([]rune(event.Detail)); got > spawnWakeTextMaxRunes {
			t.Fatalf("wake detail runes = %d, want <= %d", got, spawnWakeTextMaxRunes)
		}
	})

	t.Run("Should reject a badge and reason mismatch", func(t *testing.T) {
		t.Parallel()

		err := (SpawnWakeEvent{
			ChildSessionID: "child-1",
			Reason:         SpawnWakeReasonStopped,
			Badge:          BadgeFailed,
			WakeEventID:    "wake-1",
		}).Validate()
		if !errors.Is(err, ErrSpawnWakeValidation) {
			t.Fatalf("Validate() error = %v, want ErrSpawnWakeValidation", err)
		}
	})
}

func TestSpawnWakeReasonForBadge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		badge      Badge
		wantReason SpawnWakeReason
		wantOK     bool
	}{
		{name: "Should map stopped", badge: BadgeStopped, wantReason: SpawnWakeReasonStopped, wantOK: true},
		{name: "Should map failed", badge: BadgeFailed, wantReason: SpawnWakeReasonFailed, wantOK: true},
		{name: "Should map done to completed", badge: BadgeDone, wantReason: SpawnWakeReasonCompleted, wantOK: true},
		{
			name:       "Should map waiting for auth",
			badge:      BadgeWaitingForAuth,
			wantReason: SpawnWakeReasonNeedsAttention,
			wantOK:     true,
		},
		{
			name:       "Should map waiting for input",
			badge:      BadgeWaitingForInput,
			wantReason: SpawnWakeReasonNeedsAttention,
			wantOK:     true,
		},
		{
			name:       "Should map needs attention",
			badge:      BadgeNeedsAttention,
			wantReason: SpawnWakeReasonNeedsAttention,
			wantOK:     true,
		},
		{name: "Should not wake on plain idle", badge: BadgeIdle, wantOK: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			reason, ok := spawnWakeReasonForBadge(test.badge)
			if ok != test.wantOK || reason != test.wantReason {
				t.Fatalf(
					"spawnWakeReasonForBadge(%q) = (%q, %v), want (%q, %v)",
					test.badge, reason, ok, test.wantReason, test.wantOK,
				)
			}
		})
	}
}

func TestManagerDispatchSpawnWake(t *testing.T) {
	t.Parallel()

	t.Run("Should deliver one wake per cause episode", func(t *testing.T) {
		t.Parallel()

		notifier := &recordingSpawnWakeNotifier{}
		manager := spawnWakeTestManager(notifier, nil)
		info := spawnWakeTestInfo(true)

		manager.dispatchSpawnWake(testutil.Context(t), info, BadgeWaitingForInput)
		manager.dispatchSpawnWake(testutil.Context(t), info, BadgeWaitingForInput)

		parents, events := notifier.calls()
		if got, want := len(events), 1; got != want {
			t.Fatalf("wake calls = %d, want %d", got, want)
		}
		if parents[0] != "parent-1" || events[0].ChildSessionID != "child-1" ||
			events[0].Reason != SpawnWakeReasonNeedsAttention || events[0].Badge != BadgeWaitingForInput {
			t.Fatalf("wake delivery = parents %#v events %#v, want canonical child wake", parents, events)
		}
		if !strings.Contains(events[0].Detail, "Include vendored packages?") {
			t.Fatalf("wake detail = %q, want pending interaction title", events[0].Detail)
		}
	})

	t.Run("Should never wake the main session for a triage run", func(t *testing.T) {
		t.Parallel()

		notifier := &recordingSpawnWakeNotifier{}
		manager := spawnWakeTestManager(notifier, nil)
		info := spawnWakeTestInfo(true)
		info.Type = SessionTypeTriage

		manager.dispatchSpawnWake(testutil.Context(t), info, BadgeDone)
		manager.dispatchSpawnWake(testutil.Context(t), info, BadgeStopped)

		if _, events := notifier.calls(); len(events) != 0 {
			t.Fatalf("wake calls = %#v, want none", events)
		}
	})

	t.Run("Should deliver one completed wake for a done badge", func(t *testing.T) {
		t.Parallel()

		notifier := &recordingSpawnWakeNotifier{}
		manager := spawnWakeTestManager(notifier, nil)
		info := spawnWakeTestInfo(true)

		manager.dispatchSpawnWake(testutil.Context(t), info, BadgeDone)

		parents, events := notifier.calls()
		if got, want := len(events), 1; got != want {
			t.Fatalf("wake calls = %d, want %d", got, want)
		}
		if parents[0] != "parent-1" || events[0].ChildSessionID != "child-1" ||
			events[0].Reason != SpawnWakeReasonCompleted || events[0].Badge != BadgeDone {
			t.Fatalf("wake delivery = parents %#v events %#v, want completed wake", parents, events)
		}
	})

	t.Run("Should leave a completed turn to the creator's reply watch on that turn", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		notifier := &recordingSpawnWakeNotifier{}
		manager := spawnWakeTestManager(notifier, &logs)
		watches := &pendingReplyWatchStore{pending: []store.ReplyWatch{
			{ID: "rw-other", SenderSessionID: "parent-1", TargetSessionID: "child-2", TurnID: "turn-watched"},
			{ID: "rw-child", SenderSessionID: "parent-1", TargetSessionID: "child-1", TurnID: "turn-watched"},
		}}
		manager.replyWatches = &replyWatchService{store: watches, manager: manager}

		manager.dispatchSpawnWakeForTurn(testutil.Context(t), spawnWakeTestInfo(true), BadgeDone, "turn-watched")

		if _, events := notifier.calls(); len(events) != 0 {
			t.Fatalf("wake events = %#v, want the reply watch to carry the completed turn", events)
		}
		if watches.sender != "parent-1" {
			t.Fatalf("reply watch lookup sender = %q, want parent-1", watches.sender)
		}
		if got := logs.String(); !strings.Contains(got, "suppression_reason=reply_watch_covers") {
			t.Fatalf("wake audit log = %q, want reply_watch_covers suppression", got)
		}
	})

	t.Run("Should still wake for a completed turn a queued reply watch does not cover", func(t *testing.T) {
		t.Parallel()

		notifier := &recordingSpawnWakeNotifier{}
		manager := spawnWakeTestManager(notifier, nil)
		watches := &pendingReplyWatchStore{pending: []store.ReplyWatch{
			{ID: "rw-queued", SenderSessionID: "parent-1", TargetSessionID: "child-1"},
			{ID: "rw-earlier", SenderSessionID: "parent-1", TargetSessionID: "child-1", TurnID: "turn-earlier"},
		}}
		manager.replyWatches = &replyWatchService{store: watches, manager: manager}

		manager.dispatchSpawnWakeForTurn(testutil.Context(t), spawnWakeTestInfo(true), BadgeDone, "turn-unwatched")

		if _, events := notifier.calls(); len(events) != 1 || events[0].Reason != SpawnWakeReasonCompleted {
			t.Fatalf("wake events = %#v, want the unwatched turn's completed wake", events)
		}
	})

	tests := []struct {
		name        string
		notify      bool
		parentID    string
		notifierErr error
		wantReason  string
	}{
		{
			name: "Should audit creator opt out", notify: false,
			parentID: "parent-1", wantReason: "wake_creator_disabled",
		},
		{name: "Should audit self wake", notify: true, parentID: "child-1", wantReason: "self_wake"},
		{
			name: "Should audit a creator that is not live", notify: true, parentID: "parent-1",
			notifierErr: ErrSpawnWakeSessionNotLive, wantReason: "session_not_live",
		},
		{
			name: "Should audit delivery failure", notify: true, parentID: "parent-1",
			notifierErr: errors.New("provider rejected compozy_claim_private-token"), wantReason: "delivery_failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			notifier := &recordingSpawnWakeNotifier{err: test.notifierErr}
			manager := spawnWakeTestManager(notifier, &logs)
			info := spawnWakeTestInfo(test.notify)
			info.Lineage.ParentSessionID = test.parentID

			manager.dispatchSpawnWake(testutil.Context(t), info, BadgeWaitingForInput)

			if got := logs.String(); !strings.Contains(got, "session.spawn_wake.suppressed") ||
				!strings.Contains(got, "suppression_reason="+test.wantReason) {
				t.Fatalf("wake audit log = %q, want suppression %q", got, test.wantReason)
			} else if strings.Contains(got, "compozy_claim_private-token") {
				t.Fatalf("wake audit log leaked claim token: %q", got)
			}
		})
	}
}

type pendingReplyWatchStore struct {
	store.ReplyWatchStore
	pending []store.ReplyWatch
	sender  string
}

func (s *pendingReplyWatchStore) ReplyWatchEvidence(
	context.Context,
	store.ReplyWatch,
) (store.SessionPromptAdmission, []store.SessionInputQueueEntry, error) {
	return store.SessionPromptAdmission{}, nil, nil
}

func (s *pendingReplyWatchStore) ListPendingReplyWatchesBySender(
	_ context.Context,
	sender string,
) ([]store.ReplyWatch, error) {
	s.sender = sender
	return s.pending, nil
}

func spawnWakeTestManager(notifier SpawnWakeNotifier, logs *bytes.Buffer) *Manager {
	if logs == nil {
		logs = &bytes.Buffer{}
	}
	return &Manager{
		logger:              slog.New(slog.NewTextHandler(logs, nil)),
		spawnWakeNotifier:   notifier,
		spawnWakeEventIDs:   make(map[string]struct{}),
		spawnWakeEventOrder: make([]string, 0),
	}
}

func spawnWakeTestInfo(notify bool) *Info {
	return &Info{
		ID:                "child-1",
		AgentName:         "researcher",
		WorkspaceID:       "workspace-1",
		TranscriptEpoch:   3,
		AttentionRevision: 7,
		Lineage: &store.SessionLineage{
			ParentSessionID: "parent-1",
			RootSessionID:   "parent-1",
			SpawnDepth:      1,
			NotifyCreator:   notify,
		},
		PendingInteractions: []store.PendingInteraction{{
			InteractionID: "interaction-1",
			Title:         "Include vendored packages?",
		}},
	}
}

// Invariant: a canceled child turn must not emit a completed spawn wake, while
// the same terminal edge still wakes the creator for a normal completion.
// Owner: spawn wake dispatch; canonical spawn_wake_test.go suite.
func TestFinishPromptPumpSpawnWakeCancelSuppression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		canceled  bool
		wantWakes int
	}{
		{name: "Should suppress the completed wake for a canceled turn", canceled: true, wantWakes: 0},
		{name: "Should deliver the completed wake for a normal turn", canceled: false, wantWakes: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			notifier := &recordingSpawnWakeNotifier{}
			manager, err := NewManager(
				WithHomePaths(testHomePaths(t)),
				WithSpawnWakeNotifier(notifier),
			)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			cleanupTestManager(t, manager)
			manager.attentionStore = newPresenceAttentionStore()

			session := &Session{
				ID:          "child-1",
				AgentName:   "researcher",
				WorkspaceID: "ws-1",
				State:       StateActive,
				Lineage: &store.SessionLineage{
					ParentSessionID: "parent-1",
					RootSessionID:   "parent-1",
					SpawnDepth:      1,
					NotifyCreator:   true,
				},
				currentTurnID: "turn-1",
			}
			if test.canceled {
				session.currentPromptCancelTurn = "turn-1"
			}
			manager.mu.Lock()
			manager.sessions[session.ID] = session
			manager.mu.Unlock()

			turnState := &promptTurnDispatchState{
				session: session, turnID: "turn-1", runID: "run-1", generation: 1,
			}
			manager.finishPromptPump(
				testutil.Context(t),
				session,
				turnState,
				nil,
				nil,
				nil,
				&promptPumpFatal{},
			)

			_, events := notifier.calls()
			if got := len(events); got != test.wantWakes {
				t.Fatalf("completed wake deliveries = %d, want %d (%#v)", got, test.wantWakes, events)
			}
			if test.wantWakes == 1 {
				if events[0].Reason != SpawnWakeReasonCompleted || events[0].Badge != BadgeDone {
					t.Fatalf("wake = %#v, want completed/done", events[0])
				}
			}
		})
	}
}

// earlyStopPullRequests fakes the branch snapshot and pull request read model. saved is
// the stored branch, current the branch the saver records at settle, and refreshErr
// with refreshCause a failed forge refresh.
type earlyStopPullRequests struct {
	mu           sync.Mutex
	saved        string
	current      string
	prBranch     string
	refreshErr   error
	refreshCause string
}

func (r *earlyStopPullRequests) SessionBranches(_ context.Context, ids []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		out[id] = r.saved
	}
	return out, nil
}

func (r *earlyStopPullRequests) PullRequests(_ context.Context, keys []BranchKey) map[BranchKey]PullRequestView {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[BranchKey]PullRequestView, len(keys))
	for _, key := range keys {
		if key.Branch == r.prBranch && r.refreshErr == nil {
			out[key] = PullRequestView{PullRequest: &PullRequest{Number: 7, State: PullRequestOpen}}
		}
	}
	return out
}

func (r *earlyStopPullRequests) RefreshBranch(_ context.Context, key BranchKey) (PullRequestView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.refreshErr != nil {
		return PullRequestView{Lookup: PullRequestLookup{State: LookupError, Cause: r.refreshCause}}, r.refreshErr
	}
	if key.Branch == r.prBranch {
		return PullRequestView{PullRequest: &PullRequest{Number: 7, State: PullRequestOpen}}, nil
	}
	return PullRequestView{}, nil
}

func (r *earlyStopPullRequests) saveBranch(context.Context, *Info) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current != "" {
		r.saved = r.current
	}
	return nil
}

// flakyEarlyStopAttentionStore fails the first early-stop writes, then delegates.
type flakyEarlyStopAttentionStore struct {
	store.SessionAttentionStore
	mu       sync.Mutex
	failures int
	count    int
}

func (s *flakyEarlyStopAttentionStore) MarkSessionEndedEarly(
	ctx context.Context, sessionID string, turnID string,
) (store.SessionAttentionCommit, error) {
	s.mu.Lock()
	s.count++
	fail := s.count <= s.failures
	s.mu.Unlock()
	if fail {
		return store.SessionAttentionCommit{}, errors.New("database is locked")
	}
	return s.SessionAttentionStore.MarkSessionEndedEarly(ctx, sessionID, turnID)
}

func (s *flakyEarlyStopAttentionStore) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

func earlyStopWorkSources(open WorkSignalKind) []WorkSignalSource {
	sources := make([]WorkSignalSource, 0, len(WorkSignalKindValues()))
	for _, name := range WorkSignalKindValues() {
		kind := WorkSignalKind(name)
		if kind == WorkSignalAgentProgress {
			continue
		}
		sources = append(sources, WorkSignalSource{
			Kind: kind,
			Source: SignalSourceFunc(func(context.Context, string) ([]WorkSignal, error) {
				if kind != open {
					return nil, nil
				}
				now := time.Now().UTC()
				return []WorkSignal{{Kind: kind, Since: now, ValidUntil: now.Add(time.Hour), Ref: "open-work"}}, nil
			}),
		})
	}
	return sources
}

// Invariant: a child session whose turn ends with end_turn and nothing left to resume
// or deliver needs attention instead of reading done, durably across catalog reads, and
// wakes its parent once with the final answer even when the parent opted out of wakes.
// Owner: session supervision and spawn wake dispatch; canonical spawn_wake_test.go suite.
func TestManagerEarlyStopSupervision(t *testing.T) {
	t.Parallel()

	newEarlyStopHarness := func(
		t *testing.T, open WorkSignalKind, withParent bool, pullRequests *earlyStopPullRequests,
	) (*harness, *globaldb.GlobalDB, *recordingSpawnWakeNotifier, *Session, *Session) {
		t.Helper()
		db := openManagerInputQueueStore(t)
		notifier := &recordingSpawnWakeNotifier{}
		h := newHarness(t, WithSessionInputQueueStore(db), WithSessionCatalog(db), WithSpawnWakeNotifier(notifier))
		registerManagerInputQueueWorkspace(t, db, h)
		cleanupTestManager(t, h.manager)
		h.manager.SetWorkSignalSources(earlyStopWorkSources(open)...)
		if pullRequests != nil {
			h.manager.ConfigurePullRequests(pullRequests, pullRequests)
			h.manager.ConfigureBranchSnapshotSaver(pullRequests.saveBranch)
		}
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		var lineage *store.SessionLineage
		if withParent {
			lineage = &store.SessionLineage{
				ParentSessionID: parent.ID, RootSessionID: parent.ID, Kind: store.LineageKindProvenance,
			}
		}
		child, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Name: "executor", Workspace: h.workspaceID, Lineage: lineage,
		})
		if err != nil {
			t.Fatalf("Create(child) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, child.ID) })
		return h, db, notifier, parent, child
	}
	promptChild := func(t *testing.T, h *harness, child *Session) {
		t.Helper()
		events, err := h.manager.Prompt(testutil.Context(t), child.ID, "keep working until delivered")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)
	}

	t.Run("Should flag the early stop durably and wake the parent with the final answer", func(t *testing.T) {
		t.Parallel()
		h, db, notifier, parent, child := newEarlyStopHarness(t, "", true, nil)

		promptChild(t, h, child)

		info := child.Info()
		if info.EndedEarlyTurnID == "" || BadgeForInfo(info) != BadgeNeedsAttention {
			t.Fatalf("child badge = %s ended_early_turn_id = %q, want needs-attention", BadgeForInfo(info),
				info.EndedEarlyTurnID)
		}
		if got := BoardColumnFor(BadgeForInfo(info), ""); got != BoardNeedsYou {
			t.Fatalf("board column = %s, want %s", got, BoardNeedsYou)
		}
		parents, events := notifier.calls()
		if len(events) != 1 || parents[0] != parent.ID {
			t.Fatalf("parent wakes = %#v to %#v, want one wake to %s", events, parents, parent.ID)
		}
		if events[0].Badge != BadgeNeedsAttention || events[0].Reason != SpawnWakeReasonNeedsAttention ||
			events[0].FinalAnswer != "reply" || events[0].Detail != endedEarlyWakeDetail {
			t.Fatalf("parent wake = %#v, want the early-stop wake carrying the final answer", events[0])
		}

		attention, err := db.GetSessionAttention(testutil.Context(t), child.ID)
		if err != nil || attention.EndedEarlyTurnID != info.EndedEarlyTurnID {
			t.Fatalf("durable attention = %#v, %v; want turn %q", attention, err, info.EndedEarlyTurnID)
		}
		durable, err := h.manager.durableSessionInfoForLifecycleTransition(testutil.Context(t), child.ID)
		if err != nil {
			t.Fatalf("durable catalog read error = %v", err)
		}
		if got := BadgeForInfo(durable); got != BadgeNeedsAttention {
			t.Fatalf("durable catalog badge = %s, want needs-attention after a runtime-less read", got)
		}
	})

	t.Run("Should clear the early stop when the next prompt starts", func(t *testing.T) {
		t.Parallel()
		h, db, notifier, _, child := newEarlyStopHarness(t, "", true, nil)
		promptChild(t, h, child)
		h.manager.SetWorkSignalSources(earlyStopWorkSources(WorkSignalActiveMonitor)...)

		promptChild(t, h, child)

		if info := child.Info(); info.EndedEarlyTurnID != "" || BadgeForInfo(info) != BadgeWaiting {
			t.Fatalf("child badge = %s ended_early_turn_id = %q, want waiting with the mark cleared",
				BadgeForInfo(info), info.EndedEarlyTurnID)
		}
		attention, err := db.GetSessionAttention(testutil.Context(t), child.ID)
		if err != nil || attention.EndedEarlyTurnID != "" {
			t.Fatalf("durable attention = %#v, %v; want the mark cleared", attention, err)
		}
		if _, events := notifier.calls(); len(events) != 1 {
			t.Fatalf("parent wakes = %#v, want only the first early-stop wake", events)
		}
	})

	t.Run("Should retry a failed early-stop write before settling attention", func(t *testing.T) {
		t.Parallel()
		h, db, notifier, _, child := newEarlyStopHarness(t, "", true, nil)
		flaky := &flakyEarlyStopAttentionStore{SessionAttentionStore: h.manager.attentionStore, failures: 1}
		h.manager.attentionStore = flaky

		promptChild(t, h, child)

		attention, err := db.GetSessionAttention(testutil.Context(t), child.ID)
		if err != nil || attention.EndedEarlyTurnID == "" || flaky.calls() < 2 {
			t.Fatalf("durable attention = %#v, %v after %d writes; want the retried mark",
				attention, err, flaky.calls())
		}
		if _, events := notifier.calls(); len(events) != 1 || !events[0].EndedEarly {
			t.Fatalf("parent wakes = %#v, want one early-stop wake after the retry", events)
		}
	})

	t.Run("Should flag a child whose remote cannot host a pull request", func(t *testing.T) {
		t.Parallel()
		h, _, notifier, _, child := newEarlyStopHarness(t, "", true, &earlyStopPullRequests{
			saved: "fix/issue", refreshErr: errors.New("no supported remote"), refreshCause: "unsupported_remote",
		})

		promptChild(t, h, child)

		if info := child.Info(); info.EndedEarlyTurnID == "" || BadgeForInfo(info) != BadgeNeedsAttention {
			t.Fatalf("child badge = %s ended_early_turn_id = %q, want needs-attention",
				BadgeForInfo(info), info.EndedEarlyTurnID)
		}
		if _, events := notifier.calls(); len(events) != 1 || !events[0].EndedEarly {
			t.Fatalf("parent wakes = %#v, want one early-stop wake", events)
		}
	})

	for _, tc := range []struct {
		name         string
		open         WorkSignalKind
		withParent   bool
		pullRequests *earlyStopPullRequests
		stopReason   acp.PromptStopReason
	}{
		{name: "Should not flag a child that registered a monitor", open: WorkSignalActiveMonitor, withParent: true},
		{name: "Should not flag a child that delegated work", open: WorkSignalActiveChild, withParent: true},
		{name: "Should not flag a child awaiting another session's reply", open: WorkSignalAwaitingReply, withParent: true},
		{
			name: "Should not flag a child with a linked pull request", withParent: true,
			pullRequests: &earlyStopPullRequests{saved: "fix/issue", prBranch: "fix/issue"},
		},
		{
			name: "Should not flag a child that opened a pull request on a new branch during its turn", withParent: true,
			pullRequests: &earlyStopPullRequests{current: "fix/new", prBranch: "fix/new"},
		},
		{
			name: "Should not flag a child when the pull request lookup fails transiently", withParent: true,
			pullRequests: &earlyStopPullRequests{
				saved: "fix/issue", refreshErr: errors.New("forge timeout"), refreshCause: "provider_failed",
			},
		},
		{
			name: "Should not flag a child whose turn stopped on the token limit", withParent: true,
			stopReason: acp.PromptStopReasonMaxTokens,
		},
		{name: "Should not flag a top-level session without a parent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, _, notifier, _, child := newEarlyStopHarness(t, tc.open, tc.withParent, tc.pullRequests)
			if tc.stopReason != "" {
				h.driver.mu.Lock()
				h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
					events := make(chan acp.AgentEvent, 2)
					events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: req.TurnID, Text: "partial"}
					events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID, PromptStopReason: tc.stopReason}
					close(events)
					return events, nil
				}
				h.driver.mu.Unlock()
			}

			promptChild(t, h, child)

			if info := child.Info(); info.EndedEarlyTurnID != "" || BadgeForInfo(info) == BadgeNeedsAttention {
				t.Fatalf("child badge = %s ended_early_turn_id = %q, want no early-stop flag",
					BadgeForInfo(info), info.EndedEarlyTurnID)
			}
			if _, events := notifier.calls(); len(events) != 0 {
				t.Fatalf("parent wakes = %#v, want none for an opted-out parent", events)
			}
		})
	}
}
