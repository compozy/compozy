package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) configureUpdateGatewayGate(
	req *extensionpkg.MarketplaceUpdateRequest,
	expectedDigest string,
	actor taskpkg.ActorContext,
) map[string]extensionpkg.GatewayConfirmation {
	confirmed := make(map[string]extensionpkg.GatewayConfirmation)
	if req == nil {
		return confirmed
	}
	expectedDigest = strings.TrimSpace(expectedDigest)
	req.PreflightCandidate = func(info extensionpkg.ExtensionInfo, manifest *extensionpkg.Manifest) error {
		digest, required, err := candidateGatewayConfirmationRequirement(&info, manifest)
		if err != nil || !required {
			return err
		}
		if expectedDigest != digest {
			return &extensionpkg.GatewayConfirmationRequiredError{CurrentDigest: digest}
		}
		return nil
	}
	req.CommitCandidate = func(info extensionpkg.ExtensionInfo, manifest *extensionpkg.Manifest) error {
		digest, required, err := candidateGatewayConfirmationRequirement(&info, manifest)
		if err != nil || !required {
			return err
		}
		confirmedBy, err := extensionGatewayConfirmationActor(actor)
		if err != nil {
			return err
		}
		confirmedAt := s.now().UTC()
		if err := s.registry.ConfirmGatewayRequirement(
			extensionpkg.GlobalInstanceKey(info.Name),
			digest,
			confirmedBy,
			confirmedAt,
		); err != nil {
			return err
		}
		confirmed[info.Name] = extensionpkg.GatewayConfirmation{
			Digest: digest, ConfirmedBy: confirmedBy, ConfirmedAt: confirmedAt,
		}
		return nil
	}
	return confirmed
}

func candidateGatewayConfirmationRequirement(
	info *extensionpkg.ExtensionInfo,
	manifest *extensionpkg.Manifest,
) (string, bool, error) {
	digest, err := extensionpkg.GatewayRequirementDigest(manifest.Gateway)
	if err != nil {
		return "", false, err
	}
	if digest == "" {
		return "", false, nil
	}
	confirmed := strings.TrimSpace(info.GatewayRequirementDigest) == digest &&
		strings.TrimSpace(info.GatewayConfirmedBy) != "" && !info.GatewayConfirmedAt.IsZero()
	return digest, !confirmed, nil
}

func (s *daemonExtensionService) recordCommittedUpdateGatewayConfirmations(
	ctx context.Context,
	actor taskpkg.ActorContext,
	items []contract.ManagedExtensionUpdatePayload,
	confirmed map[string]extensionpkg.GatewayConfirmation,
) error {
	var eventErr error
	for _, item := range items {
		confirmation, ok := confirmed[item.Name]
		if !ok || item.Status != extensionpkg.MarketplaceUpdateStatusUpdated {
			continue
		}
		err := s.recordExtensionGatewayConfirmedEvent(
			ctx,
			actor,
			extensionpkg.GlobalInstanceKey(item.Name),
			confirmation,
		)
		if err != nil {
			eventErr = errors.Join(eventErr, fmt.Errorf(
				"daemon: record extension gateway confirmation %q: %w",
				item.Name,
				err,
			))
		}
	}
	return eventErr
}

func (s *daemonExtensionService) recordExtensionGatewayConfirmedEvent(
	ctx context.Context,
	actor taskpkg.ActorContext,
	key extensionpkg.InstanceKey,
	confirmation extensionpkg.GatewayConfirmation,
) error {
	key = key.Normalize()
	event := extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionGatewayConfirmed, ExtensionName: key.Name,
		WorkspaceID: key.WorkspaceID, Digest: confirmation.Digest, ConfirmedBy: confirmation.ConfirmedBy,
	}
	fields, err := event.RequiredFields()
	if err != nil {
		return err
	}
	return s.writeExtensionEvent(
		ctx,
		eventspkg.ExtensionGatewayConfirmed,
		eventspkg.OutcomeFor(eventspkg.ExtensionGatewayConfirmed),
		"extension "+key.Name+" gateway requirement confirmed",
		fields,
		string(actor.Actor.Kind.Normalize()),
		strings.TrimSpace(actor.Actor.Ref),
	)
}
