package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestSessionPresenceKeepsPerClientLeasesIndependent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC)
	attentionStore := newPresenceAttentionStore()
	manager := newPresenceTestManager(t, attentionStore, &now)
	ctx := testutil.Context(t)

	leaseA, err := manager.SessionPresence(ctx, "sess-1", "", true)
	if err != nil {
		t.Fatalf("SessionPresence(acquire A) error = %v", err)
	}
	leaseB, err := manager.SessionPresence(ctx, "sess-1", "", true)
	if err != nil {
		t.Fatalf("SessionPresence(acquire B) error = %v", err)
	}
	if leaseA == leaseB || leaseA == "" || leaseB == "" {
		t.Fatalf("presence leases = %q and %q, want distinct opaque ids", leaseA, leaseB)
	}
	if _, err := manager.SessionPresence(ctx, "sess-1", leaseB, false); err != nil {
		t.Fatalf("SessionPresence(release B) error = %v", err)
	}
	manager.presenceMu.Lock()
	liveA := manager.hasLivePresenceLocked("sess-1", now)
	manager.presenceMu.Unlock()
	if !liveA {
		t.Fatal("releasing client B cleared client A's live lease")
	}

	if _, err := manager.SessionPresence(ctx, "sess-2", leaseA, false); !errors.Is(
		err,
		ErrSessionPresenceLeaseMismatch,
	) {
		t.Fatalf("SessionPresence(foreign release) error = %v, want ownership mismatch", err)
	}
	if _, err := manager.SessionPresence(ctx, "sess-1", "unknown", true); !errors.Is(
		err,
		ErrSessionPresenceLeaseNotFound,
	) {
		t.Fatalf("SessionPresence(unknown renew) error = %v, want not found", err)
	}

	if err := manager.settleSessionAttention(ctx, "sess-1", now); err != nil {
		t.Fatalf("settleSessionAttention(live) error = %v", err)
	}
	if got := attentionStore.lastSettledSeen(); !got {
		t.Fatal("settle under client A's lease was not marked seen")
	}

	now = now.Add(SessionPresenceLeaseTTL + time.Millisecond)
	if err := manager.settleSessionAttention(ctx, "sess-1", now); err != nil {
		t.Fatalf("settleSessionAttention(expired) error = %v", err)
	}
	if got := attentionStore.lastSettledSeen(); got {
		t.Fatal("settle after lease expiry was marked seen")
	}
}

func TestSessionPresenceRestoresRenewedLeaseWhenSeenCommitFails(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC)
	attentionStore := newPresenceAttentionStore()
	manager := newPresenceTestManager(t, attentionStore, &now)
	ctx := testutil.Context(t)
	leaseID, err := manager.SessionPresence(ctx, "sess-1", "", true)
	if err != nil {
		t.Fatalf("SessionPresence(acquire) error = %v", err)
	}
	key := sessionPresenceKey{sessionID: "sess-1", leaseID: leaseID}
	manager.presenceMu.Lock()
	previous := manager.presenceLeases[key]
	manager.presenceMu.Unlock()

	attentionStore.setMarkSeenError(errors.New("injected seen failure"))
	now = now.Add(time.Second)
	if _, err := manager.SessionPresence(ctx, "sess-1", leaseID, true); err == nil {
		t.Fatal("SessionPresence(failed renew) error = nil")
	}
	manager.presenceMu.Lock()
	restored, exists := manager.presenceLeases[key]
	manager.presenceMu.Unlock()
	if !exists || !restored.expiresAt.Equal(previous.expiresAt) {
		t.Fatalf("failed renew lease = %#v, want restored %#v", restored, previous)
	}
}

