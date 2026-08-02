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
	expectedDigest string,
	actor taskpkg.ActorContext,
) (*extensionpkg.NetworkConfirmation, error) {
	confirmation, err := s.registry.NetworkConfirmation(key)
	if err != nil {
		return nil, err
	}
	confirmed := confirmation.Digest == "" ||
		(strings.TrimSpace(confirmation.ConfirmedBy) != "" && !confirmation.ConfirmedAt.IsZero())
	if confirmed {
		return nil, nil
	}
	if strings.TrimSpace(expectedDigest) != confirmation.Digest {
		return nil, &extensionpkg.NetworkConfirmationRequiredError{CurrentDigest: confirmation.Digest}
	}
	confirmationActor, err := extensionNetworkConfirmationActor(actor)
	if err != nil {
		return nil, err
	}
	confirmedAt := s.now().UTC()
	if err := s.registry.ConfirmNetworkRequirement(key, expectedDigest, confirmationActor, confirmedAt); err != nil {
		return nil, err
	}
	return &extensionpkg.NetworkConfirmation{
		Digest: confirmation.Digest, ConfirmedBy: confirmationActor, ConfirmedAt: confirmedAt,
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
