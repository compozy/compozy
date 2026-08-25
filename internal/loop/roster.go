package loop

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

var ErrInvalidRosterCursor = errors.New("invalid_cursor")

const (
	nodeCancellationOperator = "operator"
	nodeCancellationStrategy = "strategy"
)

// NodeState is the closed public roster state vocabulary.
type NodeState string

const (
	NodeStatePending        NodeState = "pending"
	NodeStateQueued         NodeState = "queued"
	NodeStateRunning        NodeState = "running"
	NodeStateRetrying       NodeState = "retrying"
	NodeStateWaiting        NodeState = "waiting"
	NodeStatePaused         NodeState = "paused"
	NodeStateAwaitingChild  NodeState = "awaiting_child"
	NodeStateControlPending NodeState = "control_pending"
	NodeStateAwaitingGoal   NodeState = "awaiting_goal"
	NodeStateSucceeded      NodeState = "succeeded"
	NodeStatePartial        NodeState = "partial"
	NodeStateFailed         NodeState = "failed"
	NodeStateCanceled       NodeState = "canceled"
	NodeStateQuarantined    NodeState = "quarantined"
	NodeStateNotTaken       NodeState = "not_taken"
)

// NodeStateFilter is the deliberately narrower CLI/HTTP filter vocabulary.
type NodeStateFilter string

const NodeStateFilterAll NodeStateFilter = "all"

var nodeStateFilters = []NodeStateFilter{
	NodeStateFilterAll, NodeStateFilter(NodeStateRunning), NodeStateFilter(NodeStateQueued),
	NodeStateFilter(NodeStateWaiting), NodeStateFilter(NodeStateRetrying), NodeStateFilter(NodeStatePaused),
	NodeStateFilter(NodeStateQuarantined), NodeStateFilter(NodeStateSucceeded), NodeStateFilter(NodeStateFailed),
	NodeStateFilter(NodeStateCanceled), NodeStateFilter(NodeStateNotTaken),
}

type RosterQuery struct {
	State      NodeStateFilter
	Generation int
	Cursor     string
	Limit      int
}

type NodeCancellation struct {
	Disposition string `json:"disposition"`
	ActorKind   string `json:"actor_kind,omitempty"`
	ActorRef    string `json:"actor_ref,omitempty"`
	Cause       string `json:"cause,omitempty"`
}

type NodeAttemptView struct {
	Attempt      int        `json:"attempt"`
	State        NodeState  `json:"state"`
	FailureClass string     `json:"failure_class,omitempty"`
	Disposition  string     `json:"disposition"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

type NodeUsage struct {
	Tokens int64 `json:"tokens"`
}

type RosterNode struct {
	Generation     int               `json:"generation"`
	NodeID         NodeID            `json:"node_id"`
	ItemIndex      int               `json:"item_index"`
	State          NodeState         `json:"state"`
	Attempt        int               `json:"attempt"`
	Attempts       []NodeAttemptView `json:"attempts"`
	NextRetryAt    *time.Time        `json:"next_retry_at,omitempty"`
	ChildLoopRunID string            `json:"child_loop_run_id,omitempty"`
	Cancellation   *NodeCancellation `json:"cancellation,omitempty"`
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	EndedAt        *time.Time        `json:"ended_at,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	CellTaskID     string            `json:"cell_task_id,omitempty"`
	Usage          *NodeUsage        `json:"usage,omitempty"`
	Action         bool              `json:"-"`
}

type FanoutRollup struct {
	Generation int    `json:"generation"`
	NodeID     NodeID `json:"node_id"`
	Done       int    `json:"done"`
	Total      int    `json:"total"`
	Failed     int    `json:"failed"`
}

type RosterPage struct {
	RunID         RunID          `json:"run_id"`
	LoopName      string         `json:"loop_name"`
	RunStatus     Status         `json:"run_status"`
	Nodes         []RosterNode   `json:"nodes"`
	FanoutRollups []FanoutRollup `json:"fanout_rollups"`
	NextCursor    string         `json:"next_cursor,omitempty"`
}

