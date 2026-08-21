package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type extensionLifecycleEventStoreSink struct {
	writer    store.EventSummaryStore
	now       func() time.Time
	actorKind string
	actorID   string
}

type extensionLifecycleEventWriter interface {
	store.EventSummaryStore
	store.EventSummaryBatchWriter
}

var _ extensionpkg.LifecycleEventSink = extensionLifecycleEventStoreSink{}

func (s extensionLifecycleEventStoreSink) RecordExtensionLifecycleEvent(
	ctx context.Context,
	event extensionpkg.LifecycleEvent,
) error {
	if s.writer == nil {
		return nil
	}
	summary, err := s.summary(ctx, event)
	if err != nil {
		return err
	}
	if err := s.writer.WriteEventSummary(context.WithoutCancel(ctx), summary); err != nil {
		return fmt.Errorf("daemon: record extension lifecycle event: %w", err)
	}
	return nil
}

func (s extensionLifecycleEventStoreSink) summary(
	ctx context.Context,
	event extensionpkg.LifecycleEvent,
) (store.EventSummary, error) {
	if ctx == nil {
		return store.EventSummary{}, errors.New("daemon: extension event context is required")
	}
	fields, err := event.RequiredFields()
	if err != nil {
		return store.EventSummary{}, err
	}
	content, err := json.Marshal(fields)
	if err != nil {
		return store.EventSummary{}, fmt.Errorf("daemon: encode extension lifecycle event: %w", err)
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	return store.EventSummary{
		ProfileID: store.DefaultProfileID,
		Type:      event.Type, Outcome: string(eventspkg.OutcomeFor(event.Type)), Content: content,
		Summary: event.Type + " " + event.ExtensionName, Timestamp: now().UTC(),
		EventCorrelation: store.EventCorrelation{ActorKind: s.actorKind, ActorID: s.actorID},
	}, nil
}

func (s *daemonExtensionService) recordCanonicalExtensionLifecycleEvent(
	ctx context.Context,
	actor taskpkg.ActorContext,
	event extensionpkg.LifecycleEvent,
) error {
	return s.canonicalLifecycleEventSink(actor).RecordExtensionLifecycleEvent(ctx, event)
}

func (s *daemonExtensionService) canonicalLifecycleEventSink(
	actor taskpkg.ActorContext,
) extensionLifecycleEventStoreSink {
	return extensionLifecycleEventStoreSink{
		writer:    s.eventWriter,
		now:       s.now,
		actorKind: string(actor.Actor.Kind.Normalize()),
		actorID:   strings.TrimSpace(actor.Actor.Ref),
	}
}

func (s *daemonExtensionService) recordCanonicalExtensionLifecycleEvents(
	ctx context.Context,
	actor taskpkg.ActorContext,
	events ...extensionpkg.LifecycleEvent,
) error {
	if len(events) == 0 {
		return nil
	}
	if s.eventWriter != nil {
		sink := s.canonicalLifecycleEventSink(actor)
		summaries := make([]store.EventSummary, 0, len(events))
		for _, event := range events {
			summary, err := sink.summary(ctx, event)
			if err != nil {
				return err
			}
			summaries = append(summaries, summary)
		}
		if err := s.eventWriter.WriteEventSummaries(context.WithoutCancel(ctx), summaries); err != nil {
			return fmt.Errorf("daemon: record extension lifecycle event batch: %w", err)
		}
	}
	for _, event := range events {
		if extensionLifecycleChangesPalette(event.Type) {
			return s.paletteNotifier.NotifyExtensionChanged(
				ctx,
				event.WorkspaceID,
				event.ExtensionName,
			)
		}
	}
	return nil
}

type extensionLifecycleEventPayload struct {
	ActorKind        string                    `json:"actor_kind,omitempty"`
	ActorID          string                    `json:"actor_id,omitempty"`
	OriginKind       string                    `json:"origin_kind,omitempty"`
	OriginRef        string                    `json:"origin_ref,omitempty"`
	Name             string                    `json:"name"`
	Slug             string                    `json:"slug,omitempty"`
	Version          string                    `json:"version,omitempty"`
	CurrentVersion   string                    `json:"current_version,omitempty"`
	LatestVersion    string                    `json:"latest_version,omitempty"`
	Status           string                    `json:"status,omitempty"`
	InstalledFrom    string                    `json:"installed_from,omitempty"`
	SourceURL        string                    `json:"source_url,omitempty"`
	ChecksumSHA256   string                    `json:"checksum_sha256,omitempty"`
	ChecksumVerified bool                      `json:"checksum_verified"`
	RegistryTier     string                    `json:"registry_tier,omitempty"`
	AllowUnverified  bool                      `json:"allow_unverified"`
	Warnings         []contract.DiagnosticItem `json:"warnings,omitempty"`
	WorkspaceID      string                    `json:"workspace_id,omitempty"`
	GenerationHash   string                    `json:"generation_hash,omitempty"`
}

type extensionDigestVerificationPayload struct {
	ActorKind           string `json:"actor_kind,omitempty"`
	ActorID             string `json:"actor_id,omitempty"`
	OriginKind          string `json:"origin_kind,omitempty"`
	OriginRef           string `json:"origin_ref,omitempty"`
	CatalogEntryID      string `json:"catalog_entry_id"`
	Version             string `json:"version"`
	ArchiveDigestSHA256 string `json:"archive_digest_sha256"`
	VerificationOutcome string `json:"verification_outcome"`
}

func (s *daemonExtensionService) recordExtensionEvent(
	ctx context.Context,
	eventType string,
	actor taskpkg.ActorContext,
	item contract.ExtensionPayload,
) error {
	payload := extensionEventPayload(item)
	return s.recordExtensionLifecycleEvent(ctx, eventType, actor, payload)
}

func (s *daemonExtensionService) recordExtensionEnableEvents(
	ctx context.Context,
	actor taskpkg.ActorContext,
	key extensionpkg.InstanceKey,
	confirmation *extensionpkg.NetworkConfirmation,
	result contract.ExtensionEnableResult,
) error {
	key = key.Normalize()
	events := make([]extensionpkg.LifecycleEvent, 0, 2)
	if confirmation != nil {
		events = append(events, extensionpkg.LifecycleEvent{
			Type: eventspkg.ExtensionNetworkConfirmed, ExtensionName: key.Name,
			WorkspaceID: key.WorkspaceID, Digest: confirmation.Digest, ConfirmedBy: confirmation.ConfirmedBy,
		})
	}
	count := len(result.AutomationStarted)
	events = append(events, extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionEnabled, ExtensionName: result.Extension.Name, AutomationCount: &count,
	})
	return s.recordCanonicalExtensionLifecycleEvents(ctx, actor, events...)
}

