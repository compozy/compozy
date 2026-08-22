package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type RunReadStore interface {
	GenerationLineageReader
	RunListSummaryReader
	GetLoopRun(context.Context, WorkspaceID, RunID) (Run, error)
	GetLoopDefinitionSnapshot(context.Context, WorkspaceID, string) (DefinitionSnapshot, error)
	ListGenerationOutputs(context.Context, WorkspaceID, RunID, int) ([]GenerationOutput, error)
	ListNodeAttempts(context.Context, WorkspaceID, RunID) ([]NodeAttempt, error)
	ListNodeControls(context.Context, WorkspaceID, RunID) ([]NodeControl, error)
	ListNodeWaits(context.Context, WorkspaceID, RunID) ([]NodeWait, error)
	ListRouteCauses(context.Context, WorkspaceID, RunID, int64) ([]RouteCause, error)
	ListRequests(context.Context, WorkspaceID, RequestQuery) (RequestPage, error)
	ListLoopRunEvents(context.Context, RunEventQuery) ([]RunEvent, error)
}

type TimelineEventReader interface {
	GetLoopRunEventHead(context.Context, WorkspaceID, RunID) (int64, error)
	ListLoopRunEventsBackward(context.Context, RunEventBackwardQuery) ([]RunEvent, error)
}

type OutputBlobAvailabilityReader interface {
	ListAvailableLoopOutputRefs(
		context.Context,
		WorkspaceID,
		RunID,
	) (map[string]bool, error)
}

type RosterBulkReader interface {
	ListLoopRosterOutputs(context.Context, WorkspaceID, RunID) ([]GenerationOutput, error)
	ListLoopRosterRouteCauses(context.Context, WorkspaceID, RunID) ([]RouteCause, error)
}

type computedRunReadService struct {
	store RunReadStore
	now   func() time.Time
}

const terminalOutcomeCauseVerified = "verified"

var _ RunReadService = (*computedRunReadService)(nil)

func NewRunReadService(store RunReadStore, now func() time.Time) RunReadService {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &computedRunReadService{store: store, now: now}
}

func (s *computedRunReadService) NodeRoster(
	ctx context.Context,
	workspaceID string,
	runID RunID,
	query RosterQuery,
) (RosterPage, error) {
	source, err := s.loadRosterSource(ctx, WorkspaceID(workspaceID), runID)
	if err != nil {
		return RosterPage{}, err
	}
	return ProjectRoster(&source, query)
}

func (s *computedRunReadService) Timeline(
	ctx context.Context,
	workspaceID string,
	runID RunID,
	query TimelineQuery,
) (TimelinePage, error) {
	ws := WorkspaceID(workspaceID)
	if _, err := s.store.GetLoopRun(ctx, ws, runID); err != nil {
		return TimelinePage{}, err
	}
	if reader, ok := s.store.(TimelineEventReader); ok {
		return s.timelineFromReader(ctx, reader, ws, runID, query)
	}
	events, err := s.store.ListLoopRunEvents(ctx, RunEventQuery{
		WorkspaceID: ws, RunID: runID, AfterSeq: 0, Limit: 500,
	})
	if err != nil {
		return TimelinePage{}, fmt.Errorf("read loop timeline events: %w", err)
	}
	return ProjectTimeline(runID, events, query)
}

