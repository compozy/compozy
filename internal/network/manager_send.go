package network

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

const networkSendCommitTimeout = 30 * time.Second

type networkSubscriptionStore interface {
	ListNetworkSubscriptions(
		ctx context.Context,
		query store.NetworkSubscriptionQuery,
	) ([]store.NetworkSubscriptionEntry, error)
}

// Send atomically accepts one session-originated envelope before notifying wakes.
func (m *Manager) Send(ctx context.Context, req SendRequest) (string, error) {
	if ctx == nil {
		return "", errors.New("network: send context is required")
	}
	if m == nil || m.router == nil {
		return "", errors.New("network: manager router is required")
	}
	ctx, cancel := m.detachedSendContext(ctx)
	defer cancel()
	prepared, err := m.router.PrepareSend(ctx, req)
	if err != nil {
		m.logPrePersistenceRejection(req.SessionID, req.Body, req.Ext, err)
		return "", err
	}
	return m.acceptPrepared(ctx, strings.TrimSpace(req.SessionID), prepared)
}

// SendFromRuntimePeer atomically accepts one AGH-runtime-originated envelope.
func (m *Manager) SendFromRuntimePeer(ctx context.Context, req RuntimeSendRequest) (string, error) {
	if ctx == nil {
		return "", errors.New("network: runtime send context is required")
	}
	if m == nil || m.router == nil {
		return "", errors.New("network: manager router is required")
	}
	ctx, cancel := m.detachedSendContext(ctx)
	defer cancel()
	prepared, err := m.router.PrepareRuntimeSend(ctx, req)
	if err != nil {
		m.logPrePersistenceRejection(runtimePeerSessionID, req.Body, req.Ext, err)
		return "", err
	}
	return m.acceptPrepared(ctx, runtimePeerSessionID, prepared)
}

func (m *Manager) acceptPrepared(
	ctx context.Context,
	senderSessionID string,
	prepared SendResult,
) (string, error) {
	if m.acceptance == nil {
		return "", errors.New("network: atomic acceptance store is required")
	}
	route, err := m.router.RoutePrepared(ctx, prepared)
	if err != nil {
		return "", err
	}
	route, err = m.applySubscriptionDecisions(ctx, route)
	if err != nil {
		return "", err
	}
	entry, ok, err := normalizeTimelineMessageEntry(
		senderSessionID,
		AuditDirectionSent,
		prepared.Envelope,
		envelopeRecordTime(prepared.Envelope, m.now),
	)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("network: envelope kind %q cannot be persisted", prepared.Envelope.Kind)
	}
	m.bindDurableSessionIdentities(&entry, senderSessionID, route)
	rootID, depth, err := m.resolveCausation(ctx, prepared.Envelope)
	if err != nil {
		return "", err
	}
	request := store.AcceptNetworkMessageRequest{
		Message:      entry,
		Dispositions: storeDispositions(route.Dispositions),
		Admissions:   m.wakeAdmissions(route.Deliveries, rootID, depth),
	}
	result, err := m.acceptance.AcceptNetworkMessage(ctx, request)
	if err != nil {
		m.recordAuditRejected(ctx, senderSessionID, prepared.Envelope, conversationPersistenceReason(err))
		return "", err
	}
	if !result.Duplicate {
		m.observeConversationWrite(ctx, entry, result.Conversation)
		m.recordAcceptedMessage(ctx, senderSessionID, prepared.Envelope, result.Dispositions)
		m.notifyInboxRecipients(result.Dispositions)
	}
	for _, notification := range result.Notify {
		if m.wakeNotifier != nil {
			m.wakeNotifier.NotifyNetworkWake(notification)
		}
	}
	if result.Duplicate {
		route = filterDuplicateWhoisResponders(route, result.Dispositions)
	}
	if err := m.acceptWhoisResponses(ctx, route); err != nil {
		return "", err
	}
	return prepared.ID, nil
}

func (m *Manager) detachedSendContext(ctx context.Context) (context.Context, context.CancelFunc) {
	detached := context.WithoutCancel(ctx)
	deadline := time.Now().Add(networkSendCommitTimeout)
	if callerDeadline, ok := ctx.Deadline(); ok && callerDeadline.Before(deadline) {
		deadline = callerDeadline
	}
	detached, cancel := context.WithDeadline(detached, deadline)
	if m.lifecycleCtx == nil {
		return detached, cancel
	}
	stopLifecycleCancel := context.AfterFunc(m.lifecycleCtx, cancel)
	return detached, func() {
		stopLifecycleCancel()
		cancel()
	}
}

