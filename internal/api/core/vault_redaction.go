package core

import (
	"sync"

	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/vault"
)

var vaultSecretRedactions vaultSecretRedactionRegistry

type vaultSecretRedactionRegistry struct {
	mu       sync.Mutex
	cleanups map[string]func()
}

func newVaultSecretRedaction(secretValue string) func() {
	return diagnostics.RegisterDynamicSecret(secretValue)
}

func replaceVaultSecretRedaction(ref string, cleanup func()) {
	normalized := vault.NormalizeRef(ref)
	if normalized == "" {
		if cleanup != nil {
			cleanup()
		}
		return
	}

	vaultSecretRedactions.mu.Lock()
	if vaultSecretRedactions.cleanups == nil {
		vaultSecretRedactions.cleanups = make(map[string]func())
	}
	previous := vaultSecretRedactions.cleanups[normalized]
	vaultSecretRedactions.cleanups[normalized] = cleanup
	vaultSecretRedactions.mu.Unlock()

	if previous != nil {
		previous()
	}
}

func unregisterVaultSecretRedaction(ref string) {
	normalized := vault.NormalizeRef(ref)
	if normalized == "" {
		return
	}

	vaultSecretRedactions.mu.Lock()
	cleanup := vaultSecretRedactions.cleanups[normalized]
	delete(vaultSecretRedactions.cleanups, normalized)
	vaultSecretRedactions.mu.Unlock()

	if cleanup != nil {
		cleanup()
	}
}