func extensionEventPayload(item contract.ExtensionPayload) extensionLifecycleEventPayload {
	payload := extensionLifecycleEventPayload{
		Name:           item.Name,
		Version:        item.Version,
		Status:         item.State,
		WorkspaceID:    item.WorkspaceID,
		GenerationHash: item.GenerationHash,
	}
	if item.Provenance != nil {
		payload.Slug = item.Provenance.Slug
		payload.InstalledFrom = item.Provenance.InstalledFrom
		payload.SourceURL = item.Provenance.SourceURL
		payload.ChecksumSHA256 = item.Provenance.ChecksumSHA256
		payload.ChecksumVerified = item.Provenance.ChecksumVerified
		payload.RegistryTier = item.Provenance.RegistryTier
		payload.AllowUnverified = item.Provenance.AllowUnverified
	} else if item.Trust != nil {
		payload.ChecksumVerified = item.Trust.ChecksumVerified
		payload.RegistryTier = item.Trust.RegistryTier
		payload.AllowUnverified = item.Trust.AllowUnverified
	}
	return payload
}

func (s *daemonExtensionService) recordExtensionRemoveEvent(
	ctx context.Context,
	actor taskpkg.ActorContext,
	item contract.ManagedExtensionRemovePayload,
) error {
	return s.recordExtensionLifecycleEvent(ctx, eventspkg.ExtensionRemoved, actor, extensionLifecycleEventPayload{
		Name: item.Name, Status: item.Status,
	})
}

