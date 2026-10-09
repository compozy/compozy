package session

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

type memorySubagents struct {
	mu       sync.Mutex
	rows     map[string]store.SessionSubagent
	wakes    map[string]store.SessionSubagentWake
	writes   int
	rewrites map[string]string
	orphans  []string
}

var _ store.SubagentStore = (*memorySubagents)(nil)

func (d *memorySubagents) ReserveSubagent(
	_ context.Context,
	row store.SessionSubagent,
) (store.SessionSubagent, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, old := range d.rows {
		if old.ParentSessionID == row.ParentSessionID && old.IdempotencyKey == row.IdempotencyKey {
			if old.RequestFingerprint != row.RequestFingerprint {
				return old, false, store.ErrSubagentIdempotencyConflict
			}
			return old, false, nil
		}
	}
	d.rows[row.ID] = row
	return row, true, nil
}
func (d *memorySubagents) LinkChild(_ context.Context, id, child string, at time.Time) (store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.rows[id]
	row.ChildSessionID = new(child)
	row.StartedAt = new(at)
	row.Status = store.SubagentStatusRunning
	d.rows[id] = row
	return row, nil
}
func (d *memorySubagents) MarkFirstPromptAdmitted(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.rows[id]
	row.PendingTask = nil
	d.rows[id] = row
	return nil
}
func (d *memorySubagents) DeleteReserved(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.rows[id]
	if row.Status == store.SubagentStatusQueued && row.ChildSessionID == nil {
		delete(d.rows, id)
	}
	return nil
}
func (d *memorySubagents) GetSubagent(_ context.Context, ws, id string) (store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	row, ok := d.rows[id]
	if !ok || (ws != "" && row.WorkspaceID != ws) {
		return row, store.ErrSubagentNotFound
	}
	return row, nil
}
func (d *memorySubagents) GetSubagentByChild(_ context.Context, child string) (store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, row := range d.rows {
		if row.ChildSessionID != nil && *row.ChildSessionID == child {
			return row, nil
		}
	}
	return store.SessionSubagent{}, store.ErrSubagentNotFound
}
func (d *memorySubagents) ListSubagents(_ context.Context, q store.SubagentListQuery) (store.SubagentPage, error) {
	if q.WorkspaceID == "" && q.ParentSessionID == "" {
		return store.SubagentPage{}, errors.New("subagent workspace or parent required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var page store.SubagentPage
	for _, row := range d.rows {
		if (q.WorkspaceID == "" || q.WorkspaceID == row.WorkspaceID) &&
			(q.ParentSessionID == "" || q.ParentSessionID == row.ParentSessionID) {
			page.Items = append(page.Items, row)
		}
	}
	slices.SortFunc(page.Items, func(a, b store.SessionSubagent) int { return cmp.Compare(a.ID, b.ID) })
	return page, nil
}
func (d *memorySubagents) Summaries(_ context.Context, ids []string) (map[string]store.SubagentSummary, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make(map[string]store.SubagentSummary)
	for _, id := range ids {
		sum := store.SubagentSummary{}
		for _, row := range d.rows {
			if row.ParentSessionID == id {
				sum.Total++
				if !store.IsSubagentStatusTerminal(row.Status) {
					sum.Live++
				}
			}
		}
		result[id] = sum
	}
	return result, nil
}
func (d *memorySubagents) UpdateProgress(_ context.Context, id, text string, at time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.rows[id]
	row.Progress = text
	row.UpdatedAt = at
	d.rows[id] = row
	d.writes++
	return nil
}

func (d *memorySubagents) UpdateSubagentState(
	_ context.Context,
	id, status, work string,
	at time.Time,
) (store.SessionSubagent, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.rows[id]
	changed := !store.IsSubagentStatusTerminal(row.Status) && (row.Status != status || row.WorkState != work)
	if changed {
		row.Status = status
		row.WorkState = work
		row.UpdatedAt = at
		d.rows[id] = row
	}
	return row, changed, nil
}
func (d *memorySubagents) RewriteSubagentWakeInput(_ context.Context, id, text string, _ json.RawMessage) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rewrites[id] = text
	return nil
}

func (d *memorySubagents) FinalizeSubagent(
	_ context.Context,
	in store.SubagentFinalize,
) (store.SessionSubagent, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	row, ok := d.rows[in.ID]
	if !ok {
		return row, false, store.ErrSubagentNotFound
	}
	if store.IsSubagentStatusTerminal(row.Status) {
		return row, false, nil
	}
	row.Status = in.Status
	row.WorkState = in.WorkState
	row.Result = in.Result
	row.Error = in.Error
	row.ResultTruncated = in.ResultTruncated
	row.SettledAt = new(in.SettledAt)
	row.PendingTask = nil
	d.rows[row.ID] = row
	return row, true, nil
}

func (d *memorySubagents) OpenOrJoinWake(
	_ context.Context,
	parent string,
	ids []string,
	id string,
) (store.SessionSubagentWake, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	wake := store.SessionSubagentWake{WakeMessageID: id, ParentSessionID: parent, State: store.SubagentWakeStateOpen}
	for _, old := range d.wakes {
		if old.ParentSessionID == parent && old.State == store.SubagentWakeStateOpen {
			wake = old
			break
		}
	}
	for _, id := range ids {
		row := d.rows[id]
		if store.IsSubagentDeliveryFinal(row.Delivery) {
			continue
		}
		if row.WakeMessageID != nil {
			wake.Attempts = max(wake.Attempts, d.wakes[*row.WakeMessageID].Attempts)
		}
		row.Delivery = store.SubagentDeliveryClaimed
		row.WakeMessageID = new(wake.WakeMessageID)
		d.rows[id] = row
		wake.WorkspaceID = row.WorkspaceID
	}
	d.wakes[wake.WakeMessageID] = wake
	return wake, nil
}

func (d *memorySubagents) GetWake(
	_ context.Context,
	id string,
) (store.SessionSubagentWake, []store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	wake, ok := d.wakes[id]
	if !ok {
		return wake, nil, store.ErrSubagentWakeNotFound
	}
	var rows []store.SessionSubagent
	for _, row := range d.rows {
		if row.WakeMessageID != nil && *row.WakeMessageID == id && row.Delivery == store.SubagentDeliveryClaimed {
			rows = append(rows, row)
		}
	}
	return wake, rows, nil
}
func (d *memorySubagents) SetWakeInput(_ context.Context, id, route, input string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	wake := d.wakes[id]
	if wake.State != store.SubagentWakeStateOpen {
		return nil
	}
	wake.Route = route
	wake.InputEntryID = input
	d.wakes[id] = wake
	return nil
}
func (d *memorySubagents) MarkWakeSteerRequeued(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	wake := d.wakes[id]
	if wake.State != store.SubagentWakeStateOpen && wake.State != store.SubagentWakeStateDispatched {
		return nil
	}
	wake.State = store.SubagentWakeStateOpen
	wake.Route = store.SubagentWakeRouteQueue
	wake.SteerRequeued = true
	d.wakes[id] = wake
	return nil
}
func (d *memorySubagents) MarkWakeDispatched(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	wake := d.wakes[id]
	wake.State = store.SubagentWakeStateDispatched
	d.wakes[id] = wake
	return nil
}
func (d *memorySubagents) SettleWake(_ context.Context, id string, canceled bool) ([]store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	wake := d.wakes[id]
	wake.State = store.SubagentWakeStateSettled
	if canceled {
		wake.State = store.SubagentWakeStateCanceled
	}
	d.wakes[id] = wake
	var rows []store.SessionSubagent
	for key, row := range d.rows {
		if row.WakeMessageID != nil && *row.WakeMessageID == id && row.Delivery == store.SubagentDeliveryClaimed {
			row.Delivery = store.SubagentDeliveryDelivered
			if canceled {
				row.Delivery = store.SubagentDeliveryPending
				if wake.Attempts == 0 {
					row.WakeMessageID = nil
				}
			}
			d.rows[key] = row
			rows = append(rows, row)
		}
	}
	return rows, nil
}
func (d *memorySubagents) SetPending(_ context.Context, ids []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, id := range ids {
		row := d.rows[id]
		if !store.IsSubagentDeliveryFinal(row.Delivery) {
			row.Delivery = store.SubagentDeliveryPending
			d.rows[id] = row
		}
	}
	return nil
}

func (d *memorySubagents) Acknowledge(
	_ context.Context,
	id, turn string,
) (store.SessionSubagent, *store.SessionSubagentWake, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.rows[id]
	row.Delivery = store.SubagentDeliveryAcknowledged
	row.AcknowledgedTurnID = turn
	var wake *store.SessionSubagentWake
	if row.WakeMessageID != nil {
		value := d.wakes[*row.WakeMessageID]
		if value.State == store.SubagentWakeStateOpen {
			wake = &value
			row.WakeMessageID = nil
		}
	}
	d.rows[id] = row
	return row, wake, nil
}
func (d *memorySubagents) Dispose(_ context.Context, f store.SubagentDisposeFilter) ([]store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var rows []store.SessionSubagent
	for id, row := range d.rows {
		selected := len(f.IDs) == 0
		for _, want := range f.IDs {
			selected = selected || want == id
		}
		if row.Origin != store.SubagentOriginProviderNative && row.ParentSessionID == f.ParentSessionID &&
			(f.ParentTurnID == "" || row.ParentTurnID == f.ParentTurnID) &&
			selected &&
			!store.IsSubagentDeliveryFinal(row.Delivery) {
			row.Delivery = store.SubagentDeliveryDisposed
			row.WakeMessageID = nil
			d.rows[id] = row
			rows = append(rows, row)
		}
	}
	return rows, nil
}
func (d *memorySubagents) UpgradeWakePolicy(_ context.Context, id string) (store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.rows[id]
	row.WakePolicy = store.SubagentWakePolicyAlways
	d.rows[id] = row
	return row, nil
}
func (d *memorySubagents) ListStaleReserved(_ context.Context, at time.Time) ([]store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var rows []store.SessionSubagent
	for _, row := range d.rows {
		if row.Status == store.SubagentStatusQueued && row.CreatedAt.Before(at) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}
func (d *memorySubagents) ListUnfinalizedDelegated(_ context.Context) ([]store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var rows []store.SessionSubagent
	for _, row := range d.rows {
		if row.Origin == store.SubagentOriginDelegated && !store.IsSubagentStatusTerminal(row.Status) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}
func (d *memorySubagents) ListOpenWakes(_ context.Context) ([]store.SessionSubagentWake, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var wakes []store.SessionSubagentWake
	for _, wake := range d.wakes {
		if wake.State == store.SubagentWakeStateOpen || wake.State == store.SubagentWakeStateDispatched {
			wakes = append(wakes, wake)
		}
	}
	return wakes, nil
}
func (d *memorySubagents) ListPending(_ context.Context) ([]store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var rows []store.SessionSubagent
	for _, row := range d.rows {
		if row.Delivery == store.SubagentDeliveryPending {
			rows = append(rows, row)
		}
	}
	return rows, nil
}
func (d *memorySubagents) ListOrphanSubagentSessions(context.Context) ([]string, error) {
	return d.orphans, nil
}

type subagentTestRuntime struct {
	inputStatuses                   map[string]string
	errorResume                     error
	mu                              sync.Mutex
	snapshots                       map[string]subagentSnapshot
	spawned                         []SpawnOpts
	admitted                        map[string]string
	stopped                         []string
	results                         map[string]string
	turnErrors                      map[string]string
	queues                          map[string]string
	publishes                       []string
	settles                         int
	spawnErr, errorAdmit, errorStop error
	steer                           acp.SteerResult
	steerErr                        error
	spawnBlock                      chan struct{}
}

var _ subagentRuntime = (*subagentTestRuntime)(nil)

func (r *subagentTestRuntime) Snapshot(_ context.Context, id string) (subagentSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.snapshots[id]
	if !ok {
		return snap, ErrSessionNotFound
	}
	return snap, nil
}
func (r *subagentTestRuntime) Resolve(_ context.Context, info *Info, target SubagentTarget) (SubagentTarget, error) {
	return resolveSubagentTarget(subagentInherited(info), target, SubagentTarget{Model: "default"}), nil
}

func (r *subagentTestRuntime) Capabilities(
	context.Context,
	*Info,
) ([]SubagentAgentOption, []SubagentProviderOption, error) {
	return nil, nil, nil
}
func (r *subagentTestRuntime) Spawn(_ context.Context, opts SpawnOpts) (string, error) {
	if r.spawnBlock != nil {
		<-r.spawnBlock
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spawned = append(r.spawned, opts)
	if r.spawnErr != nil {
		return "", r.spawnErr
	}
	id := fmt.Sprintf("child-%d", len(r.spawned))
	r.snapshots[id] = subagentSnapshot{
		Info:   &Info{ID: id, WorkspaceID: "ws", State: StateActive},
		Active: true,
		TurnID: "child-turn",
	}
	return id, nil
}
func (r *subagentTestRuntime) Admit(_ context.Context, row store.SessionSubagent, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.errorAdmit != nil {
		return r.errorAdmit
	}
	r.admitted[row.ID] = text
	return nil
}
func (r *subagentTestRuntime) HasAdmission(_ context.Context, row store.SessionSubagent) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.admitted[row.ID]
	return ok, nil
}
func (r *subagentTestRuntime) Stop(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = append(r.stopped, id)
	if r.errorStop != nil {
		return r.errorStop
	}
	snap := r.snapshots[id]
	if snap.Info != nil {
		updated := *snap.Info
		updated.State = StateStopped
		snap.Info = &updated
		snap.Active = false
		r.snapshots[id] = snap
	}
	return nil
}
func (r *subagentTestRuntime) Result(_ context.Context, id string) (subagentTurnResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return subagentTurnResult{Text: r.results[id], Error: r.turnErrors[id]}, nil
}

func (r *subagentTestRuntime) QueueWake(
	_ context.Context,
	wake store.SessionSubagentWake,
	rows []store.SessionSubagent,
) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queues[wake.WakeMessageID] = subagentWakeText(rows)
	return "input-" + wake.WakeMessageID, nil
}
func (r *subagentTestRuntime) CancelWake(_ context.Context, wake store.SessionSubagentWake) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.queues, wake.WakeMessageID)
	return nil
}
func (r *subagentTestRuntime) Steer(context.Context, string, string, string, string) (acp.SteerResult, error) {
	return r.steer, r.steerErr
}
func (r *subagentTestRuntime) PublishParent(_ context.Context, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.publishes = append(r.publishes, id)
}
func (r *subagentTestRuntime) SettleParent(context.Context, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settles++
	return nil
}
func newSubagentTestService(t *testing.T) (*subagentService, *memorySubagents, *subagentTestRuntime) {
	t.Helper()
	db := &memorySubagents{
		rows:     make(map[string]store.SessionSubagent),
		wakes:    make(map[string]store.SessionSubagentWake),
		rewrites: make(map[string]string),
	}
	runtime := &subagentTestRuntime{
		snapshots: map[string]subagentSnapshot{
			"parent": {
				Info: &Info{
					ID:                   "parent",
					WorkspaceID:          "ws",
					State:                StateActive,
					AgentName:            "coder",
					Provider:             "codex",
					Model:                "sol",
					EffectivePermissions: "approve-reads",
				},
				TurnID: "turn",
				Active: true,
			},
		},
		admitted:   make(map[string]string),
		results:    make(map[string]string),
		turnErrors: make(map[string]string),
		queues:     make(map[string]string),
	}
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	s := &subagentService{
		store:       db,
		runtime:     runtime,
		ctx:         ctx,
		now:         func() time.Time { return time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC) },
		resultLimit: func(context.Context, string) (int, error) { return 60000, nil },
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		parents:     make(map[string]*subagentParentLock),
		flights:     make(map[string]*subagentFlight),
		subscribers: make(map[string]map[*subagentSubscription]struct{}),
		progress:    make(map[string]*subagentProgress),
	}
	var idMu sync.Mutex
	next := 0
	s.newID = func() (string, error) {
		idMu.Lock()
		defer idMu.Unlock()
		next++
		return fmt.Sprintf("wake-%d", next), nil
	}
	s.launch = func(f func()) { wg.Go(f) }
	t.Cleanup(func() { cancel(); wg.Wait() })
	return s, db, runtime
}
func subagentTestRequest() SubagentRequest {
	return SubagentRequest{
		Caller: SubagentCaller{WorkspaceID: "ws", SessionID: "parent", TurnID: "turn", ToolCallID: "call"},
		Task:   "Review this change",
		Title:  "Review",
		Role:   "general",
	}
}
func requireSubagent(t *testing.T, s *subagentService, req SubagentRequest) Subagent {
	t.Helper()
	row, err := s.Delegate(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	return row
}
func settleTestChild(t *testing.T, s *subagentService, r *subagentTestRuntime, row *Subagent) {
	t.Helper()
	r.mu.Lock()
	snap := r.snapshots[*row.ChildSessionID]
	snap.Active = false
	r.snapshots[*row.ChildSessionID] = snap
	r.mu.Unlock()
	if err := s.OnChildSettled(t.Context(), *row.ChildSessionID); err != nil {
		t.Fatal(err)
	}
}
func testSubagentError() error { return errors.New("provider failed") }

func (d *memorySubagents) UpdateNativeSubagentTitle(_ context.Context, id, title string, at time.Time) (store.SessionSubagent, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.rows[id]
	changed := row.Origin == store.SubagentOriginProviderNative && row.Title != title
	if changed {
		row.Title, row.UpdatedAt = title, at
		d.rows[id] = row
	}
	return row, changed, nil
}

func (d *memorySubagents) GetSubagentByID(ctx context.Context, id string) (store.SessionSubagent, error) {
	return d.GetSubagent(ctx, "", id)
}

func (d *memorySubagents) ListWakesByParent(
	_ context.Context,
	parent string,
	states []string,
) ([]store.SessionSubagentWake, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []store.SessionSubagentWake
	for _, wake := range d.wakes {
		if wake.ParentSessionID == parent && (len(states) == 0 || slices.Contains(states, wake.State)) {
			out = append(out, wake)
		}
	}
	return out, nil
}

func (d *memorySubagents) ListUnfinalizedNative(context.Context) ([]store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var rows []store.SessionSubagent
	for _, row := range d.rows {
		if row.Origin == store.SubagentOriginProviderNative && !store.IsSubagentStatusTerminal(row.Status) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (d *memorySubagents) FailWake(
	_ context.Context,
	id string,
) (store.SessionSubagentWake, []store.SessionSubagent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	wake := d.wakes[id]
	if wake.State != store.SubagentWakeStateOpen && wake.State != store.SubagentWakeStateDispatched {
		return wake, nil, nil
	}
	wake.Attempts++
	wake.State = store.SubagentWakeStateCanceled
	d.wakes[id] = wake
	var rows []store.SessionSubagent
	for key, row := range d.rows {
		if row.WakeMessageID == nil || *row.WakeMessageID != id || row.Delivery != store.SubagentDeliveryClaimed {
			continue
		}
		row.Delivery = store.SubagentDeliveryPending
		if wake.Attempts >= 3 {
			row.Delivery = store.SubagentDeliveryDisposed
		}
		d.rows[key] = row
		rows = append(rows, row)
	}
	return wake, rows, nil
}
func (r *subagentTestRuntime) WakeInputStatus(_ context.Context, wake store.SessionSubagentWake) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inputStatuses[wake.InputEntryID], nil
}
func (r *subagentTestRuntime) ResumeChild(_ context.Context, row store.SessionSubagent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.errorResume != nil {
		return r.errorResume
	}
	snap := r.snapshots[*row.ChildSessionID]
	info := *snap.Info
	info.State = StateActive
	info.StopReason = ""
	snap.Info = &info
	snap.Active = true
	r.snapshots[*row.ChildSessionID] = snap
	return nil
}
