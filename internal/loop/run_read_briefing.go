package loop

import (
	"context"
	"fmt"
	"time"
)

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
	roster, err := projectCompleteRoster(&source, NodeStateFilterAll, 0)
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
	approvalWaitingSince, err := s.loadApprovalWaitingSince(ctx, ws, source.Run)
	if err != nil {
		return Briefing{}, err
	}
	return ProjectBriefing(&BriefingSource{
		Run:                  source.Run,
		Roster:               roster,
		Requests:             requests,
		Artifacts:            artifacts,
		Outcome:              source.Outcome,
		ApprovalWaitingSince: approvalWaitingSince,
		Now:                  s.now(),
	}), nil
}

func (s *computedRunReadService) loadApprovalWaitingSince(
	ctx context.Context,
	workspaceID WorkspaceID,
	run Run,
) (time.Time, error) {
	if run.Status != StatusNeedsApproval && run.ActiveGateID == "" {
		return time.Time{}, nil
	}
	summaries, err := s.store.ListLoopRunSummaries(ctx, workspaceID, []RunID{run.ID})
	if err != nil {
		return time.Time{}, fmt.Errorf("read loop approval opening: %w", err)
	}
	summary, ok := summaries[run.ID]
	if !ok || summary.Attention == nil || summary.Attention.Kind != gateResultApprovalKey ||
		summary.Attention.Since.IsZero() {
		return time.Time{}, fmt.Errorf("read loop approval opening: durable approval attention is unavailable")
	}
	return summary.Attention.Since.UTC(), nil
}

func projectCompleteRoster(
	source *RosterSource,
	state NodeStateFilter,
	generation int,
) (RosterPage, error) {
	query := RosterQuery{State: state, Generation: generation, Limit: 500}
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
			Name:         output.ArtifactName,
			Output:       output.OutputID,
			Ref:          output.OutputRef,
			Availability: availability,
		})
	}
	return items
}
