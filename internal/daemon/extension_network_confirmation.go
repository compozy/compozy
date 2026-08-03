package daemon

import (
	"errors"
	"fmt"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) confirmNetworkForEnable(
	key extensionpkg.InstanceKey,
	candidateDigest string,
	expectedDigest string,
	actor taskpkg.ActorContext,
) (*extensionpkg.NetworkConfirmation, error) {
	confirmation, err := s.registry.NetworkConfirmation(key)
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
		return nil, &extensionpkg.NetworkConfirmationRequiredError{CurrentDigest: candidateDigest}
	}
	confirmationActor, err := extensionNetworkConfirmationActor(actor)
	if err != nil {
		return nil, err
	}
	confirmedAt := s.now().UTC()
	if err := s.registry.ConfirmNetworkRequirement(key, candidateDigest, confirmationActor, confirmedAt); err != nil {
		return nil, err
	}
	return &extensionpkg.NetworkConfirmation{
		Digest: candidateDigest, ConfirmedBy: confirmationActor, ConfirmedAt: confirmedAt,
	}, nil
}

func devCandidateConfirmationRequired(link *extensionpkg.DevLink, digest string) bool {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return false
	}
	return link == nil || strings.TrimSpace(link.NetworkRequirementDigest) != digest ||
		strings.TrimSpace(link.NetworkConfirmedBy) == "" || link.NetworkConfirmedAt.IsZero()
}

func (s *daemonExtensionService) confirmDevCandidateNetwork(
	key extensionpkg.InstanceKey,
	digest string,
	expectedDigest string,
	actor taskpkg.ActorContext,
) (*extensionpkg.NetworkConfirmation, error) {
	if strings.TrimSpace(expectedDigest) != strings.TrimSpace(digest) {
		return nil, &extensionpkg.NetworkConfirmationRequiredError{CurrentDigest: strings.TrimSpace(digest)}
	}
	confirmedBy, err := extensionNetworkConfirmationActor(actor)
	if err != nil {
		return nil, err
	}
	confirmedAt := s.now().UTC()
	if err := s.registry.ConfirmDevelopmentNetworkCandidate(key, digest, confirmedBy, confirmedAt); err != nil {
		return nil, err
	}
	return &extensionpkg.NetworkConfirmation{
		Digest: strings.TrimSpace(digest), ConfirmedBy: confirmedBy, ConfirmedAt: confirmedAt,
	}, nil
}

func extensionNetworkConfirmationActor(actor taskpkg.ActorContext) (string, error) {
	if actor.Scope.Operator {
		return windowManagerOperatorActor, nil
	}
	if actor.Actor.Kind.Normalize() == taskpkg.ActorKindAgentSession {
		ref := strings.TrimSpace(actor.Actor.Ref)
		if ref == "" {
			return "", errors.New("daemon: agent network confirmation identity is required")
		}
		return "agent:" + ref, nil
	}
	return "", fmt.Errorf("daemon: unsupported extension network confirmation actor %q", actor.Actor.Kind)
}
