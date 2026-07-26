package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/resources"

	"github.com/compozy/agh/internal/vault"
)

type webhookSecretState struct {
	ref     string
	value   string
	present bool
}

func deleteResourceRecord[T any](
	ctx context.Context,
	resourceStore resources.Store[T],
	actor resources.MutationActor,
	record resources.Record[T],
) error {
	return resourceStore.Delete(ctx, currentResourceActor(record.Source, actor), record.ID, record.Version)
}

func restoreUpdatedResourceRecord[T any](
	ctx context.Context,
	resourceStore resources.Store[T],
	actor resources.MutationActor,
	previous resources.Record[T],
	current resources.Record[T],
) error {
	_, err := resourceStore.Put(ctx, currentResourceActor(current.Source, actor), resources.Draft[T]{
		ID:              previous.ID,
		Scope:           previous.Scope,
		ExpectedVersion: current.Version,
		Spec:            previous.Spec,
	})
	return err
}

func recreateDeletedResourceRecord[T any](
	ctx context.Context,
	resourceStore resources.Store[T],
	actor resources.MutationActor,
	previous resources.Record[T],
) error {
	_, err := resourceStore.Put(ctx, currentResourceActor(previous.Source, actor), resources.Draft[T]{
		ID:              previous.ID,
		Scope:           previous.Scope,
		ExpectedVersion: 0,
		Spec:            previous.Spec,
	})
	return err
}

func captureWebhookSecretState(
	ctx context.Context,
	manager *Manager,
	trigger Trigger,
	capture bool,
) (webhookSecretState, error) {
	if !capture {
		return webhookSecretState{}, nil
	}
	secret, err := manager.currentWebhookSecret(ctx, trigger)
	if err != nil {
		return webhookSecretState{}, err
	}
	secret = strings.TrimSpace(secret)
	return webhookSecretState{
		ref:     strings.TrimSpace(trigger.WebhookSecretRef),
		value:   secret,
		present: secret != "",
	}, nil
}

func restoreWebhookSecretStates(ctx context.Context, manager *Manager, states ...webhookSecretState) error {
	if manager == nil || manager.webhookSecrets == nil {
		return nil
	}
	restored := make(map[string]struct{}, len(states))
	var errs []error
	for _, state := range states {
		ref := strings.TrimSpace(state.ref)
		if ref == "" {
			continue
		}
		if _, exists := restored[ref]; exists {
			continue
		}
		restored[ref] = struct{}{}
		if state.present {
			if _, err := manager.webhookSecrets.PutSecret(ctx, ref, "webhook_secret", state.value); err != nil {
				errs = append(errs, fmt.Errorf("automation: restore webhook secret %q: %w", ref, err))
			}
			continue
		}
		if err := manager.webhookSecrets.DeleteSecret(
			ctx,
			ref,
		); err != nil &&
			!errors.Is(err, vault.ErrSecretNotFound) {
			errs = append(errs, fmt.Errorf("automation: delete webhook secret %q: %w", ref, err))
		}
	}
	return errors.Join(errs...)
}
