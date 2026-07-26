package bridges

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/toolmeta"
)

type projectedToolCall struct {
	toolID          string
	input           json.RawMessage
	startedAt       time.Time
	startedEmitted  bool
	terminalEmitted bool
}

func (b *Broker) projectAgentEventLocked(
	delivery *activeDelivery,
	event DeliveryProjectionEvent,
) (DeliveryEvent, bool, error) {
	if delivery == nil {
		return DeliveryEvent{}, false, ErrDeliveryNotFound
	}

	template, ok, err := ProjectAgentEvent(event, delivery.progress.ToolProgress)
	if err != nil || !ok {
		return DeliveryEvent{}, ok, err
	}

	switch template.EventType {
	case DeliveryEventTypeDelta:
		template.Content = MessageContent{Text: delivery.currentContent.Text + template.Content.Text}
		if !delivery.textStarted {
			template.EventType = DeliveryEventTypeStart
		}
	case DeliveryEventTypeFinal:
		if !delivery.textStarted && delivery.progressIndex == 0 &&
			strings.TrimSpace(delivery.currentContent.Text) == "" {
			return DeliveryEvent{}, false, nil
		}
		template.Content = delivery.currentContent
	case DeliveryEventTypeError:
		template.Content = delivery.currentContent
	case DeliveryEventTypeProgress:
		return b.projectProgressEventLocked(delivery, event, template)
	default:
		return DeliveryEvent{}, false, fmt.Errorf(
			"bridges: unsupported agent projection event type %q",
			template.EventType,
		)
	}

	return bindProjectedEvent(delivery, template), true, nil
}

func (b *Broker) projectProgressEventLocked(
	delivery *activeDelivery,
	source DeliveryProjectionEvent,
	template DeliveryEvent,
) (DeliveryEvent, bool, error) {
	progress := cloneToolProgress(template.Progress)
	if progress == nil {
		return DeliveryEvent{}, false, errors.New("bridges: projected progress payload is required")
	}
	b.rememberProjectedToolCallLocked(delivery, source, template)

	input := cloneRawJSON(source.ToolInput)
	state, hasState := delivery.toolCalls[progress.ToolCallID]
	input = hydrateProjectedToolResult(progress, input, source, state, hasState)
	if progress.ToolID == "" {
		return DeliveryEvent{}, false, errors.New("bridges: projected progress tool id is required")
	}
	if shouldSuppressProjectedProgress(delivery, source, *progress, state, hasState) {
		return DeliveryEvent{}, false, nil
	}

	metadata := b.resolveProjectedToolMetadata(delivery, progress.ToolID, source.ToolMetadata)
	presentation := toolmeta.Render(progress.ToolID, input, toolmeta.RenderOptions{
		Descriptor:   metadata,
		PreviewLimit: delivery.progress.PreviewLimit,
		Verbose:      delivery.progress.ToolProgress == ProgressModeVerbose,
	})
	progress.Label, progress.Preview = presentation.ProgressFields()
	progress.Emoji = presentation.Emoji
	progress.Index = delivery.progressIndex + 1
	template.Progress = progress
	return bindProjectedEvent(delivery, template), true, nil
}

func hydrateProjectedToolResult(
	progress *ToolProgress,
	input json.RawMessage,
	source DeliveryProjectionEvent,
	state projectedToolCall,
	hasState bool,
) json.RawMessage {
	if source.Type != projectionEventToolResult || !hasState {
		return input
	}
	if progress.ToolID == "" {
		progress.ToolID = state.toolID
	}
	if len(input) == 0 {
		input = cloneRawJSON(state.input)
	}
	if progress.DurationMS != 0 || state.startedAt.IsZero() || source.Timestamp.IsZero() {
		return input
	}
	duration := source.Timestamp.UTC().Sub(state.startedAt)
	if duration > 0 {
		progress.DurationMS = duration.Milliseconds()
	}
	return input
}

func shouldSuppressProjectedProgress(
	delivery *activeDelivery,
	source DeliveryProjectionEvent,
	progress ToolProgress,
	state projectedToolCall,
	hasState bool,
) bool {
	if source.Type == projectionEventToolCall && hasState && state.startedEmitted {
		return true
	}
	if source.Type == projectionEventToolResult && hasState && state.terminalEmitted {
		return true
	}
	return progress.Phase == ToolProgressPhaseStarted &&
		delivery.progress.ToolProgress == ProgressModeNew &&
		delivery.lastProgressTool == progress.ToolID
}

func (b *Broker) resolveProjectedToolMetadata(
	delivery *activeDelivery,
	toolID string,
	metadata toolmeta.DescriptorMetadata,
) toolmeta.DescriptorMetadata {
	if !descriptorMetadataIsZero(metadata) || b.descriptors == nil {
		return metadata
	}
	resolved, ok := b.descriptors.LookupToolMetadata(delivery.routingKey.WorkspaceID, toolID)
	if !ok {
		return metadata
	}
	return resolved
}

func descriptorMetadataIsZero(metadata toolmeta.DescriptorMetadata) bool {
	return strings.TrimSpace(metadata.ID) == "" &&
		strings.TrimSpace(metadata.DisplayTitle) == "" &&
		strings.TrimSpace(metadata.FriendlyVerb) == "" &&
		strings.TrimSpace(metadata.Preview) == ""
}

func bindProjectedEvent(delivery *activeDelivery, event DeliveryEvent) DeliveryEvent {
	event.DeliveryID = delivery.deliveryID
	event.BridgeInstanceID = delivery.bridgeInstanceID
	event.RoutingKey = delivery.routingKey
	event.DeliveryTarget = delivery.target
	event.Seq = delivery.latestSeq + 1
	event.Operation = DeliveryOperationPost
	return event
}