func TestSessionPresenceLinearizesRenewAgainstSettle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC)
	attentionStore := newPresenceAttentionStore()
	manager := newPresenceTestManager(t, attentionStore, &now)
	ctx := testutil.Context(t)
	leaseID, err := manager.SessionPresence(ctx, "sess-1", "", true)
	if err != nil {
		t.Fatalf("SessionPresence(acquire) error = %v", err)
	}
	now = now.Add(SessionPresenceLeaseTTL + time.Millisecond)

	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(2)
	var renewErr, settleErr error
	go func() {
		defer wait.Done()
		<-start
		_, renewErr = manager.SessionPresence(ctx, "sess-1", leaseID, true)
	}()
	go func() {
		defer wait.Done()
		<-start
		settleErr = manager.settleSessionAttention(ctx, "sess-1", now)
	}()
	close(start)
	wait.Wait()
	if settleErr != nil {
		t.Fatalf("settleSessionAttention() error = %v", settleErr)
	}
	if renewErr != nil && !errors.Is(renewErr, ErrSessionPresenceLeaseNotFound) {
		t.Fatalf("SessionPresence(racing renew) error = %v", renewErr)
	}
	if got := attentionStore.settleCallCount(); got != 1 {
		t.Fatalf("settle commits = %d, want exactly one", got)
	}
}

func newPresenceTestManager(
	t *testing.T,
	attentionStore *presenceAttentionStore,
	now *time.Time,
) *Manager {
	t.Helper()
	manager, err := NewManager(
		WithHomePaths(testHomePaths(t)),
		WithNow(func() time.Time { return now.UTC() }),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	cleanupTestManager(t, manager)
	manager.attentionStore = attentionStore
	manager.newPresenceLeaseID = sequentialIDGenerator("lease")
	manager.mu.Lock()
	manager.sessions["sess-1"] = &Session{ID: "sess-1", WorkspaceID: "ws-1", State: StateActive}
	manager.sessions["sess-2"] = &Session{ID: "sess-2", WorkspaceID: "ws-1", State: StateActive}
	manager.mu.Unlock()
	return manager
}

type presenceAttentionStore struct {
	mu          sync.Mutex
	projection  store.SessionAttention
	markSeenErr error
	settledSeen []bool
}

func newPresenceAttentionStore() *presenceAttentionStore {
	return &presenceAttentionStore{
		projection: store.SessionAttention{AttentionRevision: 1},
	}
}

func (s *presenceAttentionStore) GetSessionAttention(context.Context, string) (store.SessionAttention, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projection, nil
}

func (s *presenceAttentionStore) CreatePendingInteraction(
	context.Context,
	store.PendingInteractionCreate,
) (store.SessionAttentionCommit, error) {
	return store.SessionAttentionCommit{}, errors.New("unexpected CreatePendingInteraction call")
}

func (s *presenceAttentionStore) TransitionPendingInteraction(
	context.Context,
	store.PendingInteractionTransition,
) (store.SessionAttentionCommit, error) {
	return store.SessionAttentionCommit{}, errors.New("unexpected TransitionPendingInteraction call")
}

func (s *presenceAttentionStore) ListPendingInteractions(
	context.Context,
	string,
	[]string,
) ([]store.PendingInteraction, error) {
	return nil, nil
}

func (s *presenceAttentionStore) MarkSessionSettled(
	_ context.Context,
	_ string,
	seen bool,
	at time.Time,
) (store.SessionAttentionCommit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := s.projection
	s.projection.AttentionRevision++
	s.projection.LastSettledRevision = s.projection.AttentionRevision
	if seen {
		s.projection.LastSeenRevision = s.projection.AttentionRevision
		seenAt := at.UTC()
		s.projection.LastSeenAt = &seenAt
	}
	changedAt := at.UTC()
	s.projection.AttentionChangedAt = &changedAt
	s.settledSeen = append(s.settledSeen, seen)
	return store.SessionAttentionCommit{Before: before, After: s.projection, Outcome: "settled"}, nil
}

func (s *presenceAttentionStore) MarkSessionSeen(
	_ context.Context,
	_ string,
	at time.Time,
) (store.SessionAttentionCommit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markSeenErr != nil {
		return store.SessionAttentionCommit{}, s.markSeenErr
	}
	before := s.projection
	if s.projection.Unseen() {
		s.projection.AttentionRevision++
		s.projection.LastSeenRevision = s.projection.AttentionRevision
		seenAt := at.UTC()
		s.projection.LastSeenAt = &seenAt
		s.projection.AttentionChangedAt = &seenAt
	}
	return store.SessionAttentionCommit{Before: before, After: s.projection, Outcome: "seen"}, nil
}

func (s *presenceAttentionStore) setMarkSeenError(err error) {
	s.mu.Lock()
	s.markSeenErr = err
	s.mu.Unlock()
}

func (s *presenceAttentionStore) lastSettledSeen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.settledSeen) == 0 {
		return false
	}
	return s.settledSeen[len(s.settledSeen)-1]
}

