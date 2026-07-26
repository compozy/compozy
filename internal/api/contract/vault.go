package contract

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/vault"
)

// VaultSecretPayload is redacted vault metadata safe for public control surfaces.
type VaultSecretPayload struct {
	Ref       string    `json:"ref"`
	Namespace string    `json:"namespace"`
	Kind      string    `json:"kind,omitempty"`
	Present   bool      `json:"present"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// VaultSecretsResponse wraps a redacted vault metadata list.
type VaultSecretsResponse struct {
	Secrets []VaultSecretPayload `json:"secrets"`
}

// VaultSecretResponse wraps one redacted vault metadata row.
type VaultSecretResponse struct {
	Secret VaultSecretPayload `json:"secret"`
}

// PutVaultSecretRequest writes one vault-backed secret. SecretValue is write-only.
type PutVaultSecretRequest struct {
	Ref         string `json:"ref"`
	Kind        string `json:"kind,omitempty"`
	SecretValue string `json:"secret_value"`
}

// Normalize returns the canonical write request without exposing plaintext in errors.
func (r PutVaultSecretRequest) Normalize() PutVaultSecretRequest {
	r.Ref = vault.NormalizeRef(r.Ref)
	r.Kind = strings.TrimSpace(r.Kind)
	return r
}

// Validate verifies a public vault write request.
func (r PutVaultSecretRequest) Validate() error {
	normalized := r.Normalize()
	if err := vault.ValidateSecretRef(normalized.Ref); err != nil {
		return err
	}
	if strings.TrimSpace(normalized.SecretValue) == "" {
		return fmt.Errorf("%w: secret_value is required", vault.ErrMissingSecret)
	}
	return nil
}
