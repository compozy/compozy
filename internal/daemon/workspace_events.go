package daemon

import (
	"context"
	"strings"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

type workspaceEventStream struct {
	events chan apicore.WorkspaceStreamItem
	errors chan error
	close  func() error
}

var _ apicore.WorkspaceEventStream = (*workspaceEventStream)(nil)

func (m *RunManager) OpenWorkspaceStream(
	ctx context.Context,
	workspaceRef string,
) (apicore.WorkspaceEventStream, error) {
	if m == nil || m.workspaceEvents == nil {
		return nil, apicore.NewProblem(503, "service_unavailable", "workspace event service unavailable", nil, nil)
	}
	row, err := resolveWorkspaceReference(detachContext(ctx), m.globalDB, workspaceRef)
	if err != nil {
		return nil, err
	}

	stream := &workspaceEventStream{
		events: make(chan apicore.WorkspaceStreamItem, workspaceStreamBufferSize),
		errors: make(chan error, 1),
	}
	streamCtx, cancel := context.WithCancel(ctx)
	stream.close = func() error {
		cancel()
		return nil
	}

	subID, eventCh, unsubscribe := m.workspaceEvents.Subscribe()
	go m.streamWorkspaceEvents(streamCtx, stream, row.ID, subID, eventCh, unsubscribe)
	return stream, nil
}

func (m *RunManager) streamWorkspaceEvents(
	ctx context.Context,
	stream *workspaceEventStream,
	workspaceID string,
	subID eventspkg.SubID,
	eventCh <-chan apicore.WorkspaceEvent,
	unsubscribe func(),
) {
	defer close(stream.events)
	defer unsubscribe()

	for {
		if m.workspaceEvents != nil && m.workspaceEvents.DroppedFor(subID) > 0 {
			_ = sendWorkspaceStreamItem(ctx, stream.events, apicore.WorkspaceStreamItem{
				Overflow: &apicore.WorkspaceStreamOverflow{Reason: "subscriber_dropped_messages"},
			})
			return
		}

		select {
		case <-ctx.Done():
			return
		case item, ok := <-eventCh:
			if !ok {
				return
			}
			if strings.TrimSpace(item.WorkspaceID) != strings.TrimSpace(workspaceID) {
				continue
			}
			if !sendWorkspaceStreamItem(ctx, stream.events, apicore.WorkspaceStreamItem{Event: &item}) {
				return
			}
		}
	}
}

func (s *workspaceEventStream) Events() <-chan apicore.WorkspaceStreamItem {
	if s == nil {
		ch := make(chan apicore.WorkspaceStreamItem)
		close(ch)
		return ch
	}
	return s.events
}

func (s *workspaceEventStream) Errors() <-chan error {
	if s == nil {
		ch := make(chan error)
		close(ch)
		return ch
	}
	return s.errors
}

func (s *workspaceEventStream) Close() error {
	if s == nil || s.close == nil {
		return nil
	}
	return s.close()
}

func (m *RunManager) publishRunWorkspaceEvent(
	ctx context.Context,
	row globaldb.Run,
	workflowSlug string,
	kind apicore.WorkspaceEventKind,
) {
	if strings.TrimSpace(row.WorkspaceID) == "" {
		return
	}
	m.publishWorkspaceEvent(ctx, apicore.WorkspaceEvent{
		WorkspaceID:  strings.TrimSpace(row.WorkspaceID),
		WorkflowID:   cloneStringPtr(row.WorkflowID),
		WorkflowSlug: strings.TrimSpace(workflowSlug),
		RunID:        strings.TrimSpace(row.RunID),
		Mode:         strings.TrimSpace(row.Mode),
		Status:       strings.TrimSpace(row.Status),
		Kind:         kind,
	})
}

func (m *RunManager) publishWorkflowSyncWorkspaceEvent(
	ctx context.Context,
	workspaceID string,
	workflowID *string,
	workflowSlug string,
	paths []string,
) {
	if strings.TrimSpace(workspaceID) == "" {
		return
	}
	m.publishWorkspaceEvent(ctx, apicore.WorkspaceEvent{
		WorkspaceID:  strings.TrimSpace(workspaceID),
		WorkflowID:   cloneStringPtr(workflowID),
		WorkflowSlug: strings.TrimSpace(workflowSlug),
		Kind:         apicore.WorkspaceEventKindWorkflowSyncCompleted,
		Paths:        cleanWorkspaceEventPaths(paths),
	})
}

func (m *RunManager) publishArtifactWorkspaceEvent(
	ctx context.Context,
	active *activeRun,
	item artifactSyncEvent,
) {
	if active == nil || strings.TrimSpace(active.workspaceID) == "" {
		return
	}
	m.publishWorkspaceEvent(ctx, apicore.WorkspaceEvent{
		WorkspaceID:  strings.TrimSpace(active.workspaceID),
		WorkflowID:   cloneStringPtr(active.workflowID),
		WorkflowSlug: strings.TrimSpace(active.workflowSlug),
		RunID:        strings.TrimSpace(active.runID),
		Mode:         strings.TrimSpace(active.mode),
		Kind:         apicore.WorkspaceEventKindArtifactChanged,
		Paths:        cleanWorkspaceEventPaths([]string{item.RelativePath}),
	})
}

func (m *RunManager) publishWorkspaceEvent(ctx context.Context, event apicore.WorkspaceEvent) {
	if m == nil || m.workspaceEvents == nil || strings.TrimSpace(event.WorkspaceID) == "" {
		return
	}
	event.WorkspaceID = strings.TrimSpace(event.WorkspaceID)
	event.WorkflowSlug = strings.TrimSpace(event.WorkflowSlug)
	event.RunID = strings.TrimSpace(event.RunID)
	event.Mode = strings.TrimSpace(event.Mode)
	event.Status = strings.TrimSpace(event.Status)
	event.Paths = cleanWorkspaceEventPaths(event.Paths)
	if event.Kind == "" {
		return
	}
	if event.TS.IsZero() {
		event.TS = m.now().UTC()
	}
	if event.Seq == 0 {
		event.Seq = m.workspaceEventSeq.Add(1)
	}
	m.workspaceEvents.Publish(detachContext(ctx), event)
}

func cleanWorkspaceEventPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		clean := strings.TrimSpace(path)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func sendWorkspaceStreamItem(
	ctx context.Context,
	dst chan<- apicore.WorkspaceStreamItem,
	item apicore.WorkspaceStreamItem,
) bool {
	select {
	case dst <- item:
		return true
	case <-ctx.Done():
		return false
	}
}
