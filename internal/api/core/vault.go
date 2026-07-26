package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/vault"
	"github.com/gin-gonic/gin"
)

var errVaultServiceUnavailable = errors.New("vault service is not configured")

// ListVaultSecrets returns redacted metadata for vault-backed secrets.
func (h *BaseHandlers) ListVaultSecrets(c *gin.Context) {
	service, ok := h.vaultService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errVaultServiceUnavailable)
		return
	}

	prefix, err := vaultListPrefix(c)
	if err != nil {
		h.respondError(c, StatusForVaultError(err), err)
		return
	}

	rows, err := service.ListMetadata(c.Request.Context(), prefix)
	if err != nil {
		h.respondError(c, StatusForVaultError(err), err)
		return
	}

	payloads, err := VaultSecretPayloadsFromMetadata(rows)
	if err != nil {
		h.respondError(c, StatusForVaultError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.VaultSecretsResponse{Secrets: payloads})
}

// GetVaultSecretMetadata returns redacted metadata for one vault-backed secret.
func (h *BaseHandlers) GetVaultSecretMetadata(c *gin.Context) {
	service, ok := h.vaultService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errVaultServiceUnavailable)
		return
	}

	ref := vault.NormalizeRef(c.Query("ref"))
	if ref == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%w: ref is required", vault.ErrUnsupportedSecretRef))
		return
	}

	row, err := service.GetMetadata(c.Request.Context(), ref)
	if err != nil {
		h.respondError(c, StatusForVaultError(err), err)
		return
	}

	payload, err := VaultSecretPayloadFromMetadata(row)
	if err != nil {
		h.respondError(c, StatusForVaultError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.VaultSecretResponse{Secret: payload})
}

// PutVaultSecret stores one write-only vault secret and returns redacted metadata.
func (h *BaseHandlers) PutVaultSecret(c *gin.Context) {
	service, ok := h.vaultService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errVaultServiceUnavailable)
		return
	}

	var req contract.PutVaultSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, errors.New("invalid vault secret request body"))
		return
	}
	req = req.Normalize()
	if err := req.Validate(); err != nil {
		h.respondError(c, StatusForVaultError(err), err)
		return
	}

	redactionCleanup := newVaultSecretRedaction(req.SecretValue)
	row, err := service.PutSecret(c.Request.Context(), req.Ref, req.Kind, req.SecretValue)
	if err != nil {
		redactionCleanup()
		h.respondError(c, StatusForVaultError(err), err)
		return
	}
	replaceVaultSecretRedaction(req.Ref, redactionCleanup)

	payload, err := VaultSecretPayloadFromMetadata(row)
	if err != nil {
		h.respondError(c, StatusForVaultError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.VaultSecretResponse{Secret: payload})
}

// DeleteVaultSecret removes one vault-backed secret.
func (h *BaseHandlers) DeleteVaultSecret(c *gin.Context) {
	service, ok := h.vaultService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errVaultServiceUnavailable)
		return
	}

	ref := vault.NormalizeRef(c.Query("ref"))
	if ref == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%w: ref is required", vault.ErrUnsupportedSecretRef))
		return
	}

	if err := service.DeleteSecret(c.Request.Context(), ref); err != nil {
		h.respondError(c, StatusForVaultError(err), err)
		return
	}
	unregisterVaultSecretRedaction(ref)
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) vaultService() (VaultService, bool) {
	if h.Vault == nil {
		return nil, false
	}
	return h.Vault, true
}

func vaultListPrefix(c *gin.Context) (string, error) {
	prefix := vault.NormalizeRef(c.Query("prefix"))
	namespace := strings.Trim(strings.TrimSpace(c.Query("namespace")), "/")

	if namespace != "" {
		if err := vault.ValidateNamespace(namespace); err != nil {
			return "", err
		}
		if prefix == "" {
			prefix = "vault:" + namespace + "/"
		}
	}
	if err := vault.ValidateSecretRefPrefix(prefix); err != nil {
		return "", err
	}
	if namespace != "" {
		prefixNamespace, err := vault.SecretRefPrefixNamespace(prefix)
		if err != nil {
			return "", err
		}
		if prefixNamespace != namespace {
			return "", fmt.Errorf(
				"%w: prefix %q must match namespace %q",
				vault.ErrUnsupportedSecretRef,
				prefix,
				namespace,
			)
		}
	}
	return prefix, nil
}

// VaultSecretPayloadFromMetadata converts redacted vault metadata into the shared contract payload.
func VaultSecretPayloadFromMetadata(row vault.Metadata) (contract.VaultSecretPayload, error) {
	namespace, err := vault.SecretRefNamespace(row.Ref)
	if err != nil {
		return contract.VaultSecretPayload{}, err
	}
	return contract.VaultSecretPayload{
		Ref:       vault.NormalizeRef(row.Ref),
		Namespace: namespace,
		Kind:      strings.TrimSpace(row.Kind),
		Present:   row.Present,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// VaultSecretPayloadsFromMetadata converts a metadata list into public payloads.
func VaultSecretPayloadsFromMetadata(rows []vault.Metadata) ([]contract.VaultSecretPayload, error) {
	payloads := make([]contract.VaultSecretPayload, 0, len(rows))
	for _, row := range rows {
		payload, err := VaultSecretPayloadFromMetadata(row)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}
