package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/subprocess"
)

func (r *bridgeRuntime) ListSecretBindings(
	ctx context.Context,
	bridgeInstanceID string,
) ([]bridgepkg.BridgeSecretBinding, error) {
	if r == nil {
		return nil, errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return nil, errors.New("daemon: list bridge secret bindings context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.store == nil {
		return nil, errors.New("daemon: bridge store is required")
	}
	bindings, err := r.store.ListBridgeSecretBindings(ctx, strings.TrimSpace(bridgeInstanceID))
	if err != nil {
		return nil, fmt.Errorf("daemon: list bridge secret bindings: %w", err)
	}
	return bindings, nil
}

func (r *bridgeRuntime) PutSecretBinding(
	ctx context.Context,
	binding bridgepkg.BridgeSecretBinding,
	secretValue *string,
) error {
	if r == nil {
		return errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: put bridge secret binding context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.store == nil {
		return errors.New("daemon: bridge store is required")
	}
	binding.BridgeInstanceID = strings.TrimSpace(binding.BridgeInstanceID)
	binding.BindingName = strings.TrimSpace(binding.BindingName)
	ctx, unlock := r.lockInstanceLifecycleContext(ctx, binding.BridgeInstanceID)
	defer unlock()

	if validator, ok := r.secretResolver.(bridgeSecretBindingValidator); ok {
		if err := validator.ValidateBridgeSecretBinding(binding); err != nil {
			return fmt.Errorf("daemon: put bridge secret binding: %w", err)
		}
	}
	if secretValue != nil {
		if _, err := r.GetInstance(ctx, binding.BridgeInstanceID); err != nil {
			return fmt.Errorf("daemon: put bridge secret binding: load bridge instance: %w", err)
		}
		writer, ok := r.secretResolver.(bridgeSecretValueWriter)
		if !ok {
			return fmt.Errorf("daemon: put bridge secret binding: %w", errBridgeSecretResolverRequired)
		}
		if err := writer.PutBridgeSecretValue(ctx, binding, *secretValue); err != nil {
			return fmt.Errorf("daemon: put bridge secret binding value: %w", err)
		}
	}
	if err := r.store.PutBridgeSecretBinding(ctx, binding); err != nil {
		return fmt.Errorf("daemon: put bridge secret binding: %w", err)
	}
	return nil
}

func (r *bridgeRuntime) DeleteSecretBinding(ctx context.Context, bridgeInstanceID string, bindingName string) error {
	if r == nil {
		return errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: delete bridge secret binding context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.store == nil {
		return errors.New("daemon: bridge store is required")
	}
	trimmedID := strings.TrimSpace(bridgeInstanceID)
	ctx, unlock := r.lockInstanceLifecycleContext(ctx, trimmedID)
	defer unlock()
	if err := r.store.DeleteBridgeSecretBinding(ctx, trimmedID, strings.TrimSpace(bindingName)); err != nil {
		return fmt.Errorf("daemon: delete bridge secret binding: %w", err)
	}
	return nil
}

func (r *bridgeRuntime) resolveBoundSecrets(
	ctx context.Context,
	bridgeInstanceID string,
) ([]subprocess.InitializeBridgeBoundSecret, error) {
	bindings, err := r.store.ListBridgeSecretBindings(ctx, bridgeInstanceID)
	if err != nil {
		return nil, fmt.Errorf("daemon: list bridge secret bindings for %q: %w", bridgeInstanceID, err)
	}
	if len(bindings) == 0 {
		return nil, nil
	}
	if r.secretResolver == nil {
		return nil, errBridgeSecretResolverRequired
	}

	resolved := make([]subprocess.InitializeBridgeBoundSecret, 0, len(bindings))
	for _, binding := range bindings {
		value, err := r.secretResolver.ResolveBridgeSecret(ctx, binding)
		if err != nil {
			return nil, fmt.Errorf("binding %q: %w", binding.BindingName, err)
		}
		secret := subprocess.InitializeBridgeBoundSecret{
			BindingName: binding.BindingName,
			Kind:        binding.Kind,
			Value:       value,
		}
		if err := secret.Validate(); err != nil {
			return nil, fmt.Errorf("binding %q: %w", binding.BindingName, err)
		}
		resolved = append(resolved, secret)
	}
	return resolved, nil
}
