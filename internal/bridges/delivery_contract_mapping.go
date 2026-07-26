package bridges

import (
	"slices"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

func routingKeyToContract(key RoutingKey) bridgecontract.RoutingKey {
	return bridgecontract.RoutingKey{
		Scope:            bridgecontract.Scope(key.Scope),
		WorkspaceID:      key.WorkspaceID,
		BridgeInstanceID: key.BridgeInstanceID,
		PeerID:           key.PeerID,
		ThreadID:         key.ThreadID,
		GroupID:          key.GroupID,
	}
}

func routingKeyFromContract(key bridgecontract.RoutingKey) RoutingKey {
	return RoutingKey{
		Scope:            Scope(key.Scope),
		WorkspaceID:      key.WorkspaceID,
		BridgeInstanceID: key.BridgeInstanceID,
		PeerID:           key.PeerID,
		ThreadID:         key.ThreadID,
		GroupID:          key.GroupID,
	}
}

func deliveryTargetToContract(target DeliveryTarget) bridgecontract.DeliveryTarget {
	return bridgecontract.DeliveryTarget{
		BridgeInstanceID: target.BridgeInstanceID,
		PeerID:           target.PeerID,
		ThreadID:         target.ThreadID,
		GroupID:          target.GroupID,
		Mode:             bridgecontract.DeliveryMode(target.Mode),
	}
}

func deliveryTargetFromContract(target bridgecontract.DeliveryTarget) DeliveryTarget {
	return DeliveryTarget{
		BridgeInstanceID: target.BridgeInstanceID,
		PeerID:           target.PeerID,
		ThreadID:         target.ThreadID,
		GroupID:          target.GroupID,
		Mode:             DeliveryMode(target.Mode),
	}
}

func messageContentToContract(content MessageContent) bridgecontract.MessageContent {
	return bridgecontract.MessageContent{Text: content.Text}
}

func messageContentFromContract(content bridgecontract.MessageContent) MessageContent {
	return MessageContent{Text: content.Text}
}

func toolProgressToContract(progress ToolProgress) bridgecontract.ToolProgress {
	return bridgecontract.ToolProgress{
		ToolCallID: progress.ToolCallID,
		ToolID:     progress.ToolID,
		Phase:      bridgecontract.ToolProgressPhase(progress.Phase),
		Label:      progress.Label,
		Preview:    progress.Preview,
		Emoji:      progress.Emoji,
		DurationMS: progress.DurationMS,
		Error:      progress.Error,
		Index:      progress.Index,
	}
}

func toolProgressFromContract(progress bridgecontract.ToolProgress) ToolProgress {
	return ToolProgress{
		ToolCallID: progress.ToolCallID,
		ToolID:     progress.ToolID,
		Phase:      ToolProgressPhase(progress.Phase),
		Label:      progress.Label,
		Preview:    progress.Preview,
		Emoji:      progress.Emoji,
		DurationMS: progress.DurationMS,
		Error:      progress.Error,
		Index:      progress.Index,
	}
}

func deliveryMessageReferenceToContract(
	reference DeliveryMessageReference,
) bridgecontract.DeliveryMessageReference {
	return bridgecontract.DeliveryMessageReference{
		DeliveryID: reference.DeliveryID, RemoteMessageID: reference.RemoteMessageID,
	}
}

func deliveryMessageReferenceFromContract(
	reference bridgecontract.DeliveryMessageReference,
) DeliveryMessageReference {
	return DeliveryMessageReference{
		DeliveryID: reference.DeliveryID, RemoteMessageID: reference.RemoteMessageID,
	}
}

func deliveryErrorDetailToContract(detail DeliveryErrorDetail) bridgecontract.DeliveryErrorDetail {
	return bridgecontract.DeliveryErrorDetail{Message: detail.Message}
}

func deliveryResumeStateToContract(state DeliveryResumeState) bridgecontract.DeliveryResumeState {
	return bridgecontract.DeliveryResumeState{
		LatestEventType: bridgecontract.DeliveryEventType(state.LatestEventType),
	}
}

func deliveryEventToContract(event DeliveryEvent) bridgecontract.DeliveryEvent {
	converted := bridgecontract.DeliveryEvent{
		DeliveryID:       event.DeliveryID,
		BridgeInstanceID: event.BridgeInstanceID,
		RoutingKey:       routingKeyToContract(event.RoutingKey),
		DeliveryTarget:   deliveryTargetToContract(event.DeliveryTarget),
		Seq:              event.Seq,
		EventType:        bridgecontract.DeliveryEventType(event.EventType),
		Content:          messageContentToContract(event.Content),
		Final:            event.Final,
		Operation:        bridgecontract.DeliveryOperation(event.Operation),
	}
	converted.ProviderMetadata = slices.Clone(event.ProviderMetadata)
	if event.Reference != nil {
		reference := deliveryMessageReferenceToContract(*event.Reference)
		converted.Reference = &reference
	}
	if event.Error != nil {
		converted.Error = &bridgecontract.DeliveryErrorDetail{Message: event.Error.Message}
	}
	if event.Resume != nil {
		converted.Resume = &bridgecontract.DeliveryResumeState{
			LatestEventType: bridgecontract.DeliveryEventType(event.Resume.LatestEventType),
		}
	}
	if event.Progress != nil {
		progress := toolProgressToContract(*event.Progress)
		converted.Progress = &progress
	}
	return converted
}

func deliveryEventFromContract(event bridgecontract.DeliveryEvent) DeliveryEvent {
	converted := DeliveryEvent{
		DeliveryID:       event.DeliveryID,
		BridgeInstanceID: event.BridgeInstanceID,
		RoutingKey:       routingKeyFromContract(event.RoutingKey),
		DeliveryTarget:   deliveryTargetFromContract(event.DeliveryTarget),
		Seq:              event.Seq,
		EventType:        DeliveryEventType(event.EventType),
		Content:          messageContentFromContract(event.Content),
		Final:            event.Final,
		Operation:        DeliveryOperation(event.Operation),
	}
	converted.ProviderMetadata = slices.Clone(event.ProviderMetadata)
	if event.Reference != nil {
		reference := deliveryMessageReferenceFromContract(*event.Reference)
		converted.Reference = &reference
	}
	if event.Error != nil {
		converted.Error = &DeliveryErrorDetail{Message: event.Error.Message}
	}
	if event.Resume != nil {
		converted.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventType(event.Resume.LatestEventType)}
	}
	if event.Progress != nil {
		progress := toolProgressFromContract(*event.Progress)
		converted.Progress = &progress
	}
	return converted
}

