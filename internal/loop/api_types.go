package loop

import (
	"context"
	"encoding/json"
	"time"
)

// RunListQuery filters workspace-scoped loop run listings.
type RunListQuery struct {
	WorkspaceID     WorkspaceID
	LoopName        string
	Status          Status
	OriginKind      string
	OriginSessionID string
	Live            *bool
	Limit           int
	CreatedAfter    time.Time
}

// RunEventQuery filters retained loop run events.
type RunEventQuery struct {
	WorkspaceID WorkspaceID
	RunID       RunID
	AfterSeq    int64
	Limit       int
}

// RunEvent is one retained loop_run_events row.
type RunEvent struct {
	ID          string
	LoopRunID   RunID
	WorkspaceID WorkspaceID
	Seq         int64
	Kind        string
	Payload     json.RawMessage
	At          time.Time
	DeliveryKey string
}

// RouteCause is one workspace-scoped route_taken projection.
type RouteCause struct {
	Generation  int64
	NodeID      NodeID
	ItemIndex   int
	Route       NodeID
	Cause       string
	MatchedWhen string
	Default     bool
	At          time.Time
}

// RouteCauseReader loads durable route decisions for one generation.
type RouteCauseReader interface {
	ListRouteCauses(
		ctx context.Context,
		workspaceID WorkspaceID,
		runID RunID,
		generation int64,
	) ([]RouteCause, error)
}

// UIAnnotation is one editor sidecar position for a loop node.
type UIAnnotation struct {
	NodeID NodeID
	X      float64
	Y      float64
}

// RunReader exposes read-model operations outside the aggregate mutation service.
type RunReader interface {
	ListLoopRuns(ctx context.Context, query RunListQuery) ([]Run, error)
	ListLoopRunEvents(ctx context.Context, query RunEventQuery) ([]RunEvent, error)
	ListGenerationOutputs(
		ctx context.Context,
		workspaceID WorkspaceID,
		runID RunID,
		generation int,
	) ([]GenerationOutput, error)
}

// AnnotationStore persists workspace-scoped loop editor sidecars.
type AnnotationStore interface {
	ListLoopUIAnnotations(ctx context.Context, ws WorkspaceID, loopName string) ([]UIAnnotation, error)
	ReplaceLoopUIAnnotations(ctx context.Context, ws WorkspaceID, loopName string, annotations []UIAnnotation) error
}

// DefinitionState contains mutable sidecars owned by one file-backed Loop definition.
type DefinitionState struct {
	Config      *LoopConfig
	Annotations []UIAnnotation
}

// DefinitionStateStore owns atomic removal and restoration of definition sidecars.
type DefinitionStateStore interface {
	DeleteLoopDefinitionState(ctx context.Context, ws WorkspaceID, loopName string) (DefinitionState, error)
	RestoreLoopDefinitionState(
		ctx context.Context,
		ws WorkspaceID,
		loopName string,
		state DefinitionState,
	) error
}
