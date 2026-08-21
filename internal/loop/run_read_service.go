package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type RunReadStore interface {
	GetLoopRun(context.Context, WorkspaceID, RunID) (Run, error)
	GetLoopDefinitionSnapshot(context.Context, WorkspaceID, string) (DefinitionSnapshot, error)
	ListGenerations(context.Context, string, string) ([]LoopGeneration, error)
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
	ListLoopRunEventsBackward(context.Context, WorkspaceID, RunID, int64, int64, int) ([]RunEvent, error)
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

func (s *computedRunReadService) Briefing(
	ctx context.Context,
	workspaceID string,
	runID RunID,
) (Briefing, error) {
	ws := WorkspaceID(workspaceID)
	source, err := s.loadRosterSource(ctx, ws, runID)
	if err != nil {
		return Briefing{}, err
	}
	roster, err := projectCompleteRoster(&source, RosterQuery{State: NodeStateFilterAll})
	if err != nil {
		return Briefing{}, err
	}
	requests, err := s.loadAllPendingRequests(ctx, ws, runID)
	if err != nil {
		return Briefing{}, fmt.Errorf("read loop briefing requests: %w", err)
	}
	artifacts, err := s.loadArtifacts(ctx, ws, runID, source.Outputs)
	if err != nil {
		return Briefing{}, err
	}
	return ProjectBriefing(&BriefingSource{
		Run:       source.Run,
		Roster:    roster,
		Requests:  requests,
		Artifacts: artifacts,
		Now:       s.now(),
	}), nil
}

func projectCompleteRoster(source *RosterSource, query RosterQuery) (RosterPage, error) {
	query.Limit = 500
	query.Cursor = ""
	complete := RosterPage{}
	for {
		page, err := ProjectRoster(source, query)
		if err != nil {
			return RosterPage{}, err
		}
		if complete.RunID == "" {
			complete = page
			complete.Nodes = nil
		}
		complete.Nodes = append(complete.Nodes, page.Nodes...)
		if page.NextCursor == "" {
			complete.NextCursor = ""
			return complete, nil
		}
		query.Cursor = page.NextCursor
	}
}

func (s *computedRunReadService) loadAllPendingRequests(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
) ([]Request, error) {
	query := RequestQuery{RunID: runID, Limit: 200}
	requests := make([]Request, 0)
	for {
		page, err := s.store.ListRequests(ctx, workspaceID, query)
		if err != nil {
			return nil, err
		}
		requests = append(requests, page.Items...)
		if page.NextCursor == "" {
			return requests, nil
		}
		query.Cursor = page.NextCursor
	}
}

func (s *computedRunReadService) loadArtifacts(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
	outputs []GenerationOutput,
) ([]RunArtifact, error) {
	refs := outputRefs(outputs)
	available := make(map[string]bool, len(refs))
	if reader, ok := s.store.(OutputBlobAvailabilityReader); ok && len(refs) > 0 {
		var err error
		available, err = reader.ListAvailableLoopOutputRefs(ctx, workspaceID, runID)
		if err != nil {
			return nil, fmt.Errorf("read loop output availability: %w", err)
		}
	} else {
		for _, ref := range refs {
			available[ref] = true
		}
	}
	return artifactsFromOutputs(outputs, available), nil
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
		batch, readErr := reader.ListLoopRunEventsBackward(
			ctx, workspaceID, runID, fixedHead, before, batchSize,
		)
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
	if err := applyPrunedNodeEvidence(&source, events); err != nil {
		return RosterSource{}, err
	}
	return source, nil
}

func applyPrunedNodeEvidence(source *RosterSource, events []RunEvent) error {
	for _, event := range events {
		if event.Kind != string(RunEventBranchPruned) {
			continue
		}
		generation, nodeID, decodeErr := prunedNodeFromEvent(event)
		if decodeErr != nil {
			return decodeErr
		}
		if generation > 0 && nodeID != "" {
			source.PrunedNodes[rosterKey(generation, nodeID, 0)] = true
		}
	}
	return nil
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
		batch, readErr := reader.ListLoopRunEventsBackward(
			ctx,
			workspaceID,
			runID,
			head,
			before,
			batchSize,
		)
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

func outputRefs(outputs []GenerationOutput) []string {
	seen := make(map[string]bool)
	refs := make([]string, 0, len(outputs))
	for _, output := range outputs {
		if output.OutputRef == "" || seen[output.OutputRef] {
			continue
		}
		seen[output.OutputRef] = true
		refs = append(refs, output.OutputRef)
	}
	return refs
}

func artifactsFromOutputs(outputs []GenerationOutput, available map[string]bool) []RunArtifact {
	items := make([]RunArtifact, 0)
	for _, output := range outputs {
		if output.OutputRef == "" {
			continue
		}
		availability := ArtifactPruned
		if available[output.OutputRef] {
			availability = ArtifactAvailable
		}
		if available[output.OutputRef] && output.Status == string(NodeStatePartial) {
			availability = ArtifactPartial
		}
		items = append(items, RunArtifact{
			Name:         fmt.Sprintf("%s[%d]", output.NodeID, output.ItemIndex),
			Output:       output.TaskRunID,
			Ref:          output.OutputRef,
			Availability: availability,
		})
	}
	return items
}

func prunedNodeFromEvent(event RunEvent) (int, NodeID, error) {
	var payload struct {
		Generation int    `json:"generation"`
		NodeID     NodeID `json:"node_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return 0, "", fmt.Errorf("decode branch_pruned event payload: %w", err)
	}
	return payload.Generation, payload.NodeID, nil
}