func filterDuplicateWhoisResponders(
	route RouteResult,
	dispositions []store.NetworkMessageDisposition,
) RouteResult {
	if len(route.whoisPeers) == 0 {
		return route
	}
	committedRecipients := make(map[string]struct{}, len(dispositions))
	for _, disposition := range dispositions {
		if disposition.Decision == store.NetworkDispositionDeliver {
			committedRecipients[strings.TrimSpace(disposition.RecipientSessionID)] = struct{}{}
		}
	}
	responders := make([]LocalPeer, 0, len(route.whoisPeers))
	for _, responder := range route.whoisPeers {
		if _, ok := committedRecipients[responder.SessionID]; ok {
			responders = append(responders, responder)
		}
	}
	route.whoisPeers = responders
	return route
}

func (m *Manager) acceptWhoisResponses(ctx context.Context, route RouteResult) error {
	responses, err := m.router.buildWhoisResponses(route)
	if err != nil {
		return fmt.Errorf("network: build whois response: %w", err)
	}
	for _, response := range responses {
		if _, err := m.acceptPrepared(ctx, response.senderSessionID, response.prepared); err != nil {
			return fmt.Errorf("network: accept whois response: %w", err)
		}
	}
	return nil
}

func (m *Manager) bindDurableSessionIdentities(
	entry *store.NetworkConversationMessage,
	senderSessionID string,
	route RouteResult,
) {
	if entry == nil {
		return
	}
	entry.PeerFrom = strings.TrimSpace(senderSessionID)
	if route.Envelope.IsDirected() {
		for _, delivery := range route.Deliveries {
			if deliveryAddressedToPeer(delivery) {
				entry.PeerTo = strings.TrimSpace(delivery.SessionID)
				return
			}
		}
	}
	entry.PeerTo = ""
}

func storeDispositions(values []RoutingDisposition) []store.NetworkMessageDisposition {
	if len(values) == 0 {
		return nil
	}
	result := make([]store.NetworkMessageDisposition, 0, len(values))
	for _, value := range values {
		result = append(result, store.NetworkMessageDisposition{
			RecipientSessionID: strings.TrimSpace(value.SessionID),
			Decision:           strings.TrimSpace(value.Decision),
		})
	}
	return result
}