type RosterSource struct {
	Run         Run
	Graph       dsl.Graph
	Generations []LoopGeneration
	Outputs     []GenerationOutput
	Attempts    []NodeAttempt
	Controls    []NodeControl
	Waits       []NodeWait
	RouteCauses []RouteCause
	PrunedNodes map[string]bool
	Outcome     *RunOutcome
}

// MarkPrunedNodeItem adds exact durable branch-pruning evidence to a roster source.
func (s *RosterSource) MarkPrunedNodeItem(generation int, nodeID NodeID, itemIndex int) error {
	if itemIndex < 0 {
		return fmt.Errorf("%w: pruned node item index must be zero or positive", ErrValidation)
	}
	if s.PrunedNodes == nil {
		s.PrunedNodes = make(map[string]bool)
	}
	s.PrunedNodes[rosterKey(generation, nodeID, itemIndex)] = true
	return nil
}

type rosterCursor struct {
	Offset int `json:"offset"`
}

type InvalidNodeStateError struct {
	State   NodeStateFilter
	Allowed []string
}

func (e *InvalidNodeStateError) Error() string {
	return fmt.Sprintf(
		"invalid node state %q; allowed: %s",
		e.State,
		strings.Join(e.Allowed, "|"),
	)
}

func (e *InvalidNodeStateError) Unwrap() error {
	return ErrValidation
}

// ProjectRoster computes the complete node-generation roster without writes.
func ProjectRoster(source *RosterSource, query RosterQuery) (RosterPage, error) {
	query, err := normalizeRosterQuery(query)
	if err != nil {
		return RosterPage{}, err
	}
	page := RosterPage{
		RunID: source.Run.ID, LoopName: source.Run.LoopName, RunStatus: source.Run.Status,
		Nodes: []RosterNode{}, FanoutRollups: []FanoutRollup{},
	}
	if source.Run.Generation < 1 {
		return page, nil
	}
	for _, output := range source.Outputs {
		if _, err := mapOutputState(output.Status); err != nil {
			return RosterPage{}, err
		}
	}
	generations := rosterGenerations(source)
	outputs, maxOutputItems := indexOutputs(source.Outputs)
	attempts := indexAttempts(source.Attempts)
	controls := indexControls(source.Controls)
	waits := indexWaits(source.Waits)
	graphNodes := flattenGraphNodes(source.Graph)
	for _, generation := range generations {
		if query.Generation > 0 && query.Generation != generation {
			continue
		}
		routeExcluded := excludedRoutes(
			graphNodes,
			source.RouteCauses,
			source.PrunedNodes,
			generation,
		)
		for _, node := range graphNodes {
			key := rosterKey(generation, node.ID, 0)
			items := rosterItems(
				source.Run.ID, generation, node, outputs, maxOutputItems,
				attempts, controls, waits, routeExcluded,
			)
			if len(items) == 0 {
				items = []RosterNode{newRosterNode(
					source.Run.ID,
					generation,
					node,
					0,
					outputs[key],
					attempts[key],
					controls[node.ID],
					waits[key],
					routeExcluded[key],
				)}
			}
			page.Nodes = append(page.Nodes, items...)
		}
	}
	page.FanoutRollups = buildFanoutRollups(page.Nodes, source.Graph)
	filtered := filterRosterNodes(page.Nodes, query.State)
	offset, err := decodeRosterCursor(query.Cursor)
	if err != nil {
		return RosterPage{}, err
	}
	if offset > len(filtered) {
		return RosterPage{}, fmt.Errorf("%w: roster cursor is beyond the result set", ErrInvalidRosterCursor)
	}
	end := min(offset+query.Limit, len(filtered))
	page.Nodes = filtered[offset:end]
	if end < len(filtered) {
		page.NextCursor = encodeRosterCursor(rosterCursor{Offset: end})
	}
	return page, nil
}

