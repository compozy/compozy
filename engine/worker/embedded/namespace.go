package embedded

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/server/common/config"
	sqliteschema "go.temporal.io/server/schema/sqlite"
)

func createNamespace(serverCfg *config.Config, embeddedCfg *Config) error {
	if serverCfg == nil {
		return errors.New("temporal config is nil")
	}
	if embeddedCfg == nil {
		return errors.New("embedded config is nil")
	}

	storeName := serverCfg.Persistence.DefaultStore
	datastore, ok := serverCfg.Persistence.DataStores[storeName]
	if !ok || datastore.SQL == nil {
		return fmt.Errorf("sql datastore %q is not configured", storeName)
	}

	sqlCfg := cloneSQLConfig(datastore.SQL)
	namespace, err := sqliteschema.NewNamespaceConfig(
		embeddedCfg.ClusterName,
		embeddedCfg.Namespace,
		false,
		map[string]enumspb.IndexedValueType{},
	)
	if err != nil {
		return fmt.Errorf("build namespace config failed: %w", err)
	}
	if err := sqliteschema.CreateNamespaces(sqlCfg, namespace); err != nil {
		return fmt.Errorf("create namespace %q failed: %w", embeddedCfg.Namespace, err)
	}
	return nil
}

func cloneSQLConfig(src *config.SQL) *config.SQL {
	if src == nil {
		return nil
	}
	clone := *src
	clone.ConnectAttributes = core.CloneMap(src.ConnectAttributes)
	return &clone
}