func (s *daemonExtensionService) recordExtensionLifecycleEvent(
	ctx context.Context,
	eventType string,
	actor taskpkg.ActorContext,
	payload extensionLifecycleEventPayload,
) error {
	payload.ActorKind = string(actor.Actor.Kind.Normalize())
	payload.ActorID = strings.TrimSpace(actor.Actor.Ref)
	payload.OriginKind = string(actor.Origin.Kind.Normalize())
	payload.OriginRef = strings.TrimSpace(actor.Origin.Ref)
	if err := s.writeExtensionEvent(
		ctx,
		eventType,
		eventspkg.OutcomeFor(eventType),
		extensionLifecycleEventSummary(eventType, payload),
		payload,
		payload.ActorKind,
		payload.ActorID,
	); err != nil {
		return err
	}
	if extensionLifecycleChangesPalette(eventType) {
		return s.paletteNotifier.NotifyExtensionChanged(ctx, payload.WorkspaceID, payload.Name)
	}
	return nil
}

func (s *daemonExtensionService) observeExtensionDigestVerification(
	ctx context.Context,
	actor taskpkg.ActorContext,
	trust *extensionpkg.MarketplaceTrustEvidence,
	installErr error,
) {
	if trust == nil || (installErr != nil && !errors.Is(installErr, extensionpkg.ErrExtensionArchiveDigestMismatch)) {
		return
	}
	outcome := "verified"
	eventOutcome := eventspkg.OutcomeSuccess
	if installErr != nil {
		outcome = "mismatch"
		eventOutcome = eventspkg.OutcomeFailure
	}
	payload := extensionDigestVerificationPayload{
		ActorKind:           string(actor.Actor.Kind.Normalize()),
		ActorID:             strings.TrimSpace(actor.Actor.Ref),
		OriginKind:          string(actor.Origin.Kind.Normalize()),
		OriginRef:           strings.TrimSpace(actor.Origin.Ref),
		CatalogEntryID:      strings.TrimSpace(trust.CatalogEntryID),
		Version:             strings.TrimSpace(trust.Version),
		ArchiveDigestSHA256: strings.TrimSpace(trust.ArchiveDigestSHA256),
		VerificationOutcome: outcome,
	}
	summary := fmt.Sprintf("extension catalog entry %s digest %s", payload.CatalogEntryID, outcome)
	if err := s.writeExtensionEvent(
		ctx,
		eventspkg.ExtensionDigestVerify,
		eventOutcome,
		summary,
		payload,
		payload.ActorKind,
		payload.ActorID,
	); err != nil {
		s.logger.Error("daemon: record extension digest verification failed", "error", err)
	}
}

func (s *daemonExtensionService) writeExtensionEvent(
	ctx context.Context,
	eventType string,
	outcome eventspkg.Outcome,
	summary string,
	payload any,
	actorKind string,
	actorID string,
) error {
	if s.eventWriter == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: extension event context is required")
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("daemon: encode extension event: %w", err)
	}
	if err := s.eventWriter.WriteEventSummary(context.WithoutCancel(ctx), store.EventSummary{
		ProfileID: store.DefaultProfileID,
		Type:      eventType, Outcome: string(outcome), Content: content,
		Summary: summary, Timestamp: s.now().UTC(),
		EventCorrelation: store.EventCorrelation{ActorKind: actorKind, ActorID: actorID},
	}); err != nil {
		return fmt.Errorf("daemon: record extension event: %w", err)
	}
	return nil
}

func extensionLifecycleEventSummary(eventType string, payload extensionLifecycleEventPayload) string {
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = strings.TrimSpace(payload.Slug)
	}
	switch eventType {
	case eventspkg.ExtensionRemoved:
		return fmt.Sprintf("extension %s removed", name)
	case eventspkg.ExtensionEnabled:
		return fmt.Sprintf("extension %s enabled", name)
	case eventspkg.ExtensionDisabled:
		return fmt.Sprintf("extension %s disabled", name)
	case eventspkg.ExtensionDevLinked:
		return fmt.Sprintf("extension %s linked for development", name)
	case eventspkg.ExtensionDevUnlinked:
		return fmt.Sprintf("extension %s unlinked from development", name)
	default:
		return strings.TrimSpace(eventType)
	}
}