func normalizeRosterQuery(query RosterQuery) (RosterQuery, error) {
	if query.State == "" {
		query.State = NodeStateFilterAll
	}
	if !slices.Contains(nodeStateFilters, query.State) {
		return query, &InvalidNodeStateError{
			State: query.State, Allowed: NodeStateFilterValues(),
		}
	}
	if query.Generation < 0 {
		return query, fmt.Errorf("%w: generation must not be negative", ErrValidation)
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 500 {
		return query, fmt.Errorf("%w: roster limit must be between 1 and 500", ErrValidation)
	}
	return query, nil
}

func NodeStateFilterValues() []string {
	values := make([]string, len(nodeStateFilters))
	for i, value := range nodeStateFilters {
		values[i] = string(value)
	}
	return values
}

func mapOutputState(state string) (NodeState, error) {
	switch strings.TrimSpace(state) {
	case "", generationOutputPending:
		return NodeStatePending, nil
	case generationOutputEnqueued:
		return NodeStateQueued, nil
	case generationOutputRunning:
		return NodeStateRunning, nil
	case generationOutputRetrying:
		return NodeStateRetrying, nil
	case generationOutputWaiting:
		return NodeStateWaiting, nil
	case generationOutputPaused:
		return NodeStatePaused, nil
	case generationOutputAwaitingChild:
		return NodeStateAwaitingChild, nil
	case generationOutputControlPending:
		return NodeStateControlPending, nil
	case generationOutputAwaitingGoal:
		return NodeStateAwaitingGoal, nil
	case generationOutputSucceeded:
		return NodeStateSucceeded, nil
	case generationOutputPartial:
		return NodeStatePartial, nil
	case generationOutputFailed:
		return NodeStateFailed, nil
	case generationOutputCanceled:
		return NodeStateCanceled, nil
	case generationOutputQuarantined:
		return NodeStateQuarantined, nil
	default:
		return "", fmt.Errorf("%w: unsupported persisted node state %q", ErrValidation, state)
	}
}

func newRosterNode(
	runID RunID,
	generation int,
	node dsl.Node,
	item int,
	output *GenerationOutput,
	history []NodeAttempt,
	control *NodeControl,
	wait *NodeWait,
	notTaken bool,
) RosterNode {
	view := RosterNode{
		Generation: generation, NodeID: node.ID, ItemIndex: item, State: NodeStatePending,
		Attempts: []NodeAttemptView{}, Action: node.Class == dsl.NodeClassAction,
	}
	if notTaken {
		view.State = NodeStateNotTaken
		return view
	}
	if output != nil {
		applyRosterOutput(&view, runID, *output)
	}
	openAttempt := appendRosterAttempts(&view, history)
	if openAttempt {
		markRosterOpenAttemptRunning(&view)
	}
	applyRosterWait(&view, wait)
	applyRosterCancellation(&view, output, control)
	return view
}

func applyRosterOutput(view *RosterNode, runID RunID, output GenerationOutput) {
	state, err := mapOutputState(output.Status)
	if err == nil {
		view.State = state
	}
	view.Attempt = output.Attempt
	view.NextRetryAt = output.NextAttemptAt
	view.ChildLoopRunID = output.ChildLoopRunID
	view.SessionID = output.SessionID
	view.CellTaskID = NodeCellTaskID(runID, view.Generation, string(view.NodeID), view.ItemIndex)
	if output.FirstScheduledAt != nil {
		started := output.FirstScheduledAt.UTC()
		view.StartedAt = &started
	}
	if output.NextAttemptAt != nil {
		view.State = NodeStateRetrying
	}
	view.State = overlayTaskRunState(view.State, output.TaskRunStatus)
	if output.TaskRunTokensUsed > 0 {
		view.Usage = &NodeUsage{Tokens: output.TaskRunTokensUsed}
	}
}

func appendRosterAttempts(view *RosterNode, history []NodeAttempt) bool {
	openAttempt := false
	for _, attempt := range history {
		state := NodeStateRunning
		if attempt.EndedAt != nil {
			state = attemptState(attempt.Disposition)
		} else {
			openAttempt = true
		}
		failure := ""
		if attempt.FailureClass != nil {
			failure = string(*attempt.FailureClass)
		}
		view.Attempts = append(view.Attempts, NodeAttemptView{
			Attempt: attempt.Attempt, State: state, FailureClass: failure,
			Disposition: string(attempt.Disposition), StartedAt: attempt.StartedAt, EndedAt: attempt.EndedAt,
		})
		if attempt.EndedAt != nil {
			view.EndedAt = attempt.EndedAt
		}
	}
	return openAttempt
}

func markRosterOpenAttemptRunning(view *RosterNode) {
	switch view.State {
	case NodeStatePending, NodeStateQueued, NodeStateRunning, NodeStateRetrying:
		view.State = NodeStateRunning
	}
}

func applyRosterWait(view *RosterNode, wait *NodeWait) {
	if wait != nil && (wait.ClaimState == WaitClaimWaiting || wait.ClaimState == WaitClaimInterventionRequired) {
		view.State = NodeStateWaiting
	}
}

func applyRosterCancellation(
	view *RosterNode,
	output *GenerationOutput,
	control *NodeControl,
) {
	if output != nil && view.State == NodeStateCanceled {
		view.Cancellation = strategyCancellationView(*output)
	}
	if control != nil {
		switch {
		case control.Quarantined:
			view.State = NodeStateQuarantined
		case control.Paused:
			view.State = NodeStatePaused
		case control.CancelState != CancelStateNone:
			view.State = NodeStateCanceled
			view.Cancellation = cancellationView(*control)
		}
	}
}

func overlayTaskRunState(current NodeState, status task.RunStatus) NodeState {
	switch status.Normalize() {
	case task.TaskRunStatusQueued:
		if current == NodeStatePending {
			return NodeStateQueued
		}
	case task.TaskRunStatusClaimed, task.TaskRunStatusStarting, task.TaskRunStatusRunning:
		switch current {
		case NodeStatePending, NodeStateQueued, NodeStateRunning, NodeStateRetrying:
			return NodeStateRunning
		}
	case task.TaskRunStatusCompleted:
		// The worker completion and the coordinator output projection are separate
		// commits. Keep a dispatched node live during that handoff instead of
		// briefly regressing it to pending or queued on public reads.
		switch current {
		case NodeStatePending, NodeStateQueued, NodeStateRunning, NodeStateRetrying:
			return NodeStateRunning
		}
	}
	return current
}

func cancellationView(control NodeControl) *NodeCancellation {
	result := &NodeCancellation{Disposition: nodeCancellationOperator}
	if control.CancelProvenance != nil {
		result.ActorKind = control.CancelProvenance.ActorKind
		result.ActorRef = control.CancelProvenance.ActorID
		result.Cause = control.CancelProvenance.Reason
	}
	return result
}

func strategyCancellationView(output GenerationOutput) *NodeCancellation {
	if generationOutputHasKind(output, GenerationResultStrategyCanceled) {
		return &NodeCancellation{Disposition: nodeCancellationStrategy, Cause: strategyCanceledReasonCode}
	}
	if generationOutputHasKind(output, GenerationResultStrategyNotStarted) {
		return &NodeCancellation{Disposition: nodeCancellationStrategy, Cause: strategyNeverStartedReasonCode}
	}
	return nil
}

func attemptState(disposition AttemptDisposition) NodeState {
	switch disposition {
	case AttemptSucceeded:
		return NodeStateSucceeded
	case AttemptCanceled:
		return NodeStateCanceled
	case AttemptQuarantined:
		return NodeStateQuarantined
	case AttemptRetried:
		return NodeStateRetrying
	default:
		return NodeStateFailed
	}
}
