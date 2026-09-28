package daemon

import (
	"errors"
	"fmt"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) confirmGatewayForEnable(
	key extensionpkg.InstanceKey,
	candidateDigest string,
	expectedDigest string,
	actor taskpkg.ActorContext,
) (*extensionpkg.GatewayConfirmation, error) {
	confirmation, err := s.registry.GatewayConfirmation(key)
	if err != nil {
		return nil, err
	}
	candidateDigest = strings.TrimSpace(candidateDigest)
	confirmed := candidateDigest == "" ||
		(confirmation.Digest == candidateDigest &&
			strings.TrimSpace(confirmation.ConfirmedBy) != "" && !confirmation.ConfirmedAt.IsZero())
	if confirmed {
		return nil, nil
	}
	if strings.TrimSpace(expectedDigest) != candidateDigest {
		return nil, &extensionpkg.GatewayConfirmationRequiredError{CurrentDigest: candidateDigest}
	}
	confirmationActor, err := extensionGatewayConfirmationActor(actor)
	if err != nil {
		return nil, err
	}
	confirmedAt := s.now().UTC()
	if err := s.registry.ConfirmGatewayRequirement(key, candidateDigest, confirmationActor, confirmedAt); err != nil {
		return nil, err
	}
	return &extensionpkg.GatewayConfirmation{
		Digest: candidateDigest, ConfirmedBy: confirmationActor, ConfirmedAt: confirmedAt,
	}, nil
}

func devCandidateConfirmationRequired(link *extensionpkg.DevLink, digest string) bool {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return false
	}
	return link == nil || strings.TrimSpace(link.GatewayRequirementDigest) != digest ||
		strings.TrimSpace(link.GatewayConfirmedBy) == "" || link.GatewayConfirmedAt.IsZero()
}

func (s *daemonExtensionService) confirmDevCandidateGateway(
	key extensionpkg.InstanceKey,
	digest string,
	expectedDigest string,
	actor taskpkg.ActorContext,
) (*extensionpkg.GatewayConfirmation, error) {
	if strings.TrimSpace(expectedDigest) != strings.TrimSpace(digest) {
		return nil, &extensionpkg.GatewayConfirmationRequiredError{CurrentDigest: strings.TrimSpace(digest)}
	}
	confirmedBy, err := extensionGatewayConfirmationActor(actor)
	if err != nil {
		return nil, err
	}
	confirmedAt := s.now().UTC()
	if err := s.registry.ConfirmDevelopmentGatewayCandidate(key, digest, confirmedBy, confirmedAt); err != nil {
		return nil, err
	}
	return &extensionpkg.GatewayConfirmation{
		Digest: strings.TrimSpace(digest), ConfirmedBy: confirmedBy, ConfirmedAt: confirmedAt,
	}, nil
}

func extensionGatewayConfirmationActor(actor taskpkg.ActorContext) (string, error) {
	if actor.Scope.Operator {
		return windowManagerOperatorActor, nil
	}
	if actor.Actor.Kind.Normalize() == taskpkg.ActorKindAgentSession {
		ref := strings.TrimSpace(actor.Actor.Ref)
		if ref == "" {
			return "", errors.New("daemon: agent gateway confirmation identity is required")
		}
		return "agent:" + ref, nil
	}
	return "", fmt.Errorf("daemon: unsupported extension gateway confirmation actor %q", actor.Actor.Kind)
}
