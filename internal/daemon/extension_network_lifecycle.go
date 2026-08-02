package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) configureUpdateNetworkGate(
	req *extensionpkg.MarketplaceUpdateRequest,
	expectedDigest string,
	actor taskpkg.ActorContext,
) map[string]extensionpkg.NetworkConfirmation {
	confirmed := make(map[string]extensionpkg.NetworkConfirmation)
	if req == nil {
		return confirmed
	}
	req.PreflightCandidate = func(info extensionpkg.ExtensionInfo, manifest *extensionpkg.Manifest) error {
		digest, required, err := candidateNetworkConfirmationRequirement(info, manifest)
		if err != nil || !required {
			return err
		}
		if strings.TrimSpace(expectedDigest) != digest {
			return &extensionpkg.NetworkConfirmationRequiredError{CurrentDigest: digest}
		}
		return nil
	}
	req.CommitCandidate = func(info extensionpkg.ExtensionInfo, manifest *extensionpkg.Manifest) error {
		digest, required, err := candidateNetworkConfirmationRequirement(info, manifest)
		if err != nil || !required {
			return err
		}
		confirmedBy, err := extensionNetworkConfirmationActor(actor)
		if err != nil {
			return err
		}
		confirmedAt := s.now().UTC()
		if err := s.registry.ConfirmNetworkRequirement(
			extensionpkg.GlobalInstanceKey(info.Name),
			digest,
			confirmedBy,
			confirmedAt,
		); err != nil {
			return err
		}
		confirmed[info.Name] = extensionpkg.NetworkConfirmation{
			Digest: digest, ConfirmedBy: confirmedBy, ConfirmedAt: confirmedAt,
		}
		return nil
	}
	return confirmed
}

func candidateNetworkConfirmationRequirement(
	info extensionpkg.ExtensionInfo,
	manifest *extensionpkg.Manifest,
) (string, bool, error) {
	digest, err := extensionpkg.NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
	if err != nil {
		return "", false, err
	}
	if digest == "" {
		return "", false, nil
	}
	confirmed := strings.TrimSpace(info.NetworkRequirementDigest) == digest &&
		strings.TrimSpace(info.NetworkConfirmedBy) != "" && !info.NetworkConfirmedAt.IsZero()
	return digest, !confirmed, nil
}

func (s *daemonExtensionService) recordCommittedUpdateNetworkConfirmations(
	ctx context.Context,
	actor taskpkg.ActorContext,
	items []contract.ManagedExtensionUpdatePayload,
	confirmed map[string]extensionpkg.NetworkConfirmation,
) error {
	var eventErr error
	for _, item := range items {
		confirmation, ok := confirmed[item.Name]
		if !ok || item.Status != extensionpkg.MarketplaceUpdateStatusUpdated {
			continue
		}
		eventErr = errors.Join(eventErr, s.recordExtensionNetworkConfirmedEvent(
			ctx,
			actor,
			extensionpkg.GlobalInstanceKey(item.Name),
			confirmation,
		))
	}
	return eventErr
}

func (s *daemonExtensionService) recordExtensionNetworkConfirmedEvent(
	ctx context.Context,
	actor taskpkg.ActorContext,
	key extensionpkg.InstanceKey,
	confirmation extensionpkg.NetworkConfirmation,
) error {
	key = key.Normalize()
	event := extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionNetworkConfirmed, ExtensionName: key.Name,
		WorkspaceID: key.WorkspaceID, Digest: confirmation.Digest, ConfirmedBy: confirmation.ConfirmedBy,
	}
	fields, err := event.RequiredFields()
	if err != nil {
		return err
	}
	return s.writeExtensionEvent(
		ctx,
		eventspkg.ExtensionNetworkConfirmed,
		eventspkg.OutcomeFor(eventspkg.ExtensionNetworkConfirmed),
		"extension "+key.Name+" network requirement confirmed",
		fields,
		string(actor.Actor.Kind.Normalize()),
		strings.TrimSpace(actor.Actor.Ref),
	)
}
