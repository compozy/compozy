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

func (s *daemonExtensionService) recordExtensionSecretsUpdatedEvent(
	ctx context.Context,
	actor taskpkg.ActorContext,
	key extensionpkg.InstanceKey,
	payload contract.ExtensionSecretsPayload,
) error {
	count := len(payload.BoundEnvKeys)
	return s.recordExtensionSecretsEvent(ctx, eventspkg.ExtensionSecretsUpdated, actor, key, &count)
}

func (s *daemonExtensionService) recordExtensionSecretsFailedEvent(
	ctx context.Context,
	actor taskpkg.ActorContext,
	key extensionpkg.InstanceKey,
) error {
	return s.recordExtensionSecretsEvent(ctx, eventspkg.ExtensionSecretsUpdateFailed, actor, key, nil)
}

func (s *daemonExtensionService) recordExtensionSecretsEvent(
	ctx context.Context,
	eventType string,
	actor taskpkg.ActorContext,
	key extensionpkg.InstanceKey,
	boundCount *int,
) error {
	key = key.Normalize()
	event := extensionpkg.LifecycleEvent{
		Type: eventType, ExtensionName: key.Name, WorkspaceID: key.WorkspaceID, BoundCount: boundCount,
	}
	fields, err := event.RequiredFields()
	if err != nil {
		return err
	}
	return s.writeExtensionEvent(
		ctx,
		eventType,
		eventspkg.OutcomeFor(eventType),
		fmt.Sprintf("%s for extension %s", strings.TrimSpace(eventType), key.Name),
		fields,
		string(actor.Actor.Kind.Normalize()),
		strings.TrimSpace(actor.Actor.Ref),
	)
}

func isExtensionSecretValidationRefusal(err error) bool {
	return errors.Is(err, extensionpkg.ErrExtensionEnvBindingInvalid) ||
		errors.Is(err, extensionpkg.ErrExtensionEnvBindingUndeclared) ||
		errors.Is(err, extensionpkg.ErrExtensionEnvBindingDangling)
}
