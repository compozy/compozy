package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/vault"
)

func shouldSkipMissingProviderSecret(
	resolved aghconfig.ResolvedAgent,
	secretRef string,
	slot aghconfig.ProviderCredentialSlot,
	err error,
) bool {
	if resolved.Harness == aghconfig.ProviderHarnessPiACP {
		return !slot.Required &&
			(errors.Is(err, vault.ErrMissingSecret) || errors.Is(err, vault.ErrSecretNotFound))
	}
	if vault.IsSecretRef(secretRef) {
		return !slot.Required && errors.Is(err, vault.ErrSecretNotFound)
	}
	if vault.IsEnvRef(secretRef) {
		return !slot.Required && errors.Is(err, vault.ErrMissingSecret)
	}
	return false
}

type piSettingsFile struct {
	DefaultProvider string   `json:"defaultProvider"`
	DefaultModel    string   `json:"defaultModel"`
	EnabledModels   []string `json:"enabledModels,omitempty"`
}

type piModelsFile struct {
	Providers map[string]piModelsProvider `json:"providers"`
}

type piModelsProvider struct {
	BaseURL string         `json:"baseUrl,omitempty"`
	API     string         `json:"api,omitempty"`
	APIKey  string         `json:"apiKey,omitempty"`
	Models  []piModelEntry `json:"models,omitempty"`
}

type piModelEntry struct {
	ID string `json:"id"`
}

func (m *Manager) materializePiRuntime(
	session *Session,
	resolved aghconfig.ResolvedAgent,
	injectedTargetEnvs map[string]struct{},
) (string, error) {
	if session == nil {
		return "", errors.New("session: pi runtime requires a session")
	}
	runtimeDir := filepath.Join(session.sessionDir, "provider-runtime", "pi")
	if err := materializePiRuntimeAt(runtimeDir, resolved, injectedTargetEnvs); err != nil {
		return "", err
	}
	return runtimeDir, nil
}

func materializePiRuntimeAt(
	runtimeDir string,
	resolved aghconfig.ResolvedAgent,
	injectedTargetEnvs map[string]struct{},
) error {
	runtimeProvider := strings.TrimSpace(resolved.RuntimeProvider)
	if runtimeProvider == "" {
		runtimeProvider = strings.TrimSpace(resolved.Provider)
	}
	model := strings.TrimSpace(resolved.Model)
	if runtimeProvider == "" {
		return errors.New("session: pi runtime provider is required")
	}
	if model == "" {
		return errors.New("session: pi model is required")
	}
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return fmt.Errorf("session: create pi runtime directory %q: %w", runtimeDir, err)
	}
	if err := os.Chmod(runtimeDir, 0o700); err != nil {
		return fmt.Errorf("session: protect pi runtime directory %q: %w", runtimeDir, err)
	}
	settings := piSettingsFile{
		DefaultProvider: runtimeProvider,
		DefaultModel:    model,
		EnabledModels:   []string{model},
	}
	if err := writeProviderJSON(filepath.Join(runtimeDir, "settings.json"), settings); err != nil {
		return err
	}

	models := piModelsFile{
		Providers: map[string]piModelsProvider{
			runtimeProvider: {
				BaseURL: strings.TrimSpace(resolved.BaseURL),
				API:     strings.TrimSpace(resolved.Transport),
				APIKey:  piCredentialEnv(resolved.CredentialSlots, injectedTargetEnvs),
				Models:  []piModelEntry{{ID: model}},
			},
		},
	}
	if err := writeProviderJSON(filepath.Join(runtimeDir, "models.json"), models); err != nil {
		return err
	}
	return nil
}
