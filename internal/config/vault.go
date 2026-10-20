package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/vault"
)

const (
	VaultKeyProviderFile      = "file"
	VaultKeyProviderOSKeyring = "os-keyring"
)

// VaultConfig selects the machine-local encryption key source.
type VaultConfig struct {
	KeyProvider string `toml:"key_provider"`
}

func (c VaultConfig) Validate() error {
	switch c.KeyProvider {
	case "", VaultKeyProviderFile, VaultKeyProviderOSKeyring:
		return nil
	default:
		return errors.New("vault.key_provider must be file or os-keyring")
	}
}

type configuredVaultKeyProvider struct {
	home string
	file vault.KeyProvider
	ring vault.KeyProvider
}

var _ vault.KeyProvider = (*configuredVaultKeyProvider)(nil)

// NewVaultKeyProvider follows only the home config, never profile/workspace overlays.
// Selection is read per operation so an explicit migration does not leave live
// consumers bound to a removed key file. Both providers use the same migrated key.
func NewVaultKeyProvider(home string, lookupEnv func(string) (string, bool)) vault.KeyProvider {
	return &configuredVaultKeyProvider{
		home: home,
		file: vault.NewFileKeyProvider(home, lookupEnv),
		ring: vault.NewOSKeyProvider(home, nil),
	}
}

func (p *configuredVaultKeyProvider) Key() ([]byte, error) {
	var selection struct {
		Vault VaultConfig `toml:"vault"`
	}
	_, err := toml.DecodeFile(filepath.Join(p.home, ConfigName), &selection)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("vault: cannot read key provider configuration; fix the home config before retrying")
	}
	if err := selection.Vault.Validate(); err != nil {
		return nil, err
	}
	if selection.Vault.KeyProvider == VaultKeyProviderOSKeyring {
		if err := vault.RequireKeyringMigration(p.home); err != nil {
			return nil, err
		}
		return p.ring.Key()
	}
	return p.file.Key()
}
