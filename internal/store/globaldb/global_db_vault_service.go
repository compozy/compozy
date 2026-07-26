package globaldb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/vault"
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
		vault.NewFileKeyProvider(filepath.Dir(g.path), lookupEnv),
		vault.WithLookupEnv(lookupEnv),
		vault.WithNow(g.now),
	)
}
