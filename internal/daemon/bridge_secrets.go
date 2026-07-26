package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/vault"
)

const bridgeSecretNamespace = "bridges"

type bridgeSecretBindingValidator interface {
	ValidateBridgeSecretBinding(binding bridgepkg.BridgeSecretBinding) error
}

type bridgeSecretRefResolver interface {
	ResolveRef(ctx context.Context, ref string) (string, error)
}

type bridgeSecretRefStore interface {
	bridgeSecretRefResolver
	PutSecret(ctx context.Context, ref string, kind string, plaintext string) (vault.Metadata, error)
}

type vaultBridgeSecretResolver struct {
	service bridgeSecretRefStore
}

var _ BridgeSecretResolver = vaultBridgeSecretResolver{}
var _ bridgeSecretBindingValidator = vaultBridgeSecretResolver{}
var _ bridgeSecretValueWriter = vaultBridgeSecretResolver{}

func (r vaultBridgeSecretResolver) ValidateBridgeSecretBinding(binding bridgepkg.BridgeSecretBinding) error {
	if err := vault.ValidateSecretRefNamespace(binding.SecretRef, bridgeSecretNamespace); err != nil {
		return fmt.Errorf("%w: %w", bridgepkg.ErrInvalidBridgeSecretBinding, err)
	}
	return nil
}

func (r vaultBridgeSecretResolver) ResolveBridgeSecret(
	ctx context.Context,
	binding bridgepkg.BridgeSecretBinding,
) (string, error) {
	if ctx == nil {
		return "", errors.New("daemon: resolve bridge secret context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r.service == nil {
		return "", errors.New("daemon: bridge secret vault source is not configured")
	}
	if err := vault.ValidateSecretRefNamespace(binding.SecretRef, bridgeSecretNamespace); err != nil {
		return "", fmt.Errorf("%w: %w", bridgepkg.ErrInvalidBridgeSecretBinding, err)
	}

	value, err := r.service.ResolveRef(ctx, binding.SecretRef)
	if err != nil {
		return "", fmt.Errorf("%w: %w", bridgepkg.ErrInvalidBridgeSecretBinding, err)
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf(
			"%w: bridge secret %q is empty",
			bridgepkg.ErrInvalidBridgeSecretBinding,
			binding.SecretRef,
		)
	}
	diagnostics.RegisterDynamicSecret(value)

	return value, nil
}

func (r vaultBridgeSecretResolver) PutBridgeSecretValue(
	ctx context.Context,
	binding bridgepkg.BridgeSecretBinding,
	plaintext string,
) error {
	if ctx == nil {
		return errors.New("daemon: put bridge secret value context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.service == nil {
		return errors.New("daemon: bridge secret vault source is not configured")
	}
	if err := r.ValidateBridgeSecretBinding(binding); err != nil {
		return err
	}
	if strings.TrimSpace(plaintext) == "" {
		return fmt.Errorf("%w: bridge secret value is required", bridgepkg.ErrInvalidBridgeSecretBinding)
	}
	if _, err := r.service.PutSecret(ctx, binding.SecretRef, binding.Kind, plaintext); err != nil {
		return fmt.Errorf("%w: %w", bridgepkg.ErrInvalidBridgeSecretBinding, err)
	}
	diagnostics.RegisterDynamicSecret(plaintext)
	return nil
}