func (s *computedRunReadService) timelineFromReader(
	ctx context.Context,
	reader TimelineEventReader,
	workspaceID WorkspaceID,
	runID RunID,
	query TimelineQuery,
) (TimelinePage, error) {
	query, err := normalizeTimelineQuery(query)
	if err != nil {
		return TimelinePage{}, err
	}
	head, err := reader.GetLoopRunEventHead(ctx, workspaceID, runID)
	if err != nil {
		return TimelinePage{}, fmt.Errorf("read loop timeline head: %w", err)
	}
	if query.AfterSeq > head {
		return TimelinePage{}, &TimelinePositionError{Position: query.AfterSeq, Head: head}
	}
	fixedHead, before := head, head+1
	if query.Cursor != "" {
		cursor, decodeErr := decodeTimelineCursor(query.Cursor)
		if decodeErr != nil {
			return TimelinePage{}, decodeErr
		}
		if cursor.RunID != runID || cursor.View != query.View {
			return TimelinePage{}, fmt.Errorf("%w: cursor belongs to another timeline", ErrTimelineBranchChanged)
		}
		if cursor.FixedHeadSeq > head || cursor.BeforeSeq > cursor.FixedHeadSeq+1 {
			return TimelinePage{}, fmt.Errorf("%w: cursor head is beyond run head %d", ErrInvalidTimelineCursor, head)
		}
		fixedHead, before = cursor.FixedHeadSeq, cursor.BeforeSeq
	}
	const batchSize = 500
	events := make([]RunEvent, 0, batchSize)
	for {
		batch, readErr := reader.ListLoopRunEventsBackward(ctx, RunEventBackwardQuery{
			WorkspaceID: workspaceID, RunID: runID, FixedHeadSeq: fixedHead,
			BeforeSeq: before, Limit: batchSize,
		})
		if readErr != nil {
			return TimelinePage{}, fmt.Errorf("read loop timeline events: %w", readErr)
		}
		events = append(events, batch...)
		page, projectErr := projectTimelineWithHead(runID, head, events, query)
		if projectErr != nil {
			return TimelinePage{}, projectErr
		}
		pageReady := page.NextCursor != "" && timelinePageBoundaryIsComplete(page, batch)
		if pageReady || len(batch) < batchSize || len(batch) == 0 ||
			batch[len(batch)-1].Seq <= query.AfterSeq {
			return page, nil
		}
		before = batch[len(batch)-1].Seq
	}
}

func timelinePageBoundaryIsComplete(page TimelinePage, batch []RunEvent) bool {
	if len(page.Entries) == 0 || len(batch) == 0 {
		return true
	}
	lastEntry := page.Entries[len(page.Entries)-1]
	if !heartbeatKind(lastEntry.Kind) {
		return true
	}
	return lastEntry.FirstSeq > batch[len(batch)-1].Seq
}

