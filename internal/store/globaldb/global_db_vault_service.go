package globaldb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/vault"
)

func (g *VaultRepo) vaultService() (*vault.Service, error) {
	return g.vaultServiceForStore(g)
}

func (g *VaultRepo) vaultServiceForStore(vaultStore vault.Store) (*vault.Service, error) {
	if vaultStore == nil {
		return nil, errors.New("store: vault store is required")
	}
	lookupEnv := func(key string) (string, bool) {
		value, ok := os.LookupEnv(key)
		return value, ok && strings.TrimSpace(value) != ""
	}
	return vault.NewService(
		vaultStore,
		compozyconfig.NewVaultKeyProvider(filepath.Dir(g.path), lookupEnv),
		vault.WithLookupEnv(lookupEnv),
		vault.WithNow(g.now),
	)
}
