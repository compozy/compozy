package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

var _ SourceSessionManager = (*Kernel)(nil)

// ActivateSourceSession registers the active nonce and resets the snapshot version counter for one source.
func (k *Kernel) ActivateSourceSession(
	ctx context.Context,
	actor MutationActor,
	source ResourceSource,
	sessionNonce string,
) error {
	if ctx == nil {
		return errors.New("resources: activate source session context is required")
	}

	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return err
	}
	if normalizedActor.Kind == MutationActorKindExtension {
		return fmt.Errorf("%w: extension actors cannot activate source sessions", ErrPermissionDenied)
	}

	normalizedSource := source.Normalize()
	if err := normalizedSource.Validate("source"); err != nil {
		return err
	}
	trimmedNonce := strings.TrimSpace(sessionNonce)
	if trimmedNonce == "" {
		return fmt.Errorf("%w: session_nonce is required", ErrValidation)
	}

	unlock := k.lockSource(normalizedSource)
	defer unlock()

	return k.withImmediateTransaction(ctx, "activate source session", func(exec sqlExecutor) error {
		updatedAt := store.FormatTimestamp(k.now())
		if _, err := exec.ExecContext(
			ctx,
			activateSourceStateQuery,
			normalizedSource.Kind,
			normalizedSource.ID,
			trimmedNonce,
			updatedAt,
		); err != nil {
			return fmt.Errorf(
				"resources: activate source session %q/%q: %w",
				normalizedSource.Kind,
				normalizedSource.ID,
				err,
			)
		}
		return nil
	})
}

// ResetSourceIfActiveSession deletes source-owned records only if sessionNonce still owns source.
func (k *Kernel) ResetSourceIfActiveSession(
	ctx context.Context,
	actor MutationActor,
	source ResourceSource,
	sessionNonce string,
) (bool, error) {
	if ctx == nil {
		return false, errors.New("resources: reset active source session context is required")
	}

	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return false, err
	}
	if normalizedActor.Kind == MutationActorKindExtension {
		return false, fmt.Errorf("%w: extension actors cannot reset sources", ErrPermissionDenied)
	}

	normalizedSource := source.Normalize()
	if err := normalizedSource.Validate("source"); err != nil {
		return false, err
	}
	trimmedNonce := strings.TrimSpace(sessionNonce)
	if trimmedNonce == "" {
		return false, fmt.Errorf("%w: session_nonce is required", ErrValidation)
	}

	unlock := k.lockSource(normalizedSource)
	defer unlock()

	reset := false
	err = k.withImmediateTransaction(ctx, "reset active source session", func(exec sqlExecutor) error {
		state, found, stateErr := lookupSourceState(ctx, exec, normalizedSource)
		if stateErr != nil {
			return stateErr
		}
		if !found || state.SessionNonce != trimmedNonce {
			return nil
		}
		if _, execErr := exec.ExecContext(
			ctx,
			deleteSourceRecordsQuery,
			normalizedSource.Kind,
			normalizedSource.ID,
		); execErr != nil {
			return fmt.Errorf(
				"resources: delete source records %q/%q: %w",
				normalizedSource.Kind,
				normalizedSource.ID,
				execErr,
			)
		}
		if _, execErr := exec.ExecContext(
			ctx,
			deleteSourceStateQuery,
			normalizedSource.Kind,
			normalizedSource.ID,
		); execErr != nil {
			return fmt.Errorf(
				"resources: delete source state %q/%q: %w",
				normalizedSource.Kind,
				normalizedSource.ID,
				execErr,
			)
		}
		reset = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return reset, nil
}

// ResetSource deletes all source-owned records and source state in one transaction.
func (k *Kernel) ResetSource(ctx context.Context, actor MutationActor, source ResourceSource) error {
	if ctx == nil {
		return errors.New("resources: reset source context is required")
	}

	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return err
	}
	if normalizedActor.Kind == MutationActorKindExtension {
		return fmt.Errorf("%w: extension actors cannot reset sources", ErrPermissionDenied)
	}

	normalizedSource := source.Normalize()
	if err := normalizedSource.Validate("source"); err != nil {
		return err
	}

	unlock := k.lockSource(normalizedSource)
	defer unlock()

	return k.withImmediateTransaction(ctx, "reset source", func(exec sqlExecutor) error {
		if _, err := exec.ExecContext(
			ctx,
			deleteSourceRecordsQuery,
			normalizedSource.Kind,
			normalizedSource.ID,
		); err != nil {
			return fmt.Errorf(
				"resources: delete source records %q/%q: %w",
				normalizedSource.Kind,
				normalizedSource.ID,
				err,
			)
		}
		if _, err := exec.ExecContext(
			ctx,
			deleteSourceStateQuery,
			normalizedSource.Kind,
			normalizedSource.ID,
		); err != nil {
			return fmt.Errorf(
				"resources: delete source state %q/%q: %w",
				normalizedSource.Kind,
				normalizedSource.ID,
				err,
			)
		}
		return nil
	})
}