func (s *computedRunReadService) loadRosterSource(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
) (RosterSource, error) {
	run, err := s.store.GetLoopRun(ctx, workspaceID, runID)
	if err != nil {
		return RosterSource{}, err
	}
	snapshot, err := s.store.GetLoopDefinitionSnapshot(ctx, workspaceID, run.DefinitionDigest)
	if err != nil {
		return RosterSource{}, fmt.Errorf("read loop definition snapshot: %w", err)
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
	if err != nil {
		return RosterSource{}, fmt.Errorf("hydrate loop definition snapshot: %w", err)
	}
	generations, err := s.store.ListGenerations(ctx, string(workspaceID), string(runID))
	if err != nil {
		return RosterSource{}, fmt.Errorf("read loop generations: %w", err)
	}
	source := RosterSource{
		Run: run, Graph: resolved.Definition.Graph, Generations: generations,
		PrunedNodes: map[string]bool{},
	}
	if bulk, ok := s.store.(RosterBulkReader); ok {
		source.Outputs, err = bulk.ListLoopRosterOutputs(ctx, workspaceID, runID)
		if err != nil {
			return RosterSource{}, fmt.Errorf("read loop roster outputs: %w", err)
		}
		source.RouteCauses, err = bulk.ListLoopRosterRouteCauses(ctx, workspaceID, runID)
		if err != nil {
			return RosterSource{}, fmt.Errorf("read loop roster route causes: %w", err)
		}
	} else {
		for _, generation := range generations {
			outputs, readErr := s.store.ListGenerationOutputs(
				ctx,
				workspaceID,
				runID,
				int(generation.Generation),
			)
			if readErr != nil {
				return RosterSource{}, fmt.Errorf("read loop generation %d outputs: %w", generation.Generation, readErr)
			}
			source.Outputs = append(source.Outputs, outputs...)
			causes, readErr := s.store.ListRouteCauses(ctx, workspaceID, runID, generation.Generation)
			if readErr != nil {
				return RosterSource{}, fmt.Errorf(
					"read loop generation %d route causes: %w",
					generation.Generation,
					readErr,
				)
			}
			source.RouteCauses = append(source.RouteCauses, causes...)
		}
	}
	source.Attempts, err = s.store.ListNodeAttempts(ctx, workspaceID, runID)
	if err != nil {
		return RosterSource{}, fmt.Errorf("read loop node attempts: %w", err)
	}
	source.Controls, err = s.store.ListNodeControls(ctx, workspaceID, runID)
	if err != nil {
		return RosterSource{}, fmt.Errorf("read loop node controls: %w", err)
	}
	source.Waits, err = s.store.ListNodeWaits(ctx, workspaceID, runID)
	if err != nil {
		return RosterSource{}, fmt.Errorf("read loop node waits: %w", err)
	}
	events, err := s.loadRouteEvidence(ctx, workspaceID, runID)
	if err != nil {
		return RosterSource{}, fmt.Errorf("read loop route evidence: %w", err)
	}
	if err := applyRunReadEvidence(&source, events); err != nil {
		return RosterSource{}, err
	}
	return source, nil
}

func applyRunReadEvidence(source *RosterSource, events []RunEvent) error {
	for _, event := range events {
		switch event.Kind {
		case string(RunEventBranchPruned):
			generation, nodeID, itemIndexes, decodeErr := prunedNodeFromEvent(event)
			if decodeErr != nil {
				return decodeErr
			}
			if generation < 1 || nodeID == "" {
				continue
			}
			for _, itemIndex := range itemIndexes {
				if err := source.MarkPrunedNodeItem(generation, nodeID, itemIndex); err != nil {
					return err
				}
			}
		case string(RunEventStatusChanged):
			if err := applyTerminalOutcomeEvidence(source, event); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyTerminalOutcomeEvidence(source *RosterSource, event RunEvent) error {
	var payload struct {
		Status Status          `json:"status"`
		Cause  TransitionCause `json:"cause"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode terminal status event payload: %w", err)
	}
	if payload.Status != source.Run.Status || !isTerminalStatus(payload.Status) {
		return nil
	}
	if source.Outcome != nil && !event.At.After(source.Outcome.At) {
		return nil
	}
	actorKind := ""
	actorRef := ""
	if payload.Status == StatusCanceled {
		actorKind = string(source.Run.ControlActor.Kind)
		actorRef = source.Run.ControlActor.Ref
	}
	source.Outcome = &RunOutcome{
		Status: payload.Status, Cause: terminalOutcomeCause(payload.Status, payload.Cause),
		ActorKind: actorKind, ActorRef: actorRef, At: event.At.UTC(),
	}
	return nil
}

func terminalOutcomeCause(status Status, cause TransitionCause) string {
	if status == StatusDone && cause == TransitionCauseContract {
		return terminalOutcomeCauseVerified
	}
	if cause != "" {
		return string(cause)
	}
	return unknownValue
}

func (s *computedRunReadService) loadRouteEvidence(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
) ([]RunEvent, error) {
	reader, ok := s.store.(TimelineEventReader)
	if !ok {
		return s.store.ListLoopRunEvents(ctx, RunEventQuery{
			WorkspaceID: workspaceID,
			RunID:       runID,
			Limit:       500,
		})
	}

	head, err := reader.GetLoopRunEventHead(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if head == 0 {
		return nil, nil
	}

	const batchSize = 500
	events := make([]RunEvent, 0, batchSize)
	before := head + 1
	for {
		batch, readErr := reader.ListLoopRunEventsBackward(ctx, RunEventBackwardQuery{
			WorkspaceID: workspaceID, RunID: runID, FixedHeadSeq: head,
			BeforeSeq: before, Limit: batchSize,
		})
		if readErr != nil {
			return nil, readErr
		}
		events = append(events, batch...)
		if len(batch) < batchSize {
			return events, nil
		}
		before = batch[len(batch)-1].Seq
	}
}

func prunedNodeFromEvent(event RunEvent) (int, NodeID, []int, error) {
	var payload struct {
		Generation  int    `json:"generation"`
		NodeID      NodeID `json:"node_id"`
		ItemIndexes []int  `json:"item_indexes"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return 0, "", nil, fmt.Errorf("decode branch_pruned event payload: %w", err)
	}
	for _, itemIndex := range payload.ItemIndexes {
		if itemIndex < 0 {
			return 0, "", nil, fmt.Errorf("%w: branch_pruned item index is negative", ErrValidation)
		}
	}
	return payload.Generation, payload.NodeID, payload.ItemIndexes, nil
}