func (b *Broker) applyProjectionStateLocked(
	delivery *activeDelivery,
	source DeliveryProjectionEvent,
	projected DeliveryEvent,
) {
	if delivery == nil {
		return
	}
	b.rememberProjectedToolCallLocked(delivery, source, projected)
	if projected.EventType == DeliveryEventTypeProgress && projected.Progress != nil {
		progress := projected.Progress.normalize()
		b.markProjectedToolPhaseLocked(delivery, progress)
		if delivery.progress.ToolProgress == ProgressModeNew && progress.Phase == ToolProgressPhaseStarted {
			delivery.lastProgressTool = progress.ToolID
		}
	}
	b.applyQueuedEventLocked(delivery, projected)
}

func (b *Broker) rememberProjectedToolCallLocked(
	delivery *activeDelivery,
	source DeliveryProjectionEvent,
	projected DeliveryEvent,
) {
	if delivery == nil || source.Type != projectionEventToolCall || projected.Progress == nil {
		return
	}
	progress := projected.Progress.normalize()
	state := delivery.toolCalls[progress.ToolCallID]
	if progress.ToolID != "" {
		state.toolID = progress.ToolID
	}
	if len(source.ToolInput) > 0 {
		state.input = cloneRawJSON(source.ToolInput)
	}
	if state.startedAt.IsZero() && !source.Timestamp.IsZero() {
		state.startedAt = source.Timestamp.UTC()
	}
	delivery.toolCalls[progress.ToolCallID] = state
}

func (b *Broker) markProjectedToolPhaseLocked(delivery *activeDelivery, progress ToolProgress) {
	if delivery == nil {
		return
	}
	state := delivery.toolCalls[progress.ToolCallID]
	if progress.ToolID != "" {
		state.toolID = progress.ToolID
	}
	if progress.Phase == ToolProgressPhaseStarted {
		state.startedEmitted = true
	} else {
		state.terminalEmitted = true
	}
	delivery.toolCalls[progress.ToolCallID] = state
}

func (b *Broker) projectDeliveryEventLocked(
	delivery *activeDelivery,
	event DeliveryProjectionEvent,
) (*routeWorker, error) {
	if delivery == nil || delivery.final {
		return nil, nil
	}
	fingerprint := agentEventFingerprint(event)
	if fingerprint != "" {
		if _, seen := delivery.seen[fingerprint]; seen {
			return nil, nil
		}
	}

	projected, ok, err := b.projectEventLocked(delivery, event)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	if err := projected.Validate(); err != nil {
		return nil, fmt.Errorf("bridges: validate projected delivery event: %w", err)
	}

	route := b.ensureRouteLocked(delivery.routeHash, delivery.bridgeInstanceID, delivery.extensionName)
	if err := b.enqueueEventLocked(route, delivery, projected); err != nil {
		if errors.Is(err, errProgressEventDropped) {
			b.rememberProjectedToolCallLocked(delivery, event, projected)
			if fingerprint != "" {
				delivery.seen[fingerprint] = struct{}{}
			}
			return nil, nil
		}
		return nil, err
	}
	if fingerprint != "" {
		delivery.seen[fingerprint] = struct{}{}
	}
	b.applyProjectionStateLocked(delivery, event, projected)
	return route, nil
}

func (b *Broker) projectEventLocked(
	delivery *activeDelivery,
	event DeliveryProjectionEvent,
) (DeliveryEvent, bool, error) {
	return b.projectAgentEventLocked(delivery, event)
}

func (b *Broker) applyQueuedEventLocked(delivery *activeDelivery, event DeliveryEvent) {
	if delivery == nil || delivery.final {
		return
	}

	normalizedType := normalizeDeliveryEventType(event.EventType)
	if event.Seq > delivery.latestSeq {
		delivery.latestSeq = event.Seq
	}
	if normalizedType == DeliveryEventTypeProgress {
		if event.Progress != nil && event.Progress.Index > delivery.progressIndex {
			delivery.progressIndex = event.Progress.Index
		}
		delivery.updatedAt = b.now()
		return
	}
	delivery.latestEventType = normalizedType
	delivery.currentContent = event.Content
	delivery.operation = event.Operation.Normalize()
	delivery.reference = cloneDeliveryReference(event.Reference)
	delivery.providerMetadata = cloneRawJSON(event.ProviderMetadata)
	delivery.final = event.Final
	delivery.updatedAt = b.now()
	if normalizedType == DeliveryEventTypeStart || normalizedType == DeliveryEventTypeDelta {
		delivery.textStarted = true
		delivery.lastProgressTool = ""
	}

	if normalizedType == DeliveryEventTypeError {
		if event.Error != nil {
			delivery.errorText = strings.TrimSpace(event.Error.Message)
		} else {
			delivery.errorText = ""
		}
		b.recordDeliveryFailureLocked(delivery.bridgeInstanceID, delivery.errorText)
	} else if normalizedType != DeliveryEventTypeResume {
		delivery.errorText = ""
	}
}

func agentEventFingerprint(event DeliveryProjectionEvent) string {
	if fingerprint := strings.TrimSpace(event.Fingerprint); fingerprint != "" {
		return fingerprint
	}
	if event.Timestamp.IsZero() {
		return ""
	}
	return strings.TrimSpace(
		event.Type,
	) + "|" + strings.TrimSpace(
		event.TurnID,
	) + "|" + event.Timestamp.UTC().
		Format(time.RFC3339Nano) +
		"|" + event.Text + "|" + event.Error
}