func (m *Manager) wakeAdmissions(
	deliveries []Delivery,
	rootID string,
	depth int,
) []store.NetworkWakeAdmissionInput {
	if len(deliveries) == 0 {
		return nil
	}
	inputs := make([]store.NetworkWakeAdmissionInput, 0, len(deliveries))
	for _, delivery := range deliveries {
		runtime, ok := m.sessionSnapshot(delivery.SessionID)
		if !ok {
			continue
		}
		eligible := delivery.Envelope.Kind == KindSay
		trigger := store.NetworkWakeTriggerNone
		if eligible {
			switch {
			case deliveryAddressedToPeer(delivery):
				trigger = store.NetworkWakeTriggerDirect
			case deliveryMentionsPeer(delivery.Envelope, delivery.PeerID):
				trigger = store.NetworkWakeTriggerMention
			}
		}
		input := store.NetworkWakeAdmissionInput{
			WorkspaceID:        runtime.workspaceID,
			RecipientSessionID: delivery.SessionID,
			OwnerKey:           runtime.ownerKey,
			Spec:               runtime.networkParticipation,
			Trigger:            trigger,
			Eligible:           eligible,
			Addressed:          trigger != store.NetworkWakeTriggerNone,
			RootID:             rootID,
			Depth:              depth,
		}
		if input.Addressed {
			input.WakeID = store.NewID("wake")
			input.TaskRunID = store.NewID("run")
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func (m *Manager) resolveCausation(ctx context.Context, envelope Envelope) (string, int, error) {
	if envelope.CausationID == nil || strings.TrimSpace(*envelope.CausationID) == "" {
		return strings.TrimSpace(envelope.ID), 0, nil
	}
	causationID := strings.TrimSpace(*envelope.CausationID)
	if m.causation == nil {
		return causationID, 1, nil
	}
	rootID, depth, found, err := m.causation.ResolveNetworkCausation(
		ctx,
		envelope.WorkspaceID,
		causationID,
	)
	if err != nil {
		return "", 0, err
	}
	if !found {
		return causationID, 1, nil
	}
	return rootID, depth, nil
}

func (m *Manager) applySubscriptionDecisions(ctx context.Context, route RouteResult) (RouteResult, error) {
	subscriptions, ok := m.conversations.(networkSubscriptionStore)
	if !ok || len(route.Deliveries) == 0 {
		return route, nil
	}
	kept := make([]Delivery, 0, len(route.Deliveries))
	for _, delivery := range route.Deliveries {
		mode, err := deliverySubscriptionMode(ctx, subscriptions, delivery)
		if err != nil {
			return RouteResult{}, err
		}
		if mode == store.NetworkSubscriptionModeMute {
			setRoutingDecision(route.Dispositions, delivery.SessionID, store.NetworkDispositionIgnore)
			continue
		}
		delivery.Mode = store.NetworkSubscriptionModeFull
		kept = append(kept, delivery)
	}
	route.Deliveries = kept
	return route, nil
}

func deliverySubscriptionMode(
	ctx context.Context,
	subscriptions networkSubscriptionStore,
	delivery Delivery,
) (string, error) {
	if deliveryAddressedToPeer(delivery) || deliveryMentionsPeer(delivery.Envelope, delivery.PeerID) {
		return store.NetworkSubscriptionModeFull, nil
	}
	query := store.NetworkSubscriptionQuery{
		WorkspaceID: delivery.Envelope.WorkspaceID,
		Channel:     delivery.Envelope.Channel,
		ThreadID:    deliveryThreadID(delivery),
		SessionID:   delivery.SessionID,
		ExactThread: true,
		Limit:       1,
	}
	mode, found, err := lookupDeliverySubscriptionMode(ctx, subscriptions, query)
	if err != nil {
		return "", fmt.Errorf("network: list delivery subscriptions: %w", err)
	}
	if found {
		return mode, nil
	}
	if query.ThreadID == "" {
		return store.NetworkSubscriptionModeFull, nil
	}
	query.ThreadID = ""
	mode, found, err = lookupDeliverySubscriptionMode(ctx, subscriptions, query)
	if err != nil {
		return "", fmt.Errorf("network: list channel delivery subscription: %w", err)
	}
	if found {
		return mode, nil
	}
	return store.NetworkSubscriptionModeFull, nil
}

func deliveryThreadID(delivery Delivery) string {
	if delivery.Envelope.ThreadID == nil {
		return ""
	}
	return strings.TrimSpace(*delivery.Envelope.ThreadID)
}

func lookupDeliverySubscriptionMode(
	ctx context.Context,
	subscriptions networkSubscriptionStore,
	query store.NetworkSubscriptionQuery,
) (string, bool, error) {
	entries, err := subscriptions.ListNetworkSubscriptions(ctx, query)
	if err != nil {
		return "", false, err
	}
	if len(entries) == 0 {
		return "", false, nil
	}
	return entries[0].Mode, true, nil
}

func setRoutingDecision(values []RoutingDisposition, sessionID string, decision string) {
	for index := range values {
		if values[index].SessionID == sessionID {
			values[index].Decision = decision
			return
		}
	}
}

func deliveryMentionsPeer(envelope Envelope, peerID string) bool {
	return slicesContainsString(envelope.Mentions, peerID)
}

func deliveryAddressedToPeer(delivery Delivery) bool {
	return delivery.Envelope.To != nil &&
		strings.TrimSpace(*delivery.Envelope.To) == strings.TrimSpace(delivery.PeerID)
}

func envelopeRecordTime(envelope Envelope, now func() time.Time) time.Time {
	if envelope.TS > 0 {
		return time.Unix(envelope.TS, 0).UTC()
	}
	if now != nil {
		return now().UTC()
	}
	return time.Now().UTC()
}

func conversationPersistenceReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, store.ErrNetworkWorkClosed):
		return "work_closed"
	case errors.Is(err, store.ErrNetworkWorkContainerMismatch):
		return "work_container_mismatch"
	case errors.Is(err, store.ErrNetworkDirectRoomCollision):
		return "direct_room_collision"
	default:
		return "acceptance_failed"
	}
}

func (m *Manager) logPrePersistenceRejection(
	sessionID string,
	body json.RawMessage,
	ext ExtensionMap,
	err error,
) {
	if m == nil || m.logger == nil || err == nil {
		return
	}
	hasher := sha256.New()
	if _, writeErr := hasher.Write(body); writeErr != nil {
		m.logger.Warn("network.message.rejection_hash_failed", "error", writeErr)
		return
	}
	encodedExt, encodeErr := json.Marshal(ext)
	if encodeErr != nil {
		m.logger.Warn("network.message.rejection_hash_failed", "error", encodeErr)
		return
	}
	if _, writeErr := hasher.Write(encodedExt); writeErr != nil {
		m.logger.Warn("network.message.rejection_hash_failed", "error", writeErr)
		return
	}
	m.logger.Warn(
		"network.message.rejected_pre_persistence",
		"session_id", strings.TrimSpace(sessionID),
		"payload_sha256", hex.EncodeToString(hasher.Sum(nil)),
		"error", err,
	)
}