func (s *presenceAttentionStore) settleCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.settledSeen)
}

// Invariant: subagents never raise unseen-done and parents wait for live delegates; owner: attention (UT-055).
func TestSubagentAttention(t *testing.T) {
	t.Parallel()
	t.Run("Should force child seen and defer parent settle UT-055", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
		attention := newPresenceAttentionStore()
		manager := newPresenceTestManager(t, attention, &now)
		child, _ := manager.Get("sess-1")
		child.mu.Lock()
		child.Lineage = &store.SessionLineage{SpawnRole: store.SubagentSpawnRole}
		child.mu.Unlock()
		if err := manager.settleSessionAttention(t.Context(), child.ID, now); err != nil {
			t.Fatal(err)
		}
		if !attention.lastSettledSeen() {
			t.Fatal("subagent raised unseen attention")
		}
		service, db, _ := newSubagentTestService(t)
		db.rows["live"] = store.SessionSubagent{ID: "live", ParentSessionID: child.ID, Status: "running"}
		manager.SetSubagentService(service)
		calls := attention.settleCallCount()
		if err := manager.settleSessionAttention(t.Context(), child.ID, now); err != nil {
			t.Fatal(err)
		}
		if attention.settleCallCount() != calls {
			t.Fatal("settled while descendants remain live")
		}
	})
}

// Invariant: the last child settles parent attention once, unless a running turn or dispatched wake owns that edge.
// Owner: session attention; canonical UT-055 service boundary.
func TestSubagentAttentionLastChild(t *testing.T) {
	for _, state := range []string{"idle", "running", "dispatched"} {
		t.Run("Should settle parent attention exactly once when "+state, func(t *testing.T) {
			t.Parallel()
			s, db, r := newSubagentTestService(t)
			now := s.now()
			attention := newPresenceAttentionStore()
			manager := newPresenceTestManager(t, attention, &now)
			manager.SetSubagentService(s)
			s.runtime = attentionSubagentRuntime{subagentTestRuntime: r, manager: manager}
			first := requireSubagent(t, s, subagentTestRequest())
			req := subagentTestRequest()
			req.IdempotencyKey = "last"
			last := requireSubagent(t, s, req)
			snap := r.snapshots["parent"]
			snap.Active = state == "running"
			r.snapshots["parent"] = snap
			if state == "dispatched" {
				db.rows["prior"] = store.SessionSubagent{
					ID:              "prior",
					ParentSessionID: "parent",
					Status:          store.SubagentStatusCompleted,
					Delivery:        store.SubagentDeliveryClaimed,
					WakeMessageID:   new("prior-wake"),
				}
				db.wakes["prior-wake"] = store.SessionSubagentWake{
					WakeMessageID:   "prior-wake",
					ParentSessionID: "parent",
					State:           store.SubagentWakeStateDispatched,
				}
			}
			before, err := attention.GetSessionAttention(t.Context(), "parent")
			if err != nil {
				t.Fatal(err)
			}
			settleTestChild(t, s, r, &first)
			if attention.settleCallCount() != 0 {
				t.Fatal("settled while one child is live")
			}
			settleTestChild(t, s, r, &last)
			if err := s.OnChildSettled(t.Context(), *last.ChildSessionID); err != nil {
				t.Fatal(err)
			}
			after, err := attention.GetSessionAttention(t.Context(), "parent")
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if state == "idle" {
				want = 1
			}
			if attention.settleCallCount() != want {
				t.Fatal(attention.settleCallCount(), want)
			}
			if want == 1 && (after.LastSettledRevision != before.AttentionRevision+1 || !after.Unseen()) {
				t.Fatal(before, after)
			}
			if want == 0 && after.LastSettledRevision != before.LastSettledRevision {
				t.Fatal(before, after)
			}
		})
	}
}

type attentionSubagentRuntime struct {
	*subagentTestRuntime
	manager *Manager
}

func (r attentionSubagentRuntime) SettleParent(ctx context.Context, parent string) error {
	return r.manager.settleSessionAttention(ctx, parent, r.manager.now())
}
