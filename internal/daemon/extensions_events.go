package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	eventspkg "github.com/compozy/agh/internal/events"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

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
	payload := extensionLifecycleEventPayload{Name: item.Name, Version: item.Version, Status: item.State}
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
	return s.recordExtensionLifecycleEvent(ctx, eventType, actor, payload)
}

func (s *daemonExtensionService) recordExtensionUpdateEvent(
	ctx context.Context,
	actor taskpkg.ActorContext,
	item contract.ManagedExtensionUpdatePayload,
) error {
	return s.recordExtensionLifecycleEvent(ctx, eventspkg.ExtensionUpdated, actor, extensionLifecycleEventPayload{
		Name:           item.Name,
		Slug:           item.Slug,
		CurrentVersion: item.CurrentVersion,
		LatestVersion:  item.LatestVersion,
		Status:         item.Status,
		Warnings:       append([]contract.DiagnosticItem(nil), item.Warnings...),
	})
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
	return s.writeExtensionEvent(
		ctx,
		eventType,
		eventspkg.OutcomeFor(eventType),
		extensionLifecycleEventSummary(eventType, payload),
		payload,
		payload.ActorKind,
		payload.ActorID,
	)
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
		Type: eventType, Outcome: string(outcome), Content: content,
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
	case eventspkg.ExtensionInstalled:
		return fmt.Sprintf("extension %s installed", name)
	case eventspkg.ExtensionUpdated:
		return fmt.Sprintf("extension %s updated", name)
	case eventspkg.ExtensionRemoved:
		return fmt.Sprintf("extension %s removed", name)
	case eventspkg.ExtensionEnabled:
		return fmt.Sprintf("extension %s enabled", name)
	case eventspkg.ExtensionDisabled:
		return fmt.Sprintf("extension %s disabled", name)
	default:
		return strings.TrimSpace(eventType)
	}
}