func deliverySnapshotToContract(snapshot DeliverySnapshot) bridgecontract.DeliverySnapshot {
	converted := bridgecontract.DeliverySnapshot{
		DeliveryID:             snapshot.DeliveryID,
		SessionID:              snapshot.SessionID,
		TurnID:                 snapshot.TurnID,
		BridgeInstanceID:       snapshot.BridgeInstanceID,
		RoutingKey:             routingKeyToContract(snapshot.RoutingKey),
		DeliveryTarget:         deliveryTargetToContract(snapshot.DeliveryTarget),
		LatestSeq:              snapshot.LatestSeq,
		LatestEventType:        bridgecontract.DeliveryEventType(snapshot.LatestEventType),
		CurrentContent:         messageContentToContract(snapshot.CurrentContent),
		Operation:              bridgecontract.DeliveryOperation(snapshot.Operation),
		LastSentSeq:            snapshot.LastSentSeq,
		LastAckedSeq:           snapshot.LastAckedSeq,
		RemoteMessageID:        snapshot.RemoteMessageID,
		ReplaceRemoteMessageID: snapshot.ReplaceRemoteMessageID,
		Final:                  snapshot.Final,
		Error:                  snapshot.Error,
		UpdatedAt:              snapshot.UpdatedAt,
	}
	converted.ProviderMetadata = slices.Clone(snapshot.ProviderMetadata)
	if snapshot.Reference != nil {
		reference := deliveryMessageReferenceToContract(*snapshot.Reference)
		converted.Reference = &reference
	}
	return converted
}

func deliverySnapshotFromContract(snapshot bridgecontract.DeliverySnapshot) DeliverySnapshot {
	converted := DeliverySnapshot{
		DeliveryID:             snapshot.DeliveryID,
		SessionID:              snapshot.SessionID,
		TurnID:                 snapshot.TurnID,
		BridgeInstanceID:       snapshot.BridgeInstanceID,
		RoutingKey:             routingKeyFromContract(snapshot.RoutingKey),
		DeliveryTarget:         deliveryTargetFromContract(snapshot.DeliveryTarget),
		LatestSeq:              snapshot.LatestSeq,
		LatestEventType:        DeliveryEventType(snapshot.LatestEventType),
		CurrentContent:         messageContentFromContract(snapshot.CurrentContent),
		Operation:              DeliveryOperation(snapshot.Operation),
		LastSentSeq:            snapshot.LastSentSeq,
		LastAckedSeq:           snapshot.LastAckedSeq,
		RemoteMessageID:        snapshot.RemoteMessageID,
		ReplaceRemoteMessageID: snapshot.ReplaceRemoteMessageID,
		Final:                  snapshot.Final,
		Error:                  snapshot.Error,
		UpdatedAt:              snapshot.UpdatedAt,
	}
	converted.ProviderMetadata = slices.Clone(snapshot.ProviderMetadata)
	if snapshot.Reference != nil {
		reference := deliveryMessageReferenceFromContract(*snapshot.Reference)
		converted.Reference = &reference
	}
	return converted
}

func deliveryRequestToContract(request DeliveryRequest) bridgecontract.DeliveryRequest {
	converted := bridgecontract.DeliveryRequest{Event: deliveryEventToContract(request.Event)}
	if request.Snapshot != nil {
		snapshot := deliverySnapshotToContract(*request.Snapshot)
		converted.Snapshot = &snapshot
	}
	return converted
}

func deliveryAckToContract(ack DeliveryAck) bridgecontract.DeliveryAck {
	converted := bridgecontract.DeliveryAck{
		DeliveryID:             ack.DeliveryID,
		Seq:                    ack.Seq,
		RemoteMessageID:        ack.RemoteMessageID,
		ReplaceRemoteMessageID: ack.ReplaceRemoteMessageID,
		Outcome:                bridgecontract.DeliveryAckOutcome(ack.Outcome),
	}
	if ack.Error != nil {
		converted.Error = &bridgecontract.DeliveryErrorDetail{Message: ack.Error.Message}
	}
	return converted
}

func deliveryAckFromContract(ack bridgecontract.DeliveryAck) DeliveryAck {
	converted := DeliveryAck{
		DeliveryID:             ack.DeliveryID,
		Seq:                    ack.Seq,
		RemoteMessageID:        ack.RemoteMessageID,
		ReplaceRemoteMessageID: ack.ReplaceRemoteMessageID,
		Outcome:                DeliveryAckOutcome(ack.Outcome),
	}
	if ack.Error != nil {
		converted.Error = &DeliveryErrorDetail{Message: ack.Error.Message}
	}
	return converted
}
