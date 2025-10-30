package embedded

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/server/common/config"
	sqliteschema "go.temporal.io/server/schema/sqlite"
)

func createNamespace(ctx context.Context, serverCfg *config.Config, embeddedCfg *Config) error {
	log := logger.FromContext(ctx)
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
	if err := sqliteschema.SetupSchema(sqlCfg); err != nil {
		e := strings.ToLower(err.Error())
		if !strings.Contains(e, "already exists") {
			return fmt.Errorf("setup temporal schema failed: %w", err)
		}
		log.Debug("temporal schema already initialized; continuing")
	}
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
		e := strings.ToLower(err.Error())
		if strings.Contains(e, "already exists") {
			log.Debug("temporal namespace already exists; continuing", "namespace", embeddedCfg.Namespace)
			return nil
		}
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
